# FE-CV-TASK-095-07: Test quyền riêng tư và consent

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 5, 6
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/shared/review-telemetry-privacy.test.ts`, `frontend/src/renderer/src/lib/review-telemetry.test.ts` (mới)
**Depends on:** FE-CV-TASK-095-01..05
**Status:** [x] DONE (verified 2026-10-07: 5 tests review-telemetry-privacy.test.ts + 34 tests wrapper)

## Context

- Consent: `DO_NOT_TRACK`, CI, Privacy pane do desktop main xử lý; web no-op.

## Việc cần làm

1. Quét tĩnh `review-telemetry.ts`: không import kiểu có trường định danh.
2. Snapshot payload; mock consent tắt → không sự kiện ra khỏi main (nếu test desktop truy cập được, nếu không ghi chú).

## Kiểm thử

- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Test xanh.

## Rủi ro

- Phần desktop main nằm ngoài `frontend/`.

## Ghi chú triển khai (2026-10-07)

Phần consent của desktop main nằm ngoài frontend, không kiểm.
