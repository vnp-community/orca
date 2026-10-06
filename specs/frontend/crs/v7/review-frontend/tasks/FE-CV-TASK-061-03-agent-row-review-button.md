# FE-CV-TASK-061-03: Nút Review ở hàng agent đã xong

**From Solution:** [FE-CV-SOL-061](../solutions/FE-CV-SOL-061-review-entry-points.md) mục 2.2
**Priority:** P0
**Area:** frontend / renderer components
**File:** `frontend/src/renderer/src/components/dashboard/DashboardAgentRow.tsx`, `DashboardAgentRowTrailingControls.tsx`, `components/sidebar/WorktreeCardAgents.tsx`, `worktree-card-compact-agent-row.tsx` (sửa nhỏ) + test
**Depends on:** FE-CV-TASK-061-01, 061-02
**Status:** [ ] TODO

## Context

- **Chạy `gitnexus_impact` trên `DashboardAgentRow`, `DashboardAgentRowTrailingControls` trước khi sửa và báo blast radius; `detect_changes` trước commit.**
- Ô cuối `w-12` (theo CR) hẹp; thẻ gọn dùng `CompactAgentRow` riêng.

## Việc cần làm

1. Prop tuỳ chọn `onReview` cho `DashboardAgentRow` và `CompactAgentRow`; nút khi `state==='done'` và `rowSource!=='subagent'`; luôn hiện khi `isUnvisited`, còn lại hover/focus-visible; ẩn khi `sendTargetStatus`; tooltip `interrupted`.
2. `stopPropagation` + `onMouseDown`/`onKeyDown` như các nút khác.
3. `WorktreeCardAgents.tsx`: `handleReviewAgent` gọi `openReviewFromEntryPoint(…,'agent-row',{completionId})` chỉ khi `useReviewEntryAvailability.visible`.
4. Kiểm bằng mắt thẻ gọn/thẻ thường; đổi ô cuối `w-auto` nếu cần.

## Kiểm thử

- `renderToStaticMarkup` (mẫu `DashboardAgentRow.test.tsx`): có nút cho `done`, không cho `working`/`subagent`, ẩn khi `sendTargetStatus`; test tương tác dừng nổi bọt (mẫu `WorktreeCardAgents.activation.test.tsx`); không truyền `onReview` ⇒ markup như cũ.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/dashboard src/renderer/src/components/sidebar`.

## Tiêu chí hoàn thành

- [ ] Test hiện có của dashboard/sidebar không đổi.
- [ ] Nút không kích hoạt hàng/thẻ.

## Rủi ro

- Chỗ trống cuối hàng; chưa kiểm bằng mắt.
