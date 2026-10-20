# TASK-REQ-045-02: Đường chỉnh Plan chắc hơn: khoá idempotency, lần chỉnh thứ hai, kế thừa phản hồi, hotfix, dev server của reporter, job commit `revision`

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`, `proto`
**File:** `migrations/{postgres,mysql}/0102_plan_auto_jobs_revision_origin.{up,down}.sql` (mới), `proto/orca/request/v1/plan.proto` (sửa) + mã sinh, `internal/usecase/{generate_plan,plan_revision_recovery,analysis_runner}.go` (sửa/mới), `internal/domain/plan_errors.go` (sửa), `internal/adapter/grpc/server_plan.go` (sửa), `internal/usecase/run_proposal_generation.go` (lỗi `NO_CHANGE`, xem việc 6), test tương ứng
**Depends on:** TASK-REQ-045-01, **BE-REQ-TASK-044-02, 044-03, 044-06** (bảng `plan_auto_jobs`, `PlanAutoJobRepository`, `ExecuteForSystem`, worker)
**Status:** [ ] TODO

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
8. **Job commit `revision`** (đọc BE-REQ-SOL-044 mục 2.4 và 2.8 trước): (a) migration `0102`: Postgres `ALTER TABLE request.plan_auto_jobs DROP CONSTRAINT <tên check của origin>; ADD CONSTRAINT ... CHECK (origin IN ('event','retry','revision'))` (lấy tên ràng buộc từ migration `0101` của 044; nếu 044 dùng `ENUM` thì MySQL `ALTER TABLE ... MODIFY origin ENUM('event','retry','revision')`); `.down.sql` hoàn nguyên (chỉ hợp lệ khi không còn dòng `revision`: xoá chúng trước, ghi chú trong file); (b) `domain` của 044 thêm hằng `PlanAutoOriginRevision`; (c) `GeneratePlan.Execute`, trong cùng `InTx` với `StartRun`, khi `res.Created` và `cur.PlanTaskID != ""` và `run.Feedback != ""`, gọi `jobs.Insert(PlanAutoJob{Anchor: "rev-"+run.ID, Origin: revision, State: running, RunID: run.ID, Attempt: 1, MaxAttempts: 1, RequestedBy: actor, ProjectID: cur.ProjectID})`; vi phạm chỉ mục "một job hoạt động" trả `ErrPlanRevisionInProgress` và roll back (không để lượt chạy mà không có người commit); (d) repository job chưa nối (044 tắt) thì bỏ qua bước này và `approversMayRevise=false` (task 01); (e) worker của 044 không `superseded` job `revision` chỉ vì `PlanTaskID != ""` (kiểm bằng test ở task 044-06; nếu 044 chưa có luật theo `origin` thì thêm ở đó, ghi vào PR); (f) lượt thử lại không chữ ở `planning` (kế thừa phản hồi) cũng chèn job `revision`; (g) `GetPlanProposal.auto_plan` (044) hiển thị `origin = "revision"`.
9. Cập nhật `docs`/CONTRACT ở task 08, không ở đây.

## Kiểm thử

- `TestGeneratePlan_Revise_InsertsRevisionJobInSameTx`, `_FirstPlanInsertsNoJob`, `_ActiveJobConflictIsRevisionInProgress`, `_NoJobRepository_NoJobAndApproverForbidden`, `_InheritedFeedbackInsertsJob`.
- `TestGeneratePlan_IdempotentDoubleClick`, `_SecondReviserRefused`, `_SameKeyReturnsRunningRun`, `_FailedRevisionKeepsAsk` (Regenerate không chữ chạy lại với cùng phản hồi), `_NewAskReplacesInherited`, `_HotfixRevisionNotApplicable`, `_NoDevServerWritesNothing`, `_ApproverResolvesDevServerAsReporter`, `_RunAttributedToCaller`.
- `TestProposalRun_NoChangeOnRevision_IsNoChangeError`, `_FirstRunNothingToCommit_KeepsOldError`.
- `TestAnalysisRunner_PlanRunResolvesAsReporter` (danh tính nền), cùng test hiện có cho lượt Analysis xanh.
- Hợp đồng hai dialect (mở rộng `solution_revision_failure_contract.go` hoặc file mới `plan_revision_failure_contract.go`): 6 người chỉnh đồng thời → đúng một lượt, một job `revision`, 5 `REVISION_IN_PROGRESS`; lượt thất bại rồi chạy lại; `CHECK`/`ENUM` của `origin` nhận `revision` và từ chối giá trị lạ; migration `0102` xuôi/ngược.
- `go test ./services/request-service/internal/usecase/... -run 'GeneratePlan|PlanRevision|ProposalRun|AnalysisRunner' -race -count=1`.

## Tiêu chí hoàn thành

- [ ] Lượt chỉnh sinh đúng một job `revision` cùng transaction; worker 044 commit Plan và mở Approval mới khi tab đã đóng.
- [ ] Bấm đúp (cùng khoá) chỉ một lượt; người chỉnh thứ hai bị `REQUEST_PLAN_REVISION_IN_PROGRESS`.
- [ ] Phản hồi của lượt thất bại được kế thừa khi chạy lại không chữ; phản hồi mới thay hẳn.
- [ ] `hotfix` bị `REQUEST_PLAN_REVISION_NOT_APPLICABLE` ở đường chỉnh, không ghi gì.
- [ ] Approver không phải reporter chỉnh được khi dev server gắn với reporter; lượt ghi cho người gọi.
- [ ] Lượt chỉnh không đổi gì báo `REQUEST_PLAN_REVISION_NO_CHANGE`.
- [ ] Test hiện có của cổng Analysis và sinh Plan lần đầu xanh.

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
