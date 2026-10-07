# FE-CV-TASK-054-06: `StructureLens`, chi tiết file, đăng ký lens, i18n

**From Solution:** [FE-CV-SOL-054-structure-lens](../solutions/FE-CV-SOL-054-structure-lens.md) mục 4.1
**Priority:** P1
**Area:** frontend / review-map + i18n
**File:** `StructureLens.tsx`, `StructureFileDetail.tsx` (mới), `review-lens-registry.ts` (thêm `structure`), `i18n/locales/*.json`, `code-intel-locale-coverage.test.ts`
**Depends on:** FE-CV-TASK-054-04, 054-05, FE-CV-TASK-053-06, 051-05
**Status:** [x] DONE

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
