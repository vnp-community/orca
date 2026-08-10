// src/relay/agent-rpc-dispatch.ts
// JSON-RPC 2.0 router for the Orca Dev Agent.
//
// Design:
//   - createRpcDispatcher() returns a stateless RpcDispatcher object
//   - dispatch() encodes and sends the response via ws.send()
//   - v5.0 methods (git.*, fs.*, ai.provider.*) use dynamic import so that
//     modules not yet created don't break agent startup
//   - All handler exceptions are caught and returned as ServerError responses
//   - ws.readyState === 1 (OPEN) is checked before every send

import type WebSocket from 'ws'
import type { ToolDefinition, ToolResult } from './agent-tool-registry'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type { WireState } from './agent-wire'
import { encodeDataFrame } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'
import { Tracers } from '../shared/trace/tracers'

const rpcTracer = createTracer('agent:rpc')

// ─── Trace resume extraction ────────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested at params._trace.id (CR-TRACE-000 §3.3).
function extractResume(params: Record<string, unknown>): { id: string } | undefined {
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}

// ─── JSON-RPC types ───────────────────────────────────────────────────────────

export type JsonRpcRequest = {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly method: string
  readonly params?: Record<string, unknown>
}

type JsonRpcSuccess = {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly result: unknown
}

type JsonRpcError = {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly error: { code: number; message: string; data?: unknown }
}

type JsonRpcResponse = JsonRpcSuccess | JsonRpcError

// ─── RpcDispatcher ────────────────────────────────────────────────────────────

export type RpcDispatcher = {
  dispatch(ws: WebSocket, state: WireState, rpc: JsonRpcRequest): Promise<void>
}

// ─── Trace field extraction ───────────────────────────────────────────────────
// Extracts meaningful context fields from RPC params per method group.
// This centralizes all trace context logic in one place.

type TraceFields = Record<string, string | number | boolean | undefined>

function extractTraceFields(method: string, params: Record<string, unknown>): TraceFields {
  const p = params
  const str = (v: unknown) => (typeof v === 'string' ? v : undefined)
  const num = (v: unknown) => (typeof v === 'number' ? v : undefined)
  const truncPath = (v: unknown) => {
    const s = str(v)
    return s ? (s.length > 60 ? `...${  s.slice(-57)}` : s) : undefined
  }
  const truncCmd = (v: unknown) => {
    const s = str(v)
    return s ? (s.length > 80 ? `${s.slice(0, 77)  }...` : s) : undefined
  }

  if (method.startsWith('fs.') || method === 'shell.eval' || method === 'preflight.check') {
    return {
      path: truncPath(p['path'] ?? p['dir'] ?? p['filePath']),
      pattern: str(p['pattern']),
      cmd: method === 'shell.eval' ? truncCmd(p['cmd']) : undefined,
    }
  }

  if (method.startsWith('git.')) {
    return {
      repo: truncPath(p['repoPath'] ?? p['workDir']),
      cmd:  truncCmd(p['cmd'] ?? p['args']),
      branch: str(p['branch']),
      worktree: truncPath(p['worktreePath'] ?? p['path']),
    }
  }

  if (method.startsWith('github.') || method.startsWith('gitlab.')) {
    return {
      repo:    str(p['repo'] ?? p['project']),
      branch:  str(p['branch'] ?? p['sourceBranch']),
      prNum:   num(p['prNumber'] ?? p['mrIid']),
      title:   str(p['title'])?.slice(0, 40),
    }
  }

  if (method.startsWith('ai.provider.')) {
    return {
      provider: str(p['provider'] ?? p['providerId']),
      // Never log credential value — just the provider name
    }
  }

  if (method === 'tools/call') {
    return {
      tool: str(p['name']),
    }
  }

  if (method === 'ai.complete') {
    // CR-TRACE-018 BL-TG-02: this method used to fall through to `return {}`
    // — the outer agent:rpc span had no context fields at all. This is a thin
    // dispatch-level wrapper (id, timing of the whole dispatch incl. the
    // dynamic import of ./ai-complete-handler); the detailed breakdown
    // (provider-call step, contentLength, fail reason) lives in the separate
    // agent:aiComplete tracer (ai-complete-handler.ts, TASK-AG-005.1) — the two
    // are complementary, not duplicates.
    return {
      model:        str(p['model']),
      taskId:       str(p['taskId']),
      promptLength: typeof p['prompt'] === 'string' ? (p['prompt'] as string).length : undefined,
    }
  }

  if (method === 'agent.exec') {
    // CR-TRACE-015 BL-PRF-04: agent.exec (non-interactive, used by
    // ProfileAwareAgentSpawner.spawn() on the backend) has a params shape
    // completely different from agent.spawn (interactive PTY) — the generic
    // 'agent.' bucket below (session/binary/cmd) doesn't match any field of
    // agent.exec, which used to leave the span showing session=undefined
    // cmd=undefined. This dedicated bucket matches { binary, args, cwd, env,
    // timeoutMs } instead. Must be checked BEFORE the 'agent.' prefix bucket.
    return {
      // (TASK-AG-015.1) base — request shape:
      binary:         str(p['binary']),
      argsCount:      Array.isArray(p['args']) ? (p['args'] as unknown[]).length : undefined,
      hasEnvOverride: p['env'] !== undefined && p['env'] !== null,
      timeoutMs:      num(p['timeoutMs']),
      // CR-TRACE-017 BL-WF-02: StepExecutors.executeAgent() already sends
      // `stepId` — this field has a real value immediately, no backend change needed.
      stepId: str(p['stepId']),
      // CR-TRACE-017 §4: `parentTraceId` is a plain business field so the
      // TracePanel can group every step-span of the same workflow execution —
      // NOT Tracer.start()'s `resume` mechanism (CR-TRACE-000 §3.1), since that
      // core API hasn't shipped. Only populated once WorkflowOrchestrator.ts is
      // updated to send `traceId: stepSpan.id` + `parentTraceId: rootTraceId` in
      // the relay.call('agent.exec', ...) params — until then this stays
      // undefined without error (agent side is ready, no second edit needed).
      parentTraceId: str(p['parentTraceId']),
      // CR-TRACE-018 BL-TG-04: only populated once the backend
      // (ProfileAwareAgentSpawner.spawn() / TaskAgentExecutor) is updated to
      // send `taskId` as a top-level param instead of only inside
      // `env.ORCA_TASK_ID` — until then this stays undefined without error.
      taskId: str(p['taskId']),
    }
  }

  if (method.startsWith('agent.')) {
    return {
      session: str(p['sessionId'] ?? p['taskId']),
      binary:  str(p['binary'] ?? p['model'] ?? p['modelId']),
      cmd:     truncCmd(p['cmd'] ?? p['command']),
    }
  }

  return {}
}

// ─── Result field extraction (generic — extend as more methods need it) ──────
// Surfaces result-level fields the handler already computed (e.g.
// exitCode/timedOut for agent.exec) onto span.ok(), instead of only { method }.
function extractResultFields(method: string, result: unknown): TraceFields {
  if (method === 'agent.exec' && result && typeof result === 'object') {
    const r = result as Record<string, unknown>
    return {
      exitCode: typeof r['exitCode'] === 'number'  ? r['exitCode']  : undefined,
      timedOut: typeof r['timedOut'] === 'boolean' ? r['timedOut'] : undefined,
    }
  }
  return {}
}

export function createRpcDispatcher(
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger
): RpcDispatcher {
  return {
    async dispatch(ws: WebSocket, state: WireState, rpc: JsonRpcRequest): Promise<void> {
      const ctxFields = extractTraceFields(rpc.method, rpc.params ?? {})
      const span = rpcTracer.start(
        { method: rpc.method, id: String(rpc.id ?? 'notify'), ...ctxFields },
        extractResume(rpc.params ?? {})
      )
      let response: JsonRpcResponse
      try {
        response = await route(rpc, tools, config, log, ws, state)
        // JsonRpcError (method-level failures) are still 'ok' at transport level
        if ('error' in response) {
          span.fail(response.error.message, { method: rpc.method, code: response.error.code })
        } else {
          // CR-TRACE-015 BL-PRF-04: surface result-level fields the handler
          // already computed (exitCode/timedOut for agent.exec) instead of { method }.
          span.ok({ method: rpc.method, ...extractResultFields(rpc.method, response.result) })
        }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        log.error(`RPC dispatch unhandled error method=${rpc.method}: ${msg}`)
        span.fail(msg, { method: rpc.method, phase: 'dispatch' })
        response = makeError(rpc.id, AgentErrorCode.ServerError, `Internal error: ${msg}`)
      }

      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        ws.send(encodeDataFrame(state, JSON.stringify(response)))
      }
    },
  }
}


// ─── Router ───────────────────────────────────────────────────────────────────

/**
 * Builds a one-way JSON-RPC notification sender bound to this connection.
 * Used by long-lived agent-side resources (PTY sessions, fs watchers) to push
 * data back to Orca outside the request/response cycle — the same frame codec
 * as regular responses, just without an `id`.
 */
function makeNotifier(ws: WebSocket, state: WireState): (method: string, params: Record<string, unknown>) => void {
  return (method, params) => {
    if (ws.readyState !== 1 /* WebSocket.OPEN */) {return}
    ws.send(encodeDataFrame(state, JSON.stringify({ jsonrpc: '2.0', method, params })))
  }
}

async function route(
  rpc: JsonRpcRequest,
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger,
  ws: WebSocket,
  state: WireState
): Promise<JsonRpcResponse> {

  switch (rpc.method) {

    // ── MCP: tools/list ──────────────────────────────────────────────────────
    case 'tools/list':
      return {
        jsonrpc: '2.0', id: rpc.id,
        result: {
          tools: tools.map(t => ({
            name:        t.name,
            description: t.description,
            inputSchema: t.inputSchema,
          })),
        },
      }

    // ── MCP: tools/call ──────────────────────────────────────────────────────
    case 'tools/call': {
      const params = rpc.params ?? {}
      const name   = typeof params.name === 'string' ? params.name : ''
      const args   = (typeof params.arguments === 'object' && params.arguments !== null)
        ? params.arguments as Record<string, unknown>
        : {}

      const tool = tools.find(t => t.name === name)
      if (!tool) {
        return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Tool not found: ${name}`)
      }

      log.info(`tools/call name=${name} args=${JSON.stringify(args).slice(0, 120)}`)

      let result: ToolResult
      try {
        result = await tool.handler(args, config)
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        log.error(`tools/call handler threw name=${name}: ${msg}`)
        return makeError(rpc.id, AgentErrorCode.ServerError, `Tool handler error: ${msg}`)
      }

      return formatMcpResult(rpc.id, result)
    }

    // ── v5.0: git.exec ───────────────────────────────────────────────────────
    case 'git.exec': {
      try {
        const { handleGitExec } = await import('./agent-git-handler')
        return (await handleGitExec(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.exec unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.execStream ─────────────────────────────────────────────────
    case 'git.execStream': {
      try {
        const { handleGitExecStream } = await import('./agent-git-handler')
        // Streaming: fire-and-forget, sends multiple frames asynchronously
        void handleGitExecStream(ws, state, rpc.id, rpc.params ?? {}, config, log)
        return { jsonrpc: '2.0', id: rpc.id, result: { type: 'stream.started' } }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.execStream unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.history ────────────────────────────────────────────────────
    case 'git.history': {
      try {
        const { handleGitHistory } = await import('./agent-git-handler-extended')
        return (await handleGitHistory(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.history unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.branchCompare ─────────────────────────────────────────────
    case 'git.branchCompare': {
      try {
        const { handleGitBranchCompare } = await import('./agent-git-handler-extended')
        return (await handleGitBranchCompare(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.branchCompare unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.commitCompare ─────────────────────────────────────────────
    case 'git.commitCompare': {
      try {
        const { handleGitCommitCompare } = await import('./agent-git-handler-extended')
        return (await handleGitCommitCompare(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.commitCompare unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.branchDiff ────────────────────────────────────────────────
    case 'git.branchDiff': {
      try {
        const { handleGitBranchDiff } = await import('./agent-git-handler-extended')
        return (await handleGitBranchDiff(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.branchDiff unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.commitDiff ────────────────────────────────────────────────
    case 'git.commitDiff': {
      try {
        const { handleGitCommitDiff } = await import('./agent-git-handler-extended')
        return (await handleGitCommitDiff(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.commitDiff unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.checkIgnored ──────────────────────────────────────────────
    case 'git.checkIgnored': {
      try {
        const { handleGitCheckIgnored } = await import('./agent-git-handler-extended')
        return (await handleGitCheckIgnored(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.checkIgnored unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.forkSync ──────────────────────────────────────────────────
    case 'git.forkSync': {
      try {
        const { handleGitForkSync } = await import('./agent-git-handler-extended')
        return (await handleGitForkSync(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.forkSync unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.submoduleStatus ───────────────────────────────────────────
    case 'git.submoduleStatus': {
      try {
        const { handleGitSubmoduleStatus } = await import('./agent-git-handler-extended')
        return (await handleGitSubmoduleStatus(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.submoduleStatus unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.readDir ─────────────────────────────────────────────────────
    case 'fs.readDir': {
      try {
        const { handleFsReadDir } = await import('./fs-agent-extensions')
        return (await handleFsReadDir(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.readDir unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.readFile ────────────────────────────────────────────────────
    case 'fs.readFile': {
      try {
        const { handleFsReadFile } = await import('./fs-agent-extensions')
        return (await handleFsReadFile(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.readFile unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.grep ────────────────────────────────────────────────────────
    case 'fs.grep': {
      try {
        const { handleFsGrep } = await import('./fs-agent-extensions')
        return (await handleFsGrep(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.grep unavailable: ${msg}`)
      }
    }

    // ── v5.0: ai.provider.writeCredential ────────────────────────────────────
    case 'ai.provider.writeCredential': {
      try {
        const { handleWriteCredential } = await import('./agent-credential-store')
        return (await handleWriteCredential(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.writeCredential unavailable: ${msg}`)
      }
    }

    // ── v5.0: ai.provider.readCredential ─────────────────────────────────────
    case 'ai.provider.readCredential': {
      try {
        const { handleReadCredential } = await import('./agent-credential-store')
        return (await handleReadCredential(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.readCredential unavailable: ${msg}`)
      }
    }

    // ── v5.0: ai.provider.healthCheck ────────────────────────────────────────
    case 'ai.provider.healthCheck': {
      try {
        const { handleHealthCheck } = await import('./agent-credential-store')
        return (await handleHealthCheck(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.healthCheck unavailable: ${msg}`)
      }
    }

    // ── v5.0: preflight.check ────────────────────────────────────────────────
    case 'preflight.check': {
      try {
        const { handlePreflightCheck } = await import('./fs-agent-extensions')
        return (await handlePreflightCheck(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `preflight.check unavailable: ${msg}`)
      }
    }

    // ── v5.0: ai.provider.deleteCredential ───────────────────────────────────
    case 'ai.provider.deleteCredential': {
      try {
        const { handleDeleteCredential } = await import('./agent-credential-store')
        return (await handleDeleteCredential(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.provider.deleteCredential unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.pr.create ──────────────────────────────────────────────────
    case 'git.pr.create': {
      try {
        // BUG-AG-HLD-004: 'git.pr.create' and 'github.pr.create' are the same
        // action — route both to the one implementation with the idempotency
        // check (handleGitHubPrCreate) so a retry/double-click never creates
        // a duplicate PR regardless of which method name the caller used.
        const { handleGitHubPrCreate } = await import('./external-api-connector')
        return (await handleGitHubPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.pr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.list ──────────────────────────────────────────────
    case 'git.worktree.list': {
      try {
        const { handleGitWorktreeList } = await import('./agent-git-handler')
        return (await handleGitWorktreeList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.list unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.add ───────────────────────────────────────────────
    case 'git.worktree.add': {
      try {
        const { handleGitWorktreeAdd } = await import('./agent-git-handler')
        return (await handleGitWorktreeAdd(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.add unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.remove ────────────────────────────────────────────
    case 'git.worktree.remove': {
      try {
        const { handleGitWorktreeRemove } = await import('./agent-git-handler')
        return (await handleGitWorktreeRemove(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.remove unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.stat ────────────────────────────────────────────────────────
    case 'fs.stat': {
      try {
        const { handleFsStat } = await import('./fs-agent-extensions')
        return (await handleFsStat(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.stat unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.glob ────────────────────────────────────────────────────────
    case 'fs.glob': {
      try {
        const { handleFsGlob } = await import('./fs-agent-extensions')
        return (await handleFsGlob(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.glob unavailable: ${msg}`)
      }
    }

    // ── v5.0: fs.writeFile ───────────────────────────────────────────────────
    case 'fs.writeFile': {
      try {
        const { handleFsWriteFile } = await import('./fs-agent-extensions')
        return (await handleFsWriteFile(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.writeFile unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.pr.create ───────────────────────────────────────────────
    case 'github.pr.create': {
      try {
        const { handleGitHubPrCreate } = await import('./external-api-connector')
        return (await handleGitHubPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.pr.merge ────────────────────────────────────────────────
    case 'github.pr.merge': {
      try {
        const { handleGitHubPrMerge } = await import('./external-api-connector')
        return (await handleGitHubPrMerge(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.merge unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.issue.list ──────────────────────────────────────────────
    case 'github.issue.list': {
      try {
        const { handleGitHubIssueList } = await import('./external-api-connector')
        return (await handleGitHubIssueList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.issue.list unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.issue.create ────────────────────────────────────────────
    case 'github.issue.create': {
      try {
        const { handleGitHubIssueCreate } = await import('./external-api-connector')
        return (await handleGitHubIssueCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.issue.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: gitlab.mr.create ───────────────────────────────────────────────
    case 'gitlab.mr.create': {
      try {
        const { handleGitLabMrCreate } = await import('./external-api-connector')
        return (await handleGitLabMrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: gitlab.pipeline.status ─────────────────────────────────────────
    case 'gitlab.pipeline.status': {
      try {
        const { handleGitLabPipelineStatus } = await import('./external-api-connector')
        return (await handleGitLabPipelineStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.pipeline.status unavailable: ${msg}`)
      }
    }

    // ── v5.0: gitlab.mr.list ─────────────────────────────────────────────────
    case 'gitlab.mr.list': {
      try {
        const { handleGitLabMrList } = await import('./external-api-connector')
        return (await handleGitLabMrList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.list unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.auth.status ─────────────────────────────────────────────
    case 'github.auth.status': {
      try {
        const { handleGitHubAuthStatus } = await import('./external-api-connector')
        return (await handleGitHubAuthStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.auth.status unavailable: ${msg}`)
      }
    }

    // ── v5.0: gitlab.auth.status ─────────────────────────────────────────────
    case 'gitlab.auth.status': {
      try {
        const { handleGitLabAuthStatus } = await import('./external-api-connector')
        return (await handleGitLabAuthStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.auth.status unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.spawn ────────────────────────────────────────────────────
    case 'agent.spawn': {
      try {
        const { handleAgentSpawn } = await import('./agent-spawner')
        // Fire-and-forget: streaming handler sends multiple frames asynchronously
        void handleAgentSpawn(rpc.id, rpc.params ?? {}, config, log, ws, state)
        return { jsonrpc: '2.0', id: rpc.id, result: { type: 'spawn.accepted' } }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.spawn unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.kill ─────────────────────────────────────────────────────
    case 'agent.kill': {
      try {
        const { handleAgentKill } = await import('./agent-spawner')
        return (await handleAgentKill(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.kill unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.sendInput ────────────────────────────────────────────────
    // ORCH-001: Send data to a running agent PTY's stdin.
    // Used for graceful stop (Ctrl+C = '\x03') and interactive input.
    case 'agent.sendInput': {
      try {
        const { handleAgentSendInput } = await import('./agent-spawner')
        return (await handleAgentSendInput(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.sendInput unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.exec ─────────────────────────────────────────────────────
    // TG-001: Non-interactive subprocess execution (for task graph steps).
    // Returns captured stdout/stderr/exitCode instead of streaming.
    // Distinct from agent.spawn (interactive PTY) — no terminal allocation.
    // Called by:
    //   - StepExecutors.executeAgent() via relay.call('agent.exec', {...})
    //   - ProfileAwareAgentSpawner via relay.call('agent.exec', {...})
    case 'agent.exec': {
      const p       = rpc.params ?? {}
      const binary  = typeof p.binary === 'string' ? p.binary : ''
      const span = Tracers.agentOrchSpawn.start(
        { binary, taskId: typeof p.taskId === 'string' ? p.taskId : undefined },
        extractResume(p)
      )
      try {
        const { spawn } = await import('node:child_process')
        const args    = Array.isArray(p.args) ? (p.args as unknown[]).map(String) : []
        const cwd     = typeof p.cwd      === 'string' ? p.cwd                    : config.workDir
        const stdin   = typeof p.stdin    === 'string' ? p.stdin                  : null
        const extraEnv = (p.env && typeof p.env === 'object' && !Array.isArray(p.env))
          ? p.env as Record<string, string>
          : {}
        const timeoutMs = typeof p.timeoutMs === 'number'
          ? Math.min(Math.max(p.timeoutMs, 1_000), 5 * 60_000)
          : 300_000

        if (!binary) {
          span.fail('binary is required')
          return makeError(rpc.id, AgentErrorCode.InvalidParams, 'agent.exec: binary is required')
        }

        span.step('subprocess-spawn', { binary, cwd })
        const result = await new Promise<{
          stdout: string; stderr: string; exitCode: number | null; timedOut: boolean
        }>((resolve) => {
          let stdout = '', stderr = '', timedOut = false, settled = false
          const spawnEnv = { ...process.env, ...extraEnv } as NodeJS.ProcessEnv
          const child = spawn(binary, args, { cwd, env: spawnEnv, stdio: ['pipe', 'pipe', 'pipe'] })

          const finish = (r: typeof result): void => {
            if (settled) {return}
            settled = true
            clearTimeout(timer)
            resolve(r)
          }
          const timer = setTimeout(() => {
            timedOut = true
            try { child.kill('SIGKILL') } catch { /* ignore */ }
            finish({ stdout, stderr, exitCode: null, timedOut })
          }, timeoutMs)

          child.stdout?.on('data', (d: Buffer) => { stdout += d.toString('utf8') })
          child.stderr?.on('data', (d: Buffer) => { stderr += d.toString('utf8') })
          child.on('error', (err) => {
            finish({ stdout, stderr: err.message, exitCode: null, timedOut })
          })
          child.on('close', (code) => { finish({ stdout, stderr, exitCode: code, timedOut }) })

          if (stdin !== null) {child.stdin?.end(stdin)}
          else {child.stdin?.end()}
        })

        log.info(`agent.exec: binary=${binary} exitCode=${result.exitCode} timedOut=${result.timedOut}`)
        if (result.timedOut) {
          span.fail(`timeout after ${timeoutMs}ms`, { binary })
        } else if (result.exitCode !== 0) {
          span.fail(`exit code ${result.exitCode}`, { binary, exitCode: result.exitCode ?? -1 })
        } else {
          span.ok({ binary, exitCode: result.exitCode ?? 0 })
        }
        return { jsonrpc: '2.0', id: rpc.id, result }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        span.fail(err, { binary })
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.exec failed: ${msg}`)
      }
    }

    // ── v5.0: ai.complete ─────────────────────────────────────────────────────
    // TG-002: Non-interactive AI completion for task planning (TaskAIPlanner.decompose)
    // and git commit message generation.
    // Called by: relay.call('ai.complete', { prompt, format: 'json'|'text', model? })
    case 'ai.complete': {
      try {
        const p      = rpc.params ?? {}
        const prompt = typeof p['prompt'] === 'string' ? p['prompt'] : ''
        if (!prompt.trim()) {
          return makeError(rpc.id, AgentErrorCode.InvalidParams, 'ai.complete: prompt is required')
        }
        const { handleAIComplete } = await import('./ai-complete-handler')
        const result = await handleAIComplete(
          {
            prompt,
            format: typeof p['format'] === 'string' ? p['format'] as 'json' | 'text' : 'text',
            taskId: typeof p['taskId'] === 'string' ? p['taskId']  : undefined,
            model:  typeof p['model']  === 'string' ? p['model']   : undefined,
            accountId:      typeof p['accountId']      === 'string' ? p['accountId']      : undefined,
            resolvedApiKey: typeof p['resolvedApiKey'] === 'string' ? p['resolvedApiKey'] : undefined,
          },
          config,
          log
        )
        return { jsonrpc: '2.0', id: rpc.id, result }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.complete failed: ${msg}`)
      }
    }


    // Runs a short shell command and returns stdout/stderr.
    // Used by devServer.browseDir on the Orca server to resolve '~' on the remote.
    // SECURITY: only used internally via relay — not exposed to browser directly.
    case 'shell.eval': {
      try {
        const { handleShellEval } = await import('./fs-agent-extensions')
        return (await handleShellEval(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `shell.eval unavailable: ${msg}`)
      }
    }

    // ── fs.mkdir ─────────────────────────────────────────────────────────────
    // Creates a directory (recursive) on the agent's filesystem.
    case 'fs.mkdir': {
      try {
        const { handleFsMkdir } = await import('./fs-agent-extensions')
        return (await handleFsMkdir(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.mkdir unavailable: ${msg}`)
      }
    }

    // ── fs.rmdir ─────────────────────────────────────────────────────────────
    // Removes an empty directory on the agent's filesystem.
    case 'fs.rmdir': {
      try {
        const { handleFsRmdir } = await import('./fs-agent-extensions')
        return (await handleFsRmdir(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.rmdir unavailable: ${msg}`)
      }
    }

    // ── fs.watch ─────────────────────────────────────────────────────────────
    // Starts pushing `fs.changed` notifications for a path. Idempotent/refcounted.
    case 'fs.watch': {
      try {
        const { handleFsWatch } = await import('./fs-agent-extensions')
        return (await handleFsWatch(rpc.id, rpc.params ?? {}, config, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.watch unavailable: ${msg}`)
      }
    }

    // ── fs.unwatch ───────────────────────────────────────────────────────────
    case 'fs.unwatch': {
      try {
        const { handleFsUnwatch } = await import('./fs-agent-extensions')
        return (await handleFsUnwatch(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `fs.unwatch unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.create ─────────────────────────────────────────────────────
    // TM-001/TM-006: Create a PTY session in agent mode.
    // Params: { cwd, cols?, rows?, env?, shellOverride? }
    // Returns: { id, cols, rows, cwd, shell }
    // Why all six pty.* cases below pass makeNotifier(ws, state) (not just
    // create/attach): PTYs now live in the detached pty-daemon process
    // (pty-daemon-client.ts), which can push pty.data/pty.exit/pty.replay for
    // ANY live PTY at any time, independent of which request last arrived —
    // every dispatch call rebinds the client's "current notify" to the live
    // WebSocket connection so a push always reaches it.
    case 'pty.create': {
      try {
        const { handlePtyCreate } = await import('./pty-daemon-client')
        return (await handlePtyCreate(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.create unavailable: ${msg}`)
      }
    }

    // ── pty.attach ───────────────────────────────────────────────────────────
    // Reattach to a PTY that survived a WebSocket disconnect (grace period)
    // or an agent process restart (the pty-daemon process survives it).
    case 'pty.attach': {
      try {
        const { handlePtyAttach } = await import('./pty-daemon-client')
        return (await handlePtyAttach(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.attach unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.write ──────────────────────────────────────────────────────
    // Send input data to PTY stdin.
    // Params: { id, data }
    case 'pty.write': {
      try {
        const { handlePtyWrite } = await import('./pty-daemon-client')
        return (await handlePtyWrite(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.write unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.resize ─────────────────────────────────────────────────────
    // Resize PTY terminal window.
    // Params: { id, cols, rows }
    case 'pty.resize': {
      try {
        const { handlePtyResize } = await import('./pty-daemon-client')
        return (await handlePtyResize(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.resize unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.destroy ────────────────────────────────────────────────────
    // Close and cleanup a PTY session.
    // Params: { id, graceful? }
    case 'pty.destroy': {
      try {
        const { handlePtyDestroy } = await import('./pty-daemon-client')
        return (await handlePtyDestroy(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.destroy unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.scrollback ─────────────────────────────────────────────────
    // Get scrollback buffer content.
    // Params: { id, lines? }
    case 'pty.scrollback': {
      try {
        const { handlePtyScrollback } = await import('./pty-daemon-client')
        return (await handlePtyScrollback(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.scrollback unavailable: ${msg}`)
      }
    }

    // ── v5.0: pty.sendSignal ─────────────────────────────────────────────────
    // Send a signal to the PTY process (SIGTERM, SIGKILL, SIGINT, etc.).
    // Params: { id, signal }
    case 'pty.sendSignal': {
      try {
        const { handlePtySendSignal } = await import('./pty-daemon-client')
        return (await handlePtySendSignal(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.sendSignal unavailable: ${msg}`)
      }
    }

    // ── pty.listProcesses ────────────────────────────────────────────────────
    // Enumerate every PTY this daemon currently tracks. Backend's
    // DevServerPtyProvider.listProcesses() uses this so its liveness sweep can
    // detect a Dev-Server-hosted PTY that died without any client noticing
    // (BUG-FE-PTY-001) — previously there was no agent-wide enumeration RPC.
    case 'pty.listProcesses': {
      try {
        const { handlePtyListProcesses } = await import('./pty-daemon-client')
        return (await handlePtyListProcesses(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.listProcesses unavailable: ${msg}`)
      }
    }

    // ── Unknown method ───────────────────────────────────────────────────────
    default:
      return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Method not found: ${rpc.method}`)
  }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

/**
 * Format a ToolResult as an MCP-compatible JSON-RPC success response.
 * content[0].type is always 'text', isError reflects exitCode !== 0.
 */
export function formatMcpResult(id: string | number | null, result: ToolResult): JsonRpcSuccess {
  const text = [
    result.stdout || '',
    result.stderr ? `[stderr]\n${result.stderr}` : '',
  ].filter(Boolean).join('\n').trim()

  return {
    jsonrpc: '2.0', id,
    result: {
      content:  [{ type: 'text', text: text || '(no output)' }],
      isError:  result.exitCode !== 0,
      exitCode: result.exitCode,
      meta:     result.meta,
    },
  }
}

export function makeError(
  id: string | number | null,
  code: number,
  message: string,
  data?: unknown
): JsonRpcError {
  return {
    jsonrpc: '2.0', id,
    error: { code, message, ...(data !== undefined && { data }) },
  }
}
