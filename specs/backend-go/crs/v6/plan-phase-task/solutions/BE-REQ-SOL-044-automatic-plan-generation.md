# BE-REQ-SOL-044: Tự động sinh Plan sau khi duyệt Analysis: job bền, commit phía server, thử lại có giới hạn

> **📝 Chưa triển khai (0/9 task).** Soạn 2026-10-20 từ đọc mã. Mọi mô tả hiện trạng chưa được kiểm bằng chạy thật.

**CR:** [CR-REQ-044](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-044-automatic-plan-generation-after-analysis-approval.md)
**Service:** `request-service` · `api-gateway` (`wscompat`, MCP) · `proto/orca/request/v1` · `policy/orca-authz`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (không bỏ qua quyền ở RPC công khai)
**Phụ thuộc solution:** [BE-REQ-SOL-012](./BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md) (`GeneratePlan`, `CommitPlan`), [BE-REQ-SOL-026](../../solution-engines/solutions/BE-REQ-SOL-026-openspec-solution-engine.md) (run `agent_proposal`), [BE-REQ-SOL-038](./BE-REQ-SOL-038-continuous-execution-activity-feed-and-execution-summary.md) (`ProjectEngineSettings`). Frontend: [FE-REQ-SOL-044](../../../../../frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-044-automatic-plan-generation.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-20 (đường dẫn dưới `backend-go/services/request-service/`):

- `internal/usecase/solution_approval_handler.go` `OnApproved` (`:127`) duyệt Solution, ghi outbox `orca.request.solution.approved` (`:166`), chạy transition `TriggerAnalysisApproved` (`:177`). `internal/domain/request_transition.go:83` đưa tới `planning` (hoặc `completed` nếu `CompletesAfterAnalysis`). Sự kiện `status_changed` mang `request_id`, `project_id`, `to`, `trigger`, `type`, `reporter_id` (`usecase/transition_request_events.go`, struct `StatusChangedPayload`).
- Ngoài `TriggerAnalysisApproved`, các trigger đưa Request vào `planning`: `type_confirmed` khi `flow.AnalysisKind == none` (`request_transition.go:69`), `analysis_ready` khi không có cổng và không phải `PlanSingleTask` (`:74-82`; không loại nào hiện dùng), `plan_revision` (replan).
- `usecase/generate_plan.go:70` `GeneratePlan.Execute`: cần user trong `ctx` (`ErrRequestReporterRequired`), `AuthorizePlan`, `checkPlannable`, `conns.ResolveForProject` (không dev server → `ErrPlanAIUnavailable`), chọn mode theo engine ghim, `StartRun` trong transaction, rồi `spawner.Spawn`. **Không có `IdempotencyKey`** (`GeneratePlanInput` chỉ có `RequestID`, `Feedback`); `run.ActorID = actor`.
- `adapter/postgres/analysis_run_repository.go:56` `findBlocking`: trả run đã có khi `idempotency_key` khớp (mọi trạng thái) hoặc khi có run cùng `kind` đang `running`; MySQL `:68` giống nhưng nhận thêm tham số `current` (đọc khoá `FOR SHARE` sau khi thua cuộc đua chèn). Chỉ mục `analysis_runs_idem` duy nhất `(tenant_id, request_id, idempotency_key)`; `analysis_runs_one_running` duy nhất `(tenant_id, request_id, kind)` khi `running`. `ErrAnalysisBusy` = `REQUEST_ANALYSIS_BUSY` (`domain/solution_errors.go:64`) chỉ cho mode agent, trần `MaxAgentRunsPerProject` mặc định 2 (`usecase/analysis_settings.go:40`).
- `usecase/analysis_runner.go`: `reporterIdentity` (`:74`) **cố ý giữ danh tính người gọi** cho run Plan; `runCtx` dựng lại user từ `run.ActorID`; `failRun` (`:175`) và `RecoverInterruptedAnalysisRuns.RunOnce` (`recover_interrupted_analysis_runs.go:29`, mã `REQUEST_SOLUTION_RUN_INTERRUPTED` ở `:38`) đánh dấu `failed`. Không có sự kiện hay móc nào phát ra khi run Plan kết thúc.
- Hai worker kết thúc bằng cùng `finishPlanRun(... EncodePlanProposal(p))` (`run_plan_generation.go`, `run_proposal_generation.go` `finish`): `analysis_runs.raw_output` giữ proposal đã mã hoá; `usecase/plan_document.go` `DecodePlanProposal` đọc lại, `GetPlanProposal` dùng cho cả native lẫn `agent_proposal` (`get_plan_proposal.go:99-104`).
- `usecase/commit_plan.go:66` `CommitPlan.Execute`: cần user, `AuthorizePlan`, proposal do người gọi đưa; `replay` chỉ đúng khi Request đã `awaiting_plan_approval` và có `PlanTaskID` (`:82`); `CreatePlanTree` idempotent theo Request (task-service, `AlreadyExists`); `record` (`:128`) trả `REQUEST_STATE_STALE` nếu `cur.Status != planning` (`:134`).
- `usecase/resume_after_clarification.go:84` `Handle` và `adapter/eventbus/clarification_resume_consumer.go:15` (`request-service-clarification-resume`) là mẫu: lọc trigger, `processed.MarkProcessed` cùng transaction, `after()` sau commit, `ErrResumeSkipped` cho lỗi vĩnh viễn (consumer trả lỗi thì bus giao lại **ngay**, không có độ trễ: `common/eventbus/eventbus.go:123-155`). Công tắc `REQUEST_CLARIFICATION_AUTO_REGENERATE` ở `cmd/server/wire_artifact.go:140`; vòng `ClarificationSweeper.RunLoop` ở `:135`.
- `processed_events` khoá chính `(tenant_id, event_id)` (`migrations/postgres/0001_init.up.sql:19`): một sự kiện chỉ được **một** consumer đánh dấu.
- `cmd/server/wire_plan.go:76`: khi approval tắt thì `Commit` nil và chỉ ghi cảnh báo. `wirePlan` chạy trước `wireArtifact` (`main.go:191`, `:202`); `busConsumer` có ở `main.go`.
- `domain/engine_settings.go` `ProjectEngineSettings` (`SolutionAgent`, `ExecutionMode`); `adapter/postgres|mysql/project_engine_settings_repository.go` `Get`/`Upsert`; `usecase/manage_project_engine_settings.go` giữ `ExecutionMode` khi client cũ gửi rỗng; proto `solution.proto:143-175` (`ProjectEngineSettings` tới trường 6, `SetProjectEngineSettingsRequest` tới trường 6); gateway `wscompat/channels_request_engine.go`.
- Migration mới nhất của `request-service`: `0100_analysis_runs_revision_index`. `GetProjectEngineSettings` cho dự án chưa có dòng trả mặc định `native` + `step_by_step` (`manage_project_engine_settings.go`).
- RPC mới cần vào `domain/rpc_catalog.go` (mẫu `:107` cho `GeneratePlan`), `adapter/grpc/flow_gate.go` (mẫu `:114`), `api-gateway/.../request_rpc_channel_map_test.go`, và `mcpserver/tools/excluded_channels.yaml` (mẫu `request.setBaseRef`, `request.planProposal`).

### Correction relative to CR-REQ-044

1. CR nêu mã lỗi `REQUEST_SOLUTION_AI_FAILED`/`_AI_TIMEOUT` cho run Plan; thực tế đó là mã của run **Solution**. Run Plan native ghi `REQUEST_AI_COMPLETE_TIMEOUT`, `REQUEST_PLAN_AI_UNAVAILABLE`, `REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_AI_*`; run OpenSpec ghi `REQUEST_OPENSPEC_*`; sweeper ghi `REQUEST_SOLUTION_RUN_INTERRUPTED`. Bảng phân loại ở 2.4 dùng mã thật.
2. "Commit idempotent `AlreadyExists`" đúng ở task-service nhưng `CommitPlan.record` vẫn trả `REQUEST_STATE_STALE` cho bên thua cuộc đua; task 03 sửa.
3. Tính "chạy được cả hai engine" dựa vào việc hai worker cùng `finishPlanRun`; xác nhận bằng test `RawOutput` của run `agent_proposal` giải mã được (task 03), chưa chạy với agent thật.
4. Bus không có độ trễ redeliver, nên "hoãn khi bận" và "thử lại sau N giây" cần lịch trong cơ sở dữ liệu (bảng job), không thể chỉ dựa vào trả lỗi cho consumer.
5. Không có giao diện cho `ProjectEngineSettings` trong frontend (câu hỏi mở 5.1 của CR); công tắc dự án đổi qua kênh admin.

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `backend-go/`)

```
services/request-service/
  migrations/{postgres,mysql}/0101_plan_auto_generation.{up,down}.sql             (mới) bảng plan_auto_jobs, cột plan_auto_generate
  internal/domain/plan_auto.go, plan_auto_test.go                                   (mới) trạng thái, khoá, mã tạm thời, chính sách từ env
  internal/domain/engine_settings.go                                                (sửa) PlanAutoGenerate
  internal/domain/rpc_catalog.go                                                    (sửa) RetryPlanGeneration
  internal/usecase/ports_plan_auto.go                                               (mới) PlanAutoJobRepository, PlanAutoSwitch
  internal/usecase/enqueue_plan_auto.go                                             (mới) + test
  internal/usecase/plan_auto_worker.go, plan_auto_worker_start.go,
        plan_auto_worker_observe.go, plan_auto_worker_commit.go                     (mới) + test mỗi bước
  internal/usecase/retry_plan_generation.go                                         (mới) + test
  internal/usecase/generate_plan.go                                                 (sửa) IdempotencyKey, ExecuteForSystem
  internal/usecase/commit_plan.go                                                   (sửa) ExecuteForSystem, record idempotent
  internal/usecase/get_plan_proposal.go                                             (sửa) AutoPlan, không lỗi khi chỉ có job
  internal/usecase/manage_project_engine_settings.go                                (sửa) plan_auto_generate tuỳ chọn
  internal/adapter/postgres|mysql/plan_auto_job_repository.go                       (mới)
  internal/adapter/postgres|mysql/project_engine_settings_repository.go             (sửa) cột mới
  internal/adapter/contracttest/plan_auto_job_contract.go, expected_columns_plan_auto.go   (mới/sửa)
  internal/adapter/eventbus/plan_auto_consumer.go                                   (mới) durable request-service-plan-auto-generate
  internal/adapter/grpc/server_plan_auto.go                                         (mới) RetryPlanGeneration
  internal/adapter/grpc/server_plan.go, server_engine_settings.go, flow_gate.go     (sửa)
  cmd/server/wire_plan_auto.go                                                      (mới), cmd/server/main.go (sửa) gọi wirePlanAuto
  internal/usecase/plan_auto_switch.go                                              (mới) + test
services/api-gateway/internal/adapter/wscompat/
  channels_request_plan.go (sửa), channels_request_plan_auto.go (mới), channels_request_engine.go (sửa),
  channels_request_base_ref.go (sửa) requestFeatures(), request_rpc_channel_map_test.go (sửa)
services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml        (sửa) request.retryPlan
proto/orca/request/v1/plan.proto, solution.proto, request.proto                     (sửa) + mã sinh
policy/orca-authz/request_test.rego                                                 (sửa) ca RetryPlanGeneration
specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md                  (sửa, task 08)
cmd/orca-cli/internal/command/engine_cmd.go                                         (sửa) cờ --plan-auto-generate
```

### 2.2 Miền: trạng thái, khoá và chính sách

`domain/plan_auto.go` (thuần Go):

- `type PlanAutoState string`: `pending`, `running`, `committing`, `done`, `gave_up`, `skipped`, `superseded`. `IsActive()` đúng cho ba trạng thái đầu; `CanMove(to)` mô tả máy trạng thái: `pending→{running,skipped,gave_up,superseded,pending}`, `running→{committing,pending,gave_up,superseded}`, `committing→{done,gave_up,superseded,committing}`; ba trạng thái cuối không đi tiếp.
- `type PlanAutoJob struct`: `TenantID`, `RequestID`, `Anchor`, `Origin` (`event`|`retry`), `ProjectID`, `State`, `Attempt`, `MaxAttempts`, `RunID`, `ReasonCode`, `CommitAttempts`, `NextCheckAt`, `LeaseOwner`, `LeaseExpiresAt`, `RequestedBy`, `CreatedAt`, `UpdatedAt`.
- `PlanAutoKey(anchor string, attempt int) string`: `plan-auto-<anchor>` cho lượt 1, `plan-auto-<anchor>-<n>` cho n≥2 (kiểm độ dài ≤128).
- `SolutionAnchor(id)`, `TypeConfirmAnchor(eventID)` (`ty-<eventID>`), `RetryAnchor(uuid)` (`retry-<uuid>`).
- `IsTransientPlanRunCode(code string) bool`: đúng cho `REQUEST_AI_COMPLETE_TIMEOUT`, `REQUEST_PLAN_AI_UNAVAILABLE`, `REQUEST_AI_BUSY`, `REQUEST_OPENSPEC_TIMEOUT`, `REQUEST_OPENSPEC_AGENT_FAILED`, `REQUEST_SOLUTION_RUN_INTERRUPTED`; sai cho mọi mã khác (đặc biệt `REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_OPENSPEC_INVALID_OUTPUT`). Một bảng duy nhất, có test liệt kê từng mã đã biết.
- `PlanAutoPolicy`: `MaxExtraAttempts` (0..2, mặc định 1), `RetryDelay` (30 giây), `BusyDelay` (60 giây), `StartWindow` (30 phút), `MaxCommitAttempts` (5), `PollInterval` (5 giây), `LeaseTTL` (60 giây). `PlanAutoPolicyFromEnv(getenv) (PlanAutoPolicy, []string)` đọc `REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS`, `REQUEST_PLAN_AUTO_RETRY_DELAY`, `REQUEST_PLAN_AUTO_BUSY_DELAY`, `REQUEST_PLAN_AUTO_START_WINDOW`, `REQUEST_PLAN_AUTO_POLL_INTERVAL`; giá trị lạ giữ mặc định và trả cảnh báo (mẫu `EngineDefaultsFromEnv`, `PlanLimitsFromEnv`).
- `func WantsAutoPlan(req Request, flow FlowDefinition, trigger Trigger) bool`: luật nhận job của CR 2.2 (trigger, `PlanKind`, `PlanShapeFor`, `PlanTaskID == ""`).

### 2.3 Công tắc

- Env `REQUEST_PLAN_AUTO_GENERATE` (đọc bằng `envBool(..., true)` trong `wire_plan_auto.go`); tắt thì không dựng consumer, worker, và `RetryPlanGeneration` trả `REQUEST_PLAN_AUTO_DISABLED`. Cũng cần `cfg.RequestFlowEnabled`, bus, và `Commit != nil`.
- `PlanAutoSwitch.Enabled(ctx, projectID) bool` = env bật **và** `EngineSettingsRepository.Get(projectID)` (không có dòng = `true`). Lỗi đọc cài đặt không được âm thầm bật một tính năng tốn tiền: job ở lại `pending` và thử lại sau `RetryDelay` trong `StartWindow`, giống lỗi hạ tầng.
- `ProjectEngineSettings.PlanAutoGenerate bool`. `NewProjectEngineSettings` đặt `true`. `SetProjectEngineSettingsInput.PlanAutoGenerate *bool`; `nil` giữ giá trị đã lưu (hoặc `true` nếu chưa có dòng), cùng cách `ExecutionMode`. Audit metadata thêm `plan_auto_generate`.

### 2.4 Bắt đầu, quan sát, commit, thử lại

**`EnqueuePlanAuto.Handle(ctx, StatusChangedEvent)`** (consumer gọi): trong một transaction: `processed.MarkProcessed(eventID, SubjectRequestStatusChanged)`; nếu đã xử lý thì thoát; kiểm công tắc; đọc Request; `domain.WantsAutoPlan`; tìm Solution đã duyệt mới nhất (`latestApprovedAnalysis`) làm `anchor` (hoặc `TypeConfirmAnchor`); chèn job `pending` với `MaxAttempts = 1 + MaxExtraAttempts`. Chèn trùng (khoá chính hoặc chỉ mục một job hoạt động) là bình thường, ghi log mức info và thoát. Không bao giờ trả lỗi vì lý do nghiệp vụ; chỉ trả lỗi hạ tầng của chính transaction này.

**`PlanAutoWorker.RunOnce(ctx)`** nhận một lô job đến hạn (`ClaimDue(owner, leaseTTL, batch)`), xử lý từng job trong goroutine riêng có `tenant.WithTenantID` và, sau khi đọc Request, `tenant.WithUserID(ctx, req.ReporterID)`. Mỗi bước tải lại Request; nếu không còn `planning`, hoặc đã có `PlanTaskID` (chỉ với `origin` `event`/`retry`, mục 2.8), hoặc run Plan mới nhất (`ListRecent`) không phải của job và mới hơn nó, thì `superseded`.

| Trạng thái | Bước | Chuyển |
|---|---|---|
| `pending` | `GeneratePlan.ExecuteForSystem{RequestID, IdempotencyKey: PlanAutoKey(anchor, attempt)}` | Thành công (tạo mới hoặc nhận lại): `running`, ghi `run_id`. Bận: `pending`, `next_check_at += BusyDelay`; quá `StartWindow` thì `gave_up(REQUEST_ANALYSIS_BUSY)`. `Kind` ∈ {`FailedPrecondition`, `InvalidArgument`, `NotFound`, `PermissionDenied`}: `skipped(code)`. Lỗi khác: `pending`, `+= RetryDelay`, cùng cửa sổ |
| `running` | `runs.Get(run_id)` | `running`: `next_check_at += PollInterval`. `succeeded`: `committing`. `failed`: nếu `IsTransientPlanRunCode(code)` và `Attempt < MaxAttempts`: `Attempt++`, `run_id` rỗng, `pending`, `+= RetryDelay`; ngược lại `gave_up(code)` |
| `committing` | `DecodePlanProposal(run.RawOutput)` rồi `CommitPlan.ExecuteForSystem` | `AlreadyExists` hoặc đã commit: `done`. Mã `REQUEST_PLAN_*`: `gave_up(code)`. Lỗi khác: `CommitAttempts++`; đủ `MaxCommitAttempts` thì `gave_up(REQUEST_PLAN_COMMIT_FAILED)`, chưa đủ thì `+= RetryDelay` |

Mọi chuyển trạng thái là một `UPDATE ... WHERE ... AND lease_owner = $owner` (trả `false` khi mất lease; bước dừng im lặng). `ExecuteForSystem` tự sinh worker sau khi transaction của nó đóng (`GeneratePlan.Execute` đã làm vậy), nên bước `pending` không giữ transaction nào quanh việc sinh. Nếu ghi `running` sau đó thất bại, vòng sau gọi lại với cùng khoá và `findBlocking` trả run đó (`Created=false`) để nhận lại. Run mồ côi không có goroutine gia hạn lease sẽ bị sweeper đánh dấu `REQUEST_SOLUTION_RUN_INTERRUPTED` và được thử lại như lỗi tạm thời.

**Quan sát** (số liệu): mỗi lần job kết thúc ghi một dòng log có cấu trúc (`request_id`, `origin`, `state`, `reason_code`, `attempt`), không nội dung; nếu `EngineObserver` mở rộng gọn thì thêm bộ đếm `plan_auto_jobs{state,reason}`.

### 2.5 Hệ thống bắt đầu và commit thay người dùng

- `GeneratePlanInput.IdempotencyKey string`; `run.IdempotencyKey = &k` khi khác rỗng. RPC công khai không điền trường này.
- `GeneratePlan.ExecuteForSystem(ctx, in)`: giống `Execute` nhưng bỏ `AuthorizePlan`, `actor = req.ReporterID` nếu `ctx` không có user, dùng `tenant.WithUserID` để `conns.ResolveForProject` tìm dev server của người báo cáo; vẫn kiểm `checkPlannable`, `modeFor`, `StartRun`. Mẫu: `GenerateSolution.PrepareForSystem` (`generate_solution.go:160`) và `skipWithoutConnection`.
- `CommitPlan.ExecuteForSystem(ctx, in)`: bỏ `AuthorizePlan`, `actor = req.ReporterID`; mọi bước khác y nguyên. `record` coi Request đã `awaiting_plan_approval` với `PlanTaskID == tree.PlanTaskID` là thành công (không `REQUEST_STATE_STALE`).
- `ActorKind` của transition và Approval vẫn là `user` (người báo cáo) vì `OpenApprovalInput.RequestedBy` và `plan_ready` đòi một danh tính người; hành động "do hệ thống" nhận biết được qua `anchor` của job và log, không qua `ActorKind` (không thêm giá trị mới ở CR này).

### 2.6 Retry thủ công và đọc trạng thái

`RetryPlanGeneration.Execute(ctx, requestID)`: `AuthorizePlan` (người dùng thật); công tắc env tắt → `REQUEST_PLAN_AUTO_DISABLED`; Request phải `planning` (`REQUEST_STATE_STALE` nếu không); nếu có job hoạt động → trả nó; nếu run Plan mới nhất `succeeded` và Request chưa có `PlanTaskID` → chèn job `retry` ở `committing` với `run_id` đó (không tốn AI); ngược lại chèn job `retry` `pending`, `MaxAttempts = 1`. Trả `PlanAutoStateView`.

`GetPlanProposal` thêm `AutoPlan PlanAutoView{Enabled, State, Origin, Attempt, MaxAttempts, ReasonCode, NextAttemptAt}`: `Enabled` tính bằng `PlanAutoSwitch` + `Commit != nil` cho mọi Request; job mới nhất của Request được đưa vào khi Request đang `planning`. Khi không có run Plan nhưng có job, trả view rỗng (`RunID ""`) thay vì `REQUEST_PLAN_RUN_NOT_FOUND`.

### 2.7 Wiring

`cmd/server/wire_plan_auto.go` `wirePlanAuto(ctx, wg, cfg, log, stores, lifecycle, plan, solution, busConsumer)`: nếu env tắt hoặc thiếu điều kiện thì ghi một dòng log nói rõ lý do và trả về với `Retry == nil`; ngược lại dựng `PlanAutoSwitch`, `EnqueuePlanAuto`, consumer, `PlanAutoWorker` (chạy `RunLoop` trong goroutine theo `wg`), `RetryPlanGeneration`; gắn vào `PlanUseCases` (thêm trường `Retry`) và `PlanUseCases.Get.WithAutoPlan(...)`.

### 2.8 Điểm tựa cho CR-REQ-045

Bước `committing` và `CommitPlan.ExecuteForSystem` được viết để dùng lại: CR-REQ-045 chèn một job có `run_id` của lượt chỉnh (`state=running`, `max_attempts=1`) và mở rộng `CHECK` của `origin` trong migration của nó. Luật "đã có `PlanTaskID` thì `superseded`" ở worker chỉ áp cho `origin` là `event` hoặc `retry`, để job của 045 (đang replan, có `PlanTaskID`) không bị bỏ nhầm. Test của task 05 phủ luật này bằng một job `origin` giả định thứ ba.

### 2.9 Mã lỗi

`REQUEST_PLAN_AUTO_DISABLED` (FailedPrecondition, trả bởi `RetryPlanGeneration`); `REQUEST_PLAN_COMMIT_FAILED` (chỉ là `reason_code` của job, không là lỗi RPC). Các mã còn lại tái dùng mã đã có.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| S1 | Consumer chỉ chèn job; worker làm mọi việc | Một đường thực thi, dễ kiểm thử |
| S2 | Bảng `plan_auto_jobs` + lease, không dùng chỉ sự kiện | Bus không có độ trễ; cần đếm lượt và hiển thị trạng thái sau khi tải lại |
| S3 | Khoá run `plan-auto-<anchor>[-n]`; lượt thử lại dùng khoá mới | `findBlocking` trả run đã `failed` nếu khoá khớp |
| S4 | Chạy với danh tính người báo cáo; không bỏ qua quyền ở RPC công khai | Dev server gắn người báo cáo; `PrepareForSystem` là tiền lệ |
| S5 | Phân loại lỗi bắt đầu theo `Kind` thay vì liệt kê mã | Không bỏ sót mã mới; mã vĩnh viễn luôn thành `skipped`, không lặp vô hạn |
| S6 | Retry sau lỗi commit tiếp tục từ `committing` | Không trả tiền AI hai lần cho lỗi không phải của AI |
| S7 | Lỗi đọc cài đặt dự án không được bật ngầm tính năng tốn tiền | Thử lại trong cửa sổ rồi `gave_up` |
| S8 | `optional bool` cho công tắc dự án trong `Set` | Client cũ không làm đổi công tắc |
| S9 | Không thêm `ActorKind` mới | Phạm vi hẹp; nhận biết qua log và anchor |

## 4. Phụ thuộc và thứ tự

01 miền → 02 migration và repository, 03 đường hệ thống của `GeneratePlan`/`CommitPlan` (song song sau 01) → 04 công tắc dự án và `PlanAutoSwitch` (cần 01, 02) → 05 `EnqueuePlanAuto` và consumer (cần 02, 04) → 06 `PlanAutoWorker` và wiring (cần 03, 05) → 07 `RetryPlanGeneration`, `GetPlanProposal`, proto và gRPC (cần 06) → 08 gateway, MCP, authz, CLI, CONTRACT (cần 04, 07) → 09 tích hợp, hợp đồng và e2e.

## 5. Kiểm thử

- Unit miền: bảng mã tạm thời theo từng mã, máy trạng thái, khoá, `WantsAutoPlan` (11 loại × trigger × `PlanTaskID`), `PlanAutoPolicyFromEnv`.
- Usecase có fake (đồng hồ giả): từng bước của worker (mục 2.4), `EnqueuePlanAuto` (giao lại hai lần, hai công tắc, chèn trùng), `GeneratePlan.ExecuteForSystem`, `CommitPlan.ExecuteForSystem` và cuộc đua `record`, `RetryPlanGeneration` (ba nhánh), `ManageProjectEngineSettings` giữ công tắc, `GetPlanProposal` chỉ có job.
- Hợp đồng hai dialect: bảng `plan_auto_jobs` (khoá chính, chỉ mục một job hoạt động, `ClaimDue` `SKIP LOCKED` không trả trùng cho hai worker, cập nhật có điều kiện theo lease), cột `plan_auto_generate`.
- Tích hợp (Postgres thật): chuỗi `status_changed` → job → run (fake worker) → commit (fake task writer) → `done`; giao lại sự kiện hai lần; hai worker cùng lúc; người bấm tay chen ngang.
- Gateway: `wscompat` cho `request.retryPlan`, `autoPlan`, `planAutoGenerate`; `parity_test`; `request.rego` test.
- Acceptance `APG-xx` và UI live: xem task 09 và FE-REQ-TASK-044-04.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Blast radius**: `GeneratePlan.Execute` và `CommitPlan.Execute`/`record` được dùng bởi RPC và nhiều test; chạy `gitnexus_impact` cho cả hai và `GetPlanProposal.Execute`. `PlanUseCases` thêm trường nên mọi nơi dựng nó (test wiring) phải cập nhật.
- **Chi phí** (R1 của CR): duyệt Analysis của loại OpenSpec tự tốn 7 đến 13 USD; công tắc dự án là lối thoát.
- **Số migration `0101`** có thể bị CR khác giữ; kiểm lại.
- **Cột sinh MySQL cho "một job hoạt động"**: mẫu `active_key` của `analysis_runs` dùng `CONCAT(request_id, ':', kind)`; chưa thử cho trạng thái nhiều giá trị, cần test hợp đồng.
- **Lỗi tạm thời giả**: `REQUEST_OPENSPEC_AGENT_FAILED` gộp nhiều nguyên nhân; thử lại có thể tốn thêm một Plan (R2 của CR).
- Chưa chạy hệ thống; mô tả hành vi hiện tại dựa trên đọc mã ngày 2026-10-20.

## 7. Câu hỏi mở

Như CR-REQ-044 mục 5 (giao diện cho công tắc dự án, mã `REQUEST_PLAN_AI_UNAVAILABLE` ở cấp run, dòng nhật ký hoạt động, cửa sổ hoãn khi bận). Mặc định tạm đã nêu trong CR.

## 8. Tham chiếu

- `backend-go/services/request-service/internal/usecase/{solution_approval_handler,generate_plan,commit_plan,get_plan_proposal,run_plan_generation,run_proposal_generation,analysis_runner,recover_interrupted_analysis_runs,resume_after_clarification,solution_regenerator,generate_solution,manage_project_engine_settings,analysis_settings}.go`
- `backend-go/services/request-service/internal/adapter/{eventbus/clarification_resume_consumer.go,postgres/analysis_run_repository.go,mysql/analysis_run_repository.go,postgres/project_engine_settings_repository.go,grpc/server_plan.go,grpc/flow_gate.go}`, `internal/domain/{engine_settings,plan_errors,solution_errors,request_transition,rpc_catalog}.go`, `cmd/server/{wire_plan,wire_artifact,main}.go`
- `backend-go/proto/orca/request/v1/{plan,solution,request}.proto`, `backend-go/common/eventbus/eventbus.go`
- [CR-REQ-044](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-044-automatic-plan-generation-after-analysis-approval.md)
