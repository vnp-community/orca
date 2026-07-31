# Solution: Implementation Order & Dependency Graph

**Mục tiêu:** Thứ tự thực hiện tối ưu để tái sử dụng code tối đa và tránh blocking dependencies.  
**Conflict Strategy:** New File + Compile-time Flag — không chỉnh file cũ, tạo `*-v6.ts` mỚi.

---

## Phase 0: Build Config Setup (✅ DONE)

> **Đã hoàn thành** — 2026-07-30T18:49

- `src/types/build-constants.d.ts` — đã thêm `__ORCA_GIT_V6__`, `__ORCA_WORKSPACE_V6__`
- `electron.vite.config.ts` — đã thêm 2 define entries
- `vite.server.config.ts` — đã thêm 2 define entries

---

## Phase 1: Fix Server Wiring (✅ DONE — 2026-07-30T23:07 ICT)

> **Khởi đầu thực thi.** Server bootstrap đã được wire hoàn toàn.

### Task 1.1 — ✅ Register Profile + Project RPC Methods

**File:** `src/main/server-bootstrap.ts`

```typescript
// Sau step 9 (ProfileService init):
const { createProfileMethods } = await import('./profile/profile-rpc-handler')
rpcServer.addMethods(createProfileMethods(profileService, profileResolver, authManager))
console.log('[ServerBootstrap] ✅ Profile RPC methods registered (v5.0)')

// Sau step 10 (ProjectService init):
const { createProjectMethods } = await import('./project/project-rpc-handler')
rpcServer.addMethods(createProjectMethods(projectService, _projectRouter, profileResolver))
console.log('[ServerBootstrap] ✅ Project RPC methods registered (v5.0)')
```

**Estimated effort:** 10 min  
**Risk:** Low — just method registration

### Task 1.2 — Verify migrations/index.ts exports 0006-0010

**File:** `src/main/db/migrations/index.ts`

Verify `ALL_MIGRATIONS` array includes migrations 0006 through 0010.

**Estimated effort:** 5 min

### Task 1.3 — Verify shared types

Verify these files exist with correct types:
- `src/shared/task-types.ts` — OrcaTask, TaskComment, TaskGrantLevel
- `src/shared/ai-provider-types.ts` — AIProviderType, AIProviderAccount, etc.
- `src/shared/project-types.ts` — OrcaProject, ProjectMember, ProjectContext

**Estimated effort:** 10 min

---

## Phase 2: Write Missing Tests (✅ DONE — 2026-07-30T22:59 ICT)

### 2A — TDD-18 Task Tests (✅ COMPLETE)

```
1. TaskService.test.ts        ✅ 24 tests
2. TaskGrantService.test.ts   ✅ 13 tests
3. TaskAIPlanner.test.ts      ✅ 14 tests
4. TaskAgentExecutor.test.ts  ✅ 10 tests
5. task-rpc.test.ts           ✅ 13 tests
6. task-commit-advance.test.ts ✅ 7 tests
```

**Pattern to reuse:**
```typescript
// From ProjectService.test.ts — mock IConnectionPool:
const mockPool = {
  query: vi.fn().mockResolvedValue([]),
  queryOne: vi.fn().mockResolvedValue(null),
  withConnection: vi.fn(cb => cb(mockPool)),
}
```

**Estimated: ≥55 tests, ~4 hours**

### 2B — TDD-19 Workspace Tests

```
1. relay-connection-pool.test.ts   — pool lifecycle
2. WorkspaceService.test.ts        — parallel init + parsers
3. WorkspaceContext.test.tsx        — frontend event bus
```

**Pattern to reuse:**
- `relay-connection-pool.test.ts`: Mock `DevServerRelayBridge.isAlive()` + `disconnect()`
- `WorkspaceService.test.ts`: Mock relay.call() per operation

**Estimated: ≥25 tests, ~2 hours**

### 2C — TDD-14 Profile RPC Test

```
1. profile-rpc.test.ts  — 12 tests
```

**Pattern to reuse:** `project-rpc.test.ts` (same structure)

**Estimated: ≥12 tests, ~45 min**

### 2D — TDD-16 AI Provider Relay Test

```
1. ai-provider-handler.test.ts  — relay credential store (15 tests)
```

**Pattern to reuse:** `src/relay/__tests__/agent-credential-store.test.ts`

**✅ DONE: 14 tests PASS**

### 2E — TDD-17 Workflow RPC Test (✅ COMPLETE)

```
1. workflow-rpc.test.ts  ✅ 15 tests
```

---

## Phase 3: TDD-20 Remote Git + Shared Contracts (✅ DONE — 2026-07-30T23:30 ICT)

> **Quy tắc:** Chỉ tạo file mỚi — KHÔNG chỉnh `git-remote-handler.ts` và `agent-rpc-dispatch.ts`.

### T15 — git-remote-handler-v6.ts + git-remote-handler-index.ts + git-remote-rpc.ts [NEW]

```
❌ KHÔNG sửa: src/relay/git-remote-handler.ts  (v5 baseline, GIỮ NGUYÊN)
❌ KHÔNG sửa: src/relay/git-handler.ts          (53.5KB, GIỮ NGUYÊN)

✅ TẠO MỚI: src/relay/git-remote-handler-v6.ts
   → Import gitRemoteHandlers từ file cũ, kế thừa 100%
   → Thêm: git.status, git.diff, git.add, git.restore, git.commit
           git.push, git.pull, git.branch.list, git.checkout
   → git.pr.create: KHÔNG implement — agent owns (line 244 agent-git-handler.ts)

✅ TẠO MỚI: src/relay/git-remote-handler-index.ts
   → Compile-time selector (declare const __ORCA_GIT_V6__)
   → v6: git-remote-handler-v6.ts | v5: git-remote-handler.ts

✅ TẠO MỚI: src/main/runtime/rpc/methods/git-remote-rpc.ts
   → Backend RPC routing: projectId → relay.call()
   → Tên: git-remote-rpc.ts (KHÔNG phải git-remote-handler.ts — tên đó ở relay)
```

**Tái sử dụng:**
- `gitRemoteHandlers['git.exec']` (từ git-remote-handler.ts hiện tại)
- `ProjectServerRouter.getRelayForProject()` — relay routing
- `defineMethod` pattern từ `git-remote.ts`

**Estimated: 3 hours**

### T18 — ai-credential-contract.ts + ai-provider-types-shared.ts [NEW] (~30 min)

```
✅ TẠO MỚI: src/shared/ai-credential-contract.ts
   → CredentialReadResult, HealthCheckResult, CredentialWriteParams, HealthCheckParams
   → Đồng bộ API shape giữa agent-credential-store.ts và ai-provider-handler.ts

✅ TẠO MỚI: src/renderer/src/types/ai-provider-types-shared.ts
   → Re-export từ src/shared/ai-provider-types.ts + ai-credential-contract.ts
   → Cho components mới import types chính xác từ shared source-of-truth
```

### T19 — agent-spawner.ts (SubAgentSpawner — relay tier) [NEW] (~2h)

```
✅ TẠO MỚI: src/relay/agent-spawner.ts
   → export class SubAgentSpawner (KHÔNG phải ProfileAwareAgentSpawner — tên đó là Orca Server tier)
   → handleAgentSpawn (fire-and-forget), handleAgentKill, buildAgentEnv, resolveAgentSpec
   → PTY Registry (in-process singleton)
   → Tương thích với agent-rpc-dispatch.ts cases agent.spawn/agent.kill (TASK-07 DONE)
```

### T16 — git-handler-v6.test.ts + git-remote-rpc.test.ts [NEW]

```
❌ KHÔNG thêm vào git-handler.test.ts (104KB — test file cũ, GIỮ NGUYÊN)

✅ TẠO MỚI: src/relay/__tests__/git-handler-v6.test.ts  (≥15 tests)
✅ TẠO MỚI: src/main/runtime/rpc/methods/__tests__/git-remote-rpc.test.ts  (≥20 tests)
```

**Depends on T15.**

**Estimated: 2 hours**

### 3D — Frontend Git UI Components (scope riêng)

```
src/renderer/src/components/workspace/git/
  GitPanel.tsx
  CommitForm.tsx
  DiffViewer.tsx
  BranchManager.tsx
  PullRequestForm.tsx
```

**Estimated: 4-6 hours (task riêng, không thuộc scope này)**

---

## Phase 4: WorkspaceContext Frontend (TDD-19 renderer)

### T17 — WorkspaceContextV6.tsx + WorkspaceContextBridge.ts [NEW]

```
❌ KHÔNG sửa: src/renderer/src/context/WorkspaceContext.tsx  (v5, 186 lines, GIỮ NGUYÊN)

✅ TẠO MỚI: src/renderer/src/context/WorkspaceContextV6.tsx
   → Full v6 spec: switchProject, micro event bus, pendingTasks
   → fileTree: FileTreeNode[] (type đúng, v5 có FileNode | null)
   → currentWorktree, availableWorktrees

✅ TẠO MỚI: src/renderer/src/context/WorkspaceContextBridge.ts
   → Compile-time selector (declare const __ORCA_WORKSPACE_V6__)
   → v6: WorkspaceContextV6 | v5: WorkspaceContext
   → App.tsx import từ Bridge thay vì import trực tiếp

✅ TẠO MỚI: src/renderer/src/context/__tests__/WorkspaceContextV6.test.tsx
   → ≥10 tests (React Testing Library)
```

**Pattern tái sử dụng:**
- `WorkspaceContext.tsx` hiện tại — xem structure để v6 extend tương thích
- `useRpc()` hook
- React Testing Library patterns từ renderer tests

**✅ DONE: 10 tests PASS**

---

## Total Effort Estimate

| Phase | Tasks | Effort | Kết quả |
|-------|-------|--------|----------|
| Phase 0: Build config | T00 | ✅ DONE | `__ORCA_GIT_V6__`, `__ORCA_WORKSPACE_V6__` |
| Phase 1: Wiring | T01, T02, T03 | ✅ DONE | server-bootstrap L387, L399 |
| Phase 2A: TDD-18 tests | T04-T09 (6 files) | ✅ DONE | 81 tests PASS |
| Phase 2B: TDD-19 tests | T10, T11 (2 files) | ✅ DONE | 30 tests PASS |
| Phase 2C: TDD-14 test | T12 (1 file) | ✅ DONE | 19 tests PASS |
| Phase 2D: TDD-16 test | T13 (1 file) | ✅ DONE | 14 tests PASS |
| Phase 2E: TDD-17 test | T14 (1 file) | ✅ DONE | 15 tests PASS |
| Phase 3: TDD-20 git + shared | T15, T16, T18, T19 | ✅ DONE | 116 tests PASS |
| Phase 4: Frontend | T17 | ✅ DONE | 10 tests PASS |
| **Total** | **19 tasks** | **✅ ALL DONE** | **484 tests PASS** |

---

## Tái sử dụng Code Summary

| Loại tái sử dụng | Files | Phần trăm |
|-----------------|-------|----------|
| Service implementations | 20+ files | **100%** — không viết lại |
| Test mock patterns | project-rpc, ProjectService tests | **~90%** pattern reuse |
| DB pool patterns | IConnectionPool usage | **100%** — same interface |
| Relay call patterns | DevServerRelayBridge.call() | **100%** — same API |
| Migration runner | db/migrations/runner.ts | **100%** — same runner |
| **Tổng code viết mới** | ~14 test files + 1 relay handler | **~15-20% total codebase** |
