# FE-CV-TASK-090-01: Parser mô hình báo cáo

**From Solution:** [FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md) mục 2.2, bảng sửa 2
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/report/review-report-model-parser.ts` (mới) + test
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge
**Status:** [x] DONE

## Context

- Hợp đồng 4.7 `ReviewReportModel`; PQ-32: `risk.level` HOA.
- Chịu thiếu trường và enum lạ.

## Việc cần làm

1. `parseReviewReportModel(raw)`: mặc định rỗng cho mảng thiếu, `UNKNOWN`/`unknown` cho enum lạ.
2. Chuỗi từ backend giữ nguyên (escape ở lúc dựng).

## Kiểm thử

- Fixture đủ, thiếu từng phần, enum lạ.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không ném.
- [ ] Hàm thuần.

## Rủi ro

- `generatedFor` chưa có kiểu.
