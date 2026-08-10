# TASK-BIGFILE-007 — Investigate: `registerPtyHandlers` (còn lại sau TASK 001–006)

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** M
**Phụ thuộc:** TASK-BIGFILE-001..006 đã xong (file nguồn lúc này còn ~3,730
dòng thay vì 5,185)
**Status:** ⛔ Blocked (chặn bởi 001–006, xem ghi chú dưới)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md`

## ⛔ Blocked (2026-08-10) — chưa thể bắt đầu

TASK-001–006 mà task này phụ thuộc đều bị chặn (state module-private dùng
chéo pervasive, xem `TASKS-INDEX.md` § "Phát hiện khi thực thi") — file
nguồn KHÔNG được rút gọn xuống ~3,730 dòng như giả định ở trên. Khuyến
nghị: gộp luôn phạm vi Investigate của task này với toàn bộ nhóm 001–006
thành 1 task Investigate duy nhất cho `ipc/pty.ts` (thay vì 6 task Move +
1 task Investigate tách rời), vì ranh giới thực tế không tách được theo
từng hàm đơn lẻ.

## Input (đọc đúng phạm vi này, không cần đọc lại phần đã tách ở task 001-006)

- File nguồn: `frontend/src/main/ipc/pty.ts`
- Đọc **dòng 1 đầy đủ** (comment `eslint-disable max-lines` — bị cắt trong
  các bug/solution doc trước đó, đọc trọn vẹn ở đây):
  > "PTY IPC is intentionally centralized in one main-process module so
  > spawn-time environment scoping, lifecycle cleanup, foreground-process
  > inspection, and renderer IPC stay behind a single audited boundary.
  > Splitting it by line count would scatter tightly coupled terminal ..."
  (đọc tiếp phần bị cắt trực tiếp trong file).
- Đọc **dòng 1459–5150** (`registerPtyHandlers`, sau khi 001–006 đã tách,
  đây là phần LỚN NHẤT còn lại của file — ~3,691 dòng gốc, KHÔNG đổi bởi
  task 001-006 vì chúng nằm trước dòng 1459).

## Nhiệm vụ

1. Xác định các nhóm IPC channel theo domain bên trong `registerPtyHandlers`
   (gợi ý: create/attach, write/resize, serialize/scrollback, signal/kill —
   xác nhận lại tên nhóm thực tế khi đọc, không dùng nguyên gợi ý này nếu
   không khớp).
2. Xác định xem comment dòng 1 có nêu ràng buộc kỹ thuật cụ thể (vd: thứ tự
   đăng ký IPC channel quan trọng, không được tách) hay chỉ là lý do tổ chức
   chung — quyết định này ảnh hưởng trực tiếp có nên tách hay không.
3. Nếu xác nhận tách được: viết ra danh sách nhóm channel + dòng bắt đầu/kết
   thúc mỗi nhóm trong `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md` (bổ
   sung mục mới "Giai đoạn 2"), rồi tạo các task Move mới
   (`TASK-BIGFILE-036`, `037`, ... tiếp theo dãy số hiện có trong
   `TASKS-INDEX.md`) theo đúng format các task Move đã có (xem
   TASK-BIGFILE-001 làm mẫu).
4. Nếu xác nhận **không nên tách** (có ràng buộc kỹ thuật thật ngăn cản):
   ghi rõ lý do vào `BUG-FE-BIGFILE-011-ipc-pty.md` (mục mới "Quyết định:
   không tách `registerPtyHandlers`") kèm bằng chứng cụ thể trích từ file, để
   không ai lặp lại việc điều tra này trong tương lai.

## Output

- Cập nhật `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md` VÀ/HOẶC
  `../BUG-FE-BIGFILE-011-ipc-pty.md` theo đúng 1 trong 2 nhánh ở bước 3/4.
- (Nếu tách được) Task Move mới trong `TASKS-INDEX.md`.

## Không làm trong task này

Không sửa code. Đây là task đọc + viết tài liệu, không đổi hành vi ứng dụng.
