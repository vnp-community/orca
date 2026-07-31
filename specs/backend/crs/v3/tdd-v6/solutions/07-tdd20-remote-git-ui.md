# Solution: TDD-20 — Remote Git UI

**TDD Ref:** [20-remote-git-ui.md](../../../../../tdd/v5/20-remote-git-ui.md)  
**Status:** ✅ **FULLY COMPLETE** — git-remote-handler-v6.ts + index + rpc đã tạo (52 tests PASS)  
**Tái sử dụng:** 70% (nhiều git logic đã có ở relay)

---

## 1. Code Đã Tồn Tại — Tái sử dụng

### Existing Relay Git Code ✅

| File | Size | Tái sử dụng |
|------|------|------------|
| `src/relay/git-handler.ts` | 53.5KB | ✅ Local git operations — extend cho remote |
| `src/relay/git-exec-validator.ts` | 6.6KB | ✅ ALLOWED_GIT_SUBCOMMANDS + validateGitArgs() |
| `src/relay/git-handler-ops.ts` | 9.7KB | ✅ commit, push, pull, fetch operations |
| `src/relay/git-handler-status-ops.ts` | 8.4KB | ✅ status parsing |
| `src/relay/git-handler-worktree-ops.ts` | 7.4KB | ✅ worktree add/remove |
| `src/relay/git-response-stream.ts` | 6.3KB | ✅ streaming git output |
| `src/relay/ai-provider-handler.ts` | 3.4KB | ✅ credential read pattern |

### Existing Tests (Relay Git) ✅

- `src/relay/git-handler.test.ts` (104KB) — comprehensive local git tests
- `src/relay/git-exec-validator.test.ts` (7.9KB) — validation rules
- `src/relay/git-handler-status-ops.test.ts` (9.3KB) — status parsing

---

## 2. ✅ Đã Thực Thi (Delivered — 2026-07-30T23:43 ICT)

### 2.1 `src/relay/git-remote-handler-v6.ts` ✅ Đã tạo

> **❌ Không chỉnh `git-remote-handler.ts`** (93 lines hiện tại)  
> **❌ Không chỉnh `git-handler.ts`** (53.5KB)  
> **✅ Tạo file mỚi hoàn toàn:** `git-remote-handler-v6.ts` — sử dụng `gitRemoteHandlers['git.exec']` từ file cũ

**Strategy:** Re-use `git-remote-handler.ts` exports, không fork logic.

```typescript
// src/relay/git-remote-handler-v6.ts  [NEW FILE]
// Import git.exec từ file cũ (không copy, chỉ re-use):
import { gitRemoteHandlers } from './git-remote-handler'

export const gitRemoteHandlersV6 = {
  // Kế thừa tất cả từ v5:
  ...gitRemoteHandlers,

  // MỚI: high-level git commands
  'git.status': async (params: { cwd: string; worktreePath?: string }) => {
    const raw = await gitRemoteHandlers['git.exec']({
      cwd: params.worktreePath ?? params.cwd,
      args: ['status', '--porcelain=v2', '--branch'],
    })
    return { raw: raw.stdout }
  },

  'git.diff': async (params: { cwd: string; staged?: boolean; file?: string }) => {
    const args = ['diff']
    if (params.staged) args.push('--staged')
    if (params.file) args.push('--', params.file)
    return gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
  },

  'git.add': async (params: { cwd: string; files: string[] }) => {
    await gitRemoteHandlers['git.exec']({ cwd: params.cwd, args: ['add', '--', ...params.files] })
    return { ok: true }
  },

  'git.restore': async (params: { cwd: string; files: string[]; staged?: boolean }) => {
    const args = ['restore']
    if (params.staged) args.push('--staged')
    args.push('--', ...params.files)
    await gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
    return { ok: true }
  },

  'git.commit': async (params: { cwd: string; message: string }) => {
    const result = await gitRemoteHandlers['git.exec']({
      cwd: params.cwd, args: ['commit', '-m', params.message],
    })
    return { ok: true, output: result.stdout }
  },

  'git.push': async (params: { cwd: string; remote?: string; branch?: string; force?: boolean }) => {
    const args = ['push', params.remote ?? 'origin', params.branch ?? 'HEAD']
    if (params.force) args.push('--force-with-lease')
    return gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
  },

  'git.pull': async (params: { cwd: string; remote?: string; branch?: string; rebase?: boolean }) => {
    const args = ['pull', params.remote ?? 'origin']
    if (params.branch) args.push(params.branch)
    if (params.rebase) args.push('--rebase')
    return gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
  },

  'git.branch.list': async (params: { cwd: string; remote?: boolean }) => {
    const args = ['branch', '--format=%(refname:short)']
    if (params.remote) args.push('-r')
    const result = await gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
    return { branches: result.stdout.trim().split('\n').filter(Boolean) }
  },

  'git.checkout': async (params: { cwd: string; branch: string; create?: boolean }) => {
    const args = ['checkout']
    if (params.create) args.push('-b')
    args.push(params.branch)
    await gitRemoteHandlers['git.exec']({ cwd: params.cwd, args })
    return { ok: true }
  },

  // git.pr.create: PROXY xuống agent (agent-git-handler.ts line 244)
  // KHÔNG re-implement — đưa lên caller (Backend RPC layer) để route
}
```

**Cần thêm file selector:**
```typescript
// src/relay/git-remote-handler-index.ts  [NEW]
declare const __ORCA_GIT_V6__: boolean
export * from __ORCA_GIT_V6__
  ? './git-remote-handler-v6'
  : './git-remote-handler'
```

**Không viết lại** — reuse existing `git-exec-validator.ts`, `git-response-stream.ts` qua `gitRemoteHandlers['git.exec']`.

### 2.2 `src/relay/__tests__/git-handler-v6.test.ts` [NEW]

**Tái sử dụng pattern từ:** `src/relay/git-exec-validator.test.ts`

```typescript
describe('git.exec + git.execStream (remote handlers)', () => {
  describe('validateGitArgs', () => {
    it('allowed subcommand OK — no error')
    it('empty args array → GIT_NO_SUBCOMMAND')
    it('disallowed subcommand → GIT_DISALLOWED_SUBCOMMAND')
    it('shell metacharacter & in arg → GIT_SHELL_METACHARACTER_IN_ARG')
    it('shell metacharacter | in arg → error')
    it('shell metacharacter ; in arg → error')
    it('$ in arg → error')
  })

  describe('git.exec', () => {
    it('success → { stdout, stderr, exitCode: 0 }')
    it('non-zero exit → { exitCode: N, stderr } (not thrown)')
    it('timeout exceeded → error returned (not crashed)')
    it('maxBuffer exceeded → error handled')
  })

  describe('git.execStream', () => {
    it('yields lines from child process stdout')
    it('handles child process close event correctly')
    it('non-zero exit → throws after all yields')
  })
})
```

**Target: ≥ 15 tests**

### 2.3 `src/main/runtime/rpc/methods/git-remote-rpc.ts` [NEW]

> **Lưu ý tên file:** `git-remote-rpc.ts` (không phải `git-remote-handler.ts` — tên đó đã dùng ở relay tier)

**Tái sử dụng pattern từ:** `src/main/project/__tests__/project-rpc.test.ts`

```typescript
describe('git remote RPC methods', () => {
  // Mock: relay.call, projectService, providerResolver
  
  describe('git.status', () => {
    it('calls relay git.exec with correct status args')
    it('parses porcelain v2 output via WorkspaceService')
  })

  describe('git.diff', () => {
    it('staged=true → --staged flag passed to relay')
    it('file arg passed correctly')
  })

  describe('git.add', () => {
    it('files list passed to relay git.exec')
  })

  describe('git.commit', () => {
    it('message passed with -m flag')
    it('triggers onCommitComplete for task ref detection')
  })

  describe('git.push', () => {
    it('uses streaming relay.callStream')
    it('passes remote + branch args')
  })

  describe('git.generateCommitMessage', () => {
    it('gets staged diff → sends to AI → returns message string')
    it('empty diff → GIT_NO_STAGED_CHANGES error')
    it('large diff truncated to 8000 chars')
  })

  describe('git.pr.create', () => {
    it('prefers gh CLI when available (relay preflight check)')
    it('falls back to GitHub API when no gh CLI')
    it('throws GITHUB_NO_CREDENTIAL when no token found')
  })
})
```

**Target: ≥ 20 tests**

---

## 3. AI Commit Message — Tái sử dụng Chain

```
git.generateCommitMessage(projectId, worktreePath, userId)
  → router.getRelayForProject(projectId, userId)  ← tái sử dụng ProjectServerRouter
  → relay.call('git.exec', { args: ['diff', '--staged', ...] })
  → providerResolver.resolve({ devServerId, projectId, userId })  ← tái sử dụng ProviderResolver
  → relay.call('ai.complete', { accountId, prompt, maxTokens: 200 })
  → return message.trim()
```

---

## 4. Task Auto-Advance on Commit

```typescript
// Tái sử dụng TaskService + TaskGrantService:
async function onCommitComplete(commitMsg, projectId, userId, taskService, grantService) {
  const refs = [...commitMsg.matchAll(/#(TG-[\w-]+)/g)].map(m => m[1])
  for (const ref of refs) {
    const task = await taskService.findByRef(ref)
    if (task?.projectId === projectId) {
      const perm = await grantService.resolvePermission(userId, task.id)
      if (perm && PERMISSION_LEVELS[perm] >= PERMISSION_LEVELS['edit']) {
        await taskService.update(task.id, { status: 'review' })
        await taskService.addComment(task.id, userId, `Commit: ${commitMsg}`, 'activity')
      }
    }
  }
}
```

---

## 5. PR Creation — Dual Strategy

```
Strategy A (preferred):
  relay.call('preflight.check', { services: ['github-cli'] })
  → true → relay.call('git.exec', { args: ['gh', 'pr', 'create', ...] })

Strategy B (fallback):
  credentialService.get(userId, 'github')
  → fetch('https://api.github.com/repos/.../pulls', { POST })
```

Tái sử dụng `preflight-handler.ts` (đã có trong relay).

---

## 6. Frontend Components (TDD-20 §6)

```
src/renderer/src/components/workspace/git/
  GitPanel.tsx        [NEW] — main git panel UI
  CommitForm.tsx      [NEW] — stage + commit with AI message
  DiffViewer.tsx      [NEW] — file diff display
  BranchManager.tsx   [NEW] — branch list + create + checkout
  PullRequestForm.tsx [NEW] — PR creation form
```

> **Note:** Frontend components tái sử dụng `useWorkspace()` hook từ TDD-19  
> và `useRpc()` for relay calls.

---

## 7. Verification

```bash
pnpm vitest run src/relay -- --testNamePattern="git"
# Expected: ≥ 15 new git handler tests

pnpm vitest run src/main/runtime/rpc/methods
# Expected: ≥ 20 git-remote tests
```
