# FE-CV-TASK-085-05: Thêm khe `qualityNotice` vào composer và CommitArea

**From Solution:** [FE-CV-SOL-085-source-control-quality-notice](../solutions/FE-CV-SOL-085-source-control-quality-notice.md) mục 2.6 (1, 2)
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/right-sidebar/CreateHostedReviewComposer.tsx` (sửa), `frontend/src/renderer/src/components/right-sidebar/source-control-commit-area.tsx` (sửa, không thêm max-lines disable mới)
**Depends on:** FE-CV-TASK-085-04
**Status:** [x] DONE (verified 2026-10-07: 2 tests CreateHostedReviewComposer.quality-notice.test.tsx)

## Context

- Composer: `createDisabled` (~dòng 118) tính từ `primaryAction.disabled`, title, base; hàng nút ở ~dòng 253.
- CommitArea: `createPrIntentNotice` ở dòng 573; file đã có disable max-lines cũ (kế thừa).

## Việc cần làm

1. Thêm prop `qualityNotice?: React.ReactNode` ở cả hai; render ngay trên hàng nút (composer) và sau khối `createPrIntentNotice` (CommitArea).
2. KHÔNG đưa vào `createDisabled`, không đổi `primaryAction.disabled`.
3. Chỉ thêm vài dòng; không tái cấu trúc.

## Kiểm thử

- Test hiện có (`CommitArea*.test.tsx`, `PullRequestComposer.generate-tooltip.test.tsx`) xanh; test mới: có/không `qualityNotice` không đổi `disabled`.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Prop tuỳ chọn, mặc định không render gì.
- [ ] Không `max-lines` disable mới.

## Rủi ro

- Tên khe cần thống nhất với FE-CV-SOL-087 (`preSubmitNotice` trong CR-087): một khe duy nhất `qualityNotice`.

## Ghi chú triển khai (2026-10-07)

Khe `qualityNotice` đã có từ trước nhưng chưa được destructure (lỗi tsc `Cannot find name`); đã sửa ở cả composer và CommitArea.
