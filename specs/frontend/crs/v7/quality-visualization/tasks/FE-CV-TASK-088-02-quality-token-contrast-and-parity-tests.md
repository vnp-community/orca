# FE-CV-TASK-088-02: Test tương phản và parity của token chất lượng

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.3, 4, 5
**Priority:** P0
**Area:** frontend / tests
**File:** `frontend/src/renderer/src/components/quality-charts/__tests__/quality-token-contrast.test.ts` (mới), `quality-token-parity.test.ts` (mới), `frontend/src/renderer/src/test-support/css-color-resolution.ts` (mới; hàm thuần giải `var()`, `color-mix` srgb, `oklch`, hex → sRGB và tính tỉ lệ WCAG)
**Depends on:** FE-CV-TASK-088-01
**Status:** [ ] TODO

## Context

- Repo không có `axe-core`/`jest-axe` (README feature điểm 14); kiểm tương phản phải bằng hàm thuần tự viết (~60-80 dòng).
- Tiền lệ test đọc nguồn: `app-startup-routing.test.ts` dùng `readFileSync(join(process.cwd(), 'src/renderer/src/...'))` (`process.cwd()` là `frontend/`).
- Đầu vào: `main.css` và `node_modules/.pnpm/tailwindcss@4.2.4/node_modules/tailwindcss/theme.css` (đường dẫn phiên bản có thể đổi: tìm bằng `require.resolve('tailwindcss/theme.css')` hoặc thư mục `node_modules/tailwindcss/theme.css` thay vì cứng `.pnpm`; chưa kiểm chứng đường nào chạy trong vitest).
- Số mong đợi (đã tính lại độc lập, làm tròn 2 chữ số): `--quality-error` sáng/`card` 4,87, sáng/`muted` 4,47, tối/`card` 6,24; `--quality-warning` sáng 5,05/4,63, tối/`card` 10,44; `--quality-info` sáng 5,85/5,37; `--quality-pass` sáng 5,02/4,60; `--quality-unknown` sáng 4,74/4,35, tối/`card` 6,94; thang nhiệt tối bậc 3 chữ `foreground` 4,62.

## Việc cần làm

1. `css-color-resolution.ts`: `resolveColor(tokenName, theme: 'light'|'dark')` đọc `:root`/`.dark` từ chuỗi CSS, giải `var(--x)` đệ quy (chống vòng), `color-mix(in srgb, A p%, B)` (nội suy sRGB không tiền nhân alpha; `transparent` chỉ chấp nhận khi bỏ qua trong cặp chữ), `oklch(L% C h)` → sRGB (ma trận Oklab chuẩn, kẹp gamut), hex 3/6 chữ số; `contrastRatio(a, b)`.
2. `quality-token-contrast.test.ts`: bảng cặp `(chữ|đồ hoạ, nền, ngưỡng)`:
   - chữ `--quality-{error,warning,info,pass,unknown}` trên `--card` và `--background` ở sáng/tối ≥ 4,5;
   - đồ hoạ cùng token trên `--muted` ≥ 3;
   - chữ `--foreground` trên heat-1..3 và `--background` trên heat-4..5 ≥ 4,5 (cả hai chủ đề);
   - **khẳng định âm**: `--quality-error` sáng trên `--muted` **< 4,5** và `--quality-unknown` sáng trên `--muted` **< 4,5** (test này ghi lại lý do quy tắc "chữ không đặt trên muted"; nếu sau này token đổi để vượt, test báo để cập nhật quy tắc).
3. `quality-token-parity.test.ts`: mỗi token ở 088-01 có trong `:root`, `.dark` và `@theme inline` (`--color-quality-*`); không có token `--quality-*` thừa ngoài danh sách; quét file `quality-charts/**/*.{ts,tsx}` không chứa hex `#[0-9a-fA-F]{3,8}` và không chứa lớp màu Tailwind thô (`text-red-`, `bg-amber-`, ...).
4. Ngưỡng làm tròn: so sánh `ratio >= 4.5` trên số **chưa làm tròn**; khi chênh < 0,05 so với ngưỡng, test in cảnh báo (không fail) để người sửa token biết biên mỏng (tối bậc 3 đang 4,62).

## Kiểm thử

- Chính hai file test là sản phẩm; thêm test đơn vị cho `css-color-resolution.ts`: hex trắng/đen = 21:1; `oklch` biết trước; vòng `var()` ném lỗi có tên token (test dùng lỗi, không âm thầm).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/quality-charts/__tests__/quality-token src/renderer/src/test-support/css-color-resolution`.

## Tiêu chí hoàn thành

- [ ] Hai test xanh; thay một token bằng màu kém (ví dụ `amber-300`) làm test tương phản đỏ (thử một lần, hoàn nguyên).
- [ ] Không thêm dependency.
- [ ] Không có magic hex trong file test ngoài hằng của phép thử hàm (trắng/đen).

## Rủi ro

- Giải `color-mix` bằng sRGB tự viết có thể lệch trình duyệt ±0,01-0,02 (không tiền nhân alpha, làm tròn); coi là gần đúng, không thay kiểm tay.
- Đường dẫn `theme.css` phụ thuộc pnpm; nếu `require.resolve` không chạy trong vitest node thì dùng đường dẫn từ `process.cwd()` kèm `existsSync` và `it.skip` có thông báo (không im lặng).
