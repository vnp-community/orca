# Plan, Phase, Task — Change Requests (v6)

> Dùng lại Task làm Plan và Phase (D2), sinh cây từ Solution đã duyệt, thực thi theo Phase và phản hồi ngược lên Request, rồi các chính sách riêng theo loại. Xem bối cảnh, quyết định D1 đến D5 và hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-011](./CR-REQ-011-task-service-plan-phase-task-types.md) | `task_type` bị CHECK khoá, `ListTasks` chỉ lọc project, plan/phase cần trạng thái suy ra và không có số task | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-012](./CR-REQ-012-plan-phase-task-generation-from-solution.md) | Chưa có use case sinh Plan, Phase, Task từ Solution; `AIApply` không biết Plan, nhãn, `request_id` | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-013](./CR-REQ-013-phase-execution-and-feedback-loop.md) | `task-service` không phát sự kiện khi task chạy, lỗi, xong; chưa có `StartPhase` và vòng phản hồi | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-014](./CR-REQ-014-type-specific-execution-policies.md) | `hotfix`, `security`, `performance`, `ops_request`, `refactor` cần cổng và kiểm tra riêng | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-011 (task-service: type, request_id, lọc, cascade)
    └──▶ CR-REQ-012 (GeneratePlan, CreatePlanTree)
              └──▶ CR-REQ-013 (StartPhase, sự kiện task, ReportTaskOutcome, backlog khi lỗi)
                        └──▶ CR-REQ-014 (TypePolicy cho năm loại)
```

CR-REQ-011 chạy được độc lập với `request-service` và nên merge đầu tiên (migration 0015, 0016 của `task-service`). CR-REQ-012 cần Solution đã duyệt (CR-REQ-007), Approval (CR-REQ-009) và registry (CR-REQ-003). CR-REQ-013 có phần sửa `task-service` (sự kiện cùng transaction) có thể làm song song với CR-REQ-012. CR-REQ-014 chỉ móc vào các điểm `TypePolicy` mà CR-REQ-012 và 013 để sẵn, nên có thể giao sau mà không đổi đường chuẩn.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| P1 | O1: trạng thái plan/phase suy ra từ con, tính lại từ toàn bộ con mỗi lần, ghi qua repository (không qua `SetStatus`) | Giữ `ErrCannotSetInProgress`; chịu giao lặp; một nguồn sự thật |
| P2 | O2: plan/phase không nhận `TaskNumber`; sửa ở hai hàm `Create` của adapter | Chỗ duy nhất cấp số; cột đã nullable từ migration `0008` |
| P3 | `ExecuteTask` bị chặn trên plan/phase; chạy task lá, tuần tự, một worktree cho cả Plan | `selectEngine` và `ComplexExecutor` chỉ mở một cấp con; task sau cần thấy code task trước |
| P4 | Cột `request_id` nullable, không FK, trên mọi task thuộc Request; `ListTasks` mặc định ẩn plan/phase | Hợp đồng README 3.5; tương thích ngược với client cũ |
| P5 | Sự kiện task phát cùng transaction với ghi status, chỉ cho task có `request_id`, tái dùng subject `orca.task.task.statuschanged` | Mất sự kiện lỗi làm Request kẹt; tránh nhiễu; subject đã được WS bridge và notification-service nhận |
| P6 | Nhãn `Task.Labels` mang ý nghĩa gate và kiểm tra (`gate:pre_deploy`, `check:*`, `rollback`, `test:regression`) | Không thêm cột ở `task-service`; một bảng nhãn ở CR-REQ-012 |
| P7 | Đề xuất rồi mới lưu (`GeneratePlan` hai mode), lưu nguyên tử bằng RPC `CreatePlanTree` mới | Theo mẫu `AIDecompose`/`AIApply`; nhiều lần `CreateTask` rời không nguyên tử |
| P8 | Số đo và kết quả kiểm có cấu trúc ở bảng `request_checks` của `request-service` | `task-service` không lưu đầu ra có cấu trúc |

## Phạm vi ngoài feature này

Approval (CR-REQ-009, 010), máy trạng thái Request và registry luồng (CR-REQ-003), Solution và Chẩn đoán (CR-REQ-007, 008), `ReturnToBacklog`, `SpawnChildRequest` (CR-REQ-006), view backlog (CR-REQ-015), kênh gateway (CR-REQ-016), MCP (CR-REQ-017), giao diện (CR-REQ-018 đến 023). CR trong feature này chỉ tham chiếu các CR đó bằng mã.

## Điểm lệch giữa README v6 và code, phát hiện khi viết feature này

- README v6 mục 3.1 và `request-classification-and-flows.md` cho AI chạy qua `ai-provider-service`; code chỉ có `ResolveProvider` và quản lý account. AI hoàn thành văn bản đi qua relay `ai.complete` của Dev Server Agent (`task-service/internal/adapter/grpcclient/aidecompose_relay.go`) và cần project có dev server đang kết nối. CR-REQ-012 theo đường relay.
- README v6 mục 3.5 không có `returned_category` và `request_return_history` (CR-REQ-006 đã thêm), cột `approvals.subject_digest`, `idempotency_key`, `created_at` (CR-REQ-009 đã thêm); CR-REQ-012 và 013 dựa vào các cột này.
- README v6 mục 3.6 không có RPC `CommitPlan`, `RecordRequestCheck`, `ListRequestChecks`, và `task-service` cần `CreatePlanTree`, `ListExecutionStates`; mục 3.5 không có bảng `phase_starts`, `task_run_outcomes`, `request_checks`.
- README v6 mục 3.8: Execute backlog chỉ nói "dưới Phase đã approved", nhưng `bug`/`refactor` (size không L), `task`, `docs`, `security`, `performance`, `ops_request`, `hotfix` không có Phase. Xem CR-REQ-015.
- `task-service` không phát sự kiện nào khi task đổi status qua `ExecuteTask` hay `ReportTaskExecutionResult` (chỉ `UpdateTask` phát), nên chưa có gì để `request-service` tiêu thụ. CR-REQ-013 bổ sung.
- Chỉ task `review` (không `done`) sau khi agent xong; chỉ `done` mới mở khoá task phụ thuộc. CR-REQ-013 xử lý bằng cờ tự đặt `done`.
- gRPC `CreateTask` hiện bỏ `task_type`, `description`, `priority` (handler chỉ chuyển bốn trường). CR-REQ-011 sửa.
- Liên kết `docs/reference/git-compatibility.md` và `docs/STYLEGUIDE.md` trong README v6 mục 6 không tồn tại; file thật ở `/opt/repos/orca/guides/reference/git-compatibility.md` và `/opt/repos/orca/guides/STYLEGUIDE.md`.
