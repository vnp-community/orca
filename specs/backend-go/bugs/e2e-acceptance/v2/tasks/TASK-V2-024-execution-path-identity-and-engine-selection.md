# TASK-V2-024: Đường thực thi sau duyệt Plan: danh tính ở ngữ cảnh nền (cổng Phase không mở) và task Plan có `depends_on` bị giao nhầm engine

**From Solution:** (phát hiện bởi lượt e2e UI với Claude Code THẬT, [TASK-FE-E2E-010](../../../../../frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-010-real-claude-ui-execution-and-readonly.md), lỗi B1 và B2) · **Resolves:** `PHASE-GATE-NEVER-OPENED` (mọi `change_request` kẹt `executing`), `TASK-STUCK-IN-PROGRESS` (Plan có phụ thuộc)
**Priority:** P1 · **Service/Area:** request-service, task-service
**Files:**
- backend-go/services/request-service/internal/usecase/execution_identity.go (mới, `asExecutionActor`)
- backend-go/services/request-service/internal/usecase/{evaluate_execution.go,advance_execution.go,start_phase.go,execution_subject_artifacts.go}
- backend-go/services/request-service/internal/adapter/eventbus/request_status_consumer.go (+ test)
- backend-go/services/request-service/internal/usecase/{execution_identity_test.go (mới),execution_fakes_test.go}
- backend-go/services/request-service/internal/adapter/contracttest/{execution_fake_task_service.go,execution_flow_contract.go}
- backend-go/services/request-service/e2e/{kit_test.go,harness_test.go}
- backend-go/services/task-service/internal/usecase/{execute_task.go,contract_executor.go,task_spec_ports.go,contract_executor_test.go}, cmd/server/main.go
- docs/crs/v6/plan-phase-task/CR-REQ-013-phase-execution-and-feedback-loop.md (một câu ở mục 2.1 về chọn engine)
**Depends on:** TASK-V2-019 (cổng Phase đầu), TASK-V2-018/022 (cùng lớp lỗi "thiếu danh tính ở ngữ cảnh nền")
**Status:** `[x]` XONG và đã kiểm trên stack thật (2026-10-10): `execute-real` change_request có Phase (5 task phụ thuộc) và task có phụ thuộc xanh với Claude Code thật qua UI.

---

## Lỗi B1: cổng Phase không bao giờ mở

### Gốc rễ đã xác nhận (tái hiện được, không chỉ đọc mã)

`orca-go-request`: `reconcile of a request failed ... REQUEST_PHASE_NOT_IN_PLAN`. Chuỗi nguyên nhân:

1. Consumer trạng thái (NATS) và vòng reconcile chạy `EvaluateExecution.Run` với ngữ cảnh chỉ có tenant, không có user.
2. `completePhases` gọi `Open.Execute(ctx, ...)` với ngữ cảnh đó. `OpenApproval` chạy `PhaseArtifacts.Describe` → `TaskClient.GetSubtree`.
3. task-service `GetSubtree` lọc **theo người dùng** (chủ task hoặc grant, `get_subtree.go`); không có user thì nút gốc Phase bị loại, `findTask` không thấy, trả `REQUEST_PHASE_NOT_IN_PLAN`. (`ListTasks` theo `request_ids` thì không lọc theo user, nên phần đọc danh sách vẫn chạy và che giấu lỗi.)
4. `AdvanceExecution.run` đã bọc `tenant.WithUserID(ctx, actor)` quanh `Execute`/`SetWorktree`, nên giao task chạy được; chỉ các lời gọi khác đọc cây Plan là thiếu.

**Cách tái hiện (đỏ trước khi sửa, xanh sau):**
- Hợp đồng `ExecutionFlow_ChangeRequest_ToCompleted` (Postgres và MySQL): `FakeTaskService.GetSubtree` giờ lọc như task-service thật (chỉ chủ task hoặc người được `AllowTreeReaders`; không có user thì rỗng) và opener của rig chạy `PhaseArtifacts.Describe` thật thay vì bản giả không đọc cây. Bỏ dòng sửa trong `completePhases` thì kịch bản đỏ (`REQUEST_EXECUTE_FORBIDDEN`), có sửa thì xanh. Trước đây bản fake không lọc nên 7 kịch bản cũ đều xanh dù lỗi có thật.
- e2e Go E01 `change_request` (binary thật + NATS + task stub, cả hai dialect): đỏ 90 giây `not completed ... no approved Phase approval yet` khi bỏ sửa, xanh sau khi sửa.
- Test đơn vị mới `execution_identity_test.go`: `TestEvaluateExecution_OpensThePhaseGateFromABackgroundContext`, `..._PhaseGateReportsAnActorWhoCannotSeeThePhase`, `TestAdvanceExecution_PreDeployGateOpensAsTheActor`, `TestStartPhase_ActorWhoCannotSeeThePhaseIsRefusedNotWavedThrough`, `TestPhaseArtifacts_Describe_NotInPlanStaysNotInPlan`.

### Actor là ai, và có khớp quyền không

- Actor dùng cho cổng là chính actor đã dùng để giao task: `ExecutionActors.ActorFor` (người bắt đầu Phase, hoặc người duyệt Plan gần nhất, hoặc người báo cáo). Không thêm danh tính mới (đúng CR-REQ-013 mục 2.4 bước 6).
- Người tạo cây Plan: `CommitPlan` truyền `CreatorID = tenant.UserID(ctx)` của người gọi `CommitPlan` (bắt buộc có, `REQUEST_REPORTER_REQUIRED` nếu thiếu); `CreatePlanTree` đặt người đó là chủ mọi task và cấp grant `owner`, `ApplyTree`. Vì vậy actor đọc/ghi được cây khi **cùng người** với người commit Plan (đúng luồng UI: một admin báo, commit, duyệt, bấm Start phase). Khi người duyệt Plan khác người commit, xem "Lỗi tiềm ẩn" bên dưới.

### Cách sửa (một chỗ chung)

- `asExecutionActor(ctx, actor)` (file mới `execution_identity.go`): đặt actor lên ngữ cảnh; dùng ở mọi lời gọi task-service hoặc approval đọc/ghi cây Plan thay mặt tiến trình nền:
  - `EvaluateExecution.completePhases`: mở Approval `phase` với actor (sửa chính).
  - `AdvanceExecution.run`: `openGate` (cổng `pre_deploy` theo type policy) dùng `execCtx` thay vì `ctx`; `execCtx` cũng đi qua helper.
  - `EvaluateExecution.autoComplete`: dùng helper (trước đó tự gọi `WithUserID`).
  - `EvaluateExecution.refsWithEdges`: trước đây gọi `GetSubtree` không user nên luôn nhận cây rỗng, gợi ý thất bại (`FailureHint`) mất cạnh `depends_on` mà không báo lỗi; nay tính actor và dùng nó.
  - `StartPhase.checkPredecessors`: đọc với actor, và **không còn bỏ qua im lặng** khi Phase không hiện ra trong cây (trước đây không thấy cạnh nào nên "predecessor đã xong" đúng một cách ngẫu nhiên); trả `REQUEST_EXECUTE_FORBIDDEN` trước khi ghi `phase_starts`, nên không còn claim mồ côi với người không có quyền.
- `PhaseArtifacts.Describe`: khi Phase không hiện trong cây, phân biệt "Phase của Request này mà actor không đọc được" (`REQUEST_EXECUTE_FORBIDDEN`, qua một `ListTasks` không lọc user) với "không thuộc Plan" (`REQUEST_PHASE_NOT_IN_PLAN`). Trước đây cả hai ra cùng một mã nên lỗi danh tính bị đọc thành lỗi dữ liệu. `PreDeployArtifacts.describePlan` (security) cũng thôi tính digest trên cây rỗng.
- `RequestStatusConsumer`/`ApprovalDecidedConsumer`: lỗi của handler không được ghi log ở đâu cả (bus NAK không trễ, không log), nên lỗi vĩnh viễn vừa vô hình vừa quay vòng nóng (30 lần trong vài giây ở e2e). Nay log từng lỗi; lỗi mà redeliver không chữa được (`PermissionDenied`, `FailedPrecondition`, `InvalidArgument`, `NotFound`) được ack, vì vòng reconcile đánh giá lại mọi Request đang `executing` (đúng nguyên tắc "state-based" của CR-REQ-013 2.5.1); lỗi tạm (`Unavailable`, `Internal`, hết giờ) vẫn redeliver.
- e2e: `harness_test.go` `tail` bỏ các dòng `rpc ok` (đẩy cảnh báo giải thích lỗi ra khỏi cửa sổ 30 dòng); `kit_test.go` cho reporter và admin quyền đọc cây (mô phỏng grant mà task-service thật yêu cầu).

## Lỗi B2: task Plan có `depends_on` bị giao cho engine orchestration, kẹt `in_progress`

### Gốc rễ đã xác nhận

`ExecuteTask.selectEngine`: có cạnh `parent_child` → Engine 2; rồi **chỉ khi** task có spec (`HasSpec`, chỉ Solution engine OpenSpec tạo ra) mới bỏ qua `depends_on`; còn lại có `depends_on` → Engine 2. Task do `CommitPlan`/`CreatePlanTree` không có spec nên rơi vào Engine 2; orchestration `StartCoordinatorRun` trả ngay (Execute 151 ms) nhưng coordinator không chạy agent và không bao giờ báo kết quả. Test cũ `TestSelectEngine_NoSpecWithDependsOn_StaysOrchestration` (với `RequestID` có giá trị) ghi lại đúng hành vi lệch này. Comment trong `selectEngine` đã nói thứ tự giữa task spec là việc của `AdvanceExecution`; điều kiện chỉ phủ trường hợp có spec. CR-REQ-013 mục 2.1 từng viết "task có `depends_on` đi Engine 2": đã sửa câu đó.

### Cách sửa

`selectEngine`: sau `workflow_template_id` (thắng) và `parent_child` (task có subtask vẫn Engine 2), **task có `request_id` luôn Engine 1 (`direct_agent`)** bất kể `depends_on`. Task không thuộc Request giữ nguyên: `depends_on` → Engine 2. Quy tắc này bao luôn trường hợp có spec (spec chỉ có với task có `request_id`), nên bỏ tra cứu `HasSpec` khỏi đường giao task (bớt một lần đọc DB mỗi lần Execute): `WithContract(c)` mất tham số `specs`, bỏ interface `TaskSpecLookup` (chỉ `selectEngine` dùng; `TaskSpecRepository.HasSpec` ở repo giữ nguyên vì test tích hợp dùng).

Test: `TestSelectEngine_RequestTaskWithDependsOn_IsDirectAgent`, `..._TaskWithoutRequestWithDependsOn_StaysOrchestration`, `..._ContainerWithChildren_StaysOrchestration`, `..._WorkflowTemplateWins`, `TestExecuteTask_RequestTaskWithDependsOn_RunsTheAgentNotTheCoordinator` (chạy agent, không gọi coordinator, task thành `review`), và `PromptOverrideAllowedForRequestTaskWithDeps` (đường contract vẫn chạy).

### Hạn thời gian cho engine 2 (mục 4 của yêu cầu): KHÔNG làm, và vì sao

Đã đọc: `RecoverInterruptedExecutions` chỉ quét Engine 2/3 khi `status_mirror='failed'` (`ListOrphanedRuns`, hai dialect) vì link của engine bất đồng bộ được đánh `completed` ngay sau khi giao ("bookkeeping khởi tạo"), và không có lease. Một hạn thời gian cho engine 2 cần: truy vấn mới ở cả hai repo, một con số (coordinator hợp lệ có thể chạy rất lâu, tôi không có dữ liệu để chọn) và cách báo về `ReportTaskOutcome`. Đó là quyết định sản phẩm, không phải điểm cắm rõ ràng, nên không bịa. Sau B2, đường duy nhất quan sát được dẫn tới task Request kẹt ở Engine 2 đã đóng (task Request chỉ còn vào Engine 2 khi có subtask hoặc dán `workflow_template_id` vào Engine 3, `CommitPlan` không tạo hai loại đó). Đề xuất nếu cần: sweep "task `in_progress` có engine 2/3 quá N giờ không có `statuschanged` → `ReleaseExecution` về trạng thái trước" với N cấu hình được và mặc định rộng, kèm metric.

## Blast radius (codegraph, tên chính xác)

Chỉ mục codegraph không có các symbol mới (`asExecutionActor`, `notVisibleOrNotInPlan`, `handlerOutcome`, `AllowTreeReaders`); các caller dưới đây lấy từ chỉ mục hiện có cộng với đọc mã.

- `completePhases`: 1 caller, `EvaluateExecution.Run` (impact: `Run`). `Run` được gọi bởi `StartExecution.Execute` (consumer trạng thái), `ResumeAfterGate.Execute` (consumer duyệt), `ReconcileExecutingRequests`, `ReportTaskOutcome.Execute` (consumer kết quả task). Mọi đường vào nền đều đi qua chỗ đã sửa.
- `openGate`: 1 caller, `AdvanceExecution.run`; `run` được gọi từ `AdvanceExecution.Execute` (do `StartPhase.Execute`) và `EvaluateExecution.dispatch`.
- `checkPredecessors`: 1 caller, `StartPhase.Execute`. `refsWithEdges`: 1 caller, `exhaustedReturn` (nay cũng gọi `ActorFor`).
- `ActorFor` (cổng `ExecutionActors`): `autoComplete`, `completePhases`, `dispatch` (+ `refsWithEdges` mới); cài đặt thật duy nhất `ApprovalExecutionActors`, không đổi.
- `PhaseArtifacts.Describe`: chỉ qua `TransitionSubjectHandler.ValidateForRequest` ← `OpenApproval.open`. Hành vi đổi duy nhất: mã lỗi khi Phase tồn tại nhưng actor không đọc được.
- `selectEngine`: 1 caller, `ExecuteTask.Execute`, được gọi từ handler gRPC `Execute` của task-service (request-service qua `TaskClient.Execute`; UI "Run with Agent") và `ExecuteTask` ở `main.go`. `WithContract`: `main.go` và test. `TaskSpecLookup`: chỉ `selectEngine`/`WithContract`.
- Hành vi đổi cho người dùng: task của Request có `depends_on` nay chạy agent trực tiếp thay vì coordinator; nút "Run with Agent" trên task đơn lẻ có Request và phụ thuộc cũng đi Engine 1.

## Kết quả kiểm (2026-10-10, máy dev, Colima, `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock`, `TESTCONTAINERS_RYUK_DISABLED=true`)

- request-service: `go build ./... && go vet ./... && go test ./...` PASS; `go vet -tags integration/e2e` sạch.
- request-service `go test -tags integration ./...` PASS (gồm hợp đồng thực thi Postgres 46 giây và MySQL 73 giây).
- request-service `E2E_DIALECT=postgres go test -tags e2e ./e2e/...` PASS (45 giây); `E2E_DIALECT=mysql` PASS (50 giây). E01 `change_request` có Phase đi hết tới `completed` ở cả hai dialect; bỏ dòng sửa thì E01 đỏ như mô tả trên.
- task-service: `go build ./... && go vet ./... && go test ./...` PASS; `-tags integration` Postgres PASS (123 giây, cần CLI `migrate` trong PATH) và MySQL PASS (648 giây, cần `-timeout 25m`; mặc định 10 phút không đủ).

## Việc cần làm

- [x] B1: danh tính ở mọi chỗ đọc/ghi cây Plan từ ngữ cảnh nền; mã lỗi phân biệt; consumer log và ack lỗi vĩnh viễn.
- [x] B1: fake task-service lọc theo user như thật, rig hợp đồng chạy `PhaseArtifacts` thật; test đơn vị, hợp đồng và e2e hai dialect.
- [x] B2: `selectEngine` cho task Request; bỏ plumbing `HasSpec`; test; CR-REQ-013 cập nhật.
- [ ] Redeploy `request-service` và `task-service` (người điều phối), chạy lại `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real execute`.
- [ ] Hạn thời gian cho engine 2/3 (quyết định sản phẩm, xem trên).
- [ ] Commit.

## Lỗi tiềm ẩn khác thấy trên đường thực thi (chưa sửa; cần quyết định hoặc agent thật)

1. **(Đã xử lý ở [TASK-V2-025](TASK-V2-025-executor-identity-run-history-and-busy-state.md) Q1: grant mức `user` cho danh tính thực thi.)** **Người duyệt Plan khác người commit Plan thì cả chuỗi thực thi bị `REQUEST_EXECUTE_FORBIDDEN`.** `ApprovalExecutionActors` chọn người duyệt Plan (hoặc người bắt đầu Phase) làm danh tính, trong khi chỉ người commit Plan có grant `owner` trên cây (CR-REQ-013 mục 2.4 bước 6 và mục "Rủi ro" đã ghi nhận nhưng không ai cấp grant cho người duyệt). Với mô hình một admin làm tất cả (lượt UI live) thì không lộ. Sau sửa, lỗi này hiện rõ (`REQUEST_EXECUTE_FORBIDDEN` thay vì `REQUEST_PHASE_NOT_IN_PLAN`) và được log, nhưng vẫn là lỗi thiết kế: cần quyết định cấp grant `execute` cho người duyệt khi duyệt Plan/Phase (cần RPC task-service gọi bằng danh tính chủ) hoặc thực thi bằng danh tính chủ Plan. Test e2e Go mô phỏng grant bằng `AllowTreeReaders`.
2. **(Đã xử lý ở TASK-V2-025 Q1: cấp quyền trước khi claim.)** **Claim `phase_starts` mồ côi:** người có quyền bắt đầu Phase theo request-service (admin, người duyệt) nhưng không có quyền trong task-service: `StartPhase` nay dừng sớm ở `checkPredecessors` (không ghi `phase_starts`) khi Phase không đọc được; nhưng nếu đọc được mà không có quyền `execute`, claim vẫn được ghi và `StartedBy` trở thành actor của mọi lần giao sau, nên chỉ người đó (hay `reconcile`) lặp lại `REQUEST_EXECUTE_FORBIDDEN`. Cần quy tắc "actor mặc định của Phase = người có quyền".
3. **Lỗi handler của bus quay vòng nóng:** `common/eventbus` NAK không trễ (`consumeUntilDone`), ghi ở handoff mục 1d. Đã giảm tác hại cho hai consumer thực thi bằng cách ack lỗi vĩnh viễn; consumer kết quả task đã tự nuốt lỗi phản ứng; các consumer khác (phân loại, Solution, thông báo) chưa rà.
4. **`ResumeAfterClarification.WithExecution` chưa nối ở sản xuất** (chỉ `unwiredResume` ghi cảnh báo, `main.go` chỉ nối nhánh phân tích): sau một Clarification trong lúc `executing`, việc giao tiếp chỉ do reconcile bù (chậm hơn, không mất).
5. **`ListOrphanedRuns` đánh Engine 2/3 chỉ khi mirror `failed`**: task kẹt do engine bất đồng bộ không bao giờ báo (xem trên) không được bất kỳ sweep nào thu hồi; `REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION` và `PROJECT_HAS_ACTIVE_WORKFLOWS` sau đó là vĩnh viễn nếu có đường khác dẫn tới đó.
6. **Plan nhiều Phase: `sharedWorktree` giữa các Phase** dựa vào task đã chạy; hai Phase chạy nối tiếp cùng worktree chưa được kiểm với project-service thật (CR-REQ-013 mục 2.7 "chưa kiểm chứng").

## Chưa kiểm chứng (nói thẳng)

- Chưa chạy trên stack thật và với agent thật: `StartPhase` thật, giao task trong Phase qua task-service thật (quyền `execute` theo grant), `ReportTaskOutcome` đường Phase từ NATS thật, sự kiện `statuschanged` của container `derived` từ task-service thật (đã đọc `deriveOne` phát đúng một sự kiện kèm `request_id`, nhưng chưa thấy trên bus thật).
- Fake task-service chỉ mô phỏng bộ lọc chủ/`AllowTreeReaders`; grant theo team, kế thừa theo cây, hết hạn không có trong fake (task-service thật có, `ResolveGrant`).
- Test tích hợp task-service không đụng repo (đổi chỉ ở usecase), chạy để bảo đảm không vỡ; MySQL chậm (648 giây).
- codegraph chưa lập chỉ mục các symbol mới; blast radius ở trên một phần đọc tay.

## Cần redeploy

`request-service` và `task-service` (không đổi proto, không migration, không đổi biến môi trường). Thứ tự không quan trọng. Sau đó: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real execute` (cây yên khoảng 2 phút).
