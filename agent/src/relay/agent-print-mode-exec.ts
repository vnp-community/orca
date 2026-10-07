// src/relay/agent-print-mode-exec.ts
// Part A implementation of `agent.execPrompt` — resolves a workflow/task-step
// prompt request into a one-shot, non-interactive AI-CLI invocation.
//
// Distinct from `agent.exec` (a generic, tested "run this binary" primitive —
// see agent-rpc-dispatch.ts's case 'agent.exec') and from `agent.spawn`
// (interactive PTY session). StepExecutors.executeAgent() was sending a
// domain-shaped payload ({prompt, worktreePath, trustPreset, model, accountId})
// straight to agent.exec, which only accepts {binary, args, cwd, stdin, env,
// timeoutMs} — every 'agent'-type workflow step failed with InvalidParams.
// See specs/agent/api/gaps-and-findings.md.

import { spawn } from 'node:child_process'
import type WebSocket from 'ws'
import { encodeDataFrame } from 'orca-dev-agent-transport'
import type { WireState } from 'orca-dev-agent-transport'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { resolveAgentSpec, buildAgentEnv } from './agent-spawner'
import { YOLO_TUI_AGENT_ARGS } from '../shared/tui-agent-permissions'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { Tracers } from '../shared/trace/tracers'
import { parseExecPromptOptions, toExecPromptErrorResponse } from './agent-exec-prompt-options'
import { detectClaudeFlags, readonlyUnsupportedReason, readonlyUnsupportedError, buildReadonlyArgs } from './agent-readonly-tool-policy'
import { validateWorkspace, defaultScratchRoots } from './agent-workspace-validation'
import { BoundedOutputBuffer, splitOutputBudget, fitResultToFrame } from './agent-bounded-output-buffer'
import { parseResultBlock } from './agent-result-block-parser'
import { captureSnapshot, diffSnapshots } from './agent-worktree-change-snapshot'

const DEFAULT_TIMEOUT_MS = 5 * 60_000
const MAX_TIMEOUT_MS = 15 * 60_000
const MIN_TIMEOUT_MS = 1_000

type PrintModeExecResult = {
  stdout: string
  stderr: string
  exitCode: number | null
  timedOut: boolean
  stepId?: string
}

export async function handleAgentExecPrompt(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger,
  notify?: (method: string, params: Record<string, unknown>) => void
): Promise<object> {
  const prompt = typeof params.prompt === 'string' ? params.prompt : ''
  const worktreePath = typeof params.worktreePath === 'string' ? params.worktreePath : ''
  const stepId = typeof params.stepId === 'string' ? params.stepId : undefined
  // Distinct from stepId (protocol-level request id) — an explicit business
  // taskId/projectId, when the caller sends one, avoids conflating the two.
  const taskId = typeof params.taskId === 'string' ? params.taskId : ''
  const projectId = typeof params.projectId === 'string' ? params.projectId : ''
  // task-service/workflow-service's domain.BuildProjectContext — project +
  // department/"Team" context prepended ahead of the real prompt (field
  // name confirmed against this handler now, see TASK-PRF-04-06/08's own
  // "field name UNVERIFIED" caveat — this closes that gap).
  const initFile = typeof params.initFile === 'string' ? params.initFile : ''
  // 'default'/'standard'/'none' (StepExecutors.ts's and the old vocabulary's
  // non-'full' values) all mean "no extra flag" here — only 'full' is acted on.
  const trustPresetFull = params.trustPreset === 'full'
  const modelId = typeof params.model === 'string' && params.model ? params.model : 'claude'
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  // ProfileAwareAgentSpawner forwards profile-resolved env (PATH additions,
  // ORCA_PROJECT_ID/ORCA_ACCOUNT_ID/etc.) here — merged on top of
  // buildAgentEnv()'s base env via its own extraEnv slot, same override order
  // agent.spawn already uses.
  const extraEnv =
    params.env && typeof params.env === 'object' && !Array.isArray(params.env)
      ? (params.env as Record<string, string>)
      : undefined
  const timeoutMs =
    typeof params.timeoutMs === 'number'
      ? Math.min(Math.max(params.timeoutMs, MIN_TIMEOUT_MS), MAX_TIMEOUT_MS)
      : DEFAULT_TIMEOUT_MS

  const span = Tracers.agentOrchSpawn.start({ stepId, modelId })

  if (!prompt || !worktreePath) {
    const missing = [!prompt && 'prompt', !worktreePath && 'worktreePath']
      .filter(Boolean)
      .join(', ')
    span.fail(`missing ${missing}`)
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: `agent.execPrompt: missing required field(s): ${missing}`
      }
    }
  }

  // Parse CR-REQ-033 options (accessMode, workspaceKind, reportChanges, resultBlock, maxOutputBytes)
  const optsParsed = parseExecPromptOptions(params)
  if (!optsParsed.ok) {
    span.fail(optsParsed.error.code)
    return toExecPromptErrorResponse('agent.execPrompt', id, optsParsed.error)
  }
  const options = optsParsed.value

  // Validate workspace path against the declared kind
  const wsValidation = await validateWorkspace({
    kind: options.workspaceKind,
    path: worktreePath,
    accessMode: options.accessMode,
    scratchRoots: defaultScratchRoots(config.workDir)
  })
  if (!wsValidation.ok) {
    span.fail(wsValidation.code)
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: `agent.execPrompt: ${wsValidation.message} (${wsValidation.code})`,
        data: { reason: wsValidation.code }
      }
    }
  }
  // Use realPath as cwd when not worktree (worktree preserves exact caller string)
  const spawnCwd = options.workspaceKind !== 'worktree' ? wsValidation.realPath : worktreePath

  const spec = resolveAgentSpec(modelId)
  // Only claude's non-interactive `--print <prompt>` flag is a validated
  // precedent in this codebase (agent-tool-registry.ts's claude_code tool).
  // codex/gemini/opencode's print-mode flags are unverified even for the
  // existing *interactive* PTY path (see agent-spawner.ts's AGENT_SPECS
  // comments) — fail fast instead of guessing a flag that silently no-ops or
  // hangs waiting for interactive input.
  if (!spec || spec.binary !== 'claude') {
    span.fail('unsupported model for one-shot exec', { modelId })
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message:
          `agent.execPrompt: model "${modelId}" is not supported for one-shot execution yet ` +
          `(only claude is validated) — UNSUPPORTED_MODEL_FOR_ONE_SHOT_EXEC`
      }
    }
  }

  // If readonly, check that claude supports required flags before spawning
  const warnings: string[] = []
  let readonlyArgs: string[] = []
  // Note: readonly check INTENTIONALLY placed after buildAgentEnv so we
  // can merge env with process.env to get the same PATH that execPrompt uses.

  let env: Record<string, string>
  try {
    // resolvedApiKey is always null here — no live backend caller forwards a
    // plaintext key for this RPC (that's the ADR-008 gap, left dormant by
    // design; see specs/agent/api/compliance-audit-2026-08-15.md). If
    // accountId is set, buildAgentEnv() throws a clear, actionable error
    // instead of silently running unauthenticated; if accountId is absent,
    // it proceeds relying on the CLI's own already-authenticated state.
    env = await buildAgentEnv(
      {
        accountId,
        userId: '',
        taskId: taskId || (stepId ?? ''),
        projectId,
        cwd: spawnCwd,
        model: modelId,
        extraEnv
      },
      spec,
      config,
      null,
      log,
      span.id
    )
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg, { accountId })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PermissionDenied, message: msg } }
  }

  // If readonly: probe claude flags with the same PATH that spawn will use
  if (options.accessMode === 'readonly') {
    const mergedEnv = { ...process.env, ...env }
    const flags = await detectClaudeFlags(mergedEnv)
    const reason = readonlyUnsupportedReason(flags)
    if (reason) {
      span.fail(reason)
      return readonlyUnsupportedError(id, 'agent.execPrompt', reason) as object
    }
    readonlyArgs = buildReadonlyArgs()
  }

  const args = ['--print', initFile ? `${initFile}\n${prompt}` : prompt]
  if (options.accessMode === 'write' && trustPresetFull && YOLO_TUI_AGENT_ARGS.claude) {
    args.push(YOLO_TUI_AGENT_ARGS.claude)
  } else if (options.accessMode === 'readonly') {
    args.push(...readonlyArgs)
    // YOLO flags override tool restrictions — discard them to keep readonly enforced
    if (trustPresetFull) {
      log.warn('agent.execPrompt: trustPreset=full ignored in readonly mode (TRUST_PRESET_IGNORED_READONLY)')
      warnings.push('TRUST_PRESET_IGNORED_READONLY')
    }
  }

  // Snapshot before spawn (task 08)
  const snapshotMode = options.workspaceKind === 'scratch' ? 'directory' : 'git'
  const beforeSnapshot = options.reportChanges
    ? await captureSnapshot(spawnCwd, snapshotMode)
    : undefined

  // Output buffers with per-budget allocation (task 05)
  const { stdoutBytes, stderrBytes } = splitOutputBudget(options.maxOutputBytes)
  const stdoutBuf = new BoundedOutputBuffer(stdoutBytes)
  const stderrBuf = new BoundedOutputBuffer(stderrBytes)


  span.step('subprocess-spawn', { binary: spec.binary, cwd: spawnCwd })
  const result = await new Promise<PrintModeExecResult>((resolve) => {
    let timedOut = false
    let settled = false
    const child = spawn(spec.binary, args, {
      cwd: spawnCwd,
      env: { ...process.env, ...env },
      stdio: ['ignore', 'pipe', 'pipe']
    })

    const finish = (r: PrintModeExecResult): void => {
      if (settled) {
        return
      }
      settled = true
      clearTimeout(timer)
      resolve(r)
    }
    const timer = setTimeout(() => {
      timedOut = true
      try {
        child.kill('SIGKILL')
      } catch {
        /* ignore */
      }
      finish({ stdout: stdoutBuf.toString(), stderr: stderrBuf.toString(), exitCode: null, timedOut })
    }, timeoutMs)

    child.stdout?.on('data', (d: Buffer) => {
      stdoutBuf.append(d)
      notify?.('agent.execPrompt.output', { stepId, stream: 'stdout', data: d.toString('utf8') })
    })
    child.stderr?.on('data', (d: Buffer) => {
      stderrBuf.append(d)
      notify?.('agent.execPrompt.output', { stepId, stream: 'stderr', data: d.toString('utf8') })
    })
    child.on('error', (err) => {
      finish({ stdout: stdoutBuf.toString(), stderr: err.message, exitCode: null, timedOut })
    })
    child.on('close', (code) => {
      finish({ stdout: stdoutBuf.toString(), stderr: stderrBuf.toString(), exitCode: code, timedOut })
    })
  })

  log.info(
    `agent.execPrompt: stepId=${stepId ?? '(none)'} model=${modelId} ` +
      `exitCode=${result.exitCode} timedOut=${result.timedOut}`
  )
  if (result.timedOut) {
    span.fail(`timeout after ${timeoutMs}ms`)
  } else if (result.exitCode !== 0) {
    span.fail(`exit code ${result.exitCode}`, { exitCode: result.exitCode ?? -1 })
  } else {
    span.ok({ exitCode: result.exitCode ?? 0 })
  }

  // Capture after-snapshot (even on timeout — agent may have partially written files)
  let changes = undefined
  if (options.reportChanges && beforeSnapshot !== undefined) {
    const afterSnapshot = await captureSnapshot(spawnCwd, snapshotMode)
    const changeReport = diffSnapshots(beforeSnapshot, afterSnapshot)
    changes = changeReport
    // Warn if readonly but files changed (does not roll back or alter exitCode)
    if (
      options.accessMode === 'readonly' &&
      changeReport.available &&
      (changeReport.changedFiles.length > 0 || changeReport.headMoved)
    ) {
      warnings.push('READONLY_VIOLATION')
    }
  }

  // Parse result block if requested (task 08)
  const parsed = options.resultBlockNonce
    ? parseResultBlock(result.stdout, options.resultBlockNonce)
    : undefined

  // Build truncation info (task 05)
  const truncated = (stdoutBuf.truncated || stderrBuf.truncated)
    ? { stdout: stdoutBuf.truncated, stderr: stderrBuf.truncated }
    : undefined

  // applied echo: only when caller sent explicit options (task 04)
  const applied = (options.explicit.accessMode || options.explicit.workspaceKind)
    ? { accessMode: options.accessMode, workspaceKind: options.workspaceKind }
    : undefined

  let finalResult: Record<string, unknown> = {
    ...result,
    stepId,
    ...(applied !== undefined ? { applied } : {}),
    ...(changes !== undefined ? { changes } : {}),
    ...(parsed !== undefined ? { parsed } : {}),
    ...(truncated !== undefined ? { truncated } : {}),
    ...(warnings.length > 0 ? { warnings } : {})
  }

  // Ensure the JSON frame stays within websocket limit (task 05)
  finalResult = fitResultToFrame(finalResult)

  return { jsonrpc: '2.0', id, result: finalResult }
}

// ─── agent.execPromptStream ─────────────────────────────────────────────────
// CR-TG-006: streaming sibling of handleAgentExecPrompt above — same
// prompt/worktreePath/model/env setup and one-shot `claude --print` spawn,
// but delivers stdout/stderr incrementally via stream.chunk/stream.end wire
// frames (same convention as agent-git-handler.ts's handleGitExecStream)
// instead of buffering into a single buffer-then-return response. Reused by
// task-service's complex executor so callers can show live step output.
// handleAgentExecPrompt itself is untouched — every existing caller
// (SimpleExecutor, ProfileAwareAgentSpawner.spawn(), StepExecutors.executeAgent())
// keeps its buffer-then-return contract.
//
// Open Question 1 (chunking granularity) resolved: forward each raw stdout/
// stderr `data` event verbatim, no line-splitting. Matches SOL-AG-TG-002's
// original sketch and keeps latency lowest (a line-buffered approach — like
// handleGitExecStream's — would hold back a trailing partial line, e.g. an
// unterminated prompt or progress indicator, until a newline or process
// exit); git.execStream's line-buffering is about de-duplicating git's own
// \r-heavy progress output, a concern that doesn't apply here.
//
// Open Question 2 (timeout behavior) resolved: keep handleAgentExecPrompt's
// DEFAULT_TIMEOUT_MS/MAX_TIMEOUT_MS/SIGKILL-on-timeout behavior for parity
// with the non-streaming sibling (git.execStream has no such timeout, but it
// has no non-streaming sibling to stay consistent with). A killed-by-timeout
// process reports `stream.end` with `exitCode: -1` — the sentinel
// handleGitExecStream's `code ?? 0` fallback and this task's own sketch both
// use for "no real exit code", since stream.end's exitCode field is a
// required `number`, unlike handleAgentExecPrompt's nullable `exitCode`.
export async function handleAgentExecPromptStream(
  ws: WebSocket,
  wireState: WireState,
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<void> {
  const prompt = typeof params.prompt === 'string' ? params.prompt : ''
  const worktreePath = typeof params.worktreePath === 'string' ? params.worktreePath : ''
  const stepId = typeof params.stepId === 'string' ? params.stepId : undefined
  const trustPresetFull = params.trustPreset === 'full'
  const modelId = typeof params.model === 'string' && params.model ? params.model : 'claude'
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  // See handleAgentExecPrompt's identical field for the source/rationale.
  const initFile = typeof params.initFile === 'string' ? params.initFile : ''
  const extraEnv =
    params.env && typeof params.env === 'object' && !Array.isArray(params.env)
      ? (params.env as Record<string, string>)
      : undefined
  const timeoutMs =
    typeof params.timeoutMs === 'number'
      ? Math.min(Math.max(params.timeoutMs, MIN_TIMEOUT_MS), MAX_TIMEOUT_MS)
      : DEFAULT_TIMEOUT_MS

  const span = Tracers.agentOrchSpawn.start({ stepId, modelId })

  if (!prompt || !worktreePath) {
    const missing = [!prompt && 'prompt', !worktreePath && 'worktreePath']
      .filter(Boolean)
      .join(', ')
    span.fail(`missing ${missing}`)
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: `agent.execPromptStream: missing required field(s): ${missing}`
      }
    })
    return
  }

  // Parse CR-REQ-033 options
  const optsParsed = parseExecPromptOptions(params)
  if (!optsParsed.ok) {
    span.fail(optsParsed.error.code)
    sendFrame(ws, wireState, toExecPromptErrorResponse('agent.execPromptStream', id, optsParsed.error))
    return
  }
  const options = optsParsed.value

  // Validate workspace path
  const wsValidation = await validateWorkspace({
    kind: options.workspaceKind,
    path: worktreePath,
    accessMode: options.accessMode,
    scratchRoots: defaultScratchRoots(config.workDir)
  })
  if (!wsValidation.ok) {
    span.fail(wsValidation.code)
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: `agent.execPromptStream: ${wsValidation.message} (${wsValidation.code})`,
        data: { reason: wsValidation.code }
      }
    })
    return
  }
  const spawnCwd = options.workspaceKind !== 'worktree' ? wsValidation.realPath : worktreePath

  const spec = resolveAgentSpec(modelId)
  if (!spec || spec.binary !== 'claude') {
    span.fail('unsupported model for one-shot exec', { modelId })
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message:
          `agent.execPromptStream: model "${modelId}" is not supported for one-shot execution yet ` +
          `(only claude is validated) — UNSUPPORTED_MODEL_FOR_ONE_SHOT_EXEC`
      }
    })
    return
  }

  let env: Record<string, string>
  try {
    env = await buildAgentEnv(
      { accountId, userId: '', taskId: stepId ?? '', cwd: spawnCwd, model: modelId, extraEnv },
      spec,
      config,
      null,
      log,
      span.id
    )
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg, { accountId })
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.PermissionDenied, message: msg }
    })
    return
  }

  // Readonly flag check (after env, needs the same PATH)
  const warnings: string[] = []
  let readonlyArgs: string[] = []
  if (options.accessMode === 'readonly') {
    const mergedEnv = { ...process.env, ...env }
    const flags = await detectClaudeFlags(mergedEnv)
    const reason = readonlyUnsupportedReason(flags)
    if (reason) {
      span.fail(reason)
      sendFrame(ws, wireState, readonlyUnsupportedError(id, 'agent.execPromptStream', reason))
      return
    }
    readonlyArgs = buildReadonlyArgs()
  }

  const args = ['--print', initFile ? `${initFile}\n${prompt}` : prompt]
  if (options.accessMode === 'write' && trustPresetFull && YOLO_TUI_AGENT_ARGS.claude) {
    args.push(YOLO_TUI_AGENT_ARGS.claude)
  } else if (options.accessMode === 'readonly') {
    args.push(...readonlyArgs)
    // YOLO flags override tool restrictions — discard them to keep readonly enforced
    if (trustPresetFull) {
      log.warn('agent.execPromptStream: trustPreset=full ignored in readonly mode (TRUST_PRESET_IGNORED_READONLY)')
      warnings.push('TRUST_PRESET_IGNORED_READONLY')
    }
  }

  // applied echo (only when explicit params sent)
  const applied = (options.explicit.accessMode || options.explicit.workspaceKind)
    ? { accessMode: options.accessMode, workspaceKind: options.workspaceKind }
    : undefined

  // Snapshot before spawn
  const snapshotMode = options.workspaceKind === 'scratch' ? 'directory' : 'git'
  const beforeSnapshot = options.reportChanges
    ? await captureSnapshot(spawnCwd, snapshotMode)
    : undefined

  // Buffer for result block analysis only — chunks are sent fully, not truncated
  // truncated here means the analysis buffer was too small, not that chunks were cut
  const analysisBuf = options.resultBlockNonce
    ? new BoundedOutputBuffer(4 * 1024 * 1024)
    : undefined

  span.step('subprocess-spawn', { binary: spec.binary, cwd: spawnCwd })
  const child = spawn(spec.binary, args, {
    cwd: spawnCwd,
    env: { ...process.env, ...env },
    stdio: ['ignore', 'pipe', 'pipe']
  })

  let settled = false

  async function finalizeAndSendEnd(exitCode: number): Promise<void> {
    // Capture after-snapshot (even on timeout)
    let changes = undefined
    if (options.reportChanges && beforeSnapshot !== undefined) {
      const afterSnapshot = await captureSnapshot(spawnCwd, snapshotMode)
      const changeReport = diffSnapshots(beforeSnapshot, afterSnapshot)
      changes = changeReport
      if (
        options.accessMode === 'readonly' &&
        changeReport.available &&
        (changeReport.changedFiles.length > 0 || changeReport.headMoved)
      ) {
        warnings.push('READONLY_VIOLATION')
      }
    }

    const parsed = options.resultBlockNonce && analysisBuf
      ? parseResultBlock(analysisBuf.toString(), options.resultBlockNonce)
      : undefined

    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      result: {
        type: 'stream.end',
        exitCode,
        ...(applied !== undefined ? { applied } : {}),
        ...(changes !== undefined ? { changes } : {}),
        ...(parsed !== undefined ? { parsed } : {}),
        ...(warnings.length > 0 ? { warnings } : {})
      }
    })
  }

  const timer = setTimeout(() => {
    settled = true
    try {
      child.kill('SIGKILL')
    } catch {
      /* ignore */
    }
    span.fail(`timeout after ${timeoutMs}ms`)
    // Run async finalize but don't block the timeout path
    void finalizeAndSendEnd(-1)
  }, timeoutMs)

  function sendChunk(text: string, source?: 'stderr'): void {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      result: { type: 'stream.chunk', line: text, ...(source ? { source } : {}) }
    })
  }

  child.stdout?.on('data', (d: Buffer) => {
    analysisBuf?.append(d)
    sendChunk(d.toString('utf8'))
  })
  child.stderr?.on('data', (d: Buffer) => sendChunk(d.toString('utf8'), 'stderr'))
  child.on('close', (code) => {
    if (settled) {
      return
    }
    settled = true
    clearTimeout(timer)
    const exitCode = code ?? -1
    log.info(
      `agent.execPromptStream: stepId=${stepId ?? '(none)'} model=${modelId} exitCode=${exitCode}`
    )
    span.ok({ exitCode })
    void finalizeAndSendEnd(exitCode)
  })
  child.on('error', (err) => {
    if (settled) {
      return
    }
    settled = true
    clearTimeout(timer)
    span.fail(err)
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.ServerError, message: err.message }
    })
  })
}

function sendFrame(ws: WebSocket, wireState: WireState, payload: object): void {
  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)))
  }
}
