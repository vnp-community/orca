# FE-TASK-001: Map `GitStatusResult` → `GitStatus` đúng field trong `WorkspaceContext`

**Domain:** project-workspace
**Solution Ref:** FE-SOL-001 Bước 1-2
**Priority:** 🔴 P0 — prerequisite cho FE-TASK-002
**Estimated:** 30 phút
**Status:** ✅ DONE (2026-09-06)

**Kết quả thực tế:** Đúng như kế hoạch, cộng 1 điểm phát sinh: `toWorkspaceGitStatus()` dùng
`result.entries ?? []` (không chỉ `result.entries`) vì test cũ ở `WorkspaceContext.test.tsx` mock
response tối giản không kèm `entries` — bản backend thật luôn có field này, nhưng hàm map cần an
toàn với input thiếu field không bắt buộc. `tsc --noEmit` before/after: 0 lỗi type mới.

---

## Mục tiêu

`refreshGitStatus()` hiện ép kiểu response RPC thật (`GitStatusResult`) sang type sai
(`GitStatus`) không map field nào. Thêm hàm chuẩn hoá + state `gitStatusError`.

## Files cần sửa

1. `frontend/src/renderer/src/types/workspace-types.ts` — `GitStatus.branch` optional, thêm
   `branchUnavailable?: 'detached-head' | 'status-unavailable'`
2. `frontend/src/renderer/src/context/WorkspaceContext.tsx` — hàm `toWorkspaceGitStatus()`, gọi
   RPC với type thật `GitStatusResult`, thêm state `gitStatusError: boolean` + action tương ứng,
   reset về `false` ở cùng chỗ đang reset `gitStatus`/`currentWorktree`.

## Các bước thực thi

Xem code mẫu đầy đủ ở [FE-SOL-001](../solutions/FE-SOL-001-normalize-git-status-and-branch-display.md)
Bước 1-2 — copy gần như nguyên văn, không cần thiết kế lại.

## Verify

```bash
cd frontend && npx tsc --noEmit -p .
grep -n "toWorkspaceGitStatus\|gitStatusError" frontend/src/renderer/src/context/WorkspaceContext.tsx
```

## Depends on
Không có

## Blocking
FE-TASK-002, FE-TASK-003
