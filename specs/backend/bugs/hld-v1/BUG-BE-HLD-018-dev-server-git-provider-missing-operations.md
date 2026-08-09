# BUG-BE-HLD-018 — `DevServerGitProvider` ném lỗi "not supported" cho nhiều thao tác git nằm trong tiêu chí F39 (git log, AI commit message, branch/commit diff...)

**Mức độ:** 🟡 MEDIUM (Feature gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/providers/dev-server-git-provider.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.18/F39)

---

## Mô tả

`docs/features/F39-remote-git-ui.md` liệt kê các tiêu chí chấp nhận cho repo hosted trên Dev Server: git log 20 commit gần nhất với branch graph, AI commit-message generation từ staged diff, branch/commit compare & diff, submodule status, check-ignored, sync fork default branch.

`backend/src/main/providers/dev-server-git-provider.ts` khai báo:

```typescript
const NOT_SUPPORTED = (op) => new Error(`${op} is not supported for Dev Server hosts yet.`)
```

và ném lỗi này cho:
- `getHistory()` (dòng 182-184) — git log. Component `GitLog.tsx` cũng **không tồn tại** trong repo.
- `getStagedCommitContext()` (dòng 186-188) — context cho AI commit-message generation.
- `getBranchCompare()`, `getCommitCompare()`, `getBranchDiff()`, `getCommitDiff()`, `getSubmoduleStatus()`, `checkIgnoredPaths()`, `syncForkDefaultBranch()` (dòng 174-309) — đều `NOT_SUPPORTED`.

Các thao tác cơ bản khác (stage/unstage/commit/checkout/push/pull/fetch/rebase, worktree ops) đều hoạt động đầy đủ (dòng 314-491) — đây không phải toàn bộ Dev Server git bị hỏng, chỉ riêng nhóm "history/compare/diff" bị thiếu.

## Hậu quả

- Người dùng làm việc với repo **hosted trên Dev Server** (khác với repo local) không xem được git log, không dùng được AI commit-message generation, không so sánh được branch/commit — dù các thao tác này hoạt động bình thường với repo local.
- UI (`GitPanel.tsx`, `CommitForm.tsx`...) có thể hiển thị lỗi hoặc disable các nút liên quan mà không có thông báo rõ ràng cho user là do giới hạn Dev Server, không phải bug UI.

## Bằng chứng

- `backend/src/main/providers/dev-server-git-provider.ts:174-309` — danh sách đầy đủ method NOT_SUPPORTED.
- So sánh với `backend/src/main/providers/dev-server-git-provider.ts:314-491` — các thao tác đã implement đầy đủ để đối chiếu phạm vi thật của gap.
- Không tìm thấy file `GitLog.tsx` trong `frontend/src/renderer/src/components/workspace/git/`.

## Đề xuất fix

1. Implement `getHistory()` qua relay `git.exec log --oneline -20 --graph` (hoặc RPC method chuyên dụng tương tự `IGitProvider` khác) tới Dev Server Agent.
2. Implement `getStagedCommitContext()` tương tự bản local (đọc staged diff qua relay, trả về context cho AI).
3. Implement lần lượt `getBranchCompare`/`getCommitCompare`/`getBranchDiff`/`getCommitDiff` theo cùng pattern relay đã dùng cho các operation khác trong cùng file.
4. Thêm `GitLog.tsx` ở frontend để hiển thị git log + branch graph.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.18 (F39)
- Doc gốc: `docs/features/F39-remote-git-ui.md`
