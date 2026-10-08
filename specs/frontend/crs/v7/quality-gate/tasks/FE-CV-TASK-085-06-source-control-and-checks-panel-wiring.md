# FE-CV-TASK-085-06: Nối hook và notice vào SourceControl và ChecksPanel

**From Solution:** [FE-CV-SOL-085-source-control-quality-notice](../solutions/FE-CV-SOL-085-source-control-quality-notice.md) mục 2.6 (3, 4, 5)
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx` (sửa ~vài dòng gần :606, :5247, :5282), `frontend/src/renderer/src/components/right-sidebar/ChecksPanel.tsx` (sửa gần :3611)
**Depends on:** FE-CV-TASK-085-03, 085-05
**Status:** [x] DONE (verified 2026-10-07: dựa trên test hook/notice/composer; không có test render SourceControl/ChecksPanel)

## Context

- Hai nơi gọi composer sản phẩm: SourceControl.tsx:5247 và ChecksPanel.tsx:3611; `renderPullRequestComposer` chỉ là helper test. `pr-create-dialog` là code chết, không dùng.
- `handleCommit` 1792, `handleCreatePullRequest` 3008, `runCreatePrIntent` 3483 không đổi hành vi.

## Việc cần làm

1. Chạy GitNexus `impact` cho `SourceControl`/`CommitArea`/`CreateHostedReviewComposer` và báo blast radius (chưa chạy).
2. Gọi hook với `headOid`/`baseRef` từ `gitBranchCompareSummaryByWorktree` (baseRef chỉ khi `status==="ready"`); truyền node vào ba chỗ.
3. ChecksPanel: lấy `headOid` từ cùng slice (xác minh khi làm).

## Kiểm thử

- Test hồi quy: `SourceControl.*.test`, `source-control-primary-action*.test` xanh; test mới hiển thị khối ở ba nhánh.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Hành vi commit/PR không đổi.
- [ ] Cờ tắt: không đổi gì trên màn hình.

## Rủi ro

- SourceControl.tsx 6 723 dòng: chỉ thêm vài dòng.

## Ghi chú triển khai (2026-10-07)

SourceControl dùng hook trực tiếp (đã sửa `variant` -> `visible`); ChecksPanel dùng `source-control-quality-gate-slot.tsx` mới. Impact: CreateHostedReviewComposer LOW, ChecksPanel LOW.
