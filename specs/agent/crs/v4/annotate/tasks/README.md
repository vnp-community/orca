# agent Tasks — Annotate AI Diffs (F08, v4)

**Solutions:** [../solutions/](../solutions/README.md)

Không có task nào trong thư mục này.

## Why there are no tasks

[SOL-AG-ANNOTATE-001](../solutions/SOL-AG-ANNOTATE-001-zero-agent-scope.md)
— solution duy nhất, dùng chung cho cả CR-ANNOTATE-001 và CR-ANNOTATE-002 —
là assessment-only. Nó kết luận tường minh: *"`agent/` requires zero code
changes for either CR-ANNOTATE-001 or CR-ANNOTATE-002."* Cả 2 CR chỉ đổi
nơi lưu dữ liệu (`frontend`↔`backend-go`) và nơi compose prompt
(`frontend`↔`backend-go`) — không đổi cách byte được ghi vào PTY của agent,
cơ chế đó (`terminal.send`, dùng chung với F02 Terminal Splits) không đổi ở
cả 2 CR.

Tạo task cho 1 solution mà toàn bộ nội dung là "không cần sửa gì" sẽ là
bịa ra việc không tồn tại — mirrors
`specs/agent/crs/v4/task-graph/tasks/README.md`'s "Why SOL-AG-TG-001 has no
tasks" và `specs/agent/crs/v4/mobile-companion/`'s solutions-only structure
(không có thư mục `tasks/` cho các CR zero-scope).

Nếu 1 CR tương lai trong nhóm `annotate` thực sự cần `agent/` làm gì đó
(vd. agent tự phát hiện 1 định dạng review-feedback đặc biệt để auto-ack),
đó là thay đổi kiến trúc cần CR + solution riêng, không phải task ẩn trong
thư mục này.
