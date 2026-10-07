# TASK-REQ-011-03: `CreateTask` kiểm type và phân cấp, ghi `request_id`; handler gRPC chuyển đủ trường; `AIApply` chuẩn hoá type

**From Solution:** BE-REQ-SOL-011
**Priority:** P0
**Service:** `task-service`
**File:** `internal/usecase/create_task.go`, `internal/adapter/grpc/server.go` (dòng 134 đến 147), `internal/adapter/grpc/server_create_task.go` (mới), `internal/usecase/ai_apply.go`, `proto/orca/task/v1/task.proto`
**Depends on:** TASK-REQ-011-02
**Status:** `[x] DONE`

---

## Context

- `usecase/create_task.go`: `CreateTaskInput` (dòng 21 đến 48) đã có `Description`, `Type`, `Priority`, `AssigneeID`, `EstimatedHours`, `PromptTemplate`, `AIContext`, `Visibility`, `CreatorID`; chưa có `RequestID`, `Labels`. Bước đọc parent đã có: `if task.ParentID != "" { uc.repo.Get(...) }` (lỗi `TASK_PARENT_NOT_FOUND`). `task.Type = in.Type` gán thẳng, không kiểm.
- `server.go` `CreateTask` (dòng 134): chỉ chuyển `Title`, `ParentId`, `ProjectId`, `CreatorId`. Các trường `task_type`, `description`, `priority`, `estimated_hours`, `prompt_template`, `ai_context`, `visibility` (proto 5 đến 13) bị bỏ im lặng. Hệ quả: hiện chưa ai tạo được `type=bug` qua gRPC.
- `AIApply` (`ai_apply.go` dòng 80) truyền `Type: p.Type` thẳng vào `CreateTask`.
- CHECK DB: `priority IN (low,medium,high,urgent)`, `visibility IN (private,team,public)` (`0003`): giá trị sai hiện nổ thành `TASK_CREATE_FAILED`.
- `server.go` đã 661 dòng: đặt code chuyển đổi mới ở file riêng, giống `server_task_source.go`.

## Việc cần làm

1. Proto: `CreateTaskRequest` thêm `string request_id = 15; repeated string labels = 16;` (`make proto-gen`, `make proto-lint`).
2. `CreateTaskInput` thêm `RequestID string`, `Labels []string`.
3. `CreateTask.Execute`: sau gán trường, gọi `domain.ParseTaskType(in.Type)` (lỗi thành `TASK_INVALID_TYPE`, InvalidArgument). Đọc parent một lần (đã có) rồi áp bảng: `plan` không parent (`TASK_PLAN_CANNOT_HAVE_PARENT`) và cần `ProjectID` (`TASK_PLAN_PROJECT_REQUIRED`); `phase` cần parent `plan` cùng `project_id` (`TASK_PHASE_REQUIRES_PLAN_PARENT`); plan/phase dưới task làm việc (`task|bug|feature|epic`) là `TASK_CONTAINER_UNDER_WORK_TASK`. Lỗi phân cấp là FailedPrecondition.
4. Kế thừa: nếu `in.RequestID == ""` và có parent thì `task.RequestID = parent.RequestID`.
5. Kiểm `Priority` (rỗng cho qua) và `Visibility` ở `domain` (hàm `ValidatePriority`, `ValidateVisibility` trong `domain/task.go`); sai thì `TASK_INVALID` InvalidArgument.
6. `server_create_task.go` (mới): `func toCreateTaskInput(req *taskv1.CreateTaskRequest) usecase.CreateTaskInput` chuyển đủ `TaskType`, `Description`, `Priority`, `AssigneeId`, `EstimatedHours` (`GetEstimatedHours().GetValue()` khi khác nil), `PromptTemplate`, `AiContext`, `Visibility`, `RequestId`, `Labels`, `CreatorId`; `server.go` `CreateTask` gọi hàm này.
7. `AIApply`: trước `CreateTask`, nếu `p.Type` ngoài `task|bug|feature` thì dùng `task` (hàm nhỏ `normalizeProposalType` trong `ai_apply.go`).
8. Chạy `gitnexus_impact` cho `CreateTask.Execute` và `Server.CreateTask` trước khi sửa; báo blast radius trong PR.

## Kiểm thử

Thêm vào `usecase/create_task_test.go` (fake ở `fakes_test.go`): `TestCreateTask_PlanWithParent_Rejected`, `TestCreateTask_PhaseUnderTask_Rejected`, `TestCreateTask_PhaseUnderPlan_OK`, `TestCreateTask_PlanUnderWorkTask_Rejected`, `TestCreateTask_InvalidType`, `TestCreateTask_InheritsRequestID`, `TestCreateTask_InvalidPriority`. `TestAIApply_PlanTypeProposalBecomesTask` ở `ai_apply_test.go`. `adapter/grpc/server_test.go`: `TestServer_CreateTask_ForwardsAllFields` (task_type=bug, description, priority được lưu và trả lại ở `GetTask`).

`cd /opt/repos/orca/backend-go && go test ./services/task-service/internal/usecase/... ./services/task-service/internal/adapter/grpc/...`

## Tiêu chí hoàn thành

- [x] gRPC `CreateTask` với `task_type=bug`, `description`, `priority` lưu đúng, `GetTask` trả lại.
- [x] Ba lỗi phân cấp trả đúng mã; type lạ `TASK_INVALID_TYPE`; priority sai `TASK_INVALID`.
- [x] Task con kế thừa `request_id` từ parent khi không truyền.
- [x] `AIApply` với `type=plan` tạo task `type=task`.
- [x] Test cũ `create_task_test.go`, `ai_apply_test.go` xanh (không đổi chữ ký `NewTask`).

## Rủi ro và lưu ý

- Client cũ đang gửi trường sai sẽ bắt đầu bị từ chối hoặc được lưu: thông báo trong PR, kiểm gateway `task.create` (CR-REQ-016) và frontend.
- `NewCreateTask(repo, nil)` ở `AIApply` truyền `grants=nil`: không đổi.
- `request_id` do client gRPC tự khai: `request-service` là nơi duy nhất gọi với giá trị này; quyền ghi gateway thuộc CR-REQ-016.
