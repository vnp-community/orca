# Request Artifact Model: Change Requests (v6, bổ sung)

> Mô hình dữ liệu chuẩn cho Request, Solution, Plan, Phase, Task (schema có phiên bản, ID, quan hệ, tiêu chí chấp nhận, bảng phủ, provenance, bản chiếu Markdown/YAML) và vòng hỏi lại (Clarification, Decision, trạng thái `awaiting_information`). Hợp đồng chung ở [README v6](../README.md); nguồn nghiên cứu: [`artifact-formats-ontology-and-execution-readiness.md`](../../../research/receive-request/artifact-formats-ontology-and-execution-readiness.md).

> Các CR này **không sửa** CR hiện có hay README v6. Mỗi CR có mục 9 "Tác động tới CR hiện có" liệt kê chính xác phần cần sửa; việc sửa do người duyệt series thực hiện.

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-027](./CR-REQ-027-artifact-schema-and-ontology.md) | Mỗi CR tự định nghĩa JSON; không có `schema_version`, ID chéo, tiêu chí chấp nhận `AC-n`, bảng phủ, provenance, bản chiếu cho công cụ ngoài; Request đã duyệt sửa tại chỗ được | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-028](./CR-REQ-028-clarification-and-decision-records.md) | Không có vòng hỏi lại (chỉ `request_backlog`); `chosen_option` không ghi ai chọn, vì sao, không xác nhận lần hai cho lựa chọn rủi ro cao | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-002, 003, 007, 009 ──▶ CR-REQ-027 ──▶ CR-REQ-028 ──▶ (CR-REQ-029: hợp đồng thực thi, CR-REQ-026: OpenSpec)
```

| Bước | Lý do thứ tự |
|------|--------------|
| 027 trước | Định nghĩa `ValidateRequestContent`, `request_revisions`, `open_questions` và `assumptions` có cấu trúc; CR-REQ-028 và 026 đứng trên các thứ này |
| 028 sau 027 | Dùng kiểm tra mức `ready` làm Definition of Ready, ghi câu trả lời bằng `AppendRequestRevision` |
| Hai CR này phải vào trước khi CR-REQ-007 và 012 được triển khai | Cả hai đổi hợp đồng của CR-REQ-007 (`options`, `open_questions`, `assumptions`, `ChooseSolutionOption`) và CR-REQ-012 (`PlanProposal`, `CreatePlanTree`); làm sau thì thành migration dữ liệu |

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| M1 | Mô hình chuẩn là JSON trong DB; Markdown/YAML là bản chiếu, chỉ tin vùng Orca và khối `orca-json` | Một nguồn sự thật; văn bản tự do từ agent không tin cậy |
| M2 | Request đổi nội dung bằng `revision` (bảng `request_revisions`), không ghi đè; Solution và Plan ghi revision đã dùng | Truy vết và chống duyệt nội dung đã đổi |
| M3 | Nội dung Task có cấu trúc nằm ở bảng riêng `task.task_specs` (`task-service`), khóa khi Plan được duyệt | Tránh phình đường đọc nóng của `tasks`; MySQL không lập chỉ mục JSON |
| M4 | Kiểm Request hai mức: `draft` (lúc tạo) và `ready` (Definition of Ready) | Jira và GitHub không có AC; không từ chối lúc nhận |
| M5 | Một trạng thái `awaiting_information`, đích quay lại lưu ở Clarification; hết hạn về `request_backlog` (`missing_info`) | Không nhân trạng thái theo nguồn câu hỏi |
| M6 | Decision tách khỏi `solutions.chosen_option` và `approvals`; rủi ro cao cần xác nhận lần hai | Cần lý do, người chọn, lịch sử |
| M7 | Danh tính máy (MCP, dịch vụ) không xác nhận Decision, không duyệt | Cùng lý do CR-REQ-010: cơ chế nhận biết nguồn chưa có |

## Điểm lệch và việc cần chốt khi duyệt

- **Số hiệu migration của `request-service` đang chồng nhau giữa các CR hiện có:** CR-REQ-001 dùng `0001_init`, `0002_processed_events`, `0005_outbox`; CR-REQ-002 dùng `0002_request_core`; CR-REQ-004 dùng `0003_request_source_hints`; CR-REQ-006 dùng `0004_request_return_history`; CR-REQ-007 ghi "số do CR-REQ-002 cấp", CR-REQ-014 ghi "migration kế tiếp". Hai CR này cũng ghi "số chốt khi triển khai" và không tự cấp số. `task-service` tiếp theo là `0015`, `0016` (CR-REQ-011), `task_specs` lấy số sau đó.
- **Trùng lặp hợp đồng giữa 007 và 027:** `options` và `chosen_option` (CR-REQ-002 mặc định `'[]'`, CR-REQ-007 dùng đối tượng bọc) vẫn là câu hỏi mở 4 của CR-REQ-007; CR-REQ-027 giả định đối tượng bọc và thêm trường.
- **Hai CR cùng sửa `notification-service`:** CR-REQ-010 (Approval) và CR-REQ-028 (Clarification) cùng thêm vào `Subjects` của `internal/adapter/eventbus/consumer.go` và `subjectRules` của `internal/domain/notification_event.go`. Nên gộp lúc triển khai để không xung đột tệp.
- **Quyền ghi và quyền hỏi tay ở mức Request** vẫn chưa ai chốt (README v6 mục 8, cuối); CR-REQ-028 mục 7 câu 1.
- **README v6 mục 3.3 liệt kê 11 trạng thái;** CR-REQ-028 thêm `awaiting_information` (12). Test hợp đồng "tên trạng thái trong mã khớp README" (CR-REQ-003 mục 5) phải cập nhật cùng lúc.
- **Chưa kiểm chứng:** thư viện JSON Schema chưa có trong `go.mod`; hạn mặc định của Clarification (7 ngày, 72 giờ, 24 giờ) là đề xuất; chưa chạy hệ thống nào.
