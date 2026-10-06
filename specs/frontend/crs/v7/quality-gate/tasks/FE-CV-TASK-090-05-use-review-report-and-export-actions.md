# FE-CV-TASK-090-05: Hook `useReviewReport` và hành động xuất

**From Solution:** [FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/report/use-review-report.ts`, `review-report-export-actions.ts` (mới) + test
**Depends on:** FE-CV-TASK-085-01, 090-02, 090-04; FE-CV-SOL-050-store-and-query-hooks
**Status:** [ ] TODO

## Context

- `quality.report` 20 s; `inProgress` retry ≤ 90 s.
- Copy: `window.api.ui.writeClipboardText` (web nuốt khi thiếu clipboard) + fallback; tải: Blob + `a[download]` như `mcp-audit-csv.ts`.

## Việc cần làm

1. Hook gọi kênh với tham số hợp lệ (ngoài khoảng bị từ chối).
2. `copyMarkdown`, `copyForReview`, `downloadHtml` (tên tệp không đường dẫn tuyệt đối).
3. Xử lý lỗi `quality-disabled` (ẩn), lỗi khác (toast).

## Kiểm thử

- Giả `window.api.ui`; fallback; retry.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không im lặng khi copy lỗi.

## Rủi ro

- `execCommand` chưa có tiền lệ.
