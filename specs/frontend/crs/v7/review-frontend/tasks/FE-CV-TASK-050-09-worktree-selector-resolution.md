# FE-CV-TASK-050-09: `resolveCodeIntelSelector` (O-1: `projectId` của worktree)

**From Solution:** [FE-CV-SOL-050-store-and-query-hooks](../solutions/FE-CV-SOL-050-store-and-query-hooks.md) mục 4.2
**Priority:** P0
**Area:** frontend / renderer lib
**File:** `frontend/src/renderer/src/lib/code-intel-worktree-selector.ts` (mới), test `code-intel-worktree-selector.test.ts`
**Depends on:** không
**Status:** [x] DONE (verified 2026-10-07: selector.test 8 pass; resolver fixed to use worktreesByRepo (state.worktrees never existed); useCodeIntelSelector returns a stable reference)

## Context

- PQ-04: mọi kênh nhận `{projectId, worktreeId}`; `worktreeId` = `<repoId>::<path>` hợp lệ, `::workspace:` bị từ chối. O-1 chưa chốt.
- `getRuntimeEnvironmentIdForWorktree` (`lib/worktree-runtime-owner.ts:162`) trả `null` cho terminal nổi và folder workspace. `Worktree.projectId?` (`shared/types.ts:487`), `Repo.projectId?` (:247, chỉ với repo qua project-service).

## Việc cần làm

1. Hàm thuần `resolveCodeIntelSelector(state, worktreeId)` trả union ở SOL mục 4.2.
2. Bỏ tiền tố `id:` nếu có; id `workspace`/`folder` ⇒ `workspace-scope`; thiếu `projectId` ⇒ `no-project`; `environmentId === null` ⇒ `no-environment`.
3. Hook mỏng `useCodeIntelSelector(worktreeId)` (selector Zustand ổn định, không tạo object mới mỗi render).

## Kiểm thử

- Bảng ca: worktree có/không `projectId`; repo có `projectId`; floating terminal; folder workspace; tiền tố `id:`; đích local.

## Tiêu chí hoàn thành

- [ ] Không bao giờ ném; `unsupported` có `reason`; test xanh.

## Rủi ro

- Worktree cũ không có `projectId` sẽ không dùng được Review (O-1): ghi telemetry sau (CR-095).
