# FE-CV-TASK-062-03: Controller `useMobileReviewSummaryController`

**From Solution:** [FE-CV-SOL-062](../solutions/FE-CV-SOL-062-mobile-review-summary.md) mục 2.3
**Priority:** P2
**Area:** mobile / session hook
**File:** `mobile/src/session/use-mobile-review-summary-controller.ts` (mới) + test (nếu tách được hàm thuần)
**Depends on:** FE-CV-TASK-062-02
**Status:** [x] DONE

## Context

- Mẫu `use-mobile-diff-review-controller.ts` (`loadGenerationRef`, `connState`, `onReconnect`). Không polling/subscribe.

## Việc cần làm

1. `useMobileReviewSummaryController({client, connState, hostId, worktreeId, name, onReconnect})` → `{screenState, filter, setFilter, expandedKey, toggleExpanded, refresh, openFileDiff}`.
2. Tải khi vào màn và khi kéo-làm-mới; bỏ phản hồi cũ bằng bộ đếm thế hệ; mất kết nối ⇒ gọi `onReconnect`.
3. `openFileDiff(item)` dùng `buildMobileReviewFileRoute({..., area:'branch'})` chỉ khi `inChangedFiles`.

## Kiểm thử

- Tách reducer/hàm thuần (chuyển trạng thái, bỏ phản hồi cũ) để test bằng Vitest `node`; không test component RN.
- `pnpm --dir mobile test -- use-mobile-review-summary` (chưa kiểm chứng).

## Tiêu chí hoàn thành

- [ ] Không có timer/polling.
- [ ] Phản hồi cũ không ghi đè.

## Rủi ro

- Không có test hook RN; logic phải ở hàm thuần.
