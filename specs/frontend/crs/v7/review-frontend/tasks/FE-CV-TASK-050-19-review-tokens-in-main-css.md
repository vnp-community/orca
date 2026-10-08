# FE-CV-TASK-050-19: Token `--review-*` trong `main.css` và kiểm tương phản

**From Solution:** [FE-CV-SOL-050-review-tab-wiring](../solutions/FE-CV-SOL-050-review-tab-wiring.md) mục 4.5
**Priority:** P0
**Area:** frontend / assets
**File:** `frontend/src/renderer/src/assets/main.css` (sửa: `:root` :126, `.dark` :216, `@theme inline` :43), test `assets/review-tokens.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE (verified 2026-10-07: review-tokens.test 12 pass; contrast script (step 3) not run)

## Context

- Tiền lệ `--ai-action-accent: var(--color-violet-500)` (:154). STYLEGUIDE "Color roles", "Don't invent color". README v7 §8 mục 28: `--review-untested` sáng ~3,19:1 trên `card` (tính tay).

## Việc cần làm

1. Thêm `--review-changed|affected|untested|violation` và `--review-area-1..6` ở `:root`, `.dark` và bind `--color-review-*` trong `@theme inline` (giá trị ở SOL mục 4.5, không hex).
2. Test quét `main.css`: đủ token ở 3 nơi; giá trị không hex.
3. Script thuần tính tương phản từ giá trị oklch Tailwind (`node_modules/tailwindcss/theme.css`) so với `--card` sáng/tối; ghi kết quả vào PR; **không** đổi màu nếu chưa đo; ngưỡng đồ hoạ 3:1.

## Kiểm thử

- `review-tokens.test.ts`; kiểm tay sáng/tối.

## Tiêu chí hoàn thành

- [ ] Token có đủ; không hex mới; số đo tương phản được ghi.

## Rủi ro

- Chuyển oklch → sRGB có sai số; đo bằng công cụ trình duyệt thay thế được.
