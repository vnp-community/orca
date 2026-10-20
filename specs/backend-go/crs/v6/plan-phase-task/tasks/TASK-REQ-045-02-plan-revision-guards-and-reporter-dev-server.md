# TASK-REQ-045-02: Đường chỉnh Plan chắc hơn: khoá idempotency, lần chỉnh thứ hai, kế thừa phản hồi, hotfix, dev server của reporter, job commit `revision`

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`, `proto`
**File:** `migrations/{postgres,mysql}/0104_plan_auto_jobs_revision_origin.{up,down}.sql` (mới), `proto/orca/request/v1/plan.proto` (sửa) + mã sinh, `internal/usecase/{generate_plan,plan_revision_recovery,analysis_runner}.go` (sửa/mới), `internal/domain/plan_errors.go` (sửa), `internal/adapter/grpc/server_plan.go` (sửa), `internal/usecase/run_proposal_generation.go` (lỗi `NO_CHANGE`, xem việc 6), test tương ứng
**Depends on:** TASK-REQ-045-01, **BE-REQ-TASK-044-02, 044-03, 044-06** (bảng `plan_auto_jobs`, `PlanAutoJobRepository`, `ExecuteForSystem`, worker)
**Status:** [x] DONE (2026-10-20)

---

## Context

- `GeneratePlan.Execute` không có `IdempotencyKey`, bỏ qua `res.Created=false`, không kế thừa phản hồi của lượt thất bại, không loại `hotfix` ở đường chỉnh (`generate_plan.go:84-143`).
- Cổng Analysis đã có `inheritedFeedback` và `refuseSecondRevision` (`solution_revision_recovery.go`); bài học TASK-REQ-041-04: người chỉnh thứ hai nhận lượt của người thứ nhất, và phản hồi của lượt thất bại bị mất.
- `reporterIdentity` loại `RunKindPlan` (`analysis_runner.go:74-83`, comment "Plan runs keep the caller's identity"); `GeneratePlan.Execute` tra dev server bằng ngữ cảnh người gọi (dòng 93). Dev server gắn với reporter, nên approver nhận `REQUEST_PLAN_AI_UNAVAILABLE` hoặc lượt nền thất bại.
- `commitAndRead` trả lỗi "the worktree has nothing to commit" khi agent không đổi gì (`run_proposal_generation.go:328`), `finish` đổi thành `REQUEST_SPEC_AGENT_FAILED` chung chung.
- Hotfix có Plan một task tạo từ chẩn đoán (`PlanSingleTask`, Approval `pre_deploy`, `bindHotfixFixTask` trong `wire_plan.go`); chưa kiểm chứng `GeneratePlan` với feedback có chạy đúng cho hotfix.

## Việc cần làm

1. Viết test đỏ trước cho từng lỗi (G4, G5, G6, G11 của CR) trên fake hiện có, xác nhận chúng đỏ trên mã hiện tại.
2. `plan.proto`: `GeneratePlanRequest.idempotency_key = 3`; sinh lại mã Go (`make proto-gen`); `server_plan.go` truyền vào `GeneratePlanInput.IdempotencyKey`; kiểm độ dài ≤ 128 (lỗi `REQUEST_PLAN_IDEMPOTENCY_KEY_INVALID`, `InvalidArgument`) như cách `GenerateSolution` làm; `AnalysisRun.IdempotencyKey` được đặt khi có.
3. `plan_revision_recovery.go`: `inheritedPlanFeedback(ctx, runs, requestID)` trả `Feedback` của lượt Plan gần nhất nếu `failed` (dùng `ListRecent` như `inheritedFeedback`, hoặc `ListSucceeded`-cùng-họ ở task 05 nếu đã có; chọn một cách và test cả hai biên: lượt gần nhất `failed`, `succeeded`, `running`, không có lượt nào); `refuseSecondPlanRevision(in, status, existing)`: có `feedback`, `Created=false`, khác khoá → `ErrPlanRevisionInProgress`. `GeneratePlan.Execute` dùng cả hai trong `InTx`; phản hồi mới thay hẳn phản hồi kế thừa.
4. `plan_errors.go`: `ErrPlanRevisionInProgress` (`REQUEST_PLAN_REVISION_IN_PROGRESS`, `FailedPrecondition`), `ErrPlanRevisionNotApplicable` (`REQUEST_PLAN_REVISION_NOT_APPLICABLE`, `FailedPrecondition`), `ErrPlanRevisionNoChange` (`REQUEST_PLAN_REVISION_NO_CHANGE`, `FailedPrecondition`). Thêm vào bảng ánh xạ lỗi gateway nếu có (task 08 kiểm lại).
5. Hotfix: ở đường chỉnh, `flow.PlanKind == domain.PlanSingleTask` → `ErrPlanRevisionNotApplicable`, trước mọi ghi và trước khi tra dev server.
6. `NO_CHANGE`: trong `commitAndRead`, khi `len(paths) == 0` và đã có phiên bản trước (`prev` có `Status == ready`), trả `newFailure(run, "REQUEST_PLAN_REVISION_NO_CHANGE", "the agent did not change any document")` thay cho lỗi commit chung; lần sinh đầu giữ lỗi cũ. Request ở `planning`, lượt `failed`, `Raw` rỗng. (Câu hỏi mở 3 của CR: nếu người yêu cầu chọn phiên bản trùng thì thay việc này.)
7. Dev server như reporter: trong `GeneratePlan.Execute` dựng `connCtx` giống `generate_solution.go` (người gọi khác `req.ReporterID` thì `tenant.WithUserID(ctx, req.ReporterID)`) cho `ResolveForProject`; `modeFor` và `EnsureProjectGate` dùng cùng ngữ cảnh khi chúng tra kết nối. `reporterIdentity`: bỏ điều kiện `run.Kind == domain.RunKindPlan`. **Trước khi đổi**: chạy `git log -S 'RunKindPlan' -- internal/usecase/analysis_runner.go`, đọc mọi lời gọi của `RunProposalGeneration.Execute`, `openspecEngine.GenerateProposal` và `RunPlanGeneration.Execute` có dùng `tenant.UserID` (project-service `ListRepos`, git-gateway, relay), ghi kết luận vào mô tả PR; nếu có lời gọi cần danh tính người gọi (ví dụ kiểm quyền repo), chỉ áp danh tính reporter cho riêng bước tra kết nối dev server thay vì cho cả `ctx`.
8. **Job commit `revision`** (đọc BE-REQ-SOL-044 mục 2.4 và 2.8 trước): (a) migration `0104` (không phải `0102`: số nhỏ hơn bản đã áp dụng bị golang-migrate bỏ qua): Postgres `ALTER TABLE request.plan_auto_jobs DROP CONSTRAINT <tên check của origin>; ADD CONSTRAINT ... CHECK (origin IN ('event','retry','revision'))` (lấy tên ràng buộc từ migration `0101` của 044; nếu 044 dùng `ENUM` thì MySQL `ALTER TABLE ... MODIFY origin ENUM('event','retry','revision')`); `.down.sql` hoàn nguyên (chỉ hợp lệ khi không còn dòng `revision`: xoá chúng trước, ghi chú trong file); (b) `domain` của 044 thêm hằng `PlanAutoOriginRevision`; (c) `GeneratePlan.Execute`, trong cùng `InTx` với `StartRun`, khi `res.Created` và `cur.PlanTaskID != ""` và `run.Feedback != ""`, gọi `jobs.Insert(PlanAutoJob{Anchor: "rev-"+run.ID, Origin: revision, State: running, RunID: run.ID, Attempt: 1, MaxAttempts: 1, RequestedBy: actor, ProjectID: cur.ProjectID})`; vi phạm chỉ mục "một job hoạt động" trả `ErrPlanRevisionInProgress` và roll back (không để lượt chạy mà không có người commit); (d) repository job chưa nối (044 tắt) thì bỏ qua bước này và `approversMayRevise=false` (task 01); (e) worker của 044 không `superseded` job `revision` chỉ vì `PlanTaskID != ""` (kiểm bằng test ở task 044-06; nếu 044 chưa có luật theo `origin` thì thêm ở đó, ghi vào PR); (f) lượt thử lại không chữ ở `planning` (kế thừa phản hồi) cũng chèn job `revision`; (g) `GetPlanProposal.auto_plan` (044) hiển thị `origin = "revision"`.
9. Cập nhật `docs`/CONTRACT ở task 08, không ở đây.

## Kiểm thử

- `TestGeneratePlan_Revise_InsertsRevisionJobInSameTx`, `_FirstPlanInsertsNoJob`, `_ActiveJobConflictIsRevisionInProgress`, `_NoJobRepository_NoJobAndApproverForbidden`, `_InheritedFeedbackInsertsJob`.
- `TestGeneratePlan_IdempotentDoubleClick`, `_SecondReviserRefused`, `_SameKeyReturnsRunningRun`, `_FailedRevisionKeepsAsk` (Regenerate không chữ chạy lại với cùng phản hồi), `_NewAskReplacesInherited`, `_HotfixRevisionNotApplicable`, `_NoDevServerWritesNothing`, `_ApproverResolvesDevServerAsReporter`, `_RunAttributedToCaller`.
- `TestProposalRun_NoChangeOnRevision_IsNoChangeError`, `_FirstRunNothingToCommit_KeepsOldError`.
- `TestAnalysisRunner_PlanRunResolvesAsReporter` (danh tính nền), cùng test hiện có cho lượt Analysis xanh.
- Hợp đồng hai dialect (mở rộng `solution_revision_failure_contract.go` hoặc file mới `plan_revision_failure_contract.go`): 6 người chỉnh đồng thời → đúng một lượt, một job `revision`, 5 `REVISION_IN_PROGRESS`; lượt thất bại rồi chạy lại; `CHECK`/`ENUM` của `origin` nhận `revision` và từ chối giá trị lạ; migration `0104` xuôi/ngược.
- `go test ./services/request-service/internal/usecase/... -run 'GeneratePlan|PlanRevision|ProposalRun|AnalysisRunner' -race -count=1`.

## Tiêu chí hoàn thành

- [x] Lượt chỉnh sinh đúng một job `revision` cùng transaction; worker 044 commit Plan và mở Approval mới khi tab đã đóng. (`TestGeneratePlan_Revise_InsertsRevisionJobInSameTx`, `_ActiveJobConflictIsRevisionInProgress` (roll back), `TestPlanAutoWorker_CommitsARevisedPlanForAnApproverWhoMayNotCommit` (worker thật trên kho giả, Approval mở lại); "tab đã đóng" chỉ kiểm được trên stack, xem ghi chú)
- [x] Bấm đúp (cùng khoá) chỉ một lượt; người chỉnh thứ hai bị `REQUEST_PLAN_REVISION_IN_PROGRESS`. (`TestGeneratePlan_IdempotentDoubleClick`, `_SameKeyReturnsRunningRun`, `_SecondReviserRefused`; hợp đồng `ConcurrentRevisersStartOneRunAndOneJob` trên Postgres 16 và MySQL 8: 6 người đồng thời, 1 run, 1 job, 5 lỗi)
- [x] Phản hồi của lượt thất bại được kế thừa khi chạy lại không chữ; phản hồi mới thay hẳn. (`TestInheritedPlanFeedback_Boundaries`, `TestGeneratePlan_FailedRevisionKeepsAsk`, `_NewAskReplacesInherited`, `_Revise_InheritedFeedbackInsertsJob`; hợp đồng `AFailedRevisionIsRetriedWithItsAsk`)
- [x] `hotfix` bị `REQUEST_PLAN_REVISION_NOT_APPLICABLE` ở đường chỉnh, không ghi gì. (`TestGeneratePlan_HotfixRevisionNotApplicable`: không tra dev server, không run, không huỷ Approval)
- [x] Approver không phải reporter chỉnh được khi dev server gắn với reporter; lượt ghi cho người gọi. (`TestGeneratePlan_ApproverResolvesDevServerAsReporter`, `_RunAttributedToCaller`, `TestAnalysisRunner_PlanRunResolvesAsReporter`)
- [x] Lượt chỉnh không đổi gì báo `REQUEST_PLAN_REVISION_NO_CHANGE`. (`TestProposalRun_NoChangeOnRevision_IsNoChangeError`, `_FirstRunNothingToCommit_KeepsOldError`)
- [x] Test hiện có của cổng Analysis và sinh Plan lần đầu xanh. (`go test ./... -race -count=1` của request-service xanh)

## Rủi ro và lưu ý

- Đổi `reporterIdentity` ảnh hưởng mọi lượt Plan (kể cả lần đầu của reporter: khi người gọi là reporter hàm trả `ctx` nguyên nên không đổi). Chạy `gitnexus_impact` cho `reporterIdentity`, `GeneratePlan.Execute`, `commitAndRead`.
- Tên và hình dạng `plan_auto_jobs`, `origin`, `PlanAutoJobRepository.Insert` lấy từ tài liệu 044 v1.0 (chưa triển khai): đối chiếu với mã 044 thật trước khi viết việc 8; hai CR cùng sửa `generate_plan.go` (`IdempotencyKey`, `ExecuteForSystem`), hợp nhất theo hàm.
- `StartRun` cùng khoá idempotency của lượt cũ đã `failed`: kiểm hành vi unique `analysis_runs_idem` (khoá gắn vĩnh viễn với lượt cũ?). Cổng Analysis dùng khoá sinh mới mỗi lần bấm nên không gặp; giao diện Plan cũng vậy.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai

**Hoàn thành 2026-10-20.** Đường dẫn dưới `backend-go/services/request-service/` trừ khi ghi khác.

**Số migration.** `0104_plan_auto_jobs_revision_origin` (đề bài ghi `0102`; 0102 là của task 03 và 0103 của task 04, golang-migrate bỏ qua số nhỏ hơn bản đã áp dụng). Postgres: `DROP CONSTRAINT IF EXISTS plan_auto_jobs_origin_check` (tên mặc định của CHECK nội tuyến ở 0101) rồi thêm lại với `('event','retry','revision')`; MySQL: `DROP CHECK plan_auto_jobs_origin` rồi thêm lại (cột là `VARCHAR(10)`, không phải ENUM, `revision` vừa 8 ký tự). `.down.sql` xoá các dòng `revision` trước (như 0093 làm với run Plan) rồi hoàn nguyên. Đã sửa số trong file task này và trong `BE-REQ-SOL-045` (mục 2.1 và 2.3, nay 0102 redaction, 0103 bản, 0104 origin).

**Blast radius (grep).**
- `GeneratePlan.run`/`Execute`: gọi bởi `adapter/grpc/server_plan.go` và test; `ExecuteForSystem` (worker 044) đi cùng hàm `run` nhưng bỏ qua kế thừa phản hồi, kiểm hotfix và quyền. `GeneratePlan` có thêm trường `revisionJobs` và hai hàm `WithRevisionWorker`, `CommitsRevisions`.
- `reporterIdentity` (`analysis_runner.go`): một người gọi, `AnalysisRunner.execute`, nay áp cho mọi lượt Plan. Kết luận đọc mã (`git log -S 'run.Kind == domain.RunKindPlan'` ra 3 commit, `bbc202f25` đưa hàm vào cùng CR-041, loại Plan vì khi đó Plan chưa có approver): `RunPlanGeneration` dùng `ProjectAICompleter` (relay `ai_completion_relay.go` đọc `tenant.UserID` để tìm dev server), `RunProposalGeneration` dùng workspace qua git-gateway/project-service (`identity_forwarding.go` chuyển user sang dịch vụ kia, cũng chỉ để chọn kết nối của dev server); worker Plan không gọi task-service (PROPOSE không ghi), không kiểm quyền theo người gọi, không ghi audit theo `UserID`. Quyền của người gọi đã kiểm ở `GeneratePlan` trước khi tạo run. Vì vậy áp danh tính reporter cho cả `ctx` của worker là đủ, `run.ActorID` vẫn là người gọi. Đổi không ảnh hưởng reporter chỉnh lượt của chính họ (hàm trả `ctx` nguyên khi `ActorID == ReporterID`).
- `commitAndRead`/`finish`: một người gọi (`RunProposalGeneration`). Thêm `errNothingToCommit`.
- `domain.PlanAutoOrigin` thêm `PlanAutoOriginRevision`, `RevisionAnchor`; mọi nơi nhận `origin` là chuỗi nên repository, `PlanAutoView`, `PlanAutoState` proto không phải sửa. Worker 044 đã có luật chỉ áp "đã có `PlanTaskID` thì `superseded`" cho `event`/`retry` (kiểm bằng test thật ở dưới).
- `wire_plan.go`: bộ cấp quyền dùng `approversMayRevise=false`; `wire_plan_auto.go` bật (xem task 01).

**Tệp đã đổi.** Mới: `migrations/{postgres,mysql}/0104_plan_auto_jobs_revision_origin.{up,down}.sql`, `internal/usecase/plan_revision_recovery.go` (`inheritedPlanFeedback`, `refuseSecondPlanRevision`, `isPlanRevision`), `internal/adapter/contracttest/plan_revision_failure_contract.go`, `{postgres,mysql}/plan_revision_origin_integration_test.go`, test `generate_plan_revision_test.go`, `run_proposal_no_change_test.go`, `plan_auto_worker_revision_test.go`. Sửa: `proto/orca/request/v1/plan.proto` (`GeneratePlanRequest.idempotency_key = 3`) + `plan.pb.go`, `internal/domain/{plan_auto,plan_errors,solution_errors}.go`, `internal/usecase/{generate_plan,plan_revision_authorization,analysis_runner,run_proposal_generation}.go`, `internal/adapter/grpc/server_plan.go`, `cmd/server/{wire_plan,wire_plan_auto}.go`; xoá `cmd/server/plan_revision_wiring.go` (chưa từng commit); test `analysis_runner_identity_test.go`, `plan_revision_authorization_test.go`, `get_plan_proposal_auto_test.go`, `cmd/server/wire_plan_auto_test.go`.

**Hành vi.**
- `REQUEST_PLAN_IDEMPOTENCY_KEY_INVALID` (InvalidArgument) khi khoá dài hơn 128 ký tự (`GenerateSolution` không kiểm; cột MySQL rộng 128). Khoá gắn mãi với run đầu tiên dùng nó, kể cả đã `failed`: giao diện phải sinh khoá mới mỗi lần bấm.
- Kế thừa phản hồi chỉ khi `Feedback == ""`, Request `planning` và không phải đường hệ thống. `refuseSecondPlanRevision` chỉ khi có phản hồi thật.
- `ErrPlanRevisionInProgress` cũng được trả khi chèn job bị chặn bởi chỉ mục một-job-hoạt-động (roll back toàn bộ: không run, không huỷ Approval, không chuyển trạng thái). Hệ quả: ngay sau khi một lượt chỉnh thất bại, job `revision` của nó còn hoạt động cho tới khi worker thăm dò (mỗi `PollInterval`, mặc định 5 giây) ghi `gave_up`; trong khoảng đó bấm lại nhận `REQUEST_PLAN_REVISION_IN_PROGRESS` rồi thử lại là được (hợp đồng `AFailedRevisionIsRetriedWithItsAsk` kiểm cả hai bước).
- Mã `REQUEST_PLAN_REVISION_NO_CHANGE` là mã lỗi của run (hằng `domain.RunErrPlanRevisionNoChange`), không phải `ErrPlanRevisionNoChange()`: không có đường ném nào, nên không tạo hàm chết. Chỉ khi `hasPrev && prev.Status == ready`; job `revision` thấy run `failed` mã không tạm thời nên `gave_up` đúng mã đó.
- Bổ sung ngoài đề bài: luật thử lại của chính mình (task 01) cũng qua khi lượt Plan của người gọi còn `running`, để bấm đúp của approver nhận lại run (xem task 01).

**Lệnh đã chạy.**
- `go test ./... -race -count=1` toàn module request-service: xanh (`go build`, `go vet`, `go vet -tags=integration` xanh).
- Integration, `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration -p 1 ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'PlanRevisionFailure|PlanAutoJobsRevisionOrigin|PlanAutoJobContract' -count=6`: xanh trên Postgres 16 và MySQL 8 (hợp đồng đồng thời ổn định sau 6 lần; migration 0104 xuôi, ngược, xuôi lại với dòng `event` có sẵn, `revision` bị từ chối trước 0104 và sau `down`, giá trị lạ bị từ chối). Chạy toàn bộ gói adapter hai dialect: Postgres `ok` (48 giây); MySQL chỉ rớt `TestTwoRelaysConcurrentlyNoLoss` vì `nats: no servers available` (hạ tầng NATS của test outbox trên máy này, không liên quan; chạy riêng thì pass lần 1, rớt lần 2 và 3 cùng lỗi kết nối).

**Chưa kiểm chứng.** Chạy thật trên stack: approver thật, đóng tab giữa lượt, worker thật commit và mở Approval trên Postgres/MySQL và NATS thật (task 09); danh tính reporter cho git-gateway/project-service với approver không phải member repo.
