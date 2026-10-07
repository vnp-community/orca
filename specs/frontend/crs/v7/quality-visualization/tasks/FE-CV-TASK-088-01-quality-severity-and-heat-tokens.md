# FE-CV-TASK-088-01: Token `--quality-*` và `--quality-heat-1..5` trong `main.css`

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.3
**Priority:** P0
**Area:** frontend / theme tokens
**File:** `frontend/src/renderer/src/assets/main.css` (sửa; chèn sau `--annotation-highlight` ở `:root` (:206) và `.dark` (:294) và khối `@theme inline` cạnh `--color-status-success*` (:101-103))
**Depends on:** không (làm được ngay, không cần backend)
**Status:** [x] DONE

## Context

- STYLEGUIDE (Color roles, Color mixing): token mới phải có ở cả `:root` và `.dark`, bind trong `@theme inline`, không hex mới; tint dùng `color-mix` trên token có sẵn.
- Đã đọc: `main.css` không có token chất lượng; `--warning` không được định nghĩa dù `main.css:1987-1989` dùng `var(--warning, #f59e0b)`; `--chart-1..5` toàn sắc xanh, giống nhau sáng/tối (không dùng làm thang mức độ).
- `theme.css` của `tailwindcss@4.2.4` có `--color-amber-700|400`, `--color-sky-700|400` (đã đọc).
- Không sửa `--review-*`, `--chart-*`; không thêm `--warning`.

## Việc cần làm

1. Thêm vào `:root` và `.dark` đúng bảng ở SOL-088 2.3: `--quality-error|warning|info|pass|unknown`, `--quality-{error,warning,info,pass}-background|border`, `--quality-heat-1..5`.
2. Trong `@theme inline` thêm `--color-quality-error: var(--quality-error)` … cho mọi token (kể cả `-background`, `-border`, `heat`) để dùng `text-quality-error`, `bg-quality-error-background`, `border-quality-error-border`.
3. Giá trị tham chiếu biến Tailwind (`var(--color-amber-700)`). Nếu thử nghiệm bản dựng cho thấy biến không được phát (rủi ro SOL-088 6.1), thay bằng giá trị trực tiếp của đúng bước màu đó, kèm một dòng comment "Why".
4. Chú thích ngắn (một-hai dòng "Why") ở khối token: quy ước "đậm hơn = cần chú ý hơn" và "chữ trên ô nhiệt: bậc 1-3 `foreground`, bậc 4-5 `background`".
5. Không đụng khối `.dark` ngoài các dòng mới; không đổi thứ tự token có sẵn.

## Kiểm thử

- Kiểm tay: chạy ứng dụng web và Electron (nếu có thể), mở một trang thử tạm có `<span class="text-quality-error">`; sáng và tối đúng giá trị. (Chưa chạy; không có công cụ chụp ảnh trong repo.)
- Test tự động ở 088-02.

## Tiêu chí hoàn thành

- [ ] 5 token mức + 8 token nền/viền + 5 token nhiệt có ở `:root` và `.dark` và `@theme inline`.
- [ ] Không hex mới; không thay token cũ.
- [ ] `pnpm --filter orca-frontend test` không vỡ test CSS hiện có (nếu có test đọc `main.css`).

## Rủi ro

- Biến `--color-*` của Tailwind 4 có thể không có khi không dùng utility (chưa kiểm chứng); dự phòng ở Việc 3.
- Hai ô dưới 4,5 trên `muted` (`--quality-error` sáng 4,47; `--quality-unknown` sáng 4,35): không sửa token, chặn bằng quy tắc dùng ở 088-02.
