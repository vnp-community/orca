import { exec, spawn, type ChildProcess } from 'node:child_process'
import { existsSync } from 'node:fs'
import { delimiter, join } from 'node:path'
import type { RelayDispatcher, RequestContext } from './dispatcher'

const DEFAULT_TIMEOUT_MS = 60_000
const MAX_TIMEOUT_MS = 5 * 60 * 1000
const MAX_OUTPUT_BYTES = 4 * 1024 * 1024
const WINDOWS_BATCH_UNSAFE_ARGUMENTS_ERROR = 'UNSAFE_WINDOWS_BATCH_ARGUMENTS'

function getCmdExePath(): string {
  return process.env.ComSpec || `${process.env.SystemRoot ?? 'C:\\Windows'}\\System32\\cmd.exe`
}

function isWindowsBatchScript(commandPath: string): boolean {
  return process.platform === 'win32' && /\.(cmd|bat)$/i.test(commandPath)
}

function hasUnsafeWindowsBatchSyntax(value: string): boolean {
  return /[&|<>^"%!\r\n]/.test(value)
}

function quoteWindowsBatchToken(value: string): string {
  if (hasUnsafeWindowsBatchSyntax(value)) {
    throw new Error(WINDOWS_BATCH_UNSAFE_ARGUMENTS_ERROR)
  }
  return `"${value}"`
}

function resolveWindowsCommand(binary: string, env: NodeJS.ProcessEnv): string {
  if (process.platform !== 'win32') {
    return binary
  }
  if (/[\\/]/.test(binary) || /\.[a-z0-9]+$/i.test(binary)) {
    return binary
  }

  const pathEnv = env.PATH ?? env.Path
  if (!pathEnv) {
    return binary
  }
  const names = [`${binary}.cmd`, `${binary}.exe`, `${binary}.bat`, binary]
  for (const directory of pathEnv.split(delimiter).filter(Boolean)) {
    for (const name of names) {
      const candidate = join(directory, name)
      if (existsSync(candidate)) {
        return candidate
      }
    }
  }
  return binary
}

function getWindowsSafeSpawn(
  binary: string,
  args: string[],
  env: NodeJS.ProcessEnv
): { spawnCmd: string; spawnArgs: string[] } {
  const resolvedBinary = resolveWindowsCommand(binary, env)
  if (!isWindowsBatchScript(resolvedBinary)) {
    return { spawnCmd: resolvedBinary, spawnArgs: args }
  }
  const commandLine = [resolvedBinary, ...args].map(quoteWindowsBatchToken).join(' ')
  return { spawnCmd: getCmdExePath(), spawnArgs: ['/d', '/s', '/c', commandLine] }
}

// Why: mirrors src/main/text-generation/commit-message-text-generation.ts. On
// Windows, npm-installed CLIs like `claude`/`codex` are usually `.cmd` shims.
// We route those through cmd.exe so Node can launch them, and taskkill is
// needed to terminate the whole wrapper + node.exe process tree. Kept
// duplicated rather than imported because the relay ships to remote hosts.
function killProcessTree(child: ChildProcess): void {
  const pid = child.pid
  if (!pid) {
    return
  }
  if (process.platform === 'win32') {
    exec(`taskkill /pid ${pid} /T /F`, () => {
      // Best-effort; the spawn's `close` listener fires once the tree exits.
    })
    return
  }
  try {
    child.kill('SIGKILL')
  } catch {
    // Child may already have exited between the kill request and now.
  }
}

type ExecParams = {
  binary: unknown
  args: unknown
  cwd: unknown
  stdin: unknown
  timeoutMs: unknown
  env: unknown
  operation: unknown
}

type CancelParams = {
  cwd: unknown
  operation: unknown
}

function laneKeyFor(cwd: string, operation: unknown): string {
  const op = typeof operation === 'string' && operation ? operation : 'default'
  return JSON.stringify([op, cwd])
}

type InFlightExec = { child: ChildProcess; cancel: () => void }

type ExecResult = {
  stdout: string
  stderr: string
  exitCode: number | null
  timedOut: boolean
  /** Set when the user canceled the exec via `agent.cancelExec`. */
  canceled?: boolean
  /** Set when the binary could not be spawned (e.g. ENOENT). */
  spawnError?: string
}

/**
 * Non-interactive subprocess exec on the remote host. Used by the AI commit
 * message generator to spawn agent CLIs (claude, codex, …) with the staged
 * diff piped via stdin and the output captured to stdout. Distinct from
 * `pty.spawn` because we want no terminal allocation, no escape sequences,
 * and a clean exit code instead of an interactive session.
 */
export class AgentExecHandler {
  // Why: commit-message and PR-field generation can run together for one cwd;
  // operation lanes let cancel target only the user-visible job that stopped.
  private inFlightByLane = new Map<string, InFlightExec>()

  private laneKey(cwd: string, operation: unknown): string {
    return laneKeyFor(cwd, operation)
  }

  constructor(dispatcher: RelayDispatcher) {
    dispatcher.onRequest('agent.execNonInteractive', (p, context) =>
      this.exec(p as ExecParams, context)
    )
    dispatcher.onRequest('agent.cancelExec', (p) => this.cancel(p as CancelParams))
  }

  private async cancel(params: CancelParams): Promise<{ canceled: boolean }> {
    const cwd = typeof params.cwd === 'string' ? params.cwd : ''
    const entry = this.inFlightByLane.get(this.laneKey(cwd, params.operation))
    if (!entry) {
      return { canceled: false }
    }
    entry.cancel()
    return { canceled: true }
  }

  private async exec(params: ExecParams, context?: RequestContext): Promise<ExecResult> {
    const binary = typeof params.binary === 'string' ? params.binary : ''
    if (!binary) {
      throw new Error('agent.execNonInteractive: binary is required')
    }
    const args = Array.isArray(params.args) ? params.args.map((a) => String(a)) : []
    const cwd = typeof params.cwd === 'string' && params.cwd.length > 0 ? params.cwd : undefined
    const stdinPayload = typeof params.stdin === 'string' ? params.stdin : null
    const requestedTimeout =
      typeof params.timeoutMs === 'number' ? params.timeoutMs : DEFAULT_TIMEOUT_MS
    const timeoutMs = Math.max(1_000, Math.min(MAX_TIMEOUT_MS, requestedTimeout))
    const extraEnv =
      params.env && typeof params.env === 'object' && !Array.isArray(params.env)
        ? (params.env as Record<string, string>)
        : null
    const spawnEnv = extraEnv ? { ...process.env, ...extraEnv } : process.env

    return new Promise<ExecResult>((resolve) => {
      let child
      try {
        const { spawnCmd, spawnArgs } = getWindowsSafeSpawn(binary, args, spawnEnv)
        child = spawn(spawnCmd, spawnArgs, {
          cwd,
          env: spawnEnv,
          stdio: ['pipe', 'pipe', 'pipe'],
          windowsHide: true
        })
      } catch (error) {
        resolve({
          stdout: '',
          stderr: '',
          exitCode: null,
          timedOut: false,
          spawnError: error instanceof Error ? error.message : String(error)
        })
        return
      }

      let stdout = ''
      let stderr = ''
      let stdoutBytes = 0
      let stderrBytes = 0
      let timedOut = false
      let canceled = false
      let settled = false
      const laneKey = typeof cwd === 'string' ? this.laneKey(cwd, params.operation) : ''
      let entry: InFlightExec | null = null
      let timer: ReturnType<typeof setTimeout> | null = null
      let detachChildListeners = (): void => {}
      let detachRequestAbortListener = (): void => {}
      const finish = (result: ExecResult): void => {
        if (settled) {
          return
        }
        settled = true
        if (timer) {
          clearTimeout(timer)
          timer = null
        }
        detachRequestAbortListener()
        detachChildListeners()
        if (laneKey && entry && this.inFlightByLane.get(laneKey) === entry) {
          this.inFlightByLane.delete(laneKey)
        }
        resolve(result)
      }
      const cancelCurrent = (): void => {
        canceled = true
        killProcessTree(child)
      }
      if (laneKey) {
        // Why: the relay owns one visible non-interactive job per cwd+operation.
        // Replacing the lane without canceling the prior child would orphan
        // that process until timeout because future cancelExec calls reach only
        // the newest map entry.
        this.inFlightByLane.get(laneKey)?.cancel()
        entry = {
          child,
          cancel: cancelCurrent
        }
        this.inFlightByLane.set(laneKey, entry)
      }

      timer = setTimeout(() => {
        timedOut = true
        // Why: tree-kill because some CLIs trap SIGTERM and continue streaming;
        // also Windows wraps `.cmd` shims in cmd.exe, so the immediate child
        // is not the real node.exe process.
        killProcessTree(child)
        finish({ stdout, stderr, exitCode: null, timedOut, canceled })
      }, timeoutMs)

      const onStdoutData = (chunk: Buffer): void => {
        stdoutBytes += chunk.byteLength
        if (stdoutBytes > MAX_OUTPUT_BYTES) {
          killProcessTree(child)
          return
        }
        stdout += chunk.toString('utf-8')
      }
      const onStderrData = (chunk: Buffer): void => {
        stderrBytes += chunk.byteLength
        if (stderrBytes > MAX_OUTPUT_BYTES) {
          killProcessTree(child)
          return
        }
        stderr += chunk.toString('utf-8')
      }
      const onError = (error: Error): void => {
        finish({
          stdout,
          stderr,
          exitCode: null,
          timedOut,
          spawnError: error.message
        })
      }
      const onClose = (code: number | null): void => {
        finish({ stdout, stderr, exitCode: code, timedOut, canceled })
      }
      child.stdout?.on('data', onStdoutData)
      child.stderr?.on('data', onStderrData)
      child.on('error', onError)
      child.on('close', onClose)
      detachChildListeners = () => {
        child.stdout?.off('data', onStdoutData)
        child.stderr?.off('data', onStderrData)
        child.off('error', onError)
        child.off('close', onClose)
      }

      if (context?.signal) {
        if (context.signal.aborted) {
          cancelCurrent()
        } else {
          context.signal.addEventListener('abort', cancelCurrent, { once: true })
          detachRequestAbortListener = () => {
            context.signal?.removeEventListener('abort', cancelCurrent)
          }
        }
      }

      if (stdinPayload !== null) {
        child.stdin?.end(stdinPayload)
      } else {
        child.stdin?.end()
      }
    })
  }
}

// ─── TG-001: handleAgentExec — Non-interactive AI agent execution ─────────────
//
// Called by:
//   - StepExecutors.executeAgent() via relay.call('agent.exec', {...})
//   - ProfileAwareAgentSpawner via relay.call('agent.exec', {...})
//
// Difference from agent.spawn (interactive PTY):
//   - No terminal allocation (runs as subprocess with piped stdio)
//   - Returns captured stdout/stderr in JSON-RPC response (not streamed)
//   - Has a fixed timeout (default 5min)
//   - Structured result includes stepId for workflow tracking
// ─────────────────────────────────────────────────────────────────────────────

import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

export interface AgentExecRequest {
  prompt:       string
  worktreePath: string
  trustPreset?: 'standard' | 'full' | 'none'
  model?:       string
  accountId?:   string
  taskId?:      string
  stepId?:      string
  timeoutMs?:   number
}

function parseAgentExecRequest(params: Record<string, unknown>): AgentExecRequest | null {
  if (typeof params.prompt       !== 'string' || !params.prompt)       return null
  if (typeof params.worktreePath !== 'string' || !params.worktreePath) return null
  return {
    prompt:       params.prompt,
    worktreePath: params.worktreePath,
    trustPreset:  typeof params.trustPreset === 'string' ? params.trustPreset as AgentExecRequest['trustPreset'] : 'standard',
    model:        typeof params.model       === 'string' ? params.model       : undefined,
    accountId:    typeof params.accountId   === 'string' ? params.accountId   : undefined,
    taskId:       typeof params.taskId      === 'string' ? params.taskId      : undefined,
    stepId:       typeof params.stepId      === 'string' ? params.stepId      : undefined,
    timeoutMs:    typeof params.timeoutMs   === 'number' ? params.timeoutMs   : undefined,
  }
}

/**
 * handleAgentExec — Run an AI agent CLI non-interactively and capture output.
 *
 * Supports Claude (--print mode), Codex, Gemini, and opencode.
 * Returns { stdout, stderr, exitCode, latencyMs, timedOut, stepId }.
 */
export async function handleAgentExec(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const req = parseAgentExecRequest(params)
  if (!req) {
    return {
      jsonrpc: '2.0', id,
      error: { code: -32602, message: 'agent.exec: prompt and worktreePath are required' },
    }
  }

  // Resolve binary based on model
  const { resolveAgentSpec } = await import('./agent-spawner')
  const spec = resolveAgentSpec(req.model ?? 'claude')
  if (!spec) {
    return {
      jsonrpc: '2.0', id,
      error: { code: -32602, message: `agent.exec: unknown model "${req.model ?? 'claude'}"` },
    }
  }

  const { homedir } = await import('node:os')
  const toolEnv: NodeJS.ProcessEnv = {
    HOME: homedir(),
    PATH: config.toolPath ?? process.env.PATH ?? '/usr/local/bin:/usr/bin:/bin',
    TERM: 'dumb',
    // Non-interactive mode — no color output
    NO_COLOR: '1',
    ...(req.taskId ? { ORCA_TASK_ID: req.taskId } : {}),
    ...(req.worktreePath ? { ORCA_WORKTREE_PATH: req.worktreePath } : {}),
  }

  // Build CLI args for non-interactive (print) mode
  // Claude uses: --print <prompt> --output-format text
  const args: string[] = []
  if (req.model)        args.push('--model', req.model)
  args.push('--print', req.prompt, '--output-format', 'text')
  if (req.trustPreset && req.trustPreset !== 'standard') {
    args.push('--allowedTools', req.trustPreset === 'full' ? 'all' : 'none')
  }

  const timeoutMs = Math.min(req.timeoutMs ?? 300_000, 600_000)
  log.info(`agent.exec: model=${req.model ?? 'claude'} cwd=${req.worktreePath} stepId=${req.stepId ?? '-'}`)

  const start = Date.now()
  const { spawn: nodeSpawn } = await import('node:child_process')

  const result = await new Promise<{
    stdout: string; stderr: string; exitCode: number | null; timedOut: boolean
  }>((resolve) => {
    let stdout = '', stderr = '', timedOut = false, settled = false

    const finish = (r: typeof result): void => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      resolve(r)
    }

    const child = nodeSpawn(spec.binary, args, {
      cwd:   req.worktreePath,
      env:   toolEnv,
      stdio: ['pipe', 'pipe', 'pipe'],
    })

    const timer = setTimeout(() => {
      timedOut = true
      try { child.kill('SIGKILL') } catch { /* best effort */ }
      finish({ stdout, stderr, exitCode: null, timedOut: true })
    }, timeoutMs)

    child.stdout?.on('data', (d: Buffer) => { stdout += d.toString('utf8') })
    child.stderr?.on('data', (d: Buffer) => { stderr += d.toString('utf8') })
    child.on('error',  (err) => { finish({ stdout, stderr: err.message, exitCode: null, timedOut }) })
    child.on('close',  (code) => { finish({ stdout, stderr, exitCode: code, timedOut }) })

    child.stdin?.end()
  })

  const latencyMs = Date.now() - start
  log.info(`agent.exec: done exitCode=${result.exitCode} latency=${latencyMs}ms timedOut=${result.timedOut}`)

  return {
    jsonrpc: '2.0', id,
    result: {
      stdout:    result.stdout,
      stderr:    result.stderr,
      exitCode:  result.exitCode,
      latencyMs,
      timedOut:  result.timedOut,
      stepId:    req.stepId,
    },
  }
}
