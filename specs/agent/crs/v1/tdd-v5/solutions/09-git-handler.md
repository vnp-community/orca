# SOL-09: git-handler.ts — Whitelisted Git (v5.0)

**TDD Ref:** TDD-AG-10  
**File:** `src/relay/git-handler.ts` [NEW]  
**Mức độ:** 🟡 Trung bình  
**Thời gian ước tính:** 2h

---

## Full Implementation

```typescript
// src/relay/git-handler.ts

import { spawn } from 'node:child_process'
import type WebSocket from 'ws'
import { encodeDataFrame } from './agent-wire'
import type { WireState } from './agent-wire'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ─── Whitelist ────────────────────────────────────────────────────────────────

const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag', 'show', 'rev-parse',
  'config',   // read-only config queries
  'describe', // version tagging
  'shortlog',
])

// Characters that would allow shell injection or command chaining
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
    throw new GitValidationError('GIT_NO_SUBCOMMAND', 'git args must not be empty')
  }
  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new GitValidationError(
      'GIT_DISALLOWED_SUBCOMMAND',
      `git subcommand not allowed: ${args[0]}`
    )
  }
  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new GitValidationError(
        'GIT_SHELL_METACHARACTER_IN_ARG',
        `Unsafe character in git arg: ${arg}`
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
  const cwd = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const timeout = Math.min(typeof params.timeout === 'number' ? params.timeout : 30_000, 60_000)

  try {
    validateGitArgs(rawArgs)
  } catch (err: unknown) {
    if (err instanceof GitValidationError) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: err.message } }
    }
    throw err
  }

  return new Promise<object>((resolve) => {
    const child = spawn('git', rawArgs, {
      cwd,
      env: config.toolEnv,
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false,
    })

    const stdout: string[] = []
    const stderr: string[] = []

    const timer = setTimeout(() => {
      child.kill('SIGTERM')
      resolve({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: 'git.exec timeout' } })
    }, timeout)

    child.stdout?.on('data', (c: Buffer) => stdout.push(c.toString()))
    child.stderr?.on('data', (c: Buffer) => stderr.push(c.toString()))
    child.on('close', (code) => {
      clearTimeout(timer)
      log.info(`git.exec: ${rawArgs.join(' ')} exitCode=${code}`)
      resolve({
        jsonrpc: '2.0', id,
        result: { stdout: stdout.join(''), stderr: stderr.join(''), exitCode: code ?? 0 },
      })
    })
    child.on('error', (err) => {
      clearTimeout(timer)
      resolve({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: err.message } })
    })

    child.stdin?.end()
  })
}

// ─── git.execStream ───────────────────────────────────────────────────────────

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
      const frame = { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: err.message } }
      if (ws.readyState === 1) ws.send(encodeDataFrame(wireState, JSON.stringify(frame)))
      return
    }
    throw err
  }

  const child = spawn('git', rawArgs, {
    cwd,
    env: config.toolEnv,
    stdio: ['pipe', 'pipe', 'pipe'],
    shell: false,
  })

  function sendChunk(line: string, source?: 'stderr'): void {
    if (ws.readyState !== 1 /* OPEN */) return
    const frame = { jsonrpc: '2.0', id, result: { type: 'stream.chunk', line, ...(source && { source }) } }
    ws.send(encodeDataFrame(wireState, JSON.stringify(frame)))
  }

  function sendEnd(exitCode: number): void {
    if (ws.readyState !== 1) return
    const frame = { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode } }
    ws.send(encodeDataFrame(wireState, JSON.stringify(frame)))
  }

  child.stdout?.on('data', (chunk: Buffer) => {
    chunk.toString('utf8').split('\n').filter(l => l.length > 0).forEach(l => sendChunk(l))
  })

  // git progress (fetch/push) goes to stderr
  child.stderr?.on('data', (chunk: Buffer) => {
    chunk.toString('utf8').split('\n').filter(l => l.length > 0).forEach(l => sendChunk(l, 'stderr'))
  })

  child.on('close', (code) => {
    log.info(`git.execStream: ${rawArgs.join(' ')} exitCode=${code}`)
    sendEnd(code ?? 0)
  })

  child.on('error', (err) => {
    if (ws.readyState === 1) {
      const frame = { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: err.message } }
      ws.send(encodeDataFrame(wireState, JSON.stringify(frame)))
    }
  })

  child.stdin?.end()
}
```

---

## Validation Test Cases

```typescript
// src/relay/__tests__/git-handler.test.ts

describe('validateGitArgs', () => {
  it('allows "status"', () => expect(() => validateGitArgs(['status'])).not.toThrow())
  it('allows "log", "--oneline", "-10"', () => expect(() => validateGitArgs(['log', '--oneline', '-10'])).not.toThrow())
  it('throws GIT_NO_SUBCOMMAND on empty', () => { ... })
  it('throws GIT_DISALLOWED_SUBCOMMAND on "bisect"', () => { ... })
  it('throws GIT_DISALLOWED_SUBCOMMAND on "clean"', () => { ... })
  it('throws GIT_SHELL_METACHARACTER_IN_ARG on "|"', () => { ... })
  it('throws on "&"', () => { ... })
  it('throws on ";"', () => { ... })
  it('throws on "$HOME"', () => { ... })
  it('throws on "`rm -rf`"', () => { ... })
})
```

---

## Definition of Done

- [x] `src/relay/git-handler.ts` created
- [x] `tsc` passes
- [x] `validateGitArgs` covers all whitelist + metachar tests (≥ 15)
- [x] `git.exec`: mock spawn, verify args passed correctly
- [x] `git.execStream`: mock ws, verify stream.chunk + stream.end frames
- [x] Invalid args → error frame (no crash)
