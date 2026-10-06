# FE-CV-TASK-088-06: `ChartFrame`, bảng thay thế, hover card, hook kích thước và vẽ lười

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.5, 2.7, 2.8
**Priority:** P0
**Area:** frontend / components + hooks
**File:** `frontend/src/renderer/src/components/quality-charts/ChartFrame.tsx`, `ChartTextAlternative.tsx`, `ChartHoverCard.tsx`, `useChartSize.ts`, `useLazyChartMount.ts` (mới) và `__tests__/ChartFrame.test.tsx`, `ChartTextAlternative.test.tsx`, `useChartSize.test.tsx`, `useLazyChartMount.test.tsx`
**Depends on:** FE-CV-TASK-088-03
**Status:** [ ] TODO

## Context

- `ui/table.tsx`, `ui/toggle.tsx`, `ui/skeleton.tsx`, `ui/tooltip.tsx`/`hover-card.tsx` có sẵn (đã liệt kê); không có `alert`.
- CR-088 2.8 bảng trạng thái: `loading` → Skeleton đúng `minHeight`; `empty` → giải thích vì sao trống (không "không có vấn đề"); `error` → inline persistent + "Thử lại", không toast; `stale` → vẫn vẽ + `staleNote`.
- STYLEGUIDE UX rule 1 và "pre-reserve space": `minHeight` cố định, không nhảy bố cục; không hiện spinner trong 100 ms đầu (hook gọi quyết định `status`).
- Bảng thay thế là đường truy cập chuẩn: luôn có trong DOM (`sr-only` khi chưa bật).

## Việc cần làm

1. `ChartFrame.tsx` theo `ChartFrameProps` ở SOL-088 2.5: `figure`/`figcaption`, tiêu đề, mô tả cách đọc, nút `ui/toggle` "Xem dạng bảng" (`aria-pressed`), chú giải, dòng `staleNote`, vùng vẽ có `minHeight`; truyền `{width,height}` qua `useChartSize` cho `children`. `table` là prop bắt buộc (TypeScript không cho bỏ). Trạng thái `error` không cho phép `children` vẽ.
2. `ChartTextAlternative.tsx`: `ui/table` có `<caption>`, tiêu đề cột (`scope="col"`), `null` hiển thị "—"; số căn phải; khi không bật: lớp `sr-only` (không `display:none`).
3. `ChartHoverCard.tsx`: thẻ nổi định vị theo toạ độ khung, `pointer-events-none`, không nhận tiêu điểm; nội dung chỉ là bản sao dòng của bảng thay thế; render văn bản thuần.
4. `useChartSize.ts`: một `ResizeObserver` cho khung, gộp bằng `requestAnimationFrame`, trả `{width,height}` làm tròn; gỡ khi unmount; môi trường không có `ResizeObserver` (happy-dom/SSR) trả kích thước ban đầu từ `getBoundingClientRect` hoặc 0 mà không ném.
5. `useLazyChartMount.ts`: `IntersectionObserver`; trả `{ref, mounted}`; một khi `mounted` thì không huỷ lại khi cuộn ra; môi trường thiếu `IntersectionObserver` thì `mounted=true` ngay.
6. Không `transition`/`animation`; lớp `motion-reduce` không cần vì không có hoạt ảnh (test khẳng định không có lớp `transition`).
7. Khoá i18n `auto.components.qualityCharts.frame.{viewTable,viewChart,retry,showing,stale}`.

## Kiểm thử

- `ChartFrame`: năm trạng thái; `minHeight` đặt đúng style; `empty` không chứa "không có vấn đề"/"sạch"; `error` có nút "Thử lại" gọi `onRetry`; bật/tắt bảng đổi `aria-pressed`; bảng luôn tồn tại trong DOM.
- `ChartTextAlternative`: `<caption>` có; `null` → "—" (không "0"); văn bản backend chứa `<script>` hiển thị như chữ (U9).
- Hook: mô phỏng `ResizeObserver`/`IntersectionObserver`; gỡ đúng; gộp rAF (dùng `vi.useFakeTimers` + giả rAF).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/quality-charts/__tests__/ChartFrame src/renderer/src/components/quality-charts/__tests__/ChartTextAlternative src/renderer/src/components/quality-charts/__tests__/use`.

## Tiêu chí hoàn thành

- [ ] `table` bắt buộc (kiểm biên dịch bằng `// @ts-expect-error` trong test).
- [ ] Không hex, không `transition`, không toast; chuỗi `translate()`.
- [ ] Mỗi `ChartFrame` chỉ tạo một `ResizeObserver`.

## Rủi ro

- Hoạt động `IntersectionObserver` trong Electron nền ẩn (cửa sổ không hiển thị) chưa kiểm chứng; mặc định `mounted=true` khi `document.visibilityState !== 'visible'`.
