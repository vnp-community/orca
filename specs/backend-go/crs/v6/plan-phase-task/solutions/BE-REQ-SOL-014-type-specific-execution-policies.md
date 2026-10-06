# BE-REQ-SOL-014: Chính sách thực thi theo loại (`TypePolicy`): hotfix, security, performance, ops_request, refactor

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Chỉ chạm `request-service`; không đổi `task-service`. Móc vào các điểm `TypePolicy` mà SOL-012 và SOL-013 để sẵn nên có thể giao sau mà không đổi đường chuẩn.

**CR:** [CR-REQ-014](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-014-type-specific-execution-policies.md)
**Service:** `request-service` (mới) · `proto/orca/request/v1`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần, chính sách là interface), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, RLS, append-only), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (quyền, cổng duyệt), [`services/task-service.md`](../../../../tdd/services/task-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-06: `task-service/internal/domain/task.go` (`Labels`, `LastExecutionOutput`), `usecase/update_task.go` (`UpdateTaskInput.Labels` thay cả danh sách), `usecase/report_execution_result.go` (chỉ nhận `success`, `actual_hours`, `error_message`), proto `ReportTaskExecutionResultRequest` (6 trường, không có đầu ra có cấu trúc), `migrations/postgres/0011_task_widened_fields.up.sql` (`labels`).

- `task-service` không lưu đầu ra có cấu trúc: `LastExecutionOutput` là chuỗi cắt tối đa 8 KB (`UpdateLastExecutionOutput`). Backend không thể tự suy "benchmark cải thiện 20%" hay "số test không đổi" từ kết quả task. Vì vậy số đo phải có chỗ ghi riêng (`request_checks`).
- Orca không có khái niệm "deploy": task chỉ chạy agent, kết quả là `review` hoặc lỗi. `pre_deploy` là **cổng duyệt**, không phải cổng kỹ thuật (README v6 mục 8 điều 6).
- Không có lệnh git mới (README v6 mục 6): `tests_modified` do agent khai, backend không xác minh.
- `request-service` chưa tồn tại; các cổng `TypePolicy` mà solution này hiện thực do SOL-012 (`PlanPreconditions`, tại `GeneratePlan`/`CommitPlan`) và SOL-013 (`PreExecutionGate` ở `AdvanceExecution`, `CompletionChecks` và `OnCompleted` ở `ReportTaskOutcome`) gọi. Cần `SpawnChildRequest` (CR-REQ-006) và Approval `pre_deploy` (CR-REQ-009).

### Correction relative to CR-REQ-014

1. CR chỉ nêu `UpdateTask` thay nhãn "có thể xoá `gate:pre_deploy`". Đã xác nhận ở `update_task.go`: `UpdateTaskInput.Labels` thay cả danh sách; vì vậy `PreExecutionGate` nên đọc nhãn **tại thời điểm chạy**, không cache lúc lập Plan, và ghi nhận rủi ro "khoá nhãn" ở mục 6.
2. `RecordRequestCheck`/`ListRequestChecks` và bảng `request_checks` đã là một phần README v6 mục 8 điều 12 (RPC) và điều 3 (bảng) sau khi soạn CR; Q1 của CR coi như đã giải.
3. `SpawnChildRequest` dựng khoá idempotency từ `actor` của người dùng; lời gọi từ hệ thống cần quy ước actor (CR Q5), chưa có: task 07 dùng `actor_kind=system` và `client_request_id = hotfix-followup:<request_id>:<bug|task>`, chờ CR-REQ-006 xác nhận.

## 2. Giải pháp

### 2.1 Cây file (đều "mới" dưới `backend-go/services/request-service/`)

```
migrations/{postgres,mysql}/NNNN_request_checks.{up,down}.sql
internal/domain/request_check.go            kiểu RequestCheck, CheckKind, CheckStatus, MetricsSchema
internal/domain/type_policy.go              interface TypePolicy, GateRequirement, CheckVerdict, FollowUp
internal/domain/type_policy_registry.go     PolicyFor(RequestType) TypePolicy, noopPolicy
internal/domain/type_policy_hotfix.go
internal/domain/type_policy_security.go
internal/domain/type_policy_performance.go
internal/domain/type_policy_ops_request.go
internal/domain/type_policy_refactor.go
internal/usecase/record_request_check.go    RecordRequestCheck, ListRequestChecks
internal/usecase/type_policy_pre_deploy_handler.go   SubjectHandler pre_deploy
internal/usecase/hotfix_followups.go        OnCompleted -> SpawnChildRequest
internal/adapter/{postgres,mysql}/request_checks.go
internal/adapter/grpc/server_request_check.go
```

### 2.2 Cổng `TypePolicy`

```go
type TypePolicy interface {
    PlanPreconditions(req Request, p PlanProposal) error                              // SOL-012, PROPOSE và COMMIT
    PreExecutionGate(req Request, task TaskRef) (*GateRequirement, error)            // SOL-013, AdvanceExecution
    CompletionChecks(req Request, checks []RequestCheck) ([]CheckVerdict, error)      // SOL-013, trước execution_finished
    OnCompleted(req Request) ([]FollowUp, error)                                      // sau request.completed
}
```

`PolicyFor(RequestType) TypePolicy`; loại không có chính sách (`bug`, `task`, `docs`, `change_request`...) dùng `noopPolicy` (mọi hàm trả rỗng/`nil`), nên đường chuẩn không đổi. `GateRequirement{SubjectType, SubjectID, Stage}`; `CheckVerdict{Kind, Stage, Status(passed|failed|missing), Summary}`. Mỗi loại một file, không rẽ nhánh theo loại trong use case chung.

### 2.3 Cổng `pre_deploy`

| Loại | Vị trí | `subject_id` | Kích hoạt |
|---|---|---|---|
| `hotfix` | `StartGate`, chiếm `awaiting_plan_approval` | id task fix duy nhất | SOL-012 `CommitPlan` (single_task) gọi `OpenApproval`; duyệt thì `plan_approved`; người duyệt bắt buộc là người (CR-REQ-010) |
| `security` | `StartGate` | id Plan | `CommitPlan` mở `pre_deploy` thay `plan` |
| `ops_request` | trong `executing` | id từng task nhãn `gate:pre_deploy` | `PreExecutionGate` trả `GateRequirement`; `AdvanceExecution` gọi `OpenApproval` (một `pending` mỗi chủ thể) và dừng task đó |

`SubjectHandler` `pre_deploy`: `ValidateForRequest` kiểm chủ thể đúng bảng và lấy `digest`; `OnApproved`: `hotfix`/`security` thì `TransitionRequest(plan_approved)`, `ops_request` không đổi trạng thái (consumer `approval.decided` kích `AdvanceExecution`); `OnRejected`: `hotfix`/`security` thì `plan_rejected`, `ops_request` thì `ReturnToBacklog(stage=task, category=rejected)`. `PreExecutionGate` chỉ trả yêu cầu khi chưa có Approval `pre_deploy` `approved`; task bị chặn giữ `open`.

### 2.4 Bảng `request_checks` (append-only)

| Cột | Postgres | MySQL | Ràng buộc |
|---|---|---|---|
| `id` | UUID | CHAR(36) | PK |
| `tenant_id`, `request_id` | UUID | CHAR(36) | NOT NULL, không FK |
| `kind` | TEXT | VARCHAR(30) | CHECK IN (`perf_baseline`,`perf_after`,`tests_before`,`tests_after`,`security_recheck`,`ops_result`) |
| `status` | TEXT | VARCHAR(10) | CHECK IN (`passed`,`failed`) |
| `metrics` | JSONB | JSON | NOT NULL DEFAULT `{}` |
| `summary` | TEXT | TEXT | NOT NULL DEFAULT '' |
| `source` | TEXT | VARCHAR(10) | CHECK IN (`agent`,`manual`) |
| `task_id`, `recorded_by` | UUID NULL | CHAR(36) NULL | |
| `created_at` | TIMESTAMPTZ | TIMESTAMP(6) | NOT NULL, đồng hồ DB |

Chỉ mục `(tenant_id, request_id, kind, created_at DESC)`; RLS `tenant_isolation` trên Postgres. Bản ghi mới nhất theo `(request_id, kind)` có hiệu lực; không có đường sửa hay xoá. RPC: `RecordRequestCheck{request_id, kind, status, metrics_json, summary, task_id}` (lỗi `REQUEST_CHECK_NOT_ALLOWED_NOW`, `REQUEST_CHECK_INVALID_METRICS`) và `ListRequestChecks{request_id}`; người gọi là user (`manual`) hoặc agent qua tool MCP `request_record_check` (CR-REQ-017; chưa kiểm chứng agent gọi được tool khi đang chạy task).

### 2.5 Chính sách từng loại

- **performance:** baseline `kind=perf_baseline`, `metrics = {"metrics":[{"name","unit","direction","baseline","target_change_percent"}],"method":"..."}`. `PlanPreconditions`: thiếu `perf_baseline` `passed` thì `REQUEST_PERF_BASELINE_MISSING`; Plan phải có task `check:baseline` đầu và `check:after` cuối (`REQUEST_PLAN_PERF_CHECK_TASKS_MISSING`). `CompletionChecks`: `improvement% = (value - baseline) / baseline * 100` đảo dấu theo `direction`; metric đạt khi `improvement% >= target_change_percent`; thiếu `perf_after` hoặc thiếu metric là `missing` (chờ); không đạt là `failed`; `baseline = 0` là `failed` với tóm tắt "baseline bằng 0".
- **ops_request:** `PlanPreconditions`: bước `irreversible` có `gate:pre_deploy`; có ít nhất một task `rollback`; mỗi bước `irreversible` có task `rollback` đứng sau theo `depends_on` hoặc `rollback_note` khác rỗng (`REQUEST_RUNBOOK_ROLLBACK_MISSING`, `REQUEST_RUNBOOK_IRREVERSIBLE_STEP_UNGATED`). Không tự rollback; bước lỗi thì đi backlog stage `task` kèm tên task `rollback`. Hoàn tất cần `kind=ops_result` (`summary` khác rỗng).
- **refactor:** Plan có `check:tests_before` đầu, `check:tests_after` cuối (`REQUEST_PLAN_TEST_CHECK_TASKS_MISSING`). `tests_before = {"total","passed","failed","command"}`, `tests_after` thêm `"tests_modified"`. Đạt khi `tests_after.failed = 0`, `tests_after.total >= tests_before.total`, `tests_modified = false`; `tests_modified` là lời khai của agent, ghi rõ ở tóm tắt.
- **security:** `kind=security_recheck` (`passed`) bắt buộc trước hoàn tất; `failed` thì backlog stage `task`.
- **hotfix:** `OnCompleted` trả hai `FollowUp`: Request `bug` ("Nguyên nhân gốc và test hồi quy cho <title>") và `task` ("Review sau hotfix: <title>"), qua `SpawnChildRequest` `link_reason=followup_hotfix`; hai Request mới vào `classifying`.

Mã lỗi (FailedPrecondition trừ khi ghi khác): `REQUEST_PRE_DEPLOY_REQUIRED`, `REQUEST_PERF_BASELINE_MISSING`, `REQUEST_PLAN_PERF_CHECK_TASKS_MISSING`, `REQUEST_PLAN_TEST_CHECK_TASKS_MISSING`, `REQUEST_RUNBOOK_ROLLBACK_MISSING`, `REQUEST_RUNBOOK_IRREVERSIBLE_STEP_UNGATED`, `REQUEST_CHECK_NOT_ALLOWED_NOW`, `REQUEST_CHECK_INVALID_METRICS` (InvalidArgument).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Hiện thực qua `TypePolicy`, không rẽ nhánh trong use case chung | Thêm hoặc đổi chính sách không chạm đường chuẩn; test từng loại độc lập |
| Số đo ở `request_checks`, không lấy từ đầu ra task | Task-service không có đầu ra có cấu trúc; 8 KB chuỗi không đáng tin |
| `pre_deploy` gắn bằng nhãn `gate:pre_deploy` | `Task.Labels` có sẵn, không đổi `task-service` |
| Không tự rollback | Rollback tự động trên bước không đảo ngược có thể làm hỏng thêm |
| `tests_modified` do agent khai | Không thêm lệnh git; nói rõ giới hạn |
| Cổng thất bại đi backlog, không thêm loại Approval | README 3.5 chỉ có tám `subject_type` |
| Follow-up hotfix qua `SpawnChildRequest` idempotent | Dùng đường chung của CR-REQ-006 |

## 4. Phụ thuộc và thứ tự

Cần SOL-012 (nhãn, `PlanPreconditions` hook), SOL-013 (`PreExecutionGate`, `CompletionChecks`, `OnCompleted` hook), CR-REQ-009 (Approval `pre_deploy`), CR-REQ-006 (`SpawnChildRequest`, `ReturnToBacklog`), CR-REQ-008 (Chẩn đoán chừa trường `measurements[]`). Thứ tự task: 01 migration và repository; 02 RPC ghi và đọc; 03 interface, registry, noop, nối hook; 04 `pre_deploy` handler và hotfix/security; 05 performance và refactor; 06 ops_request; 07 hotfix follow-up và kiểm thử tích hợp. 05 và 06 song song sau 03. Mở khoá CR-REQ-025 (e2e).

## 5. Kiểm thử

- **Unit:** mỗi `TypePolicy` table-driven (`PlanPreconditions`, `PreExecutionGate`, `CompletionChecks`, `OnCompleted`); công thức cải thiện hai `direction`, baseline 0, thiếu metric; quy tắc `tests_after`; `GateRequirement` theo nhãn.
- **Integration hai dialect:** repository `request_checks` (append-only, bản mới nhất, tenant isolation); `OnCompleted` idempotent qua `client_request_id`; `AdvanceExecution` với `PreExecutionGate` chặn rồi mở.
- **Hợp đồng:** schema `request_checks` đọc từ `information_schema`; JSON `metrics` mẫu mỗi `kind`.
- **E2E (CR-REQ-025):** một Request mỗi loại đi hết luồng; agent giả ghi số đo qua RPC. Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/...` (chưa chạy).

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa kiểm chứng agent gọi được `RecordRequestCheck` qua MCP từ trong task; nếu không, số đo chỉ vào tay (`source=manual`) và `performance`, `refactor` thành nửa tự động.
- Số đo do agent khai không được xác minh; cổng duyệt của người (Approval `plan`) là lớp bảo vệ duy nhất.
- `pre_deploy` là cổng duyệt, không phải cổng kỹ thuật: agent vẫn chạy được lệnh deploy trong task không nhãn.
- `UpdateTask` thay cả danh sách nhãn: người sửa tay có thể xoá `gate:pre_deploy`; chưa có khoá nhãn (đề xuất: `request-service` kiểm lại nhãn trước khi chạy dựa vào `ai_plan_json` đã duyệt, chưa làm).
- Ngưỡng `target_change_percent` do AI/Chẩn đoán đề xuất có thể phi thực tế.
- Hai Request theo dõi sau hotfix có thể thừa; chưa có dữ liệu dùng thực.
- Số migration `request_checks` chưa cố định (xem SOL-013 mục 1, điều 4).

## 7. Câu hỏi mở

- `pre_deploy` của `hotfix` chiếm `awaiting_plan_approval` (duyệt trước khi chạy task fix) trong khi tên gọi gợi ý sau khi sửa xong: xác nhận ý nghĩa với CR-REQ-003.
- Bug size L, `refactor` có cần kiểm hoàn tất như `performance`? Hiện `bug` chỉ có task `test:regression`.
- `ops_request` thất bại giữa runbook: có cần RPC cho người kích rollback từ UI? Chưa thêm.
- Quy ước actor hệ thống cho `SpawnChildRequest` (ví dụ `system:request-service`) ở CR-REQ-006.
- Tham chiếu tiến: CR-REQ-029 (TaskSpec, ReadinessGate) và CR bổ sung về ExecutionResult có thể cung cấp kết quả có cấu trúc (số đo, test) thay cho `request_checks` do agent khai; khi đó `request_checks` có thể trở thành nơi ghi kết quả nhận từ ExecutionResult. Solution này không phụ thuộc vào đó.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.4, 3.5, 3.6, 6, 8 (điều 6, 12)
- `/opt/repos/orca/docs/crs/v6/plan-phase-task/CR-REQ-012-plan-phase-task-generation-from-solution.md`, `CR-REQ-013-phase-execution-and-feedback-loop.md`
- `/opt/repos/orca/docs/crs/v6/request-lifecycle/CR-REQ-003-request-state-machine-and-flow-registry.md`, `CR-REQ-006-return-to-backlog-reopen-cancel-child-requests.md`; `/opt/repos/orca/docs/crs/v6/approval/CR-REQ-009-generic-approval-domain-and-api.md`; `/opt/repos/orca/docs/crs/v6/solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go`, `usecase/update_task.go`, `usecase/report_execution_result.go`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0012_task_sources.up.sql` (mẫu RLS), `0011_task_widened_fields.up.sql`
- `/opt/repos/orca/specs/backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md`, `BE-REQ-SOL-013-phase-execution-and-feedback-loop.md`
