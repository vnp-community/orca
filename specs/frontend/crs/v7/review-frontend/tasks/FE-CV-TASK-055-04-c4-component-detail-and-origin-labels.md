# FE-CV-TASK-055-04: `C4ComponentDetail`, `C4InferredNotice`, nhãn nguồn

**From Solution:** [FE-CV-SOL-055-architecture-c4-lens](../solutions/FE-CV-SOL-055-architecture-c4-lens.md) mục 4.3
**Priority:** P1
**Area:** frontend / review-map
**File:** `C4ComponentDetail.tsx`, `C4InferredNotice.tsx` (mới), tests
**Depends on:** FE-CV-TASK-055-03, FE-CV-TASK-053-06
**Status:** [x] DONE (verified 2026-10-07: vitest C4ComponentDetail.test.tsx 9/9)

## Context

- `origin` (`derived|merged|declared`), `descriptionSource`, `evidence: SymbolRef[]`; `hasOverrides`, `overridesVersion`; `c4.get` ⇒ `updatedBy`, `updatedAt`.

## Việc cần làm

1. Drawer cho component: tên, `kind`, `path` (sao chép), mô tả (+ nguồn mô tả), `techHint`, `symbolCount`, cờ lớp phủ, quan hệ vào/ra (bấm ⇒ chọn cạnh); cạnh: loại, hai đầu, `count`, `confidence`, bằng chứng (≤ 20 + xem thêm; "Xem diff"/"Mở trong editor").
2. `C4InferredNotice`: luôn khi `hasOverrides=false`; có ghi đè ⇒ người sửa + thời gian; `Badge` "Suy luận"/"Đã chỉnh một phần"/"Khai báo"; "Thêm mô tả" mở trình soạn.
3. Chuỗi từ backend render văn bản thuần (U9).

## Kiểm thử

- Mỗi `origin`; `descriptionSource:'none'`; bằng chứng; không HTML thô.

## Tiêu chí hoàn thành

- [ ] Không bao giờ trình bày sơ đồ như đã xác minh.

## Rủi ro

- `updatedBy` là id hay tên hiển thị: hợp đồng không nói.

## Ghi chú triển khai (2026-10-07)

`C4ComponentDetail` là panel bên phải lens (không dùng drawer khung). `C4InferredNotice` hiện người sửa/thời gian chỉ khi truyền `updatedBy`; lens chưa gọi `c4.get` để lấy (chỉ `hasOverrides` từ view) nên hiển thị bản "có ghi đè thủ công" không tên. Khoá i18n `descSource.c4_yaml` (dấu chấm đổi `_`).
