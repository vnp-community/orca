# FE-CV-TASK-095-04: Bộ theo dõi "agent xong → quyết định"

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.3
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/lib/review-decision-tracker.ts` (mới) + test
**Depends on:** FE-CV-TASK-095-03; FE-CV-SOL-061-review-entry-points (`AgentTurnCompletion`)
**Status:** [x] DONE (verified 2026-10-07: 8 tests)

## Context

- Trạng thái trong bộ nhớ; không bền; một sự kiện/lượt.

## Việc cần làm

1. `registerCompletion`, `noteReviewOpened`, `decide`; lượt mới thay lượt cũ → `abandon`.
2. `used_review`, `gate` (`none` khi không có), `open_findings` bucket.

## Kiểm thử

- Đồng hồ giả: một quyết định → một sự kiện; thứ hai bỏ qua; abandon; không phát khi cờ tắt.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Hàm thuần-ish, không phụ thuộc React.

## Rủi ro

- Mất khi tắt app.

## Ghi chú triển khai (2026-10-07)

Viết lại: `registerCompletion/noteReviewOpened/decide`, một sự kiện/lượt, abandon khi lượt mới (bản cũ dùng `require()` nên 5/11 test fail).
