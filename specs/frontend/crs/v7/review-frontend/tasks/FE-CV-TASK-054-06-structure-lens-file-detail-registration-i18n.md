# FE-CV-TASK-054-06: `StructureLens`, chi tiết file, đăng ký lens, i18n

**From Solution:** [FE-CV-SOL-054-structure-lens](../solutions/FE-CV-SOL-054-structure-lens.md) mục 4.1
**Priority:** P1
**Area:** frontend / review-map + i18n
**File:** `StructureLens.tsx`, `StructureFileDetail.tsx` (mới), `review-lens-registry.ts` (thêm `structure`), `i18n/locales/*.json`, `code-intel-locale-coverage.test.ts`
**Depends on:** FE-CV-TASK-054-04, 054-05, FE-CV-TASK-053-06, 051-05
**Status:** [x] DONE (verified 2026-10-07: structure/StructureLens.test 5/5 pass against fake `structure` responses; i18n coverage pass; lens registered in review-lens-registry)

## Context

- Drawer nội dung qua `ReviewDrawerContent`; `openReviewDiffAtSymbol` (053-06); bộ lọc chip (051-04).

## Việc cần làm

1. `StructureLens` ghép toolbar + `ResizablePanelGroup` dọc (treemap, cây); trạng thái: tải gốc, rỗng ("Index chưa có thông tin cấu trúc cho đường dẫn này" + Lên một cấp), lỗi từng thư mục, `truncated`, 5 000 nút.
2. `StructureFileDetail`: đường dẫn (sao chép), ngôn ngữ, `symbolCount`, `loc`, cờ, symbol đã đổi trong file (bấm ⇒ lens Ảnh hưởng), "Xem diff", "Mở trong editor"; không tải mã.
3. Đăng ký lens; khoá i18n đủ 5 locale.

## Kiểm thử

- `StructureLens.test.tsx` tích hợp với fake backend (tải gốc, mở thư mục, drawer file); coverage i18n.

## Tiêu chí hoàn thành

- [ ] Lens hiện đúng tab; không `max-lines` disable; hoạt động web.

## Rủi ro

- Chip `affected` không có ở lens này (đã ghi trong chú giải).

## Ghi chú triển khai (2026-10-07, W3-A)

**Sai lệch so với spec**
- File lens đặt trong `components/review-map/structure/`. Treemap tái dùng `squarify` của `components/quality-charts/treemap-squarified-layout.ts` (đã squarified, có `omitted`); `workspace-space-layout.ts` (chia đôi) không dùng. `layoutSquarifiedTreemap` trả `{cells, mergedIds}` (cần danh sách id gộp vào "+N nhỏ"), ô dùng `{x,y,w,h}`.
- `use-structure-tree.ts` tự quản lý truy vấn theo thư mục (≤ 2 đồng thời, mỗi thư mục một lần, ≤ 5 000 nút) thay vì `useCodeIntelPagedQuery` vì số thư mục là động; không có cache xuyên lần mount theo `headCommit`.
- Nút có thư mục cha chưa được trả về được treo dưới tổ tiên gần nhất đã tải (`childIndex`), nên phản hồi thưa vẫn hiện.
- Chi tiết file hiển thị inline dưới hai khung (không qua drawer); "Mở symbol" đặt tâm lens Ảnh hưởng.
- Cờ `affected` không dùng; "File bị xoá" chỉ là dòng ghi chú. Ô lớp phủ dùng viền `overlaySvgProps` + chấm `--review-changed`; thư mục có chấm + số.
- Chưa kiểm: `color-mix()` trong `fill` ở Electron, hiệu năng khi kéo panel.
