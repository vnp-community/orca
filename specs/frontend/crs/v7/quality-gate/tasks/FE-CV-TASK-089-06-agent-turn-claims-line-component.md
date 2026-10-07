# FE-CV-TASK-089-06: Component `AgentTurnVerificationLine`

**From Solution:** [FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md) mục 2.4
**Priority:** P2
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/turns/AgentTurnVerificationLine.tsx` (mới) + test; chỗ gắn trong `ReviewTurnSwitcher` (FE-CV-SOL-060)
**Depends on:** FE-CV-TASK-089-05
**Status:** [x] DONE — `AgentTurnVerificationLine.tsx` chưa tồn tại. Chỗ gắn trong SOL-060 `ReviewTurnSwitcher` cũng chưa có. Rà soát 2026-10-07.

## Context

- STYLEGUIDE: token, lucide (`TriangleAlert`, `CircleHelp`, `Terminal`), `size-3.5`, tooltip chỉ để gọi tên, cảnh báo quan trọng hiện inline.

## Việc cần làm

1. Một dòng/mỗi claim; icon + chữ (không chỉ màu); liên kết "Xem lần chạy" khi có `verifyingRunId`; nút "Chạy lại kiểm tra" khi `canRun`.
2. Tooltip: "Ghi nhận, không phải xác thực".

## Kiểm thử

- Bốn trạng thái, nút gọi callback (`// @vitest-environment happy-dom`).
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không hex, không emoji.
- [ ] Văn bản thuần.

## Rủi ro

- Chỗ đặt trong 060 chưa tồn tại.
