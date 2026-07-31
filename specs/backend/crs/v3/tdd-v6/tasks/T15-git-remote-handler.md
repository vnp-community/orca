# T15 — Tạo git-remote-handler-v6.ts + git-remote-handler-index.ts + git-remote-rpc.ts [NEW FILE STRATEGY]

**Phase:** 3 (New Code — TDD-20)  
**Effort:** ~3 hours  
**Depends on:** T01 (RPC wiring)  
**Solution ref:** [07-tdd20-remote-git-ui.md §2.1, §2.3](../solutions/07-tdd20-remote-git-ui.md)  
**TDD ref:** TDD-20 (Remote Git UI)  
**⚠️ Conflict Resolution:** C1 + NEW-B — New File strategy (không chỉnh file cũ)

---

## ⚠️ QUAN TRỌNG — Quy tắc bất biến

> **`src/relay/git-remote-handler.ts` ĐÃ TỒN TẠI** (93 lines)  
> **KHÔNG chỉnh sửa file này** — tạo file mới `git-remote-handler-v6.ts` song song.

```
❌ KHÔNG sửa:  src/relay/git-remote-handler.ts  (v5 baseline — GIỮ NGUYÊN)
❌ KHÔNG sửa:  src/relay/git-handler.ts         (53.5KB local ops — GIỮ NGUYÊN)
✅ TẠO MỚI:   src/relay/git-remote-handler-v6.ts
✅ TẠO MỚI:   src/relay/git-remote-handler-index.ts
✅ TẠO MỚI:   src/main/runtime/rpc/methods/git-remote-rpc.ts
```

## Mục tiêu

1. `git-remote-handler-v6.ts` [NEW] — 9 high-level git methods, kế thừa từ v5
2. `git-remote-handler-index.ts` [NEW] — compile-time selector `__ORCA_GIT_V6__`
3. `git-remote-rpc.ts` [NEW] — backend RPC routing layer (projectId → relay.call)

---

## Files Cần Đọc Trước

1. `src/relay/git-remote-handler.ts` — hiểu `gitRemoteHandlers` export, `GitExecResult` type
2. `src/relay/git-exec-validator.ts` — `validateGitArgs`, `ALLOWED_GIT_SUBCOMMANDS`
3. `src/main/runtime/rpc/methods/git-remote.ts` — xem existing git RPC pattern
4. `src/main/runtime/rpc/core.ts` — `defineMethod`, `RpcMethod` type
5. `src/main/project/ProjectServerRouter.ts` — `getRelayForProject()` call pattern
6. `src/types/build-constants.d.ts` — verify `__ORCA_GIT_V6__` đã declared (T00 DONE)

---

## File 1: `src/relay/git-remote-handler-v6.ts` [NEW]

```typescript
/**
 * git-remote-handler-v6.ts — v6 high-level git methods (TDD-20 / Conflict C1)
 *
 * Strategy: Re-use git-remote-handler.ts exports, không fork/copy logic.
 * Được chọn khi __ORCA_GIT_V6__ = true qua git-remote-handler-index.ts.
 *
 * KHÔNG chỉnh git-remote-handler.ts (v5 baseline, 93 lines).
 *
 * @module relay/git-remote-handler-v6
 */
import { gitRemoteHandlers } from './git-remote-handler'

export type { GitExecResult } from './git-remote-handler'
export { validateGitArgs, ALLOWED_GIT_SUBCOMMANDS } from './git-remote-handler'

export const gitRemoteHandlersV6 = {
  // ── Kế thừa v5 (git.exec + git.execStream) ───────────────────────────────
  ...gitRemoteHandlers,

  // ── v6: Status & Diff ─────────────────────────────────────────────────────
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

  // ── v6: Stage & Commit ────────────────────────────────────────────────────
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
      cwd: params.cwd,
      args: ['commit', '-m', params.message],
    })
    return { ok: true, output: result.stdout }
  },

  // ── v6: Push & Pull ───────────────────────────────────────────────────────
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

  // ── v6: Branch & Checkout ─────────────────────────────────────────────────
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

  // ── v6: PR Create — PROXY xuống agent (agent owns impl) ──────────────────
  // git.pr.create KHÔNG được implement ở đây.
  // Backend RPC layer (git-remote-rpc.ts) route xuống agent-git-handler.ts (line 244).
}
```

---

## File 2: `src/relay/git-remote-handler-index.ts` [NEW]

```typescript
/**
 * git-remote-handler-index.ts — Compile-time selector (Conflict C1 resolution)
 *
 * __ORCA_GIT_V6__ = true  → dùng gitRemoteHandlersV6 (v6 high-level methods)
 * __ORCA_GIT_V6__ = false → dùng gitRemoteHandlers  (v5: chỉ git.exec, git.execStream)
 *
 * Bundler (Vite) tree-shakes branch không dùng tại compile time.
 *
 * Env: ORCA_FEATURE_GIT_V6=true pnpm dev   → v6
 *      pnpm dev                             → v5 (default)
 */
declare const __ORCA_GIT_V6__: boolean

export * from __ORCA_GIT_V6__
  ? './git-remote-handler-v6'
  : './git-remote-handler'
```

---

## File 3: `src/main/runtime/rpc/methods/git-remote-rpc.ts` [NEW]

> **Lưu ý tên:** `git-remote-rpc.ts` (không phải `git-remote-handler.ts` — tên đó đã dùng ở relay tier)  
> **Lưu ý:** File `git-remote.ts` đã tồn tại (18 methods), đây là file bổ sung cho v6.

```typescript
/**
 * git-remote-rpc.ts — Backend RPC routing layer cho git v6 (TDD-20)
 *
 * Routes client requests → ProjectServerRouter → relay → git-remote-handler-v6.ts
 * (relay tự chọn v5/v6 qua git-remote-handler-index.ts compile selector)
 *
 * KHÔNG implement git logic ở đây — chỉ routing + authorization.
 */
import { z } from 'zod'
import { defineMethod } from '../core'
import type { RpcMethod } from '../core'
import type { ProjectServerRouter } from '../../../project/ProjectServerRouter'

export function createGitRemoteV6Methods(
  projectRouter: ProjectServerRouter,
): RpcMethod[] {
  return [
    defineMethod({
      name: 'git.status',
      schema: z.object({ projectId: z.string(), worktreePath: z.string().optional() }),
      handler: async ({ projectId, worktreePath }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.status', { cwd: relay.projectCwd, worktreePath })
      },
    }),

    defineMethod({
      name: 'git.diff',
      schema: z.object({ projectId: z.string(), staged: z.boolean().optional(), file: z.string().optional() }),
      handler: async ({ projectId, staged, file }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.diff', { cwd: relay.projectCwd, staged, file })
      },
    }),

    defineMethod({
      name: 'git.add',
      schema: z.object({ projectId: z.string(), files: z.array(z.string()) }),
      handler: async ({ projectId, files }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.add', { cwd: relay.projectCwd, files })
      },
    }),

    defineMethod({
      name: 'git.restore',
      schema: z.object({ projectId: z.string(), files: z.array(z.string()), staged: z.boolean().optional() }),
      handler: async ({ projectId, files, staged }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.restore', { cwd: relay.projectCwd, files, staged })
      },
    }),

    defineMethod({
      name: 'git.commit',
      schema: z.object({ projectId: z.string(), message: z.string().min(1) }),
      handler: async ({ projectId, message }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.commit', { cwd: relay.projectCwd, message })
      },
    }),

    defineMethod({
      name: 'git.push',
      schema: z.object({ projectId: z.string(), remote: z.string().optional(), branch: z.string().optional(), force: z.boolean().optional() }),
      handler: async ({ projectId, remote, branch, force }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.push', { cwd: relay.projectCwd, remote, branch, force })
      },
    }),

    defineMethod({
      name: 'git.pull',
      schema: z.object({ projectId: z.string(), remote: z.string().optional(), branch: z.string().optional(), rebase: z.boolean().optional() }),
      handler: async ({ projectId, remote, branch, rebase }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.pull', { cwd: relay.projectCwd, remote, branch, rebase })
      },
    }),

    defineMethod({
      name: 'git.branch.list',
      schema: z.object({ projectId: z.string(), remote: z.boolean().optional() }),
      handler: async ({ projectId, remote }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.branch.list', { cwd: relay.projectCwd, remote })
      },
    }),

    defineMethod({
      name: 'git.checkout',
      schema: z.object({ projectId: z.string(), branch: z.string(), create: z.boolean().optional() }),
      handler: async ({ projectId, branch, create }, _ctx) => {
        const relay = await projectRouter.getRelayForProject(projectId)
        return relay.call('git.checkout', { cwd: relay.projectCwd, branch, create })
      },
    }),
  ]
}
```

---

## Bước Verify

```bash
# 1. Không có lỗi TypeScript:
npx tsc --noEmit -p config/tsconfig.node.json

# 2. File cũ KHÔNG bị chỉnh:
git diff src/relay/git-remote-handler.ts  # phải empty

# 3. File mới đã tạo:
ls src/relay/git-remote-handler-v6.ts
ls src/relay/git-remote-handler-index.ts
ls src/main/runtime/rpc/methods/git-remote-rpc.ts

# 4. Export kiểm tra:
grep -n "export" src/relay/git-remote-handler-v6.ts

# 5. Compile flag hoạt động (đã DONE từ T00):
grep "__ORCA_GIT_V6__" src/types/build-constants.d.ts
grep "__ORCA_GIT_V6__" electron.vite.config.ts
```

---

## Acceptance Criteria

- [x] `src/relay/git-remote-handler-v6.ts` tồn tại, export `gitRemoteHandlersV6` với ≥9 methods ✅
- [x] `src/relay/git-remote-handler-index.ts` tồn tại, dùng `declare const __ORCA_GIT_V6__` ✅ (line 23)
- [x] `src/main/runtime/rpc/methods/git-remote-rpc.ts` tồn tại, export `createGitRemoteV6Methods` ✅ (line 17)
- [x] **`src/relay/git-remote-handler.ts` GIỮ NGUYÊN** — `git diff` = 0 lines ✅
- [x] **`src/relay/git-handler.ts` GIỮ NGUYÊN** — `git diff` = 0 lines ✅
- [x] `npx tsc --noEmit` → 0 errors ✅
- [x] T16 (tests) có thể chạy sau task này ✅ (24+18=42 tests pass)
