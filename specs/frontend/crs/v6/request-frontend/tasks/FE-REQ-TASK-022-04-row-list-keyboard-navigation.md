# FE-REQ-TASK-022-04: Hook điều hướng bằng phím `j`/`k`/`Enter` cho danh sách hàng

**From Solution:** [FE-REQ-SOL-022](../solutions/FE-REQ-SOL-022-approval-inbox.md) mục 2.5 (D6)
**Priority:** P1
**Area:** frontend (hooks, tiếp cận)
**File:** `frontend/src/renderer/src/hooks/useRowListKeyboardNavigation.ts` (mới), `useRowListKeyboardNavigation.test.tsx` (mới)
**Depends on:** không (độc lập); được CR-REQ-023 (FE-REQ-TASK-023-04) dùng lại
**Status:** [x] DONE (verified 2026-10-07: useRowListKeyboardNavigation.test.tsx 4/4; oxlint sạch, không thêm lỗi tsc ở file của task)

## Context

- Repo chưa có bộ xử lý `j`/`k` (đã grep `key === 'j'` trong `renderer/src`, không có). Phím tắt toàn cục của app đăng ký ở nơi khác; hook này **chỉ** gắn `onKeyDown` vào container danh sách, nên không xung đột với phím toàn cục khi tiêu điểm ở ngoài danh sách.
- `lib/screen-submit-shortcut.ts` cho thấy cách repo xử lý IME: kiểm `event.isComposing` và `event.nativeEvent?.isComposing`.
- CR-022 và CR-023 đều quy định: `j`/`k` di chuyển, `Enter` mở; bỏ qua khi tiêu điểm ở ô nhập. Không có phím sửa đổi nên không có nhánh Mac/Windows; `Mod+Enter` thuộc `RejectReasonDialog`, không thuộc hook này.

## Việc cần làm

1. Hook: `useRowListKeyboardNavigation<T>({ items, getKey, onOpen, enabled = true })` trả `{ containerProps, getRowProps, activeKey }`:
   - `containerProps`: `{ role: 'listbox' | undefined, tabIndex: 0, onKeyDown, onFocus }` (dùng `role="listbox"` và `role="option"` cho hàng nếu danh sách dạng thẻ; bảng `table` dùng `role="grid"` do component quyết, hook nhận `role` qua tham số tuỳ chọn `containerRole`).
   - `getRowProps(key)`: `{ 'data-row-key', 'aria-selected', tabIndex: active ? 0 : -1, ref }` (roving tabindex), và cuộn hàng đang chọn vào khung nhìn (`scrollIntoView({block:'nearest'})`).
2. `onKeyDown`: bỏ qua khi `event.defaultPrevented`, `isComposing`, `ctrlKey|metaKey|altKey|shiftKey`, hoặc `event.target` là phần tử nhập liệu (`input`, `textarea`, `select`, `[contenteditable=""|"true"]`, hoặc nằm trong `[role="dialog"]`). `j` hoặc `ArrowDown` tăng, `k` hoặc `ArrowUp` giảm (dừng ở đầu/cuối, không vòng); `Home`/`End` tuỳ chọn; `Enter` gọi `onOpen(item)` cho hàng đang chọn và `preventDefault`.
3. Khi `items` đổi: nếu `activeKey` biến mất thì chọn hàng gần nhất cùng vị trí; nếu rỗng thì `activeKey = null`.
4. Nút bên trong hàng (Duyệt nhanh, Từ chối, liên kết) vẫn nhận `Enter` riêng: bỏ qua khi `event.target` là `button` hoặc `a` khác với hàng (tránh `Enter` trên nút Từ chối lại "mở" hàng).
5. Xuất kèm `isTypingTarget(target: EventTarget | null): boolean` để CR-REQ-023 dùng cho phím `1/2/3` (đặt trong cùng file; không tạo file `utils`).

## Kiểm thử

`useRowListKeyboardNavigation.test.tsx` (happy-dom, `@testing-library/react`, `fireEvent.keyDown`):
- `j`, `k`, `Enter` trên danh sách 3 hàng; dừng ở biên.
- `j` bị bỏ qua khi `target` là `<input>`; khi có `ctrlKey`/`metaKey`; khi `isComposing`.
- `Enter` trên `<button>` trong hàng không gọi `onOpen`.
- Xoá hàng đang chọn chuyển lựa chọn sang hàng gần nhất.
- Hai nền tảng: không cần giả `navigator.userAgent` vì không dùng phím sửa đổi (có một test khẳng định `metaKey` trên "Mac" và `ctrlKey` trên "Windows" cùng bị bỏ qua).

Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/hooks/useRowListKeyboardNavigation.test.tsx`.

## Tiêu chí hoàn thành

- [ ] `j`/`k`/`Enter` chỉ hoạt động khi tiêu điểm nằm trong container.
- [ ] Bị bỏ qua trong ô nhập, trong `dialog`, khi IME đang soạn, khi có phím sửa đổi.
- [ ] Roving tabindex: đúng một hàng có `tabIndex=0`.
- [ ] Không dùng `any`; không thêm phím toàn cục.

## Rủi ro và lưu ý

- `j`/`k` có thể trùng với phím của trình đọc màn hình ở chế độ duyệt; mũi tên cũng được hỗ trợ để không phụ thuộc `j`/`k`.
- Hook dùng chung nên đặt tên theo khái niệm "danh sách hàng" (đã tránh `utils`/`helpers`).

## Ghi chú triển khai (2026-10-07)

- `isTypingTarget` xuất cùng file; thêm `ArrowUp/ArrowDown/Home/End`; `shiftKey` cũng bị bỏ qua.
