# Task Index — TDD v6 Implementation Tasks

**From:** Solutions in `specs/backend/crs/v3/tdd-v6/solutions/`  
**TDD Ref:** `specs/backend/tdd/v5/` (TDD-14 → TDD-20)  
**Source:** `src/main/` + `src/relay/` + `src/renderer/`  
**Date:** 2026-07-30  
**Conflict Strategy:** New File + Compile-time Flag (xem `conflict-analysis-tdd-v6.md`)  
**AC Status:** ✅ **Tất cả Acceptance Criteria đã được tick — 484 tests PASS / 2026-07-30T23:56 ICT**

---

## Tổng quan

Tổng **19 tasks** chia thành 5 phases, được thiết kế để AI thực thi độc lập mỗi task.  
Mỗi task có: mục tiêu rõ ràng, files cần đọc, files cần tạo/sửa, acceptance criteria.

### Quy tắc bất biến (New File Strategy)
> ❌ **KHÔNG chỉnh** `src/relay/git-remote-handler.ts` (93 lines — v5 baseline)  
> ❌ **KHÔNG chỉnh** `src/renderer/src/context/WorkspaceContext.tsx` (186 lines — v5 baseline)  
> ❌ **KHÔNG chỉnh** `src/relay/agent-rpc-dispatch.ts` (TASK-07 agent đã DONE)  
> ✅ Mọi v6 code → file mới `*-v6.ts` / `*-v6.tsx`  
> ✅ Compile-time selector → `*-index.ts` / `*Bridge.ts`

---

## Task List

| ID | Phase | Task | File(s) tạo mới | Effort | Depends On |
|----|-------|------|----------------|--------|-----------| 
| **T00** | **P0** | **[Build config flags ✅ DONE](../../../../../../../electron.vite.config.ts)** | — | — | — |
| T01 | P1 | [server-bootstrap: register profile + project RPC](./T01-server-bootstrap-rpc-wiring.md) **✅ DONE** | 0 (sửa 1 file) | 10 min | — |
| T02 | P1 | [verify migrations index 0006-0010](./T02-verify-migrations-index.md) **✅ DONE** | 0 (không cần sửa) | 5 min | — |
| T03 | P1 | [verify + fix shared types](./T03-verify-shared-types.md) **✅ DONE** | +1 line (TaskGrantLevel alias) | 15 min | — |
| T04 | P2 | [TaskService.test.ts](./T04-task-service-tests.md) **✅ DONE** | 1 test file | 2h | T03 |
| T05 | P2 | [TaskGrantService.test.ts](./T05-task-grant-service-tests.md) **✅ DONE** | 1 test file | 1h | T04 |
| T06 | P2 | [TaskAIPlanner.test.ts](./T06-task-ai-planner-tests.md) **✅ DONE** | 1 test file | 1h | T04 |
| T07 | P2 | [TaskAgentExecutor.test.ts](./T07-task-agent-executor-tests.md) **✅ DONE** | 1 test file | 1h | T04, T05 |
| T08 | P2 | [task-rpc.test.ts](./T08-task-rpc-tests.md) **✅ DONE** | 1 test file | 1h | T04, T05 |
| T09 | P2 | [task-commit-advance.test.ts](./T09-task-commit-advance-tests.md) **✅ DONE** | 2 files (impl+test) | 30min | T04, T05 |
| T10 | P2 | [relay-connection-pool.test.ts](./T10-relay-pool-tests.md) **✅ DONE** | file đã tồn tại | 1h | — |
| T11 | P2 | [WorkspaceService.test.ts](./T11-workspace-service-tests.md) **✅ DONE** | 1 test file | 1h | T10 |
| T12 | P2 | [profile-rpc.test.ts](./T12-profile-rpc-tests.md) **✅ DONE** | 1 test file (19 tests) | 45min | T01 |
| T13 | P2 | [ai-provider-handler.test.ts (relay)](./T13-ai-provider-relay-tests.md) **✅ DONE** | 1 test file (14 tests) | 1h | — |
| T14 | P2 | [workflow-rpc.test.ts](./T14-workflow-rpc-tests.md) **✅ DONE** | 1 test file (15 tests) | 45min | — |
| T15 | P3 | [NEW git-remote-handler-v6.ts + git-remote-handler-index.ts + git-remote-rpc.ts](./T15-git-remote-handler.md) **✅ DONE** | **3 files mới** | 3h | T01 |
| T16 | P3 | [NEW git-handler-v6.test.ts + git-remote-rpc.test.ts](./T16-git-remote-tests.md) **✅ DONE** | **2 test files** (24+18=42 tests) | 2h | T15 |
| T17 | P4 | [NEW WorkspaceContextV6.tsx + WorkspaceContextBridge.ts](./T17-workspace-context-frontend.md) **✅ DONE** | **3 files mới** (10 tests) | 2h | T11 |
| T18 | P3 | [NEW ai-credential-contract.ts + ai-provider-types-shared.ts](./T18-shared-contract-files.md) **✅ DONE** | **2 files mới** (type-only) | 30min | T03 |
| T19 | P3 | [NEW agent-spawner.ts (SubAgentSpawner, relay tier)](./T19-agent-spawner.md) **✅ DONE** | **2 files mới** (26 tests) | 2h | — |

---

## Execution Order (Dependency-aware)

```
Phase 0 (DONE):   T00 — build config flags (electron.vite.config.ts, build-constants.d.ts)

Phase 1 (DONE ✅ 2026-07-30T22:59):
  T01 ✅ — createProfileMethods + createProjectMethods added to server-bootstrap.ts
  T02 ✅ — ALL_MIGRATIONS đã đủ 10 entries (0001→0010), không cần sửa
  T03 ✅ — Shared types OK, thêm alias TaskGrantLevel = TaskPermission

Phase 2 batch A:     T04 ✅ DONE (24 tests) | T05 ✅ DONE (13 tests) | T06 ✅ DONE (14 tests)
                     T07 ✅ DONE (10 tests) | T08 ✅ DONE (13 tests) | T09 ✅ DONE (7 tests)
Phase 2 batch B:     T10 ✅ DONE (15 tests) | T11 ✅ DONE (15 tests)
Phase 2 batch C:     T12 ✅ DONE (19 tests) | T13 ✅ DONE (14 tests) | T14 ✅ DONE (15 tests)

Phase 3 (parallel):  T15 ✅ DONE (3 files mới) | T18 ✅ DONE (2 files type-only) | T19 ✅ DONE (26 tests)
                     T16 ✅ DONE (24+18=42 tests)

Phase 4:             T17 ✅ DONE (10 tests)

--- VERIFIED 2026-07-30 23:43 ICT: 484 tests PASS / 32 test files ---
```

---

## File Ownership — Bất biến

```
[UNCHANGED — GIỮ NGUYÊN HOÀN TOÀN]
src/relay/git-remote-handler.ts              → Backend (v5 baseline, 93 lines)
src/relay/agent-git-handler.ts               → Agent  (git.pr.create line 244)
src/relay/agent-rpc-dispatch.ts              → Agent  (TASK-07 DONE, 443 lines)
src/renderer/src/context/WorkspaceContext.tsx → Frontend (v5, 186 lines)
src/renderer/src/types/ai-provider-types.ts  → Frontend (v5, giữ nguyên)

[NEW FILES — tạo mới trong T15-T19]
src/relay/git-remote-handler-v6.ts           → Backend (T15 — v6 methods)
src/relay/git-remote-handler-index.ts        → Backend (T15 — compile selector)
src/main/runtime/rpc/methods/git-remote-rpc.ts → Backend (T15 — routing layer)
src/renderer/src/context/WorkspaceContextV6.tsx → Frontend (T17 — v6 spec)
src/renderer/src/context/WorkspaceContextBridge.ts → Frontend (T17 — compile selector)
src/shared/ai-credential-contract.ts         → Shared  (T18 — contract C4)
src/renderer/src/types/ai-provider-types-shared.ts → Frontend (T18 — re-export C6)
src/relay/agent-spawner.ts                   → Agent   (T19 — SubAgentSpawner)

[BUILD CONFIG — DONE (T00)]
src/types/build-constants.d.ts               → ✅ +__ORCA_GIT_V6__, __ORCA_WORKSPACE_V6__
electron.vite.config.ts                      → ✅ +2 define entries
vite.server.config.ts                        → ✅ +2 define entries
```

---

## Acceptance Criteria Tổng thể

```bash
# TypeScript — 0 lỗi
pnpm tsc --noEmit

# Unit tests
pnpm vitest run src/main/task          # ≥70 tests
pnpm vitest run src/main/profile       # ≥37 tests
pnpm vitest run src/main/ai-providers  # ≥55 tests
pnpm vitest run src/main/workflow      # ≥53 tests
pnpm vitest run src/main/workspace     # ≥15 tests
pnpm vitest run src/main/dev-server    # ≥15 tests (relay pool)
pnpm vitest run src/relay              # ≥15 tests mới (git v6)
pnpm vitest run src/renderer           # ≥10 tests (WorkspaceContextV6)

# Verify file cũ không bị chỉnh (kiểm tra sau mỗi PR)
git diff src/relay/git-remote-handler.ts        # phải empty
git diff src/renderer/src/context/WorkspaceContext.tsx  # phải empty
git diff src/relay/agent-rpc-dispatch.ts        # phải empty

# Build
ORCA_FEATURE_GIT_V6=true ORCA_FEATURE_WORKSPACE_V6=true pnpm build:relay
```

---

## Test Target Summary

| Module | Files hiện có | Files cần thêm | Grand Total |
|--------|--------------|---------------|------------|
| profile | 2 files (≈25 tests) | +1 file (+12) | ≥37 |
| project | 4 files (≈35 tests) | — | ≥35 ✅ DONE |
| ai-providers | 3 files (≈40 tests) | +1 file (+15) | ≥55 |
| workflow | 3 files (≈45 tests) | +1 file (+8) | ≥53 |
| task | 1 file (≈15 tests) | +5 files (+55) | ≥70 |
| workspace + relay pool | 0 | +2 files (+25) | ≥25 |
| WorkspaceContextV6 (renderer) | 0 | +1 file (+10) | ≥10 |
| git relay v6 + remote rpc | 0 | +2 files (+35) | ≥35 |
| **TOTAL mới** | | **+13 files (~160 tests)** | |
