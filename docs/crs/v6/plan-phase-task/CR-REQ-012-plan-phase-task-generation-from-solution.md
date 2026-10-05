# CR-REQ-012 — Sinh Plan, Phase và Task từ Solution đã duyệt

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-012 |
| **Tên** | Use case `GeneratePlan` (đề xuất rồi mới lưu): Plan, Phase, task làm việc, Fix plan, task list, runbook; RPC `CreatePlanTree` ở `task-service` |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-011 (type, `request_id`, quy tắc phân cấp), CR-REQ-003 (registry `FlowFor`, `PhasesFor`, trigger `plan_ready`), CR-REQ-007 (Solution đã duyệt), CR-REQ-009 (Approval), CR-REQ-002 (`requests.plan_task_id`) |
| **Mở khoá** | CR-REQ-013, CR-REQ-014, CR-REQ-021 |
| **Tác động** | `backend-go/services/request-service` (usecase, domain, adapter/grpcclient, proto `orca.request.v1`); `backend-go/services/task-service` (RPC `CreatePlanTree`, proto `task.proto`) |

## 1. Bối cảnh và vấn đề

Sau khi Solution (hoặc Chẩn đoán) được duyệt, hệ thống phải tạo cây Plan → Phase → Task thật ở `task-service` để duyệt và thực thi. Code hiện có:

- `AIDecompose` (`task-service/internal/usecase/ai_decompose.go`) đề xuất `SubtaskProposal` chưa lưu; `AIApply` (`ai_apply.go`) lưu trong một transaction qua `TxRunner.RunInTx`, thêm cạnh `parent_child` và `depends_on` (AddEdge có kiểm vòng và tự `blocked`). Đó là mẫu "đề xuất rồi mới lưu" cần giữ, nhưng nó chỉ sinh task con của một task có sẵn, không biết Plan, Phase, nhãn, `request_id`.
- AI không đi qua `ai-provider-service`: `ai-provider-service` chỉ có `ResolveProvider` và quản lý account/credential (`proto/orca/aiprovider/v1`), không có RPC hoàn thành văn bản. `AIDecompose` gọi `ai.complete` của Dev Server Agent qua `infra-fleet-service` `Relay` (`adapter/grpcclient/aidecompose_relay.go`), và cần project có dev server đang kết nối (`TASK_AI_DECOMPOSE_NO_CONNECTION`). README v6 (3.1) ghi "ai-provider-service: phân loại, Solution, Plan, Phase" chưa khớp code (xem Q1).
- `CreateTask` không phải nguyên tử qua nhiều lần gọi RPC; `DeleteTask` một Plan xoá cascade con nhưng không nên dùng làm bù trừ cho tạo lỗi dở.
- Chưa có ai quyết định khi nào có Phase, Plan gồm gì với từng loại Request (README 3.4).

CR này sở hữu việc sinh cây. Thực thi là CR-REQ-013; các cổng và kiểm tra riêng theo loại là CR-REQ-014.

## 2. Giải pháp đề xuất

### 2.1 Hình dạng Plan theo loại (`internal/domain/plan_shape.go`, mới)

Suy từ `FlowDefinition.Plan.Kind` và `PhasesFor(size)` của CR-REQ-003, không lặp lại bảng ở đây:

| `Plan.Kind` | Loại Request | Hình dạng tạo ra |
|---|---|---|
| `plan` + `PhasesFor` đúng | `change_request` (mọi size), `bug`/`refactor` khi `size=L` | Plan → ≥1 Phase → task làm việc dưới từng Phase |
| `plan` + `PhasesFor` sai | `bug`, `refactor` (S, M), `security`, `performance`, `ops_request` | Plan → task làm việc trực tiếp |
| `task_list` | `task`, `docs` | một task `plan` làm vỏ danh sách → task làm việc (không Phase). Approval `task_list` trỏ vào task `plan` này |
| `single_task` | `hotfix` | đúng một task làm việc, không Plan; `requests.plan_task_id` để NULL, liên kết bằng `tasks.request_id` |
| `none` | `spike`, `question` | `REQUEST_PLAN_NOT_APPLICABLE` |

Nội dung theo loại: `bug`, `security` là Fix plan, bắt buộc có ít nhất một task nhãn `test:regression`. `ops_request` là runbook: mỗi bước một task, bước không đảo ngược mang `irreversible=true` (thành nhãn `gate:pre_deploy`), kèm ít nhất một task nhãn `rollback`. `performance` gồm task nhãn `check:baseline` đầu và `check:after` cuối. `refactor` gồm task nhãn `check:tests_before` đầu và `check:tests_after` cuối. Việc kiểm các điều kiện này ở CR-REQ-014 (`PlanPreconditions`); CR này chỉ sinh nhãn.

Bảng nhãn (`internal/domain/plan_labels.go`, mới; một nguồn cho CR-012, 013, 014): `gate:pre_deploy`, `test:regression`, `rollback`, `check:baseline`, `check:after`, `check:tests_before`, `check:tests_after`. `Task.Labels` đã có ở `task-service` và `CreateTaskRequest.labels` do CR-REQ-011 thêm.

### 2.2 `PlanProposal` (proto `orca.request.v1`, file `request_plan.proto`, mới)

```
message PlanProposal {
  string title = 1; string summary = 2;           // summary vào plan.description
  repeated PhaseProposal phases = 3;               // rỗng nếu không chia Phase
  repeated TaskProposal tasks = 4;                 // task trực tiếp dưới Plan khi không Phase
  string notes = 5;
}
message PhaseProposal {
  string title = 1; string description = 2;        // description là ngữ cảnh agent đọc (xem 2.5)
  repeated TaskProposal tasks = 3;
  repeated int32 depends_on_phase_indices = 4;     // chỉ số Phase cùng Plan
}
message TaskProposal {
  string title = 1; string description = 2;
  string task_type = 3;                            // task|bug|feature
  google.protobuf.DoubleValue estimated_hours = 4;
  string prompt_template = 5;
  repeated int32 depends_on_indices = 6;           // chỉ số anh em cùng cha
  repeated string labels = 7;
  bool irreversible = 8;                           // thêm nhãn gate:pre_deploy khi lưu
}
```

Giới hạn, cấu hình bằng biến môi trường, giá trị mặc định đề xuất chưa kiểm chứng: `REQUEST_PLAN_MAX_PHASES=8`, `REQUEST_PLAN_MAX_TASKS_PER_CONTAINER=20`, `REQUEST_PLAN_MAX_TASKS_TOTAL=100`, tiêu đề tối đa 200 ký tự.

### 2.3 RPC và luồng `GeneratePlan` (`internal/usecase/generate_plan.go`, mới)

README v6 chỉ có một RPC `GeneratePlan`. Để giữ mẫu hai bước của `AIDecompose`/`AIApply` mà không thêm RPC, dùng `mode` (Q2 đề nghị tách `CommitPlan`):

```
message GeneratePlanRequest {
  string request_id = 1;
  PlanMode mode = 2;                 // PROPOSE | COMMIT
  PlanProposal proposal = 3;         // bắt buộc khi COMMIT (có thể đã sửa tay)
  string raw_ai_response = 4;        // lưu vào plan.ai_plan_json, như AIApply
}
message GeneratePlanResponse {
  PlanProposal proposal = 1;         // PROPOSE: bản AI; COMMIT: bản đã lưu
  string plan_task_id = 2;           // chỉ COMMIT, rỗng nếu single_task
  repeated string phase_task_ids = 3; repeated string task_ids = 4;
  bool already_exists = 5;
}
```

**PROPOSE** (không ghi gì, không đổi trạng thái):
1. `tenant.RequireTenantID`; đọc Request; `FlowFor(type)`; `Plan.Kind = none` thì `REQUEST_PLAN_NOT_APPLICABLE`.
2. Trạng thái phải là `planning` (hoặc `analyzing` khi `single_task`, xem 2.6); khác thì `REQUEST_STATE_STALE`.
3. Nạp đầu vào: Request (`title`, `body`, `type`, `size`), Solution `approved` (`chosen_option`, nội dung qua `content_ref`) hoặc Chẩn đoán; thiếu thì `REQUEST_PLAN_SOLUTION_NOT_APPROVED`. Tech stack và tên project qua project-service như `AIDecompose`.
4. Gọi cổng `PlanGenerator.Generate(ctx, in) (PlanProposal, raw string)`; adapter `adapter/grpcclient/plan_generator_relay.go` (mới) dựng prompt JSON theo loại (mục 2.1) rồi `ai.complete` qua `infra-fleet-service`. Không có dev server: `REQUEST_PLAN_AI_UNAVAILABLE`. JSON hỏng: `REQUEST_PLAN_AI_INVALID_JSON` (trả `raw` để người xem).
5. `ValidateProposal(flow, size, proposal)` (mục 2.4), trả bản đề xuất. Không lưu: người dùng sửa ở UI rồi COMMIT.

**COMMIT** (một `TxRunner.InTx` cho phần ở `request-service`, task-service gọi trước):
1. Bước 1 và 2 như trên, thêm `ValidateProposal` lần nữa (không tin dữ liệu client).
2. `TaskPlanWriter.CreatePlanTree` (mục 2.5) hoặc `CreateTask` cho `single_task`; truyền `creator_id = actor` để chủ sở hữu có `Grant` kế thừa theo cây.
3. Một transaction: `UPDATE requests SET plan_task_id = ? ... WHERE version = ?` (CAS CR-REQ-002; `already_exists` thì cùng giá trị nên idempotent), `InsertOutboxEvent` `orca.request.plan.generated` `{request_id, plan_task_id, phase_task_ids, task_ids, type, size, task_count}`.
4. `OpenApproval` (CR-REQ-009, nội bộ) với `subject_type=plan` (Plan.Kind `plan`) hoặc `task_list` (Plan.Kind `task_list`, `subject_id` = id task `plan` vỏ), `stage=plan`. Loại có `StartGate=pre_deploy` (`security`) mở Approval `pre_deploy` thay `plan` (CR-REQ-014). `hotfix`: không mở ở đây (xem 2.6).
5. `TransitionRequest(plan_ready)` với `ExpectedFrom=planning`.
6. Thất bại giữa chừng: bước 2 thành công mà 3 lỗi thì COMMIT lặp lại gặp `already_exists=true` (mục 2.5) và đi tiếp; không cần bù trừ. Bước 4 hoặc 5 lỗi cũng retry được: `OpenApproval` trả dòng `pending` cũ khi `subject_digest` trùng (CR-REQ-009, chỉ mục duy nhất một `pending` mỗi chủ thể), `TransitionRequest` trả `Applied=false` khi giao lặp.

**Sinh lại (replan):** khi Request quay về `planning` (`plan_revision`, hoặc mở lại từ backlog) mà `plan_task_id` đã có, COMMIT truyền `supersedes_plan_id = plan_task_id`; `request-service` huỷ Approval cũ (`CancelPendingForRequest`, CR-REQ-009) trước khi gọi.

### 2.4 Kiểm đề xuất (`plan_proposal_validation.go`, mới)

| Lỗi | Điều kiện | Mã (FailedPrecondition trừ khi ghi khác) |
|---|---|---|
| Có Phase khi không được phép | `PhasesFor(size)` sai mà `phases` không rỗng | `REQUEST_PLAN_PHASES_NOT_ALLOWED` |
| Thiếu Phase | `PhasesFor` đúng mà `phases` rỗng | `REQUEST_PLAN_PHASES_REQUIRED` |
| Vừa Phase vừa task trực tiếp | cả `phases` và `tasks` không rỗng | `REQUEST_PLAN_MIXED_CHILDREN` |
| Quá lớn | vượt giới hạn mục 2.2 | `REQUEST_PLAN_TOO_LARGE` |
| Phụ thuộc sai | chỉ số ngoài phạm vi, tự phụ thuộc, hoặc có vòng (`domain.DetectCycle` ở `task-service` kiểm lại; ở đây kiểm sớm bằng Kahn) | `REQUEST_PLAN_INVALID_DEPENDENCY` |
| Thiếu test hồi quy | `bug`/`security` không có task nhãn `test:regression` | `REQUEST_PLAN_REGRESSION_TEST_MISSING` |
| Type task sai | ngoài `task|bug|feature` | `REQUEST_PLAN_INVALID_TASK_TYPE` (InvalidArgument) |
| Tiêu đề rỗng hoặc quá dài | | `REQUEST_PLAN_INVALID_TITLE` (InvalidArgument) |

Quy tắc nhãn riêng (`rollback`, `check:*`, `irreversible`) do CR-REQ-014 thêm qua `PlanPreconditions`.

### 2.5 `task-service`: RPC `CreatePlanTree` (thuộc CR này, dùng phần nền CR-REQ-011)

```
message CreatePlanTreeRequest {
  string request_id = 1; string project_id = 2;
  string title = 3; string description = 4;
  string ai_plan_json = 5;                          // PlanProposal + raw AI, lưu vào plan.ai_plan_json
  repeated PlanTreePhase phases = 6; repeated PlanTreeTask tasks = 7;
  string creator_id = 8; string supersedes_plan_id = 9;
  string plan_task_type = 10;                       // "plan" (mặc định)
}
message PlanTreePhase { string title = 1; string description = 2; repeated PlanTreeTask tasks = 3; repeated int32 depends_on_phase_indices = 4; }
message PlanTreeTask { string title = 1; string description = 2; string task_type = 3; google.protobuf.DoubleValue estimated_hours = 4; string prompt_template = 5; string ai_context = 6; repeated string labels = 7; repeated int32 depends_on_indices = 8; }
message CreatePlanTreeResponse { Task plan = 1; repeated Task phases = 2; repeated Task tasks = 3; bool already_exists = 4; }
```

Use case `usecase/create_plan_tree.go` (mới), mẫu `AIApply`: `TxRunner.RunInTx` (cả hai adapter đã có `RunInTx`) rồi trong transaction:
1. Nếu có Plan hoạt động cho `request_id`: `supersedes_plan_id` rỗng thì đọc cây hiện có trả `already_exists=true`; `supersedes_plan_id` khớp thì kiểm không có con `in_progress` (`TASK_PLAN_HAS_RUNNING_TASKS`, FailedPrecondition), đặt Plan cũ và mọi con chưa `done` thành `cancelled` rồi đi tiếp; không khớp thì `TASK_PLAN_SUPERSEDE_MISMATCH`.
2. Tạo Plan (`NewCreateTask(tasks, nil)`, `Type=plan`, `RequestID`, `AIPlanJSON` qua `UpdateAIPlanJSON` như `AIApply`), từng Phase, từng task; mỗi task con thêm cạnh `parent_child` bằng `addEdgeWithinTx` (cùng mẫu `AIApply`, để `selectEngine`/`ComplexExecutor.buildSpec` đọc được cạnh). Task con kế thừa `request_id`, `project_id`.
3. Lượt hai: cạnh `depends_on` giữa task anh em và giữa Phase (`FromTaskID` phụ thuộc `ToTaskID`, như `AIApply`). Cạnh Phase không `blocked` (CR-REQ-011). Cạnh task làm `from` thành `blocked` khi đích chưa `done`, đúng hành vi hiện có.
4. Task có `irreversible` đã nằm trong `labels` (request-service thêm `gate:pre_deploy` trước khi gọi).
5. Sau commit, ngoài transaction: `GrantRepository.Grant` owner cho `creator_id` trên Plan với `ApplyTree=true` (best-effort, như `CreateTask`).
Thua đua chỉ mục duy nhất "một Plan hoạt động mỗi Request" thì đọc lại và trả `already_exists=true`.

### 2.5b `SubjectHandler` cho `plan` và `task_list` (CR-REQ-009 giao cho CR này)

`internal/usecase/plan_subject_handler.go` (mới), đăng ký cho hai `subject_type`:
- `ValidateForRequest`: gọi `task-service` `GetSubtree(subject_id)`; Plan phải thuộc Request (`request_id` khớp, `type=plan`) và không `cancelled`. `digest` = SHA-256 của danh sách có thứ tự `(id, title, task_type, parent_id)` của cây cộng các cạnh `depends_on`; nếu Plan bị sửa sau khi người duyệt xem, `Approve` với `expected_digest` cũ bị từ chối theo cơ chế của CR-REQ-009.
- `OnApproved`: `TransitionRequest(plan_approved)` (`ExpectedFrom=awaiting_plan_approval`); chỉ ghi DB của `request-service`.
- `OnRejected`: `TransitionRequest(plan_rejected)`, `category=rejected` (CR-REQ-003, 006).
- `OnClosedWithoutDecision`: không làm gì (huỷ do replan hoặc đổi loại đã được lệnh gây ra xử lý).
Handler `phase` thuộc CR-REQ-013 (mục 2.3b), `pre_deploy` thuộc CR-REQ-014.

### 2.6 `single_task` (hotfix)

CR-REQ-003 cho `analysis_ready` của `hotfix` đi thẳng `awaiting_plan_approval` (không qua `planning`), nên task fix phải có trước. CR-REQ-008 (Chẩn đoán nhanh) gọi nội bộ `GeneratePlan` mode COMMIT, không có bước đề xuất duyệt, ngay trước trigger `analysis_ready`; `request-service` gọi `CreateTask` với `request_id`, `project_id`, không `parent_id`, rồi `OpenApproval` `pre_deploy` với `subject_id` là id task (CR-REQ-014). `requests.plan_task_id` giữ NULL.

### 2.7 Nội dung tới agent

`SimpleExecutor.buildExecutePrompt` ghép `PromptTemplate`, `Title`, `Description`, `AIContext`, tiêu đề và mô tả của parent, và các dependency đã xong. Vì vậy `request-service` điền: `ai_context` mỗi task = `REQ-<number>: <title>` + tóm tắt phương án đã chọn; `description` của Phase = phạm vi và tiêu chí xong của Phase (agent đọc nó như "Parent task"). Plan là cha của Phase nên mô tả Plan không vào prompt task lá.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Đề xuất rồi mới lưu, hai mode trên một RPC | Theo `AIDecompose`/`AIApply`; README chỉ có `GeneratePlan` |
| `CreatePlanTree` nguyên tử ở `task-service` | Nhiều `CreateTask` rời không nguyên tử; `RunInTx` đã có ở cả hai dialect |
| Plan vỏ cho `task_list` | Approval `task_list` cần `subject_id` là id task (README 3.5); một nơi gom task |
| Nhãn làm kênh mang ý nghĩa tới CR-014 | `Task.Labels` có sẵn, không thêm cột; UpdateTask thay cả danh sách nên chỉ đặt lúc tạo |
| Kiểm đề xuất ở server, hai lần | Không tin client, không tin AI |
| Không dùng `AIDecompose` cho từng Phase | Nó cần cây có sẵn, không biết nhãn, `request_id`, phụ thuộc giữa Phase; không đề xuất được trước khi lưu |
| Phase approval không tạo ở đây | Tạo lần lượt khi tới lượt (CR-REQ-013), nội dung Phase sau còn chỉnh được |

## 4. Tiêu chí chấp nhận

- [ ] PROPOSE không ghi gì ở `request-service` lẫn `task-service`; trả `PlanProposal` hợp lệ hoặc đúng mã lỗi mục 2.4.
- [ ] `change_request` size S vẫn có ít nhất một Phase; `bug` size M bị `REQUEST_PLAN_PHASES_NOT_ALLOWED` khi đề xuất có Phase; `bug` size L thiếu Phase bị `REQUEST_PLAN_PHASES_REQUIRED`.
- [ ] COMMIT tạo đủ Plan, Phase, task, cạnh `parent_child` và `depends_on` trong một transaction; lỗi giữa chừng không để lại task nào (test chèn lỗi ở task thứ N).
- [ ] COMMIT hai lần liên tiếp hoặc đồng thời: cùng một Plan, lần sau `already_exists=true`, không có Plan thứ hai (cả hai dialect).
- [ ] Task phụ thuộc task chưa xong được `blocked`; Phase phụ thuộc Phase không bị `blocked`.
- [ ] Sau COMMIT: `requests.plan_task_id` đặt; có đúng một Approval `plan` hoặc `task_list` `pending`; Request ở `awaiting_plan_approval`; có một sự kiện `orca.request.plan.generated`.
- [ ] `task`/`docs` tạo Plan vỏ và Approval `task_list` trỏ vào nó; `hotfix` tạo đúng một task không `parent_id`, `plan_task_id` NULL.
- [ ] `spike`, `question` bị `REQUEST_PLAN_NOT_APPLICABLE`.
- [ ] Replan: Plan cũ và con chưa `done` thành `cancelled`, Approval cũ `cancelled`, Plan mới là Plan hoạt động duy nhất; Plan cũ có con `in_progress` thì `TASK_PLAN_HAS_RUNNING_TASKS`.
- [ ] Người tạo có quyền trên mọi task của cây qua `Grant` kế thừa (`ResolvePermission` read và execute).
- [ ] Mọi mã lỗi mục 2.4 có test; không có tên file `helpers`, `utils`, `common`, `misc`.

## 5. Kiểm thử

- **Unit:** `plan_shape` theo 11 loại và 3 size; `ValidateProposal` từng dòng bảng 2.4 (cycle, chỉ số lệch, vượt giới hạn); parse JSON AI hỏng; dựng prompt theo loại; `GeneratePlan` với fake `PlanGenerator`, `TaskPlanWriter`, `ApprovalRequester`, thứ tự bước và retry từng điểm lỗi.
- **Integration, cả hai dialect:** `CreatePlanTree` (tx, rollback, idempotent, supersede, đua chỉ mục duy nhất, `blocked` đúng chỗ); `request-service` repository CAS `plan_task_id`.
- **Hợp đồng:** proto `buf breaking`; test JSON mẫu AI cho từng loại qua `PlanGenerator` giả.
- **Thủ công, môi trường có dev server:** PROPOSE thật qua relay `ai.complete` (chưa kiểm chứng).
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Đường AI: relay `ai.complete` cần dev server đang kết nối và chưa có bằng chứng chạy với prompt cỡ này; chất lượng và độ ổn định JSON của AI chưa đo. Giới hạn kích thước mặc định chỉ là giả định.
- Task trong một Phase dùng chung mã nguồn nhưng `EnsureWorktree` tạo worktree riêng `task/<id>` cho từng task (`worktree_provisioner.go`). Cây sinh ra ở đây chỉ có ý nghĩa nếu CR-REQ-013 giải được việc chia sẻ worktree (xem CR-REQ-013 mục 6).
- `Grant` kế thừa dựa vào `ApplyTree`; `usecase/resolve_permission.go` đi theo chuỗi tổ tiên (`GetAncestors`) và `domain/grant_resolution.go` chỉ tính grant `ApplyTree` ở tổ tiên, nên khả năng cao phủ ba cấp; chưa chạy.
- Hai dialect: `AddEdge` trong transaction MySQL dùng `ListByKindForUpdate` (`SELECT ... FOR UPDATE` toàn bộ cạnh theo loại); Plan lớn tạo nhiều cạnh làm khoá rộng. Chưa đo.
- `ProposalJSON` lưu vào `ai_plan_json` (JSONB/JSON) có thể lớn; chưa đặt trần.

## 7. Câu hỏi mở

- **Q1.** README v6 3.1 nói AI chạy qua `ai-provider-service`; code chỉ có `ResolveProvider`. CR dùng relay `ai.complete` như `AIDecompose`. Cần chốt hoặc thêm RPC hoàn thành vào `ai-provider-service`.
- **Q2.** Tách `CommitPlan` khỏi `GeneratePlan` cho rõ nghĩa? README 3.6 chỉ liệt kê `GeneratePlan`.
- **Q3.** `security` có Approval `pre_deploy` chiếm `awaiting_plan_approval` (CR-REQ-003) nên không có Approval `plan`; Plan của `security` vẫn có cổng duyệt nội dung không? README 3.4 liệt kê `solution` và `pre_deploy`.
- **Q4.** Hotfix gọi `GeneratePlan` nội bộ trong trạng thái `analyzing` (CR-REQ-003 không có `planning` cho `single_task`); xác nhận với CR-REQ-008.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.4, 3.5, 3.6, 3.7
- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-011-task-service-plan-phase-task-types.md`
- `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/ai_apply.go`, `ai_decompose.go`, `add_edge.go`, `create_task.go`, `ports.go` (`TxRunner`)
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/aidecompose_relay.go`, `worktree_provisioner.go`, `simple_executor.go` (`buildExecutePrompt`), `complex_executor.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task_edge.go`, `grant_resolution.go`, `internal/usecase/resolve_permission.go`
- `/opt/repos/orca/backend-go/proto/orca/aiprovider/v1/` (chỉ có `ResolveProvider`), `/opt/repos/orca/backend-go/proto/orca/task/v1/task.proto`
- Mới: `request-service/internal/usecase/generate_plan.go`, `internal/domain/plan_shape.go`, `plan_labels.go`, `plan_proposal_validation.go`, `internal/adapter/grpcclient/plan_generator_relay.go`, `task_plan_writer.go`, proto `request_plan.proto`; `task-service/internal/usecase/create_plan_tree.go`
