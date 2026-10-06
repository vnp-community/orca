# FE-CV-TASK-095-05: Nối tracker vào commit, tạo review, gửi ghi chú, đánh dấu đã xem

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.4
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx` (sửa vài dòng: sau `handleCommit` trả true; trong/sau `handlePullRequestCreated`), điểm gọi ở 060 và 052
**Depends on:** FE-CV-TASK-095-04
**Status:** [ ] TODO

## Context

- `handleCommit` :1792 trả boolean; `handlePullRequestCreated` :2488 được gọi ở 3078, 3116, 3295, 3318.

## Việc cần làm

1. Chạy GitNexus `impact` và báo blast radius (chưa chạy).
2. Mỗi điểm một dòng gọi `decide(...)`; không đổi hành vi.

## Kiểm thử

- Test hiện có `SourceControl.*.test` xanh; test mới giả `decide`.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Hành vi commit/PR không đổi.

## Rủi ro

- SourceControl.tsx rất lớn.
