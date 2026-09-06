// src/relay/agent-git-handler.ts
// Whitelisted git command execution for Orca Dev Agent v5.0.
// NOTE: This is distinct from git-handler.ts (GitHandler class used by relay daemon).
//       This module is for agent-side RPC handlers: git.exec and git.execStream.
//
// Security model:
//   - ALLOWED_GIT_SUBCOMMANDS: only safe read/write ops (no bisect, gc, clean, etc.)
//   - SHELL_METACHARACTERS: reject args containing shell special chars
//   - spawn() with shell: false — prevents shell injection
//
// Two modes:
//   git.exec — captures stdout/stderr, returns single JSON-RPC response
//   git.execStream — streams output lines as they arrive (good for push/fetch)

import { spawn } from 'node:child_process'
import type WebSocket from 'ws'
import { encodeDataFrame } from 'orca-dev-agent-transport'
import type { WireState } from 'orca-dev-agent-transport'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'
import { assertNoGitInjectionFlags } from './agent-git-exec-validator'
import { getConnectionGitIdentity, buildGitIdentityEnv } from './git-identity-registry'

const gitTracer = createTracer('agent:git')

// ─── Trace propagation helper ───────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested at params._trace.id (CR-TRACE-000 §3.3),
// not a flat params.traceId — avoids colliding with JSON-RPC 2.0's own `id` field.
export function resumeFrom(params: Record<string, unknown>): { id: string } | undefined {
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}

// ─── Whitelist ────────────────────────────────────────────────────────────────

const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status',
  'diff',
  'add',
  'restore',
  'commit',
  'push',
  'pull',
  'fetch',
  'branch',
  'checkout',
  'merge',
  'rebase',
  'stash',
  'log',
  'worktree',
  'remote',
  'tag',
  'show',
  'rev-parse',
  'config',
  'describe',
  'shortlog'
])

/**
 * Regex of characters that could enable shell injection or command chaining.
 * Checked against every argument (not just the subcommand).
 */
export const SHELL_METACHARACTERS = /[&|;$`<>\\!]/

// ─── Validation ───────────────────────────────────────────────────────────────

export class GitValidationError extends Error {
  constructor(
    public readonly code:
      | 'GIT_NO_SUBCOMMAND'
      | 'GIT_DISALLOWED_SUBCOMMAND'
      | 'GIT_SHELL_METACHARACTER_IN_ARG',
    message: string
  ) {
    super(message)
    this.name = 'GitValidationError'
  }
}

export function validateGitArgs(args: string[]): void {
  if (args.length === 0) {
    throw new GitValidationError(
      'GIT_NO_SUBCOMMAND',
      'git args must not be empty — provide a subcommand'
    )
  }

  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new GitValidationError(
      'GIT_DISALLOWED_SUBCOMMAND',
      `git subcommand not allowed: "${args[0]}". Allowed: ${[...ALLOWED_GIT_SUBCOMMANDS].sort().join(', ')}`
    )
  }

  // Why: BUG-AG-HLD-003 — identity must come from preflight.setGitIdentity's
  // per-client registry, not a global config write that leaks to every
  // other client sharing this dev server agent.
  if (
    args[0] === 'config' &&
    (args.includes('--global') || args.includes('--system')) &&
    (args.includes('user.name') || args.includes('user.email'))
  ) {
    throw new GitValidationError(
      'GIT_SHELL_METACHARACTER_IN_ARG',
      'git config --global/--system user.name|user.email is not allowed via git.exec — use preflight.setGitIdentity'
    )
  }

  // Why: closes the git-native injection/RCE footguns the subcommand
  // allowlist + shell-metacharacter check don't cover on their own (a
  // `-c core.sshCommand=...` global flag, `--upload-pack=`/`--receive-pack=`,
  // unrestricted `git config` writes, ...) — see
  // agent-git-exec-validator.ts's header for the full rationale and
  // specs/agent/api/gaps-and-findings.md #4.
  try {
    assertNoGitInjectionFlags(args)
  } catch (err: unknown) {
    throw new GitValidationError(
      'GIT_DISALLOWED_SUBCOMMAND',
      err instanceof Error ? err.message : String(err)
    )
  }

  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new GitValidationError(
        'GIT_SHELL_METACHARACTER_IN_ARG',
        `Unsafe character in git argument: "${arg}"`
      )
    }
  }
}

// ─── git.exec ─────────────────────────────────────────────────────────────────

export async function handleGitExec(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger,
  ws?: WebSocket
): Promise<object> {
  const rawArgs = Array.isArray(params.args) ? params.args.map(String) : []
  const cwd = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const timeout = Math.min(typeof params.timeout === 'number' ? params.timeout : 30_000, 60_000)
  const argsStr = rawArgs.join(' ').slice(0, 80)
  const span = gitTracer.start({ method: 'git.exec', cmd: argsStr, cwd }, resumeFrom(params))

  try {
    validateGitArgs(rawArgs)
  } catch (err: unknown) {
    if (err instanceof GitValidationError) {
      span.fail(`validation: ${err.message}`, { cmd: argsStr })
      return {
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.InvalidParams, message: err.message }
      }
    }
    throw err
  }

  // Why: BUG-AG-HLD-003 parity for Part A — preflight.setGitIdentity stores
  // identity per-connection (git-identity-registry.ts), never global config.
  // Only applied for `commit` (the one subcommand that reads author/committer
  // identity) so every other subcommand's env is unchanged.
  const identityEnv =
    ws && rawArgs[0] === 'commit' ? buildGitIdentityEnv(getConnectionGitIdentity(ws)) : {}

  return new Promise<object>((resolve) => {
    const child = spawn('git', rawArgs, {
      cwd,
      env: { ...config.toolEnv, ...identityEnv },
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false // mandatory: no shell injection
    })

    const stdout: string[] = []
    const stderr: string[] = []

    const timer = setTimeout(() => {
      child.kill('SIGTERM')
      span.fail(`timeout after ${timeout}ms`, { cmd: argsStr })
      resolve({
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.ServerError, message: `git.exec timeout after ${timeout}ms` }
      })
    }, timeout)

    child.stdout?.on('data', (chunk: Buffer) => stdout.push(chunk.toString()))
    child.stderr?.on('data', (chunk: Buffer) => stderr.push(chunk.toString()))

    child.on('close', (code) => {
      clearTimeout(timer)
      const exitCode = code ?? 0
      log.info(`git.exec: ${rawArgs.join(' ')} → exitCode=${exitCode}`)
      const outLen = stdout.join('').length
      if (exitCode === 0) {
        span.ok({ cmd: argsStr, exitCode, outLen })
      } else {
        span.fail(`git exit ${exitCode}`, { cmd: argsStr, exitCode, outLen })
      }
      resolve({
        jsonrpc: '2.0',
        id,
        result: { stdout: stdout.join(''), stderr: stderr.join(''), exitCode }
      })
    })

    child.on('error', (err) => {
      clearTimeout(timer)
      span.fail(err, { cmd: argsStr })
      resolve({
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.ServerError, message: err.message }
      })
    })

    child.stdin?.end()
  })
}

// ─── git.execStream ───────────────────────────────────────────────────────────

/**
 * Stream git output line-by-line via WebSocket frames.
 *
 * Frame format (JSON-RPC result field):
 *   { type: 'stream.chunk', line: string }                    — stdout line
 *   { type: 'stream.chunk', line: string, source: 'stderr' }  — stderr line
 *   { type: 'stream.end',   exitCode: number }                 — process exited
 *
 * The initial 'stream.started' response is sent by the dispatcher before calling here.
 */
export async function handleGitExecStream(
  ws: WebSocket,
  wireState: WireState,
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<void> {
  const rawArgs = Array.isArray(params.args) ? params.args.map(String) : []
  const cwd = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir

  try {
    validateGitArgs(rawArgs)
  } catch (err: unknown) {
    if (err instanceof GitValidationError) {
      sendFrame(ws, wireState, {
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.InvalidParams, message: err.message }
      })
      return
    }
    throw err
  }

  // Why: BUG-AG-HLD-003 parity — see handleGitExec's identical comment.
  const identityEnv =
    rawArgs[0] === 'commit' ? buildGitIdentityEnv(getConnectionGitIdentity(ws)) : {}

  const child = spawn('git', rawArgs, {
    cwd,
    env: { ...config.toolEnv, ...identityEnv },
    stdio: ['pipe', 'pipe', 'pipe'],
    shell: false
  })

  function sendChunk(line: string, source?: 'stderr'): void {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      result: { type: 'stream.chunk', line, ...(source ? { source } : {}) }
    })
  }

  function sendEnd(exitCode: number): void {
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode } })
  }

  // Stream stdout lines (main output)
  child.stdout?.on('data', (chunk: Buffer) => {
    chunk
      .toString('utf8')
      .split('\n')
      .filter((l) => l.length > 0)
      .forEach((l) => sendChunk(l))
  })

  // Stream stderr lines — git push/fetch progress goes to stderr
  child.stderr?.on('data', (chunk: Buffer) => {
    chunk
      .toString('utf8')
      .split('\n')
      .filter((l) => l.length > 0)
      .forEach((l) => sendChunk(l, 'stderr'))
  })

  child.on('close', (code) => {
    const exitCode = code ?? 0
    log.info(`git.execStream: ${rawArgs.join(' ')} → exitCode=${exitCode}`)
    sendEnd(exitCode)
  })

  child.on('error', (err) => {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.ServerError, message: err.message }
    })
  })

  child.stdin?.end()
}

// ─── Helper ───────────────────────────────────────────────────────────────────

function sendFrame(ws: WebSocket, wireState: WireState, payload: object): void {
  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)))
  }
}

// ─── git.worktree.list/add/remove: see agent-git-worktree-handler.ts ──────────
// ─── git.baseRefDefault/searchRefs: see agent-git-base-ref-handler.ts ─────────
