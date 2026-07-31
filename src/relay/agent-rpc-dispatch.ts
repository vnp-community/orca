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

const rpcTracer = createTracer('agent:rpc')

// ─── JSON-RPC types ───────────────────────────────────────────────────────────

export interface JsonRpcRequest {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly method: string
  readonly params?: Record<string, unknown>
}

interface JsonRpcSuccess {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly result: unknown
}

interface JsonRpcError {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly error: { code: number; message: string; data?: unknown }
}

type JsonRpcResponse = JsonRpcSuccess | JsonRpcError

// ─── RpcDispatcher ────────────────────────────────────────────────────────────

export interface RpcDispatcher {
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
    return s ? (s.length > 60 ? '...' + s.slice(-57) : s) : undefined
  }
  const truncCmd = (v: unknown) => {
    const s = str(v)
    return s ? (s.length > 80 ? s.slice(0, 77) + '...' : s) : undefined
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

  if (method.startsWith('agent.')) {
    return {
      session: str(p['sessionId']),
      cmd:     truncCmd(p['cmd'] ?? p['command']),
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
      const span = rpcTracer.start({ method: rpc.method, id: String(rpc.id ?? 'notify'), ...ctxFields })
      let response: JsonRpcResponse
      try {
        response = await route(rpc, tools, config, log, ws, state)
        // JsonRpcError (method-level failures) are still 'ok' at transport level
        if ('error' in response) {
          span.fail(response.error.message, { method: rpc.method, code: response.error.code })
        } else {
          span.ok({ method: rpc.method })
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
        const { handleGitPrCreate } = await import('./agent-git-handler')
        return (await handleGitPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
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

    // ── shell.eval ───────────────────────────────────────────────────────────
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
