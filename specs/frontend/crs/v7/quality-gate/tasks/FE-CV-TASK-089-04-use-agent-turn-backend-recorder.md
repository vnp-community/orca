# FE-CV-TASK-089-04: Hook ghi lượt lên backend

**From Solution:** [FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/turns/use-agent-turn-backend-recorder.ts` (mới) + test; `frontend/src/renderer/src/components/review-map/turns/use-review-turn-recorder.ts` (của FE-CV-SOL-060, thêm một lời gọi)
**Depends on:** FE-CV-TASK-085-01, 089-01..03; FE-CV-SOL-060-review-notes-and-turn-compare; FE-CV-SOL-061-review-entry-points
**Status:** [ ] TODO

## Context

- `gitStatusByWorktree` (editor.ts:612), `gitBranchCompareSummaryByWorktree[..].headOid`.
- PQ-35: chỉ nguồn renderer.

## Việc cần làm

1. Cờ `quality` tắt → không subscribe store, không gọi kênh.
2. Nhận `AgentTurnCompletion` → chờ debounce git của 060 → dựng params → `queue.enqueue`.
3. Đọc `agentTurnStorePromptExcerpt` từ `settings.get`.
4. Không sửa nội dung `ReviewTurnMarker`.

## Kiểm thử

- Cờ tắt 0 lời gọi; một completion → một `quality.turn.record`; `send` ném mà marker 060 vẫn tạo.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Lỗi RPC không chặn ghi mốc cục bộ.
- [ ] Chạy Electron và web.

## Rủi ro

- Chữ ký của 060/061 là giả định.
