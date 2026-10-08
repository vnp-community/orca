# FE-CV-TASK-062-03: Controller `useMobileReviewSummaryController`

**From Solution:** [FE-CV-SOL-062](../solutions/FE-CV-SOL-062-mobile-review-summary.md) mục 2.3
**Priority:** P2
**Area:** mobile / session hook
**File:** `mobile/src/session/use-mobile-review-summary-controller.ts` (mới) + test (nếu tách được hàm thuần)
**Depends on:** FE-CV-TASK-062-02
**Status:** [~] PARTIAL — hook written; only the request guard is unit-tested (mobile-review-summary-request-guard.test.ts PASS); hook itself untested (no RN hook infra), mobile typecheck cannot run (mobile/node_modules and expo tsconfig absent)

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

## Ghi chú triển khai (2026-10-07)

- Thêm `mobile-review-summary-request-guard.ts` (hàm thuần, có test) cho bộ đếm thế hệ. Hook nhận `onNavigate(route)` thay vì tự dùng router. `onReconnect` chỉ bọc thành `reconnect` cho UI; chưa gọi tự động khi mất kết nối (sai lệch so với spec).
