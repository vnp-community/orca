# FE-CV-TASK-050-15: Union `review`, schema phiên, `toVisibleTabType`, `isRenderableTab`, selectors

**From Solution:** [FE-CV-SOL-050-review-tab-wiring](../solutions/FE-CV-SOL-050-review-tab-wiring.md) mục 4.2
**Priority:** P0
**Area:** frontend / shared + store
**File:** `shared/types.ts` (:799-808), `shared/workspace-session-schema.ts` (:96-104), `store/slices/tabs.ts` (:412, :1909-1920), `store/slices/worktrees.ts` (:301), `store/selectors.ts` (:86), tests
**Depends on:** không
**Status:** [ ] TODO

## Context

- Mẫu `simulator`: không bản ghi nền; `isRenderableTab` trả `true`. `toVisibleTabType` có hai bản (tabs.ts:412, worktrees.ts:301).
- Chạy `impact` GitNexus trên `toVisibleTabType`, `isRenderableTab` (chưa chạy).

## Việc cần làm

1. Thêm `'review'` vào `TabContentType`, `WorkspaceVisibleTabType`, hai `z.enum`.
2. `toVisibleTabType` ở hai nơi trả `'review'`; `isRenderableTab`: `review` ⇒ `true`.
3. `selectors.ts:86`: đọc ngữ cảnh rồi đếm tab Review như simulator (nếu hợp lý).
4. `rg "'simulator'" frontend/src` + `tsc` + `pnpm lint:switch-exhaustiveness`; ghi mọi `switch` thiếu vào PR (task 050-17/18 xử lý phần UI).

## Kiểm thử

- `workspace-session-schema.test.ts` mở rộng: parse/hydrate tab `review`; `store/slices/tabs.review.test.ts`: hydrate không loại tab review.

## Tiêu chí hoàn thành

- [ ] Hai union + hai schema có `review`; tab hydrate không bị loại; test cũ xanh.

## Rủi ro

- Bản app cũ gặp `review` có thể hỏng phiên (`z.enum`): chưa kiểm chứng `catch` cấp trên.
