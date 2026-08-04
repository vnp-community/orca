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

import { spawn, execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
import type WebSocket from 'ws'
import { encodeDataFrame } from './agent-wire'
import type { WireState } from './agent-wire'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'
import { Tracers } from '../shared/trace/tracers'

const gitTracer = createTracer('agent:git')

// ─── Trace propagation helper ───────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested at params._trace.id (CR-TRACE-000 §3.3),
// not a flat params.traceId — avoids colliding with JSON-RPC 2.0's own `id` field.
function resumeFrom(params: Record<string, unknown>): { id: string } | undefined {
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
  'shortlog',
])

/**
 * Regex of characters that could enable shell injection or command chaining.
 * Checked against every argument (not just the subcommand).
 */
const SHELL_METACHARACTERS = /[&|;$`<>\\!]/

// ─── Validation ───────────────────────────────────────────────────────────────

export class GitValidationError extends Error {
  constructor(
    public readonly code: 'GIT_NO_SUBCOMMAND' | 'GIT_DISALLOWED_SUBCOMMAND' | 'GIT_SHELL_METACHARACTER_IN_ARG',
    message: string
  ) {
    super(message)
    this.name = 'GitValidationError'
  }
}

export function validateGitArgs(args: string[]): void {
  if (args.length === 0) {
    throw new GitValidationError('GIT_NO_SUBCOMMAND', 'git args must not be empty — provide a subcommand')
  }

  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new GitValidationError(
      'GIT_DISALLOWED_SUBCOMMAND',
      `git subcommand not allowed: "${args[0]}". Allowed: ${[...ALLOWED_GIT_SUBCOMMANDS].sort().join(', ')}`
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
  log: AgentLogger
): Promise<object> {
  const rawArgs = Array.isArray(params.args) ? params.args.map(String) : []
  const cwd     = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const timeout = Math.min(typeof params.timeout === 'number' ? params.timeout : 30_000, 60_000)
  const argsStr = rawArgs.join(' ').slice(0, 80)
  const span    = gitTracer.start({ method: 'git.exec', cmd: argsStr, cwd }, resumeFrom(params))

  try {
    validateGitArgs(rawArgs)
  } catch (err: unknown) {
    if (err instanceof GitValidationError) {
      span.fail(`validation: ${err.message}`, { cmd: argsStr })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: err.message } }
    }
    throw err
  }

  return new Promise<object>((resolve) => {
    const child = spawn('git', rawArgs, {
      cwd,
      env:   config.toolEnv,
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false,  // mandatory: no shell injection
    })

    const stdout: string[] = []
    const stderr: string[] = []

    const timer = setTimeout(() => {
      child.kill('SIGTERM')
      span.fail(`timeout after ${timeout}ms`, { cmd: argsStr })
      resolve({
        jsonrpc: '2.0', id,
        error: { code: AgentErrorCode.ServerError, message: `git.exec timeout after ${timeout}ms` },
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
        jsonrpc: '2.0', id,
        result: { stdout: stdout.join(''), stderr: stderr.join(''), exitCode },
      })
    })

    child.on('error', (err) => {
      clearTimeout(timer)
      span.fail(err, { cmd: argsStr })
      resolve({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: err.message } })
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
  const cwd     = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir

  try {
    validateGitArgs(rawArgs)
  } catch (err: unknown) {
    if (err instanceof GitValidationError) {
      sendFrame(ws, wireState, { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: err.message } })
      return
    }
    throw err
  }

  const child = spawn('git', rawArgs, {
    cwd,
    env:   config.toolEnv,
    stdio: ['pipe', 'pipe', 'pipe'],
    shell: false,
  })

  function sendChunk(line: string, source?: 'stderr'): void {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      result: { type: 'stream.chunk', line, ...(source ? { source } : {}) },
    })
  }

  function sendEnd(exitCode: number): void {
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode } })
  }

  // Stream stdout lines (main output)
  child.stdout?.on('data', (chunk: Buffer) => {
    chunk.toString('utf8').split('\n')
      .filter(l => l.length > 0)
      .forEach(l => sendChunk(l))
  })

  // Stream stderr lines — git push/fetch progress goes to stderr
  child.stderr?.on('data', (chunk: Buffer) => {
    chunk.toString('utf8').split('\n')
      .filter(l => l.length > 0)
      .forEach(l => sendChunk(l, 'stderr'))
  })

  child.on('close', (code) => {
    const exitCode = code ?? 0
    log.info(`git.execStream: ${rawArgs.join(' ')} → exitCode=${exitCode}`)
    sendEnd(exitCode)
  })

  child.on('error', (err) => {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.ServerError, message: err.message },
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

// ─── git.pr.create (via gh CLI) ──────────────────────────────────────────────

/**
 * Create a GitHub Pull Request via `gh pr create`.
 * Uses GH_CONFIG_DIR per userId for auth isolation.
 * Requires: `gh` binary in PATH + `gh auth login` configured for this user.
 */
export async function handleGitPrCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const title  = typeof params.title  === 'string' ? params.title.trim()  : ''
  const body   = typeof params.body   === 'string' ? params.body           : ''
  const base   = typeof params.base   === 'string' ? params.base.trim()   : 'main'
  const draft  = params.draft === true
  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId          : ''

  const span = gitTracer.start({ method: 'git.pr.create', title: title.slice(0, 40), base })

  if (!title) {
    span.fail('missing title', { method: 'git.pr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
    span.fail('unsafe characters in params', { method: 'git.pr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in PR params' } }
  }

  const ghArgs: string[] = ['pr', 'create', '--title', title, '--body', body, '--base', base]
  if (draft) ghArgs.push('--draft')

  const { homedir } = await import('node:os')
  const env: NodeJS.ProcessEnv = {
    ...config.toolEnv,
    ...(userId ? { GH_CONFIG_DIR: `${homedir()}/.config/gh/${userId}/` } : {}),
    GH_NO_UPDATE_NOTIFIER: '1',
    GH_PROMPT_DISABLED:    '1',
  }

  span.step('ghExec', { base })
  try {
    const { stdout, stderr } = await execFileAsync('gh', ghArgs, { cwd, env, timeout: 30_000 })
    const url = stdout.trim()
    log.info(`git.pr.create: PR created → ${url}`)
    span.ok({ url })
    return { jsonrpc: '2.0', id, result: { url, stdout, stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`git.pr.create failed: ${msg}`)
    span.fail(err, { method: 'git.pr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── git.worktree.list ────────────────────────────────────────────────────────

/**
 * List git worktrees — returns structured WorktreeInfo[] (WT-Issue-4).
 * Parses `git worktree list --porcelain` output for typed response.
 */
export async function handleGitWorktreeList(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const cwd = typeof params.cwd === 'string' ? params.cwd : config.workDir
  try {
    const { execFile } = await import('node:child_process')
    const { promisify } = await import('node:util')
    const { parseWorktreePorcelain } = await import('./git-handler')
    const execAsync = promisify(execFile)
    const { stdout } = await execAsync('git', ['worktree', 'list', '--porcelain'], { cwd, timeout: 10_000 })
    const worktrees = parseWorktreePorcelain(stdout)
    log.info(`git.worktree.list: cwd=${cwd} count=${worktrees.length}`)
    return { jsonrpc: '2.0', id, result: { worktrees } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `git.worktree.list failed: ${msg}` } }
  }
}

// ─── git.worktree.add ─────────────────────────────────────────────────────────

export async function handleGitWorktreeAdd(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const worktreePath = typeof params.path   === 'string' ? params.path.trim()   : ''
  const branch       = typeof params.branch === 'string' ? params.branch.trim() : ''
  const createBranch = params.createBranch === true
  const cwd          = typeof params.cwd    === 'string' ? params.cwd           : config.workDir

  const span = Tracers.worktreeCreate.start({ path: worktreePath, branch, cwd }, resumeFrom(params))

  if (!worktreePath || !branch) {
    span.fail('missing required params', { path: worktreePath, branch })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required params: path, branch' } }
  }
  if (SHELL_METACHARACTERS.test(worktreePath) || SHELL_METACHARACTERS.test(branch)) {
    span.fail('unsafe characters in params', { path: worktreePath, branch })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in worktree params' } }
  }

  // WT-Issue-1: Security validation — prevent path traversal
  try {
    const { validateWorktreePath } = await import('./git-handler')
    span.step('validate-path', { path: worktreePath })
    validateWorktreePath(['worktree', 'add', worktreePath], cwd)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg, { path: worktreePath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: msg } }
  }

  const args = createBranch
    ? ['worktree', 'add', '-b', branch, worktreePath]
    : ['worktree', 'add', worktreePath, branch]

  span.step('git-worktree-add-exec', { branch })
  const result = await handleGitExec(
    id,
    { args, cwd: params.cwd, timeout: 15_000, _trace: { id: span.id } },
    config,
    log
  )
  if (result && typeof result === 'object' && 'error' in result) {
    span.fail((result as { error: { message: string } }).error.message, { path: worktreePath })
  } else {
    span.ok({ path: worktreePath, branch })
  }
  return result
}

// ─── git.worktree.remove ──────────────────────────────────────────────────────

export async function handleGitWorktreeRemove(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const path  = typeof params.path  === 'string' ? params.path.trim() : ''
  const force = params.force === true

  const span = Tracers.worktreeDelete.start({ path, force }, resumeFrom(params))

  if (!path) {
    span.fail('missing required param: path')
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }
  if (SHELL_METACHARACTERS.test(path)) {
    span.fail('unsafe characters in path', { path })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in path' } }
  }

  const args = ['worktree', 'remove', path]
  if (force) args.push('--force')

  span.step('git-worktree-remove-exec', { force })
  const result = await handleGitExec(
    id,
    { args, cwd: params.cwd, timeout: 15_000, _trace: { id: span.id } },
    config,
    log
  )
  if (result && typeof result === 'object' && 'error' in result) {
    span.fail((result as { error: { message: string } }).error.message, { path })
  } else {
    span.ok({ path, force })
  }
  return result
}
