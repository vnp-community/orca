# FE-CV-TASK-089-03: Hàng đợi gửi, thử lại, khử trùng lặp

**From Solution:** [FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md) mục 2.3 (5)
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/turns/agent-turn-record-queue.ts` (mới) + test
**Depends on:** FE-CV-TASK-089-01; bộ phân loại lỗi của 050
**Status:** [x] DONE (verified 2026-10-07: 10 tests)

## Context

- Backend idempotent theo `clientTurnId`; lỗi không được chặn UI; không persist.

## Việc cần làm

1. `enqueue` khử trùng lặp theo `clientTurnId`; thử lại 3 lần backoff 2/6/18 s với `offline|timeout|tool-failed|unknown|rate-limited`.
2. Bỏ ngay với `quality-disabled|disabled|unsupported|forbidden|validation|not-found|no-binding`.
3. `dispose` huỷ timer.

## Kiểm thử

- Đồng hồ giả; khử trùng; backoff; không thử lại lỗi cố định.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không rò timer.
- [ ] Không ném ra ngoài.

## Rủi ro

- Mất lượt chưa gửi khi đóng app (chấp nhận).

## Ghi chú triển khai (2026-10-07)

Sửa số lần retry (3 lần sau lần gửi đầu, backoff 2/6/18 s), thêm `pending()`, khử trùng lặp cả sau khi gửi xong.
