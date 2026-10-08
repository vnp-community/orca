# Solutions backend: request-artifact-model (v6)

> 🚧 SOL-027: 6/8 task xong (027-01 đến 06; 027-07, 027-08 một phần). SOL-028: 6/8 task xong (028-01 đến 05 và 08; 028-06, 028-07 một phần). Kiểm chứng 2026-10-08, xem [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md).

## Bảng CR, Solution, Task

| CR | Solution | Task | Ghi chú |
|----|----------|------|---------|
| [CR-REQ-027](../../../../../../docs/crs/v6/request-artifact-model/CR-REQ-027-artifact-schema-and-ontology.md) | [BE-REQ-SOL-027](./BE-REQ-SOL-027-artifact-schema-ontology-and-task-specs.md) | TASK-REQ-027-01 đến 08 | Schema `schema_version`, `request_revisions`, `artifact_index`, `artifact_relations`, `request_coverage`, `task.task_specs` và 3 RPC task-service, bản chiếu, kiểm ngữ nghĩa |
| [CR-REQ-028](../../../../../../docs/crs/v6/request-artifact-model/CR-REQ-028-clarification-and-decision-records.md) | [BE-REQ-SOL-028](./BE-REQ-SOL-028-clarification-decision-and-awaiting-information.md) | TASK-REQ-028-01 đến 08 | `awaiting_information`, Clarification, Definition of Ready, Decision, xác nhận lần hai, thông báo · 🚧 5/8 (028-01 đến 05 xong 2026-10-08; 028-06, 07, 08 một phần) |

Frontend và agent: N/A trong thư mục này (UI hỏi đáp và bảng phủ do solution frontend; agent không đổi).

## Thứ tự phụ thuộc

```
SOL-002, 003, 007, 009, 010 ─▶ SOL-027 ─▶ SOL-028 ─▶ (CR-REQ-029: hợp đồng thực thi)
SOL-011, 012 ───────────────────▲  │
                                   └──▶ SOL-026 (bản chiếu, RenderArtifact)
```

SOL-027 làm trước TASK-REQ-007-02 và TASK-REQ-012-03 để không phải migration dữ liệu (đổi `open_questions`, `assumptions`, thêm `requirement_coverage`, `Satisfies`). SOL-028 cần `ValidateRequestContent` và `AppendRequestRevision` của SOL-027 (task 027-05).

## Quyết định chung

| # | Quyết định | Lý do |
|---|-----------|-------|
| M1 | Mô hình chuẩn là JSON trong DB; Markdown/YAML chỉ là bản chiếu | Một nguồn sự thật |
| M2 | Request đổi nội dung bằng `request_revisions`, chỉ một use case ghi cột nội dung (có test `go/parser`) | Truy vết, chống duyệt nội dung đã đổi |
| M3 | `task.task_specs` là bảng riêng, khoá bằng `locked_at` khi Plan được duyệt (consumer `approval.decided`) | `tasks` là đường đọc nóng; khoá không trong transaction của `OnApproved` |
| M4 | Hai mức kiểm Request: `draft` lúc tạo, `ready` (Definition of Ready) | Jira và GitHub không có AC |
| M5 | Một trạng thái `awaiting_information`; đích quay lại ở `clarifications.resume_status`; `NextStatusWithResume` thay vì đổi chữ ký `NextStatus` | Không vỡ SOL-003 |
| M6 | Decision tách khỏi `solutions.chosen_option` và `approvals`; rủi ro cao cần xác nhận lần hai | Cần lý do, người chọn, lịch sử |
| M7 | Danh tính máy không xác nhận Decision, không duyệt | Cơ chế nhận biết nguồn máy chưa có |

## Số migration

`request-service/migrations` chưa tồn tại lúc soạn và số của CR-REQ-001/002/004/006 chồng nhau (README `request-artifact-model` của docs). Hai solution **không tự cấp số**: task 027-01 và 028-01 ghi `NNNN`, bắt buộc `ls` thư mục thật lúc làm, lấy số kế tiếp cùng cho hai dialect; 027-01 đi trước 028-01 (cả hai đổi bảng `requests`). `task-service`: số sau `0016` của SOL-011 (ghi `00NN` ở task 027-02).

## Mâu thuẫn và điểm chưa kiểm chứng

- **JSON Schema:** CR-REQ-027 nói chưa có thư viện; thực tế `services/api-gateway/go.mod` có trực tiếp `github.com/google/jsonschema-go v0.4.3`. Cần spike (task 027-03).
- **`open_questions`, `assumptions`:** SOL-007 định nghĩa `string[]`; CR-REQ-027 đổi thành đối tượng (Q-n, A-n) và CR-REQ-028 dựa vào đó. Phải sửa hợp đồng trước TASK-REQ-007-02.
- **`NextStatus` không có đích động:** CR-REQ-028 cần `ResumeStatus`; giải bằng `NextStatusWithResume` (SOL-028 C2).
- **README v6 mục 3.3** liệt kê 11 trạng thái; CR-REQ-028 thêm `awaiting_information` (12). Test hợp đồng trạng thái (TASK-REQ-003 mục 5) cần cập nhật cùng lúc; test đối chiếu README ở task 028-08 sẽ đỏ tới khi README đổi.
- **Hai CR cùng sửa `notification-service`:** CR-REQ-010 và CR-REQ-028 cùng sửa `consumer.go`, `notification_event.go`; làm task 028-06 sau hoặc cùng PR với TASK-REQ-010-05.
- **`CreateRequest`** (SOL-004) phải ghi `request_revisions` revision 1 cùng transaction (task 027-05).
- **Khoá spec** chưa chốt phạm vi (Q3 của SOL-027): hiện chỉ `title` và `task_specs`; `description` chưa có trong `UpdateTaskInput`.
- **`TxRunner.RunInTx` của task-service** không có chỗ cho `TaskSpecRepository`: dùng phương thức mới `RunInTxWithSpecs`, không đổi chữ ký cũ.
- **Chưa chạy** bất kỳ test hay migration nào; mọi con số (giới hạn kích thước, hạn Clarification, 3 vòng, ngưỡng `high`) là đề xuất.
