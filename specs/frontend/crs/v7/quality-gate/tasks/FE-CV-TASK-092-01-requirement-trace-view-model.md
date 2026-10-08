# FE-CV-TASK-092-01: View-model và quy tắc chữ của truy vết

**From Solution:** [FE-CV-SOL-092-requirement-trace-view](../solutions/FE-CV-SOL-092-requirement-trace-view.md) mục 2.2, 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/requirements/requirement-trace-view-model.ts` (mới) + test
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (kiểu `RequirementTrace`)
**Status:** [x] DONE (verified 2026-10-07: 16 tests)

## Context

- Hợp đồng 4.7: năm `state`; `inferred` không tính vào state; `unknown ≠ no_evidence`.
- F10: cấm "đã đáp ứng/hoàn thành".

## Việc cần làm

1. `buildRequirementTraceViewModel` gom nhóm (Chưa thấy bằng chứng đứng đầu), tách `suggestions` (inferred).
2. Khoá nhãn i18n theo state; mã `warnings` lạ hiển thị nguyên văn.

## Kiểm thử

- Bảng ca đủ state x origin x confidence; trace null.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không từ cấm.
- [ ] Hàm thuần.

## Rủi ro

- Thứ tự nhóm cần người dùng xác nhận.

## Ghi chú triển khai (2026-10-07)

Viết lại theo `RequirementTrace` hợp đồng; inferred-only => "no evidence" + gợi ý, `unknown` != `no_evidence`.
