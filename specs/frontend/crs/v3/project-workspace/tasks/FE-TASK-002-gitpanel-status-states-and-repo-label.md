# FE-TASK-002: `GitPanel` — hiển thị đúng trạng thái branch + nhãn repo

**Domain:** project-workspace
**Solution Ref:** FE-SOL-001 Bước 4
**Priority:** 🔴 P0
**Estimated:** 30 phút
**Status:** ✅ DONE (2026-09-06)

**Kết quả thực tế:** Đúng như kế hoạch — `describeBranch()` + `currentRepo` lookup qua
`getRepoIdFromWorktreeId`. Test: 5 case mới (3 trạng thái branch, có/không nhãn repo), 7 case cũ
giữ nguyên hành vi. `vitest run`: 12/12 pass trên file này.

---

## Mục tiêu

Thay fallback cứng `'(no branch)'` bằng logic phân biệt 3 trạng thái (branch thật /
detached-HEAD / status-unavailable / RPC lỗi), và thêm nhãn tên repo cạnh branch chip (CR-PW-002).

## Files cần sửa

1. `frontend/src/renderer/src/components/workspace/git/GitPanel.tsx`
2. `frontend/src/renderer/src/components/workspace/git/__tests__/GitPanel.test.tsx`

## Các bước thực thi

1. Import `useAppStore`, `getRepoIdFromWorktreeId` (từ `frontend/src/shared/worktree-id.ts`),
   `gitStatusError` từ `useWorkspace()`.
2. Viết hàm nhỏ `describeBranch(gitStatus, gitStatusError)` trả label hiển thị:
   - `gitStatusError` → `'Git status unavailable'`
   - `gitStatus?.branch` → chính branch đó
   - `gitStatus?.branchUnavailable === 'detached-head'` → `'Detached HEAD'`
   - `gitStatus?.branchUnavailable === 'status-unavailable'` → `'Git unavailable'`
   - còn lại (đang load) → `'—'`
3. Tra repo hiện tại: `useAppStore(s => currentWorktree ? s.repos.find(r => r.id === getRepoIdFromWorktreeId(currentWorktree.id)) : undefined)`,
   hiển thị `repo?.displayName` cạnh branch chip nếu có (`data-testid="git-panel-repo-label"`).
4. Cập nhật test: mock `useWorkspace` trả `gitStatus: { branch: 'feat/test', branchUnavailable: undefined, aheadBy: 2, behindBy: 0 }`
   (field mới), thêm case test cho `branchUnavailable === 'detached-head'` và `gitStatusError`.

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/components/workspace/git/__tests__/GitPanel.test.tsx
```

## Depends on
FE-TASK-001

## Blocking
Không có
