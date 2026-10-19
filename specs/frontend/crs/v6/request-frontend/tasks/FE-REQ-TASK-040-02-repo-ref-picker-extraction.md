# FE-REQ-TASK-040-02: Tách `RepoRefPicker` dùng chung, giữ `settings/BaseRefPicker`

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `components/git-ref-picker/{RepoRefPicker.tsx,use-repo-ref-search.ts}` (mới), `components/settings/BaseRefPicker.tsx` (sửa thành lớp mỏng), test
**Depends on:** Không
**Status:** [ ] TODO

---

## Context

- `BaseRefPicker` có logic tìm kiếm (debounce, giới hạn truy vấn), mặc định, đếm remote; props công khai `{repoId, currentBaseRef?, onSelect, onUsePrimary?}`.
- Dùng ở Settings và `SourceControl.tsx`; không được đổi hành vi của hai nơi này.

## Việc cần làm

1. Tách logic tìm ref/mặc định/đếm remote thành `use-repo-ref-search.ts` (hook) và phần trình bày thành `RepoRefPicker` với props `{repoId, value?, onSelect, onUseDefault?, placeholder?, disabled?}`; trạng thái `loading|empty|error|offline`.
2. `settings/BaseRefPicker` giữ chữ ký, gọi `RepoRefPicker`; test hiện có của nó xanh không sửa kỳ vọng.
3. Hỗ trợ bàn phím (mũi tên, Enter, Esc) và nhãn truy cập (`role=listbox`, `aria-activedescendant`).
4. Ref remote `a/b/c` hiển thị đầy đủ, cắt giữa nếu dài, có tooltip; không dùng màu làm tín hiệu duy nhất.
5. Tên file theo nội dung (không `helpers`/`utils`).

## Kiểm thử

- Test hook: debounce, huỷ yêu cầu cũ, giới hạn độ dài truy vấn, lỗi mạng, mặc định không bịa.
- Test hiện có của `BaseRefPicker` và `SourceControl` liên quan ref xanh.
- Test bàn phím và truy cập.

## Tiêu chí hoàn thành

- [ ] `RepoRefPicker` dùng được độc lập; Settings và SourceControl không đổi hành vi.
- [ ] Đủ trạng thái và truy cập bàn phím.

## Rủi ro và lưu ý

- `SourceControl.tsx` lớn: chỉ chạm qua `BaseRefPicker`, không sửa trực tiếp.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
