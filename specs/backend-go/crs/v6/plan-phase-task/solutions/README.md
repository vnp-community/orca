# plan-phase-task: solutions backend (BE-REQ-SOL-011 đến 014)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Tài liệu ngày 2026-10-06; số dòng và đường dẫn đã đối chiếu với code `backend-go/services/task-service` cùng ngày. `request-service` chưa có thư mục: mọi đường dẫn của nó là "(mới)".

Nguồn: [docs/crs/v6/plan-phase-task](../../../../../../docs/crs/v6/plan-phase-task/README.md). README v6 [mục 8](../../../../../../docs/crs/v6/README.md) thắng mục 3 khi mâu thuẫn. Tài liệu thiết kế tham chiếu: [`tdd/README.md`](../../../../tdd/README.md), `architecture/03, 05, 08, 09`, `services/task-service.md`, `services/orchestration-service.md`, `services/project-service.md`, `services/infra-fleet-service.md`.

## Bảng CR → Solution → Task

| CR | Solution | Service | Task (xem [tasks/README](../tasks/README.md)) |
|---|---|---|---|
| CR-REQ-011 type plan/phase, lọc, cascade, không số task, `request_id` | [BE-REQ-SOL-011](./BE-REQ-SOL-011-task-service-plan-phase-task-types.md) | `task-service` | TASK-REQ-011-01 đến 07 |
| CR-REQ-012 sinh Plan/Phase/Task, `CreatePlanTree` | [BE-REQ-SOL-012](./BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md) | `task-service`, `request-service` | TASK-REQ-012-01 đến 07 |
| CR-REQ-013 `StartPhase`, sự kiện task, `ReportTaskOutcome`, đối soát | [BE-REQ-SOL-013](./BE-REQ-SOL-013-phase-execution-and-feedback-loop.md) | `task-service`, `request-service` | TASK-REQ-013-01 đến 07 |
| CR-REQ-014 chính sách theo loại | [BE-REQ-SOL-014](./BE-REQ-SOL-014-type-specific-execution-policies.md) | `request-service` | TASK-REQ-014-01 đến 07 |

## Thứ tự phụ thuộc

```
SOL-011 (task-service: migration 0015/0016, domain, CreateTask, ListTasks, SyncContainerStatus, guards)
   │
   ├──▶ SOL-012 (CreatePlanTree ở task-service ─▶ GeneratePlan/CommitPlan ở request-service)
   │        │
   │        ▼
   └──▶ SOL-013 nửa task-service (sự kiện cùng tx)  ┐   có thể làm song song với SOL-012
                                                    ▼
                 SOL-013 nửa request-service (StartPhase, consumer, đối soát)  ◀── cần SOL-012
                                │
                                ▼
                            SOL-014 (TypePolicy cho năm loại, request_checks)
```

Điều kiện ngoài feature: CR-REQ-001 và 002 (module và bảng của `request-service`), CR-REQ-003 (registry, `TransitionRequest`), CR-REQ-006 (`ReturnToBacklog`, `SpawnChildRequest`), CR-REQ-007 (Solution đã duyệt), CR-REQ-009 (`OpenApproval`, `SubjectHandler`).

## Quyết định chung

| # | Quyết định | Lý do |
|---|---|---|
| P1 | `task-service` merge đầu tiên (SOL-011), độc lập `request-service` | Là nền của mọi thứ còn lại, rủi ro lớn nhất nằm ở migration và đổi cổng |
| P2 | Số migration `task-service` thật là **0015** (`task_type`) và **0016** (`request_id`) ở cả hai dialect; luôn `ls` lại trước khi tạo file | `ls migrations/{postgres,mysql}` ngày 2026-10-06 đều dừng ở `0014_task_sources_site` |
| P3 | Plan/Phase/Task dựng một transaction ở `task-service` (`CreatePlanTree`) | Mẫu `AIApply` (`RunInTx`), một chỉ mục duy nhất chống đua |
| P4 | Container (plan/phase) không bao giờ chạy: `ExecuteTask` trên container bị chặn | Một coordinator, một worktree cho cả Phase trái README v6 |
| P5 | Sự sai khác tên RPC: tách `GeneratePlan` (đề xuất) và `CommitPlan` (lưu) | README v6 mục 8 điều 12 liệt kê `CommitPlan` |
| P6 | Sự kiện `statuschanged` cùng transaction, chỉ cho task có `request_id` | Mất sự kiện lỗi làm Request kẹt `executing`; không nhiễu task thường |
| P7 | Hai dialect cho mọi migration, repository và truy vấn; chỉ mục duy nhất MySQL dùng generated column | `task-service` có `adapter/postgres` và `adapter/mysql` |
| P8 | Mọi RPC của `request-service` tự kiểm quyền | README v6 mục 8 điều 13 |
| P9 | Không thêm lệnh git; không `max-lines` disable; tên file theo khái niệm | AGENTS.md |

## Đã kiểm chứng và điểm khác CR gốc

- v4 task-graph (BE-SOL-001, TASK-TG-001-0x) **đã triển khai** trong code (progress, subtree, `TxRunner`, `AddEdge` nguyên tử, `labels`); không làm lại. Số migration trong tài liệu v4 đã lệch thực tế.
- `ReleaseExecution` và `ClaimForExecution` nằm ở `usecase/execution_lease.go`, không ở `ports.go` như CR-REQ-013 viết (SOL-013 mục 1).
- `request-service` có xung đột số migration giữa các CR khác (nhiều CR cùng dùng `0002`, `0005`): task migration phía `request-service` dùng quy tắc "số lớn nhất hiện có cộng 1" và báo người điều phối.
- Chưa chạy bất kỳ test nào; mọi lệnh test là lệnh dự kiến.

## Tham chiếu tiến (không phụ thuộc)

Các CR bổ sung 026 đến 036 đang soạn song song: CR-REQ-029 (TaskSpec, ReadinessGate) sẽ đổi cách render prompt và cổng chạy; CR về ExecutionResult và `Failure.class` sẽ thêm phân loại lỗi; CR-REQ-028 (Clarification) có thể chặn sinh Plan. Các solution ở đây chỉ để chỗ nối (cổng `PreExecutionGate`, cột `error_message`/`cause`, trường proto thêm mới) và ghi ở mục Câu hỏi mở; không phụ thuộc vào chúng.
