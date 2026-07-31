# TASK-03: Extend Git Handler — git.pr.create + git.worktree.*

> ✅ **STATUS: DONE** — Completed 2026-07-30T17:44

**Phase:** 2
**File:** `src/relay/agent-git-handler.ts`
**Operation:** EXTEND (append + modify import)
**CR:** [CR-AG-10](../solutions/CR-AG-10-git-handler.md)
**TDD:** TDD-AG-10
**Depends on:** Không có dependency (standalone)
**Blocked by:** Không

---

## Mục tiêu

Thêm vào `agent-git-handler.ts` (240 lines hiện tại):
1. **`handleGitPrCreate()`** — tạo PR qua `gh pr create` (not git CLI)
2. **`handleGitWorktreeList()`** — list worktrees (REUSE `handleGitExec`)
3. **`handleGitWorktreeAdd()`** — add worktree (REUSE `handleGitExec`)
4. **`handleGitWorktreeRemove()`** — remove worktree (REUSE `handleGitExec`)

---

## Context đọc trước

```
src/relay/agent-git-handler.ts  (240 lines)
  - Line 15: import { spawn }   ← cần thêm execFile
  - Line 54: SHELL_METACHARACTERS   ← sẽ reuse trong validatePrArgs
  - Line 68-88: validateGitArgs()   ← pattern tham khảo
  - Line 92-150: handleGitExec()   ← sẽ REUSE cho worktree functions
  - Line 235-239: sendFrame() helper
  - Line 240: EOF — APPEND vào đây
```

**Import line 15 hiện tại:**
```typescript
import { spawn } from 'node:child_process'
```

**`SHELL_METACHARACTERS` line 54:**
```typescript
const SHELL_METACHARACTERS = /[&|;$`<>\\!]/
```

---

## Thay đổi cần thực hiện

### Edit 1 — Thêm `execFile` + `promisify` vào import (line 15)

```diff
-import { spawn } from 'node:child_process'
+import { spawn, execFile } from 'node:child_process'
+import { promisify } from 'node:util'
+const execFileAsync = promisify(execFile)
```

### Edit 2 — APPEND toàn bộ đoạn sau vào **cuối file** (sau line 240)

```typescript
// ─── git.pr.create (via gh CLI) ──────────────────────────────────────────────

/**
 * Create a GitHub Pull Request via `gh pr create`.
 * Uses GH_CONFIG_DIR per userId for auth isolation.
 * Requires: `gh` binary in PATH + `gh auth login` configured for this user.
 */
export async function handleGitPrCreate(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const title  = typeof params.title  === 'string' ? params.title.trim()  : ''
  const body   = typeof params.body   === 'string' ? params.body           : ''
  const base   = typeof params.base   === 'string' ? params.base.trim()   : 'main'
  const draft  = params.draft === true
  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId          : ''

  if (!title) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }

  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
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

  try {
    const { stdout, stderr } = await execFileAsync('gh', ghArgs, { cwd, env, timeout: 30_000 })
    const url = stdout.trim()
    log.info(`git.pr.create: PR created → ${url}`)
    return { jsonrpc: '2.0', id, result: { url, stdout, stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`git.pr.create failed: ${msg}`)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── git.worktree.list ────────────────────────────────────────────────────────

/**
 * List git worktrees — REUSES handleGitExec with args ['worktree', 'list', '--porcelain']
 * 'worktree' is already in ALLOWED_GIT_SUBCOMMANDS.
 */
export async function handleGitWorktreeList(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  return handleGitExec(id, {
    args:    ['worktree', 'list', '--porcelain'],
    cwd:     params.cwd,
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

  return handleGitExec(id, {
    args:    ['worktree', 'add', path, branch],
    cwd:     params.cwd,
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

---

## Verify

```bash
# TypeScript compile
npx tsc --noEmit -p config/tsconfig.node.json

# Check exports
grep -n "^export async function" src/relay/agent-git-handler.ts
# Expected exports:
# handleGitExec
# handleGitExecStream
# handleGitPrCreate       ← NEW
# handleGitWorktreeList   ← NEW
# handleGitWorktreeAdd    ← NEW
# handleGitWorktreeRemove ← NEW
```

---

## Done criteria

- [ ] `execFile` và `promisify` import được thêm vào
- [ ] `handleGitPrCreate()` — validate title/base metachar, sử dụng `execFileAsync('gh', ...)`
- [ ] `handleGitWorktreeList()` — gọi `handleGitExec` với args `['worktree', 'list', '--porcelain']`
- [ ] `handleGitWorktreeAdd()` — gọi `handleGitExec` với `['worktree', 'add', path, branch]`
- [ ] `handleGitWorktreeRemove()` — gọi `handleGitExec` với `['worktree', 'remove', path]`
- [ ] TypeScript compile không lỗi
