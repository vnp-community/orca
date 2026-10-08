# FE-CV-TASK-062-03: Controller `useMobileReviewSummaryController`

**From Solution:** [FE-CV-SOL-062](../solutions/FE-CV-SOL-062-mobile-review-summary.md) mục 2.3
**Priority:** P2
**Area:** mobile / session hook
**File:** `mobile/src/session/use-mobile-review-summary-controller.ts` (mới) + test (nếu tách được hàm thuần)
**Depends on:** FE-CV-TASK-062-02
**Status:** [x] DONE (verified 2026-10-08: use-mobile-review-summary-controller.test.ts 8 test PASS — tải 1 lần không polling, chờ desktop khi chưa kết nối, tự gọi `onReconnect(hostId)` đúng 1 lần khi chuyển sang `disconnected` (không gọi lại khi `reconnecting`), phản hồi cũ không ghi đè, `openFileDiff` chỉ khi `inChangedFiles` với area `branch`; cùng các test mobile-review-summary-* 5 file / 33 test PASS; typecheck cục bộ (tsconfig tạm ngoài repo, @types/react thật) không lỗi ở file controller)

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

## Ghi chú triển khai (2026-10-08)

- Hook đã tự gọi `onReconnect` khi mất kết nối (sửa sai lệch ghi ở trên). Test hook dùng `react-dom/client` + `// @vitest-environment happy-dom`; chạy bằng vitest gốc repo với config tạm `root: mobile` (mobile/node_modules chưa cài). Rủi ro: `happy-dom` không nằm trong devDependencies của `mobile/` — nếu CI mobile cài độc lập (không có node_modules gốc) cần đổi sang `react-test-renderer` (đã có trong devDeps mobile).
