# BE-REQ-SOL-012: Sinh Plan, Phase và Task từ Solution đã duyệt (`GeneratePlan`, `CreatePlanTree`)

> **🚧 Đang triển khai: 2/7 task xong (012-01, 012-02 ở `task-service`, kiểm chứng 2026-10-08). Còn 012-03 đến 012-07 ở `request-service`.**

**CR:** [CR-REQ-012](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-012-plan-phase-task-generation-from-solution.md)
**Service:** `request-service` (mới, dựng ở CR-REQ-001) · `task-service` (RPC `CreatePlanTree`) · `proto/orca/request/v1`, `proto/orca/task/v1`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (cổng và adapter), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (không FK chéo service, outbox), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (gRPC nội bộ, idempotency), [`services/task-service.md`](../../../../tdd/services/task-service.md), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md) (Relay `ai.complete`)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-06: `task-service/internal/usecase/ai_apply.go` (toàn file), `ai_decompose.go`, `add_edge.go`, `create_task.go`, `ports.go` (`TxRunner` dòng 431), `adapter/grpcclient/aidecompose_relay.go` (`AICompleter.Complete`, `Relay` method `ai.complete`, chỉ gửi `prompt`), `adapter/{postgres,mysql}/repository.go` (`RunInTx`), `domain/subtask_proposal.go`.

- `backend-go/services/request-service` **chưa tồn tại** (`ls` báo không có thư mục). Mọi đường dẫn `request-service/...` bên dưới là "(mới)", do CR-REQ-001 và 002 dựng. Solution này giả định các cổng `TransitionRequest`, `OpenApproval`, `CancelPendingForRequest`, `FlowFor`, `PhasesFor` tồn tại theo CR-REQ-003 và 009; chưa kiểm chứng tên chữ ký cuối cùng.
- `AIApply` đã nguyên tử: một `TxRunner.RunInTx`, lượt một tạo task và cạnh `parent_child`, lượt hai thêm cạnh `depends_on` qua `AddEdge.Execute` (kiểm vòng, tự `blocked`). Mẫu này dùng lại nguyên cho `CreatePlanTree`. Chỉ có `NewCreateTask(tasks, nil)` (không `GrantRepository` trong tx), nên `Grant` chủ sở hữu phải làm sau commit.
- AI **không** đi qua `ai-provider-service` (chỉ có `ResolveProvider`): đường thật là `ai.complete` của Dev Server Agent qua `infra-fleet-service` `Relay`, cần project có dev server đang kết nối. README v6 mục 8 (điều 1) đã chốt điều này.

### Correction relative to CR-REQ-012

1. CR ghi `UpdateAIPlanJSON` "như `AIApply`": hàm có thật (`postgres/repository.go` dòng 519) nhưng nằm trên `TaskRepository`; trong tx dùng được vì `RunInTx` đưa `TaskRepository` scoped.
2. Chỉ mục duy nhất "một Plan hoạt động mỗi Request" do SOL-011 task 01 tạo; `CreatePlanTree` phải bắt lỗi vi phạm duy nhất (Postgres mã `23505`, MySQL `1062`) và đọc lại thành `already_exists=true`.
3. README v6 mục 8 điều 12 thêm RPC `CreatePlanTree` (task-service) và `CommitPlan` (request-service); CR vẫn mô tả mode `PROPOSE|COMMIT` trên `GeneratePlan` và hỏi Q2 về tách. Solution này **tách** theo điều 12: `GeneratePlan` chỉ PROPOSE, `CommitPlan` là COMMIT (chốt ở mục 3).

## 2. Giải pháp

### 2.1 Cây file

```
backend-go/proto/orca/request/v1/request_plan.proto                    (mới)  PlanProposal, GeneratePlan*, CommitPlan*
backend-go/proto/orca/task/v1/task.proto                               (sửa)  CreatePlanTree*
backend-go/services/task-service/internal/usecase/create_plan_tree.go  (mới)
backend-go/services/task-service/internal/usecase/ports.go             (sửa)  PlanTreeGrantor (sau commit)
backend-go/services/task-service/internal/adapter/grpc/server_plan_tree.go (mới)
backend-go/services/request-service/
  internal/domain/plan_shape.go                  (mới)  PlanShapeFor(flow, size)
  internal/domain/plan_labels.go                 (mới)  hằng nhãn một nguồn cho CR-012/013/014
  internal/domain/plan_proposal.go               (mới)  PlanProposal thuần (không proto)
  internal/domain/plan_proposal_validation.go    (mới)  ValidateProposal, Kahn
  internal/usecase/generate_plan.go              (mới)  PROPOSE
  internal/usecase/commit_plan.go                (mới)  COMMIT
  internal/usecase/plan_subject_handler.go       (mới)  SubjectHandler plan, task_list
  internal/usecase/ports.go                      (sửa)  PlanGenerator, TaskPlanWriter
  internal/adapter/grpcclient/plan_generator_relay.go (mới)
  internal/adapter/grpcclient/task_plan_writer.go     (mới)
  internal/adapter/grpc/server_plan.go           (mới)
```

### 2.2 Hình dạng Plan theo loại (`domain/plan_shape.go`)

Suy từ `FlowDefinition.Plan.Kind` và `PhasesFor(size)` (CR-REQ-003), không lặp bảng ở nhiều nơi:

| `Plan.Kind` | Loại Request | Hình dạng |
|---|---|---|
| `plan` + `PhasesFor` đúng | `change_request`, `bug`/`refactor` size L | Plan → Phase → task |
| `plan` + `PhasesFor` sai | `bug`/`refactor` S/M, `security`, `performance`, `ops_request` | Plan → task |
| `task_list` | `task`, `docs` | task `plan` vỏ → task |
| `single_task` | `hotfix` | một task, không Plan, `requests.plan_task_id` NULL |
| `none` | `spike`, `question` | `REQUEST_PLAN_NOT_APPLICABLE` |

```go
type PlanShape int
const (ShapePlanPhases PlanShape = iota; ShapePlanTasks; ShapeTaskListShell; ShapeSingleTask; ShapeNone)
func PlanShapeFor(f FlowDefinition, size Size) PlanShape
```

Nhãn (`plan_labels.go`): `gate:pre_deploy`, `test:regression`, `rollback`, `check:baseline`, `check:after`, `check:tests_before`, `check:tests_after`. Một nguồn duy nhất cho CR-012, 013, 014; `UpdateTask` thay cả danh sách nhãn nên chỉ đặt lúc tạo.

### 2.3 `PlanProposal` và proto

`domain.PlanProposal` thuần Go, proto `request_plan.proto` ánh xạ 1-1 (`PlanProposal{title, summary, phases[], tasks[], notes}`, `PhaseProposal{title, description, tasks[], depends_on_phase_indices[]}`, `TaskProposal{title, description, task_type, estimated_hours, prompt_template, depends_on_indices[], labels[], irreversible}`). Giới hạn qua env: `REQUEST_PLAN_MAX_PHASES=8`, `REQUEST_PLAN_MAX_TASKS_PER_CONTAINER=20`, `REQUEST_PLAN_MAX_TASKS_TOTAL=100`, tiêu đề tối đa 200 ký tự (mặc định chưa kiểm chứng).

```proto
rpc GeneratePlan(GeneratePlanRequest) returns (GeneratePlanResponse); // PROPOSE: không ghi gì
rpc CommitPlan(CommitPlanRequest) returns (CommitPlanResponse);       // COMMIT
message GeneratePlanRequest { string request_id = 1; string feedback = 2; }
message GeneratePlanResponse { PlanProposal proposal = 1; string raw_ai_response = 2; }
message CommitPlanRequest { string request_id = 1; PlanProposal proposal = 2; string raw_ai_response = 3; }
message CommitPlanResponse { string plan_task_id = 1; repeated string phase_task_ids = 2; repeated string task_ids = 3; bool already_exists = 4; }
```

### 2.4 `GeneratePlan` (PROPOSE) và `CommitPlan`

PROPOSE: `RequireTenantID`; đọc Request; `PlanShapeFor` bằng `ShapeNone` thì `REQUEST_PLAN_NOT_APPLICABLE`; trạng thái phải là `planning` (hoặc `analyzing` với `single_task`) nếu không `REQUEST_STATE_STALE`; nạp Solution `approved` (`chosen_option`, nội dung qua `content_ref`) hoặc Chẩn đoán, thiếu thì `REQUEST_PLAN_SOLUTION_NOT_APPROVED`; `PlanGenerator.Generate` dựng prompt JSON theo loại và gọi `ai.complete`; không có dev server thì `REQUEST_PLAN_AI_UNAVAILABLE`; JSON hỏng thì `REQUEST_PLAN_AI_INVALID_JSON` kèm `raw`; `ValidateProposal` rồi trả đề xuất. Không lưu.

COMMIT: lặp lại các kiểm tra và `ValidateProposal` (không tin client); `TaskPlanWriter.CreatePlanTree` (hoặc `CreateTask` cho `single_task`) với `creator_id = actor`; một transaction ở `request-service`: `UPDATE requests SET plan_task_id = ? WHERE version = ?` (CAS CR-REQ-002) và outbox `orca.request.plan.generated` `{request_id, plan_task_id, phase_task_ids, task_ids, type, size, task_count}`; `OpenApproval` (`plan` hoặc `task_list`; `security` mở `pre_deploy` theo CR-REQ-014); `TransitionRequest(plan_ready)` với `ExpectedFrom=planning`. Thất bại giữa chừng chạy lại an toàn: bước task-service trả `already_exists`, `OpenApproval` trả dòng `pending` cũ khi digest trùng, `TransitionRequest` trả `Applied=false` khi giao lặp. Replan: `plan_task_id` đã có thì truyền `supersedes_plan_id` và gọi `CancelPendingForRequest` trước.

### 2.5 Kiểm đề xuất (`ValidateProposal`)

Mã lỗi (FailedPrecondition trừ khi ghi khác): `REQUEST_PLAN_PHASES_NOT_ALLOWED`, `REQUEST_PLAN_PHASES_REQUIRED`, `REQUEST_PLAN_MIXED_CHILDREN`, `REQUEST_PLAN_TOO_LARGE`, `REQUEST_PLAN_INVALID_DEPENDENCY` (chỉ số ngoài phạm vi, tự phụ thuộc, vòng: Kahn ở đây, `domain.DetectCycle` của task-service kiểm lần nữa), `REQUEST_PLAN_REGRESSION_TEST_MISSING` (`bug`, `security`), `REQUEST_PLAN_INVALID_TASK_TYPE` và `REQUEST_PLAN_INVALID_TITLE` (InvalidArgument). Quy tắc nhãn `rollback`, `check:*`, `irreversible` do `TypePolicy.PlanPreconditions` (BE-REQ-SOL-014), gọi từ đây qua cổng rỗng.

### 2.6 `task-service`: `CreatePlanTree`

```proto
rpc CreatePlanTree(CreatePlanTreeRequest) returns (CreatePlanTreeResponse);
message CreatePlanTreeRequest { string request_id = 1; string project_id = 2; string title = 3; string description = 4;
  string ai_plan_json = 5; repeated PlanTreePhase phases = 6; repeated PlanTreeTask tasks = 7;
  string creator_id = 8; string supersedes_plan_id = 9; string plan_task_type = 10; }
message PlanTreePhase { string title = 1; string description = 2; repeated PlanTreeTask tasks = 3; repeated int32 depends_on_phase_indices = 4; }
message PlanTreeTask { string title = 1; string description = 2; string task_type = 3; google.protobuf.DoubleValue estimated_hours = 4;
  string prompt_template = 5; string ai_context = 6; repeated string labels = 7; repeated int32 depends_on_indices = 8; }
message CreatePlanTreeResponse { Task plan = 1; repeated Task phases = 2; repeated Task tasks = 3; bool already_exists = 4; }
```

`usecase/create_plan_tree.go`, mẫu `AIApply`: một `TxRunner.RunInTx` với các bước: (1) nếu đã có Plan hoạt động cho `request_id`: `supersedes_plan_id` rỗng thì đọc cây trả `already_exists=true`; khớp thì kiểm không con `in_progress` (`TASK_PLAN_HAS_RUNNING_TASKS`), đặt Plan cũ và con chưa `done` thành `cancelled`; không khớp `TASK_PLAN_SUPERSEDE_MISMATCH`; (2) tạo Plan, từng Phase và task bằng `NewCreateTask(tasks, nil)` với `RequestID`, nhãn, rồi cạnh `parent_child` qua `AddEdge`; (3) lượt hai cạnh `depends_on` giữa task anh em và giữa Phase (cạnh Phase không `blocked` nhờ SOL-011); (4) `UpdateAIPlanJSON`; (5) sau commit, `Grant` owner `ApplyTree=true` cho `creator_id`, best-effort. Thua đua chỉ mục duy nhất thì đọc lại và trả `already_exists=true`.

### 2.7 `SubjectHandler` `plan`, `task_list`

`ValidateForRequest` gọi `GetSubtree(subject_id)`: Plan phải thuộc Request (`request_id` khớp, `type=plan`, không `cancelled`); `digest` = SHA-256 của danh sách có thứ tự `(id, title, task_type, parent_id)` cộng cạnh `depends_on`. `OnApproved`: `TransitionRequest(plan_approved)` (`ExpectedFrom=awaiting_plan_approval`); `OnRejected`: `plan_rejected`, `category=rejected`; `OnClosedWithoutDecision`: không làm gì. Handler `phase` thuộc SOL-013, `pre_deploy` thuộc SOL-014.

### 2.8 Nội dung tới agent

`SimpleExecutor.buildExecutePrompt` ghép `PromptTemplate`, `Title`, `Description`, `AIContext`, tiêu đề và mô tả của parent, và dependency đã xong. Vì vậy `ai_context` mỗi task = `REQ-<number>: <title>` + tóm tắt phương án đã chọn; `description` của Phase = phạm vi và tiêu chí xong của Phase. Lưu ý: CR-REQ-029 (TaskSpec, ReadinessGate) có thể đổi cách render prompt; không phụ thuộc vào đó (xem mục 7).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Tách `GeneratePlan` (PROPOSE) và `CommitPlan` | README v6 mục 8 điều 12 liệt kê `CommitPlan`; tên RPC đúng nghĩa, quyền ghi tách khỏi quyền đọc đề xuất |
| `CreatePlanTree` nguyên tử ở `task-service` | Nhiều `CreateTask` rời không nguyên tử; `RunInTx` đã có hai dialect |
| Plan vỏ cho `task_list` | Approval `task_list` cần `subject_id` là id task |
| Nhãn làm kênh mang ý nghĩa tới CR-014 | `Task.Labels` có sẵn, không thêm cột |
| Kiểm đề xuất hai lần ở server | Không tin client, không tin AI |
| Không dùng `AIDecompose` cho từng Phase | Nó cần cây có sẵn, không biết nhãn, `request_id`, phụ thuộc giữa Phase |
| Approval `phase` không tạo ở đây | Tạo lần lượt khi tới lượt (SOL-013) |

## 4. Phụ thuộc và thứ tự

Cần SOL-011 task 01 đến 04, 07 (chỉ mục, `CreateTask` validate, `AddEdge` bỏ block cho container). Thứ tự task: 01 `CreatePlanTree` usecase; 02 handler gRPC và integration hai dialect; 03 domain `plan_shape`, nhãn, validation; 04 proto và adapter `PlanGenerator`, `TaskPlanWriter`; 05 `GeneratePlan` và `CommitPlan`; 06 `SubjectHandler` và `single_task`; 07 integration và e2e. 01 và 03 song song. Mở khoá SOL-013, 014.

## 5. Kiểm thử

- **Unit:** `PlanShapeFor` theo 11 loại và 3 size; từng dòng `ValidateProposal`; JSON AI hỏng; prompt theo loại; `CommitPlan` với fake `TaskPlanWriter` và `ApprovalRequester`, retry từng điểm lỗi.
- **Integration hai dialect:** `CreatePlanTree` (tx, rollback khi lỗi ở task thứ N, idempotent, supersede, đua chỉ mục duy nhất, `blocked` đúng chỗ); CAS `plan_task_id`.
- **Hợp đồng:** `make proto-lint`; JSON mẫu AI từng loại qua `PlanGenerator` giả.
- **Thủ công, có dev server:** PROPOSE thật qua relay (chưa kiểm chứng). Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/task-service/... ./services/request-service/...`.

## 6. Rủi ro và điểm chưa kiểm chứng

- Relay `ai.complete` chưa có bằng chứng chạy với prompt cỡ này; chất lượng JSON chưa đo; giới hạn kích thước chỉ là giả định.
- `EnsureWorktree` tạo worktree riêng `task/<id>` cho từng task (`worktree_provisioner.go`): cây sinh ra chỉ có ý nghĩa nếu SOL-013 giải được worktree dùng chung.
- `Grant` kế thừa dựa vào `ApplyTree` và `GetAncestors`; phủ ba cấp chưa chạy.
- `AddEdge` MySQL dùng `ListByKindForUpdate` khoá toàn bộ cạnh theo loại: Plan lớn tạo nhiều cạnh làm khoá rộng. Chưa đo.
- `ai_plan_json` lưu cả đề xuất và raw có thể lớn; chưa có trần.

## 7. Câu hỏi mở

- Chốt mô hình AI: relay `ai.complete` hay thêm RPC hoàn thành vào `ai-provider-service` (README v6 mục 3.1 và mục 8 điều 1 lệch nhau).
- `security` mở `pre_deploy` thay `plan` nên không có cổng duyệt nội dung Plan: có cần cả hai?
- Hotfix gọi COMMIT nội bộ trong `analyzing` (CR-REQ-003 không có `planning` cho `single_task`): xác nhận với CR-REQ-008.
- Tham chiếu tiến: CR-REQ-029 (TaskSpec, ReadinessGate) sẽ đổi `TaskProposal` (thêm tiêu chí nghiệm thu, tệp liên quan) và cách render prompt; CR-REQ-028 (Clarification) có thể chặn `GeneratePlan` khi còn câu hỏi mở. Solution này không phụ thuộc, chỉ để chỗ nối: `PlanGenerator` đọc nội dung phương án qua `content_ref`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.4, 3.5, 3.6, 8 (điều 1, 12)
- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-011-task-service-plan-phase-task-types.md`, `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`, `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/ai_apply.go`, `ai_decompose.go`, `add_edge.go`, `create_task.go`, `ports.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/aidecompose_relay.go`, `worktree_provisioner.go`, `simple_executor.go`, `complex_executor.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/repository.go` (`RunInTx`, `UpdateAIPlanJSON`), `adapter/mysql/repository.go`
- `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`, `/opt/repos/orca/backend-go/proto/orca/aiprovider/v1/`
- `/opt/repos/orca/specs/backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-011-task-service-plan-phase-task-types.md`
