# FE-CV-TASK-089-06: Component `AgentTurnVerificationLine`

**From Solution:** [FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md) mục 2.4
**Priority:** P2
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/turns/AgentTurnVerificationLine.tsx` (mới) + test; chỗ gắn trong `ReviewTurnSwitcher` (FE-CV-SOL-060)
**Depends on:** FE-CV-TASK-089-05
**Status:** [x] DONE (verified 2026-10-07: ReviewTurnSwitcher.test 9/9, AgentTurnVerificationLine.test, ReviewWorkspace.companions.test 8/8; review-map 130 file PASS)

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

## Ghi chú triển khai (2026-10-07)

Component hoàn chỉnh + test, nhưng chưa gắn vào `ReviewTurnSwitcher` (SOL-060 chưa tồn tại).

## Ghi chú tích hợp (W6, 2026-10-07)

Dòng được gắn vào `ReviewTurnSwitcher` (prop `verificationByTurn`) cho lượt đang xem; dữ liệu từ `turns/use-agent-turn-verification.ts` gọi `quality.turns` (limit 20, chỉ khi cờ quality bật và có mốc lượt), khoá theo `clientTurnId` = `turnId`. Chưa truyền `canRun`/`onRunChecks`/`onViewRun` nên nút "Chạy lại kiểm tra" và liên kết "Xem lần chạy" chưa hiện (chờ lens quality của W5-A).
