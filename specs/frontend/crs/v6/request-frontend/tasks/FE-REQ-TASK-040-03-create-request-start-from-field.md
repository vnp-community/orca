# FE-REQ-TASK-040-03: Ô "Start from" trong Create request (cả từ issue Jira/GitHub)

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `components/request/{StartFromField.tsx,start-from-state.ts}` (mới), `CreateRequestDialog.tsx` (sửa), `create-request-from-issue.ts` (kiểm tra, không đổi hợp đồng), test
**Depends on:** FE-REQ-TASK-040-01, 040-02
**Status:** [ ] TODO

---

## Context

- `CreateRequestDialog.submit` dựng tham số tạo; hộp thoại dùng chung cho tạo tay và `CreateRequestIssueButton`.
- `CreateRequestProjectRepoNote` đã nêu repo/host của dự án (repo đầu).

## Việc cần làm

1. `start-from-state.ts`: reducer thuần `default | custom(ref)` + hành động `select`, `useDefault`, `projectChanged`, `hostOffline`.
2. `StartFromField`: chip "Default branch (`<tên>`)" (tên từ `getRuntimeRepoBaseRefDefault` của repo đầu; chưa có thì "Default branch"), popover `RepoRefPicker`, nút "Use default branch", nhập tay khi offline (kiểm `validateBaseRefSyntax`, lỗi cụ thể), dòng giải thích offline.
3. Hiển thị ngay dưới `CreateRequestProjectRepoNote`; ẩn khi `!supportsBaseRef`, dự án chưa có repo, hoặc đang tải repo.
4. `submit`: thêm `baseRef` **chỉ khi** `custom`; sau thành công gọi `verifyBaseRefEcho`; nếu `mismatch` thì toast cảnh báo rõ "This server didn't apply the starting branch.".
5. `baseRefUnverified` trong kết quả tạo → toast thông tin kèm nút mở Request.
6. Lỗi `INVALID`/`NOT_FOUND` từ backend hiển thị dưới ô (kèm gợi ý), giữ hộp thoại mở, không mất dữ liệu đã nhập.
7. Đổi dự án trong hộp thoại đặt lại về mặc định.
8. Nhãn tiếng Anh, khoá i18n mới (hoàn thiện ở task 05), `aria-label` đầy đủ; Tab/Enter/Esc hoạt động.

## Kiểm thử

- Component: mặc định không gửi `baseRef`; chọn thì gửi; đổi dự án reset; `NOT_FOUND` giữ dữ liệu; offline nhập tay; ẩn theo `supportsBaseRef`; `mismatch` hiện cảnh báo.
- Đường issue: `CreateRequestIssueButton` mở hộp thoại có ô (Jira và GitHub); PR vẫn không có nút.
- Test `CreateRequestDialog` hiện có xanh.

## Tiêu chí hoàn thành

- [ ] Tạo Request với nhánh khác mặc định gửi đúng `baseRef`; không chọn thì không gửi gì.
- [ ] Gateway cũ: ô ẩn, tạo Request không đổi.
- [ ] Cả tạo tay và từ issue dùng chung ô.
- [ ] Lỗi hiển thị đúng, không mất dữ liệu nhập.

## Rủi ro và lưu ý

- Hộp thoại có thể quá dài trên màn hình thấp; kiểm bố cục ở 3 kích thước.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
