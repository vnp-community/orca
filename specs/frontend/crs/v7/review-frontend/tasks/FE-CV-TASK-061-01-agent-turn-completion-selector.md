# FE-CV-TASK-061-01: Selector sự kiện "agent xong" và hook `useAgentTurnCompletions`

**From Solution:** [FE-CV-SOL-061](../solutions/FE-CV-SOL-061-review-entry-points.md) mục 2.1
**Priority:** P0
**Area:** frontend / renderer (hàm thuần + hook)
**File:** `frontend/src/renderer/src/components/review-map/entry/agent-turn-completion.ts`, `useAgentTurnCompletions.ts` (mới) + test
**Depends on:** không (đọc `agent-status` slice có sẵn)
**Status:** [ ] TODO

## Context

- Đã xác minh: `agentStatusByPaneKey` (`agent-status.ts:101`), `retainedAgentsByPaneKey` (:115), `stateStartedAt` (`agent-status-types.ts:112`). `done` không bị hạ về idle theo TTL (theo CR, chưa kiểm lại).
- Danh tính `${paneKey}:${doneAt}` dùng chung với FE-CV-SOL-060 (`turnId`) và FE-CV-SOL-089.

## Việc cần làm

1. `selectAgentTurnCompletions(state, worktreeId)`: hợp live `done` + retained `done`, loại `subagent`, khử trùng theo `id`, sắp `doneAt` giảm dần; `selectLatestCompletion`; `hasUnreviewedCompletion(completions, reviewedTurnId)`.
2. `useAgentTurnCompletions(worktreeId)` subscribe store với so sánh nông (không re-render khi không đổi).
3. Không gọi RPC; không đọc `prompt`.

## Kiểm thử

- Live done, retained done, subagent bị loại, thứ tự, `interrupted`, id, trùng lặp live+retained, entry lạ không ném; hook không re-render thừa.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/entry`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần có test; hook có test render-count.
- [ ] Không phụ thuộc cờ code-intel.

## Rủi ro

- `done` có thể báo lặp/sớm tuỳ agent (chưa thống kê).
