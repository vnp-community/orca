# FE-REQ-TASK-020-03: Thẻ phương án và bảng so sánh

**From Solution:** [FE-REQ-SOL-020](../solutions/FE-REQ-SOL-020-solution-review-ui.md) mục 2.1, 2.3
**Priority:** P0
**Area:** frontend / request / solution
**File:** `frontend/src/renderer/src/components/request/solution/SolutionOptionCompare.tsx`, `SolutionOptionCard.tsx`, `SolutionComparisonTable.tsx` (mới); test cùng tên
**Depends on:** FE-REQ-TASK-020-01, 020-02
**Status:** [ ] TODO

## Context

- `SolutionOption {id, title, summary, pros[], cons[], effort?, risk?, recommended?, raw}` (018-01, schema tạm). `buildComparisonRows` và `canApproveSolution` ở 020-01.
- `ui/table.tsx` sẵn có; khuôn bảng trong repo: `TaskDAGView`/danh sách khác. Radix radio không nằm trong `ui/*`: dùng `role="radiogroup"` với `button role="radio" aria-checked`, hoặc `ui/toggle-group.tsx` (có sẵn).
- Màu: chỉ token; điểm khác biệt đánh dấu bằng icon và chữ, không chỉ màu.

## Việc cần làm

1. `SolutionOptionCompare({solution, selectedId, onSelect, readOnly})`: lưới 1 đến 3 cột theo bề rộng (container query hoặc `ResizeObserver`), nút "So sánh" chuyển sang bảng; nếu `canApproveSolution` trả `needTwoOptions` hiện cảnh báo "Cần ít nhất 2 phương án" với nút "Sinh lại" (gọi `generate`).
2. `SolutionOptionCard`: tiêu đề, tóm tắt, danh sách ưu/nhược, công sức, rủi ro, huy hiệu "Khuyến nghị" (`recommended`), đánh dấu "Đã chọn" khi `solution.chosenOption===option.id` (sau duyệt: icon `Check` + `status-success`). `readOnly` thì không chọn được.
3. `SolutionComparisonTable`: hàng là tiêu chí, cột là phương án; ô khác biệt có icon nhỏ và `aria-label`; thiếu dữ liệu "Không có dữ liệu".
4. Bàn phím: `ArrowLeft/ArrowRight` đổi thẻ khi nhóm radio có tiêu điểm, `Space`/`Enter` chọn; không phím tắt toàn cục. Giữ lựa chọn khi `refetch` nếu phương án còn.
5. `selectedId` là state cục bộ của `SolutionPanel`; khi `conflict` và phương án đã biến mất thì xoá lựa chọn và báo.

## Kiểm thử

- Component: 1/2/3 phương án; chọn đổi `aria-checked`; chuyển Cards ↔ Table; khoá khi `readOnly`; cảnh báo dưới 2 phương án; phương án thiếu trường; điều hướng bàn phím; lựa chọn giữ sau refetch.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/solution/SolutionOption src/renderer/src/components/request/solution/SolutionComparison`.

## Tiêu chí hoàn thành

- [ ] 2+ phương án hiện thẻ và bảng; 1 phương án (change_request) không duyệt được.
- [ ] Phương án đã chọn đánh dấu rõ sau `approved`.
- [ ] Truy cập bằng bàn phím; không chỉ dựa màu.
- [ ] i18n `SolutionOptionCard.*`, `SolutionOptionCompare.*`, `SolutionComparisonTable.*` đủ 5 locale.

## Rủi ro và lưu ý

- Số tiêu chí chung của phương án phụ thuộc schema chưa chốt.
- Màn hẹp: bảng cuộn ngang trong khung (`scrollbar-sleek` theo `check-styled-scrollbars`, kiểm quy ước ở `guides/STYLEGUIDE.md`).
