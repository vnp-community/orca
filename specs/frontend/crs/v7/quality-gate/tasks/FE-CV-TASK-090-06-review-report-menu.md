# FE-CV-TASK-090-06: Menu "Xuất báo cáo"

**From Solution:** [FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md) mục 2.4
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/report/ReviewReportMenu.tsx` (mới) + test
**Depends on:** FE-CV-TASK-090-05; FE-CV-SOL-051-review-workspace-shell
**Status:** [ ] TODO

## Context

- `ui/dropdown-menu`; chỉ hiện khi `flags.quality`.

## Việc cần làm

1. Ba mục menu; khoá ngay khi bấm; toast xác nhận.
2. Gắn vào `ReviewSummaryBar` qua khe của 051.

## Kiểm thử

- Cờ tắt không render.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Phím Esc đóng; không phím tắt giả.

## Rủi ro

- Khe của 051 chưa tồn tại.
