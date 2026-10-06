# FE-CV-TASK-055-02: Container mặc định và truy vấn `architecture`

**From Solution:** [FE-CV-SOL-055-architecture-c4-lens](../solutions/FE-CV-SOL-055-architecture-c4-lens.md) mục 4.2
**Priority:** P1
**Area:** frontend / review-map + hooks
**File:** `c4-container-default.ts`, `hooks/useC4Architecture.ts` (mới), `store/slices/review-ui.ts` (thêm `c4ContainerId`, `c4Drafts`), tests
**Depends on:** FE-CV-TASK-050-13, FE-CV-TASK-051-01
**Status:** [ ] TODO

## Context

- `architecture {container?, includeHidden?}` ⇒ `{containers, view|null}`; `ContainerRef.path` tương đối gốc repo.

## Việc cần làm

1. `pickDefaultContainer(containers, changedFiles)`: nhiều file đổi nhất theo tiền tố `path`; hoà theo tên; không khớp ⇒ đầu tiên.
2. `useC4Architecture(worktreeId, {container, includeHidden})`: lần đầu không tham số lấy `containers`, sau đó theo container; cache `(scopeKey, container, includeHidden)`; trạng thái `view===null`.
3. Thêm `c4ContainerId`, `c4Drafts` vào `ReviewUiState` và khoá dọn rò rỉ.

## Kiểm thử

- Chọn mặc định; hoà; không thay đổi; đổi container; `includeHidden`; rò rỉ hai đường.

## Tiêu chí hoàn thành

- [ ] Lựa chọn giữ khi đổi lens trong cùng tab.

## Rủi ro

- Hai lời gọi `architecture` nối tiếp qua SSH (~200 ms mỗi lần); chấp nhận.
