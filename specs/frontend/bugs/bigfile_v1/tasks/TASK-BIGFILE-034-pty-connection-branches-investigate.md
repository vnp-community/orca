# TASK-BIGFILE-034 — Investigate: `connectPanePty` (~6,650 dòng, không có ranh giới sẵn)

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-033 đã xong (bắt buộc — cần test che phủ trước
khi đọc/thiết kế split cho 1 hàm có lịch sử race-condition)
**Status:** ✅ Done (2026-08-11) — kết luận: KHÔNG sinh task Move con, xem
`../solutions/SOLUTION-FE-BIGFILE-006-pty-connection.md` Bước 1–2
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-006-pty-connection.md`
(Bước 1–2)

## Input

- File nguồn: `frontend/src/renderer/src/components/terminal-pane/pty-connection.ts`
- Đọc **toàn bộ hàm `connectPanePty`** (dòng 943–hết file, ~6,650 dòng) —
  không có ranh giới export sẵn để trích đoạn nhỏ hơn, đây là lý do task này
  cần đọc nhiều hơn các task Investigate khác trong đợt.

## Nhiệm vụ

1. Xác định các nhánh theo loại transport (gợi ý: local / SSH / remote-
   runtime / retry-cold-start-reattach dùng chung — xác nhận tên nhánh thực
   tế khi đọc).
2. Với mỗi nhánh, ước tính số dòng, xác định phụ thuộc closure/state chia sẻ
   với nhánh khác.
3. Đánh giá 2 phương án (theo đúng gợi ý trong solution doc):
   - **Phương án A (strategy pattern)**: tách mỗi nhánh thành
     `ConnectPtyStrategy` riêng file, `connectPanePty` chỉ còn hàm điều phối
     ngắn gọn.
   - **Phương án B (nhẹ hơn)**: giữ `connectPanePty` là 1 hàm, chỉ trích các
     khối lớn (vd nhánh remote-runtime) thành hàm con export riêng, gọi từ
     `connectPanePty`.
   Chọn phương án dựa trên mức độ chia sẻ state phát hiện ở bước 2 — nếu
   chia sẻ nhiều, chọn B (an toàn hơn).

## Output

- Cập nhật `../solutions/SOLUTION-FE-BIGFILE-006-pty-connection.md` với kế
  hoạch cụ thể: phương án chọn (A hoặc B) + danh sách nhánh + dòng bắt
  đầu/kết thúc mỗi nhánh.
- Task Move mới (`TASK-BIGFILE-036`, ... tiếp theo dãy số hiện có) cho từng
  nhánh — **thứ tự đề xuất: local trước (rủi ro thấp nhất) → SSH → remote-
  runtime (rủi ro cao nhất, vừa có bug gần đây) → cuối cùng rút gọn
  `connectPanePty`**. Mỗi task Move phải yêu cầu chạy lại test từ
  TASK-BIGFILE-033 sau khi tách, KHÔNG chỉ typecheck.

## Không làm trong task này

Không sửa code. Đây là file rủi ro cao nhất trong toàn bộ `bigfile_v1` (xem
`../BUG-FE-BIGFILE-006-pty-connection.md`) — mọi task Move sinh ra từ đây
phải test kỹ hơn mức bình thường, lý tưởng có thêm bước test thủ công trên
môi trường thật (tương tự cách investigation `BUG-FE-PTY-001` đã làm).
