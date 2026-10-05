# Approval — Change Requests (v6)

> Cổng duyệt tổng quát cho mọi cấp của luồng Request (loại, Solution, Findings, Answer, Plan, Phase, danh sách task, trước deploy): thực thể, API, vòng đời, chính sách ai được duyệt, thông báo, hết hạn. Hợp đồng chung ở [README v6](../README.md). `DecisionGate` của `orchestration-service` giữ riêng (README O4).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-009](./CR-REQ-009-generic-approval-domain-and-api.md) | Không có thực thể duyệt tổng quát; `DecisionGate` không kiểm quyền, gắn với điều phối, không UI; cần máy trạng thái và hiệu ứng lên Request | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-010](./CR-REQ-010-approval-authorization-notification-expiry.md) | Chưa có chính sách ai được duyệt theo loại/size, tách nhiệm vụ, thông báo, hết hạn | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-002, 003 ──▶ CR-REQ-009 ──▶ CR-REQ-010
```

| Bước | Lý do thứ tự |
|------|--------------|
| 009 trước | Tạo bảng `approvals`, máy trạng thái, `ApprovalService`, cơ chế `SubjectHandler`; kèm người duyệt tạm (`role=admin` hoặc người báo cáo) để các CR 005, 007, 008 chạy được |
| 010 sau 009 | Thay người duyệt tạm bằng chính sách (`approval_policies`, `approval_approvers`), thêm tách nhiệm vụ, thông báo, hết hạn, nhắc; sửa nhỏ `notification-service` |

## Phân chia sở hữu

| Hạng mục | CR |
|----------|----|
| Bảng `approvals`, trạng thái `pending|approved|rejected|cancelled|expired`, `ApprovalService`, sự kiện `approval.requested`/`approval.decided` | CR-REQ-009 |
| `Approve`/`Reject` kích hoạt chuyển trạng thái Request (gọi `TransitionRequest` của CR-REQ-003) | CR-REQ-009 |
| Ai được duyệt theo `subject_type`/loại/size, tách nhiệm vụ, thông báo, hết hạn `due_at`, nhắc | CR-REQ-010 |
| Nội dung chủ thể và `SubjectHandler` cụ thể | 005 (`request_type`), 007 (`solution`), 008 (`findings`, `answer`), 012/013 (`plan`, `task_list`, `phase`), 014 (`pre_deploy`) |

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| P1 | `Approve`, cập nhật chủ thể và chuyển trạng thái Request cùng một transaction; hiệu ứng sang `task-service` qua outbox | Không có trạng thái nửa vời; không gọi gRPC trong transaction |
| P2 | Mỗi `subject_type` có một `SubjectHandler` đăng ký lúc khởi động; thiếu thì service không chạy | Approval điều phối, CR sở hữu nội dung cài logic |
| P3 | Một Approval `pending` cho mỗi chủ thể (partial index Postgres; cột sinh trên MySQL) | Chặn cổng trùng; cùng ngữ nghĩa hai DB |
| P4 | Bốn trạng thái cuối bất biến; sửa chủ thể tạo dòng mới | Lịch sử audit |
| P5 | `subject_digest` chống duyệt nội dung đã đổi; `Reject` bắt buộc `comment` | Tiền lệ `ParamsHash` của `mcp-service`; lý do từ chối là đầu vào cho bước sau |
| P6 | Chính sách lưu DB, đánh giá bằng Go thuần; người duyệt chụp lúc mở vào `approval_approvers` | Quy tắc phụ thuộc thuộc tính Request; join portable cho hai DB |
| P7 | Agent/MCP không được duyệt; thông báo không chứa `comment` | Cổng tồn tại để người kiểm soát agent; thông báo được lưu |
| P8 | Tên tách khỏi MCP: subject `orca.request.approval.*`, kiểu thông báo `request.approval_*` | `mcp-service` đã dùng `orca.mcp.approval.requested` và `mcp.approval`, và trạng thái `denied` thay `rejected` |

## Điểm lệch với README v6 (đã ghi ở mục "Câu hỏi mở" của từng CR)

- README 3.5 thiếu các cột `subject_digest`, `self_approval_allowed`, `idempotency_key`, `reminded_at`, `created_at`, `updated_at` của `approvals`; `stage` không có định nghĩa.
- README 3.7 không có sự kiện nhắc hay hết hạn riêng; feature dùng `reason` trong `approval.requested` và `decision` trong `approval.decided`.
- README 3.6 không có RPC quản trị chính sách hay gia hạn Approval.
- Quyền duyệt ở mức tổ chức chưa tồn tại (chỉ `admin|user` toàn cục và team); feature dựng người duyệt đặc biệt bằng team.
- Cách nhận biết lời gọi từ agent MCP để chặn tự duyệt chưa có (cần CR-REQ-016/017).
