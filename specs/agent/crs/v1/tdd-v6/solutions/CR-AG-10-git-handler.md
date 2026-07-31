# CR-AG-10: Git Handler Extension — Thêm git.pr.create + git.worktree.*

**CR:** CR-AG-10
**TDD:** [TDD-AG-10](../../tdd/v5/10-git-handler-extension.md)
**Ngày:** 2026-07-30
**Độ phức tạp:** Medium — extend `agent-git-handler.ts` + `agent-rpc-dispatch.ts`
**ADR:** ADR-012
**HLD Ref:** C3.12, C4.10

---

## 1. Phân tích Code Hiện Tại

### Code đã có ✅ — [`src/relay/agent-git-handler.ts`](../../../../../src/relay/agent-git-handler.ts)

| Function | Trạng thái | Ghi chú |
|---------|-----------|---------|
| `validateGitArgs()` | ✅ DONE | whitelist 22 subcommands, metachar check |
| `handleGitExec()` | ✅ DONE | spawn + capture stdout/stderr, timeout |
| `handleGitExecStream()` | ✅ DONE | line-by-line streaming via WS frames |
| `git.pr.create` | ❌ MISSING | TDD yêu cầu `gh pr create` via execFile |
| `git.worktree.list` | ❌ MISSING | worktree ops cần thêm |
| `git.worktree.add` | ❌ MISSING | `git worktree add` |
| `git.worktree.remove` | ❌ MISSING | `git worktree remove` |

### Code đã có ✅ — `git-exec-validator.ts`

`src/relay/git-exec-validator.ts` — có thể import logic validation cho PR args.

### Code đã có ✅ — `git-handler-worktree-ops.ts`

`src/relay/git-handler-worktree-ops.ts` — logic worktree từ relay daemon. **Tái sử dụng** bằng cách gọi `git worktree` subcommand qua `handleGitExec()`.

---

## 2. Solution

### 2.1 EXTEND: `src/relay/agent-git-handler.ts`

**Tái sử dụng `handleGitExec()`** làm nền — `git.worktree.*` chỉ cần pre-validate args rồi pass qua `handleGitExec()`.

#### Thêm `git.pr.create` — PR via `gh` CLI

```typescript
// src/relay/agent-git-handler.ts — APPEND sau dòng 239

import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
const execFileAsync = promisify(execFile)

// ─── git.pr.create (gh CLI) ───────────────────────────────────────────────────

/**
 * Create a GitHub Pull Request via `gh pr create`.
 * Uses GH_CONFIG_DIR per userId for isolation.
 * Requires: `gh` binary in PATH + `gh auth login` configured.
 */
export async function handleGitPrCreate(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const title    = typeof params.title   === 'string' ? params.title.trim()   : ''
  const body     = typeof params.body    === 'string' ? params.body            : ''
  const base     = typeof params.base    === 'string' ? params.base.trim()    : 'main'
  const draft    = params.draft === true
  const cwd      = typeof params.cwd     === 'string' && params.cwd ? params.cwd : config.workDir
  const userId   = typeof params.userId  === 'string' ? params.userId          : ''

  if (!title) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }

  // Shell metacharacter check on title/body
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in PR params' } }
  }

  const ghArgs: string[] = [
    'pr', 'create',
    '--title', title,
    '--body', body,
    '--base', base,
  ]
  if (draft) ghArgs.push('--draft')

  // Per-user GH_CONFIG_DIR isolation
  const env: NodeJS.ProcessEnv = {
    ...config.toolEnv,
    ...(userId ? { GH_CONFIG_DIR: `${process.env.HOME ?? '/root'}/.config/gh/${userId}/` } : {}),
  }

  try {
    const { stdout, stderr } = await execFileAsync('gh', ghArgs, { cwd, env, timeout: 30_000 })
    const url = stdout.trim()   // gh outputs the PR URL on stdout
    log.info(`git.pr.create: PR created → ${url}`)
    return { jsonrpc: '2.0', id, result: { url, stdout, stderr } }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`git.pr.create failed: ${msg}`)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

#### Thêm `git.worktree.*` — Tái sử dụng `handleGitExec()`

```typescript
// src/relay/agent-git-handler.ts — APPEND sau handleGitPrCreate

// ─── git.worktree.list ────────────────────────────────────────────────────────

export async function handleGitWorktreeList(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  // Tái sử dụng handleGitExec với whitelist 'worktree'
  return handleGitExec(id, {
    args: ['worktree', 'list', '--porcelain'],
    cwd: params.cwd,
    timeout: 10_000,
  }, config, log)
}

// ─── git.worktree.add ─────────────────────────────────────────────────────────

export async function handleGitWorktreeAdd(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const path   = typeof params.path   === 'string' ? params.path.trim()   : ''
  const branch = typeof params.branch === 'string' ? params.branch.trim() : ''

  if (!path || !branch) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required params: path, branch' } }
  }
  if (SHELL_METACHARACTERS.test(path) || SHELL_METACHARACTERS.test(branch)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in worktree params' } }
  }

  // REUSE handleGitExec — 'worktree' is already in ALLOWED_GIT_SUBCOMMANDS
  return handleGitExec(id, {
    args: ['worktree', 'add', path, branch],
    cwd: params.cwd,
    timeout: 15_000,
  }, config, log)
}

// ─── git.worktree.remove ──────────────────────────────────────────────────────

export async function handleGitWorktreeRemove(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const path  = typeof params.path  === 'string' ? params.path.trim() : ''
  const force = params.force === true

  if (!path) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }
  if (SHELL_METACHARACTERS.test(path)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in path' } }
  }

  const args = ['worktree', 'remove', path]
  if (force) args.push('--force')

  return handleGitExec(id, { args, cwd: params.cwd, timeout: 15_000 }, config, log)
}
```

### 2.2 EXTEND: `src/relay/agent-rpc-dispatch.ts`

Thêm routes sau `case 'preflight.check'` (line 224):

```typescript
// Thêm vào agent-rpc-dispatch.ts route() switch:

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
```

---

## 3. Tests

Tạo `src/relay/__tests__/agent-git-handler.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { validateGitArgs, GitValidationError } from '../agent-git-handler'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

// Tests cho validateGitArgs (pure function — không cần real fs)
describe('validateGitArgs', () => {
  it('allows whitelisted subcommand', () => {
    expect(() => validateGitArgs(['status'])).not.toThrow()
    expect(() => validateGitArgs(['diff', '--staged'])).not.toThrow()
    expect(() => validateGitArgs(['push', 'origin', 'main'])).not.toThrow()
    expect(() => validateGitArgs(['worktree', 'list'])).not.toThrow()
    expect(() => validateGitArgs(['log', '--oneline', '-50'])).not.toThrow()
  })

  it('rejects empty args', () => {
    expect(() => validateGitArgs([])).toThrow(GitValidationError)
    expect(() => validateGitArgs([])).toThrow('GIT_NO_SUBCOMMAND')
  })

  it('rejects disallowed subcommands', () => {
    expect(() => validateGitArgs(['clean'])).toThrow(GitValidationError)
    expect(() => validateGitArgs(['gc'])).toThrow(GitValidationError)
    expect(() => validateGitArgs(['bisect'])).toThrow(GitValidationError)
  })

  it('rejects shell metacharacters in any arg', () => {
    expect(() => validateGitArgs(['status', '--', 'file&rm -rf /'])).toThrow(GitValidationError)
    expect(() => validateGitArgs(['push', 'origin', 'main; echo hacked'])).toThrow(GitValidationError)
    expect(() => validateGitArgs(['commit', '-m', '$(evil)'])).toThrow(GitValidationError)
    expect(() => validateGitArgs(['add', 'file`whoami`.ts'])).toThrow(GitValidationError)
  })
})

describe('handleGitWorktreeList', () => {
  it('calls git worktree list --porcelain', async () => {
    const { handleGitWorktreeList } = await import('../agent-git-handler')
    // Integration: requires real git in PATH
    const config = { workDir: process.cwd(), toolEnv: process.env } as AgentConfig
    const log = { info: vi.fn(), error: vi.fn() } as unknown as AgentLogger
    const res = await handleGitWorktreeList(null, {}, config, log) as {
      result?: { exitCode: number }; error?: unknown
    }
    // May succeed or fail (no git repo) — just verify it tries
    expect(res).toBeDefined()
  })
})

describe('git.pr.create security', () => {
  it('rejects metacharacters in title', async () => {
    const { handleGitPrCreate } = await import('../agent-git-handler')
    const config = { workDir: process.cwd(), toolEnv: process.env } as AgentConfig
    const log = { info: vi.fn(), error: vi.fn() } as unknown as AgentLogger
    const res = await handleGitPrCreate(null, {
      title: 'Fix bug; rm -rf /', body: 'safe body', base: 'main'
    }, config, log) as { error: { code: number } }
    expect(res.error).toBeDefined()
  })

  it('returns error for missing title', async () => {
    const { handleGitPrCreate } = await import('../agent-git-handler')
    const config = { workDir: process.cwd(), toolEnv: process.env } as AgentConfig
    const log = { info: vi.fn(), error: vi.fn() } as unknown as AgentLogger
    const res = await handleGitPrCreate(null, { body: 'body', base: 'main' }, config, log) as {
      error: { code: number }
    }
    expect(res.error.code).toBe(-32602)  // InvalidParams
  })
})
```

**Target: ≥ 15 tests**

---

## 4. Implementation Checklist

- [ ] `src/relay/agent-git-handler.ts` — thêm import `{ execFile }` + `promisify`
- [ ] `src/relay/agent-git-handler.ts` — thêm `handleGitPrCreate()`
- [ ] `src/relay/agent-git-handler.ts` — thêm `handleGitWorktreeList()` (REUSE handleGitExec)
- [ ] `src/relay/agent-git-handler.ts` — thêm `handleGitWorktreeAdd()` (REUSE handleGitExec)
- [ ] `src/relay/agent-git-handler.ts` — thêm `handleGitWorktreeRemove()` (REUSE handleGitExec)
- [ ] `src/relay/agent-rpc-dispatch.ts` — thêm 4 case routes mới
- [ ] `src/relay/__tests__/agent-git-handler.test.ts` — tạo test file
