# BE-REQ-SOL-045: Xem tài liệu OpenSpec của Plan và chỉnh Plan bằng prompt: quyền approver, lịch sử phiên bản, làm sạch bí mật, prompt sửa tại chỗ

> **📝 Chưa triển khai (0/9 task).** Soạn 2026-10-20 từ đọc mã. Mọi mô tả hiện trạng chưa được kiểm bằng chạy thật.

**CR:** [CR-REQ-045](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-045-plan-documents-viewer-and-revision-by-prompt.md)
**Service:** `request-service` · `proto/orca/request/v1` · `api-gateway` (`wscompat`, `httpgateway`, MCP loại trừ) · `backend-go/prompts` · `backend-go/policy/orca-authz` · `cmd/orca-cli`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, chỉ mục, giữ dữ liệu), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (quyền, không lộ nội dung, làm sạch bí mật)
**Phụ thuộc solution:** [BE-REQ-SOL-012](./BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md) (`GeneratePlan`, `CommitPlan`), [BE-REQ-SOL-041](../../solution-analysis/solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md) (mẫu `AuthorizeRevise`, `inheritedFeedback`, `refuseSecondRevision`, activity), [BE-REQ-SOL-039](./BE-REQ-SOL-039-request-worktree-base-branch.md) (khoá nhánh gốc chỉ ảnh hưởng lượt chạy), CR-REQ-026 (engine OpenSpec), CR-REQ-042 (danh mục prompt). **[BE-REQ-SOL-044](./BE-REQ-SOL-044-automatic-plan-generation.md) (commit Plan phía server: `plan_auto_jobs`, `PlanAutoJobRepository`, `GeneratePlan.ExecuteForSystem`, `CommitPlan.ExecuteForSystem`, bước `committing`) là phụ thuộc cứng** (đã đọc v1.0 ngày 2026-10-20, chưa triển khai). Frontend: [FE-REQ-SOL-045](../../../../../frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-20 (đường dẫn dưới `backend-go/services/request-service/` trừ khi ghi khác):

- `internal/usecase/generate_plan.go`: `Execute` (dòng 84-143) kiểm quyền bằng `uc.auth.AuthorizePlan` (dòng 86), `checkPlannable`, tra dev server `uc.conns.ResolveForProject(ctx, ...)` (dòng 93, ngữ cảnh người gọi), rồi `InTx`: `StartRun` (lượt kiểu `plan`), nếu Request ở `awaiting_plan_approval` thì `CancelPending(..., "revision_requested")` (dòng 131) và `TriggerPlanRevision` (dòng 136). `checkPlanState` (dòng 199) cho `planning`, hoặc `awaiting_plan_approval` khi có phản hồi; lỗi `ErrPlanWrongState` mang mã `REQUEST_STATE_STALE` (`domain/plan_errors.go:17`). Lượt không có `IdempotencyKey`; `res.Created=false` được bỏ qua.
- `internal/usecase/plan_ports.go`: `PlanActorAuthorizer.AuthorizePlan`; `planActorFromSolution` gọi `inner.AuthorizeGenerate` và đổi `REQUEST_SOLUTION_FORBIDDEN` thành `REQUEST_PLAN_FORBIDDEN`. `cmd/server/wire_plan.go:27` dựng nó trên `ApproverAwareAuthorizer` nhưng chỉ nhánh Generate được dùng. `CommitPlan.Execute` cũng gọi `AuthorizePlan` (`commit_plan.go:78`) và thay Plan cũ qua `SupersedesPlanID` (dòng 106).
- `internal/usecase/solution_authorization.go`: `ApproverAwareAuthorizer.AuthorizeRevise` dùng `orPendingApprover(ctx, req, baseErr, analysisGateSubjects...)`; đã có mẫu để thêm bản cho cổng Plan.
- `internal/usecase/analysis_runner.go:74-83`: `reporterIdentity` trả `ctx` nguyên khi `run.Kind == domain.RunKindPlan`; `execute` (dòng 130) áp nó cho mọi lượt.
- `internal/usecase/run_proposal_generation.go`: `Execute` đọc `existing, hasChange` (dòng 109) và dùng lại `changeID`; `finish` (dòng 285) ghi `OpenSpecChange{... Bundle: bundle}` và `Upsert` cùng `finishPlanRun` trong một `InTx`; `commitAndRead` (dòng 314-358) lưu `bundle[rel] = string(b)` (dòng 353, thô), lược file vượt ngân sách còn lại thành `omittedMarker` (dòng 385), trả lỗi "the worktree has nothing to commit" khi không có thay đổi (dòng 328). `maxBundleBytes = 1<<20` (dòng 275).
- `internal/usecase/engine_openspec.go`: `prepare` gọi `Ensure` rồi `DiscardAll` (dòng 214) nên file của phiên bản trước có mặt; `runAgent` dựng prompt qua `RenderOpenSpecPlanPrompt` với `Feedback: in.Feedback`, `RetryNote`; không có thông tin "đã có phiên bản trước".
- `internal/usecase/get_plan_proposal.go`: `addChange` đặt `Size: int64(len(content))` (dòng 133); `find` lấy lượt Plan mới nhất trong `ListRecent` (`RecentRunsLimit = 10`). `get_plan_bundle_file.go`: đọc `change.Bundle[rel]`, `Omitted` theo tiền tố `<omitted size=`.
- `internal/usecase/tasks_md_sync.go` (quanh dòng 300): `restoreDrift` lấy `stored` từ `change.Bundle["tasks.md"]` và có thể ghi lại vùng Plan vào file thật.
- `internal/usecase/plan_document.go:96-103`: `EncodePlanProposal` chạy `RedactDocument` (đề xuất native đã sạch). `analysis_secret_redaction.go:12`: `RedactSecrets(s) (string, int)`. `common/secretscan/scan.go`: cửa sổ quét 1 MiB.
- Domain: `domain/openspec_change.go` (`OpenSpecChange`, `BundleDigest`); `domain/proposal_cleanup.go` (`DecideProposalCleanup`: change `ready` của Request chờ duyệt không bị dọn).
- Lưu trữ: `request.openspec_changes` (`0007`, `0094`), migration mới nhất `0100_analysis_runs_revision_index`; mẫu RLS Postgres ở `0007` (`ENABLE/FORCE ROW LEVEL SECURITY`, policy `tenant_isolation`); MySQL không RLS, khoá theo `tenant_id`. Repository: `internal/adapter/{postgres,mysql}/openspec_change_repository.go`.
- Activity: `adapter/{postgres,mysql}/request_activity.go` có nhánh `analysis.revision_requested` đọc `analysis_runs` không lọc `kind` (Postgres dòng 39-40, MySQL dòng 38); hằng `ActivityAnalysisRevisionRequested` trong `domain/request_activity.go`.
- Kênh: `wscompat/channels_request_plan.go` (`request.generatePlan`, `request.planProposal`, `request.planBundleFile`); `httpgateway/request_routes_flow.go:41-42`; `mcpserver/tools/excluded_channels.yaml:645-651`; `domain/rpc_catalog.go:107-110` (`GeneratePlan` nhóm `plan`, `AgentAllowed=false`; `GetPlanProposal`, `GetPlanBundleFile` nhóm `read`); `adapter/grpc/flow_gate.go:56`; `server_plan.go`.
- OPA: `policy/orca-authz/request.rego` (`group_roles["plan"] = {"owner","reporter"}`, `member_steering_rpcs` chỉ có `GenerateSolution`, `ChooseSolutionOption` cho hành động `analyze`).
- Prompt: `backend-go/prompts/plan_openspec/{plan.tmpl,plan_openspec.go}`, `plan_native/{plan.tmpl,plan_native.go}`, `request_context/_feedback.tmpl`; golden ở `services/request-service/internal/usecase/testdata/prompt-golden/BuildOpenSpecPlanPrompt/*.txt` (18 file) và `BuildPlanPrompt/`; kiểm bằng `backend-go/ci/check-prompt-golden.sh`; `prompts/prompt_size_budget_test.go` (ngân sách `plan_openspec/plan` 75000, `plan_native/plan` 72000).
- `skills/tdd-openspec-author/SKILL.md`: không có chỉ dẫn sửa phiên bản đã có.
- BE-REQ-SOL-044 (đề xuất): bảng `request.plan_auto_jobs` (khoá chính `(tenant_id, request_id, anchor)`, cột `origin` kiểu `event|retry`, `state`, `run_id`, `max_attempts`, một job hoạt động mỗi Request), `GeneratePlanInput.IdempotencyKey` và `ExecuteForSystem` (danh tính = người báo cáo), `CommitPlan.ExecuteForSystem` (với `record` idempotent), worker `committing` đọc `DecodePlanProposal(run.RawOutput)`; mục 2.8 của nó ghi rằng luật "đã có `PlanTaskID` thì `superseded`" chỉ áp cho `origin` `event`/`retry` để job của CR này sống được; migration `0101_plan_auto_generation`. Nó cũng nêu `reporterIdentity` "cố ý giữ danh tính người gọi cho run Plan" (nên run hệ thống đặt `ActorID` là người báo cáo).
- CLI đã có `orca plan generate --feedback` và `orca plan bundle --file` (`cmd/orca-cli/internal/command/plan_cmd.go`).

### Correction relative to CR-REQ-045 / brief

1. Brief nói file đơn quá cỡ được lưu thành dấu `<omitted>`. Thực tế chỉ vượt **ngân sách còn lại của cả bộ** (1 MiB) mới thành dấu; file đơn > 256 KiB làm hỏng cả lượt (`ReadFile` từ chối). Placeholder "too large" vì vậy chỉ xuất hiện khi tổng vượt ngân sách.
2. `Size` của file bị lược trong danh sách là độ dài dấu, không phải kích thước thật (task 03 sửa).
3. Brief gợi ý "nút Request changes gọi reject với regenerate"; theo mã đó là tuỳ chọn trong hộp thoại từ chối, và cả hai đường sinh lại cũ đều lỗi trạng thái (CR mục 1.4 G1).
4. Quyền cửa trước OPA là một lớp riêng cần sửa (nhóm `plan`), điều brief không nêu.
5. `reporterIdentity` đang **loại** lượt Plan; brief nói "dev server tra như reporter, như cổng Analysis" nên cần sửa thêm ở đó.
6. Activity: nhánh hiện có sẽ gắn nhầm nhãn "analysis" cho lượt Plan (G12).
7. Lượt chỉnh do người bấm tay **không** được 044 commit (D3 của 044: "việc đó thuộc 045"); CR này tạo job `origin = 'revision'` để tái dùng worker, không tự viết đường commit thứ hai.
8. Sau khi Approval bị huỷ, approver mất tư cách; thử lại lượt thất bại cần quy tắc riêng (mục 2.2).

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `backend-go/`)

```
policy/orca-authz/request.rego, request_test.rego                                  (sửa) member_plan_steering_rpcs
proto/orca/request/v1/plan.proto                                                    (sửa) GeneratePlanRequest.idempotency_key=3; GetPlanProposalRequest.revision=3;
                                                                                          GetPlanBundleFileRequest.revision=3; PlanBundleFile.sha256=4, change=5;
                                                                                          PlanBundle.redacted_secrets=10; ListPlanRevisions + PlanRevision
proto/orca/request/v1/request.proto                                                 (sửa) rpc ListPlanRevisions
services/request-service/
  migrations/{postgres,mysql}/0104_plan_auto_jobs_revision_origin.{up,down}.sql      (mới) mở rộng CHECK/ENUM origin của plan_auto_jobs (0101 là của BE-REQ-SOL-044; 0104 chứ không phải 0102, vì golang-migrate bỏ qua số nhỏ hơn bản đã áp dụng)
  migrations/{postgres,mysql}/0102_openspec_change_redaction.{up,down}.sql          (mới) cột redacted_secrets
  migrations/{postgres,mysql}/0103_openspec_change_revisions.{up,down}.sql          (mới) bảng bản
  internal/domain/openspec_change_revision.go (+ _test)                             (mới) PlanRevision, trạng thái, nhãn đổi, hằng PlanRevisionTextKept
  internal/domain/openspec_change.go                                                (sửa) RedactedSecrets
  internal/domain/plan_errors.go                                                    (sửa) 5 mã lỗi mới
  internal/domain/request_activity.go                                               (sửa) ActivityPlanRevisionRequested
  internal/domain/rpc_catalog.go (+ test)                                           (sửa) ListPlanRevisions
  internal/usecase/ports_solution.go, solution_authorization.go                     (sửa) AuthorizeRevisePlan
  internal/usecase/plan_ports.go, plan_revision_authorization.go                    (sửa/mới) AuthorizePlanRevise
  internal/usecase/generate_plan.go, plan_revision_recovery.go                      (sửa/mới) chọn quyền, khoá idempotency, kế thừa phản hồi, từ chối lần hai, hotfix
  internal/usecase/analysis_runner.go                                               (sửa) reporterIdentity cho lượt Plan
  internal/usecase/run_proposal_generation.go, plan_bundle_redaction.go             (sửa/mới) làm sạch, ghi bản, NO_CHANGE
  internal/usecase/tasks_md_sync.go                                                 (sửa) restoreDrift tôn trọng redacted_secrets
  internal/usecase/get_plan_proposal.go, get_plan_bundle_file.go, list_plan_revisions.go (sửa/mới)
  internal/usecase/openspec_plan_prompt.go, plan_prompt.go, run_plan_generation.go, engine_openspec.go (sửa) PreviousFiles, PriorPlan
  internal/usecase/ports.go                                                         (sửa) OpenSpecChangeRevisionRepository, AnalysisRunStore.ListSucceeded
  internal/adapter/{postgres,mysql}/openspec_change_revision_repository.go           (mới)
  internal/adapter/{postgres,mysql}/{openspec_change_repository,analysis_run_repository,request_activity}.go (sửa)
  internal/adapter/grpc/server_plan.go, flow_gate.go                                 (sửa)
  internal/adapter/contracttest/                                                    (sửa/mới) bản, activity, redaction
  cmd/server/wire_plan.go                                                           (sửa)
prompts/plan_openspec/{plan.tmpl,_revise_in_place.tmpl,plan_openspec.go,_test}      (sửa/mới)
prompts/plan_native/{plan.tmpl,_prior_plan.tmpl,plan_native.go,_test}               (sửa/mới)
prompts/prompt_size_budget_test.go                                                  (sửa) fixture mới
services/api-gateway/internal/adapter/wscompat/channels_request_plan.go, testdata/channel_inventory.json (sửa)
services/api-gateway/internal/adapter/httpgateway/request_routes_flow.go            (sửa)
services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml        (sửa)
cmd/orca-cli/internal/{apiclient/request.go,command/plan_cmd.go}                    (sửa)
tests/stages/e2e/orca-flows/suite_plan_revision.py, plan_documents_seed.py          (mới)
```

### 2.2 Quyền chỉnh Plan

- `SolutionActorAuthorizer` thêm `AuthorizeRevisePlan(ctx, req) error`. `ReporterOrAdminAuthorizer.AuthorizeRevisePlan` = `AuthorizeGenerate`. `ApproverAwareAuthorizer.AuthorizeRevisePlan` = `orPendingApprover(ctx, req, a.Base.AuthorizeGenerate(ctx, req), planGateSubjects...)` với `planGateSubjects = {plan, task_list, pre_deploy}`. Lỗi hạ tầng trả nguyên (đã đúng trong `orPendingApprover`).
- `PlanActorAuthorizer` thêm `AuthorizePlanRevise`. `planActorFromSolution.AuthorizePlanRevise` gọi `inner.AuthorizeRevisePlan` và đổi mã lỗi thành `REQUEST_PLAN_FORBIDDEN` (thông điệp nêu cả approver).
- `GeneratePlan.Execute`: chọn `AuthorizePlanRevise` khi `req.Status == awaiting_plan_approval && in.Feedback != ""`, còn lại `AuthorizePlan`. Chạy trước `InTx`.
- **Thử lại lượt thất bại của chính mình**: `planActorFromSolution` nhận thêm `AnalysisRunStore`; `AuthorizePlanRevise` qua nếu nhánh Base qua, hoặc nếu `req.Status == planning`, `req.PlanTaskID != ""`, lượt Plan gần nhất (`ListRecent`) có `ActorID == người gọi`, `Status == failed` và `Feedback != ""`; ngược lại trả lỗi của nhánh approver. `GeneratePlan.Execute` chọn `AuthorizePlanRevise` khi `req.PlanTaskID != ""` và (`awaiting_plan_approval` kèm phản hồi, hoặc `planning`); chọn `AuthorizePlan` còn lại (lần sinh Plan đầu).
- **Khi không có worker commit** (044 tắt hoặc thiếu `PlanAutoJobRepository`): `AuthorizePlanRevise` chỉ chấp nhận nhánh Base (reporter/admin), vì không còn ai commit thay approver; approver nhận `REQUEST_PLAN_FORBIDDEN`. Cờ `approversMayRevise` do `wire_plan.go` đặt (true khi repository job được nối).
- `request.rego`: `member_plan_steering_rpcs := {"GeneratePlan"}` và luật `allow` cho hành động `plan` với `caller_project_role == "member"`, cùng cấu trúc luật `member_steering_rpcs`. Test: member qua `GeneratePlan`, không qua `CommitPlan`, `StartPhase`; không phải member vẫn bị chặn; agent không đổi (`agent_rpcs` không có `GeneratePlan`).

### 2.3 Đường sinh lại cứng hơn (`GeneratePlan`)

- `GeneratePlanRequest.idempotency_key` (trường 3, tối đa 128 ký tự, kiểm như `GenerateSolution`) chuyển vào `GeneratePlanInput.IdempotencyKey` (trường usecase do BE-REQ-SOL-044 thêm cho đường hệ thống; nếu 044 chưa vào thì thêm giống hệt ở đây để hai CR không xung đột) và vào `AnalysisRun.IdempotencyKey`.
- `inheritedPlanFeedback`: khi `in.Feedback == ""` và Request ở `planning`, lấy `Feedback` của lượt Plan gần nhất nếu `failed` (qua `AnalysisRunStore.ListSucceeded`-cùng-họ hoặc `ListRecent`, xem task 02); lượt `succeeded` hoặc `running` không kế thừa. Phản hồi mới thay hẳn.
- `refuseSecondPlanRevision`: có `feedback`, `res.Created == false`, khác khoá idempotency → `ErrPlanRevisionInProgress` (`REQUEST_PLAN_REVISION_IN_PROGRESS`, `FailedPrecondition`). Gọi hệ thống và không phản hồi giữ cách cũ (trả lượt đang chạy).
- **Job commit phía server**: khi `req.PlanTaskID != ""` và `run.Feedback != ""` (thật hoặc kế thừa) và repository job đã nối, trong cùng `InTx` với `StartRun` (chỉ khi `res.Created`) chèn `PlanAutoJob{Origin: "revision", State: running, RunID: run.ID, MaxAttempts: 1, Anchor: "rev-<run.ID>", RequestedBy: actor}`; migration `0104` mở rộng giá trị `origin`. Worker của 044 xử lý tiếp (`running → committing → done/gave_up`). Không tạo job ở lần sinh Plan đầu. `GetPlanProposal.auto_plan` (044) cho giao diện thấy `origin = 'revision'`.
- Hotfix: ở đường chỉnh (`awaiting_plan_approval` + phản hồi), `flow.PlanKind == PlanSingleTask` → `ErrPlanRevisionNotApplicable` (`REQUEST_PLAN_REVISION_NOT_APPLICABLE`, `FailedPrecondition`), trước mọi ghi.
- Dev server: `connCtx` như `generate_solution.go` (người gọi khác reporter thì tra bằng reporter); `reporterIdentity` bỏ loại trừ `RunKindPlan` (chỉ đổi ngữ cảnh tra kết nối/chạy; `run.ActorID` vẫn là người gọi). Trước khi đổi: `git log -S 'RunKindPlan' -- analysis_runner.go`, đọc mọi lời gọi của hai worker Plan với `tenant.UserID`.

### 2.4 Làm sạch bí mật và kích thước file bị lược

- `plan_bundle_redaction.go`: `redactBundle(files map[string][]byte, budget int) (bundle map[string]string, redacted int)`: với mỗi file: `RedactSecrets(string(b))`, đếm; file vượt ngân sách vẫn thành `omittedMarker(b)` tính trên **byte thô** (kích thước thật; hash thô không lộ nội dung vì là hash một chiều, nhưng ghi rõ điều đó trong comment).
- `OpenSpecChange.RedactedSecrets int`; cột `openspec_changes.redacted_secrets INT NOT NULL DEFAULT 0`; `planChecksFor` thêm `SECRETS_REDACTED` (info) khi > 0.
- `GetPlanBundleFile.Execute`: `RedactSecrets` lên nội dung trước khi trả (che dòng cũ). `omitted` không đổi.
- `ParseOmittedMarker(s) (size int64, sha256 string, ok bool)` trong `domain`; `addChange` dùng nó để đặt `Size` thật cho file bị lược.
- `restoreDrift`: `if change.RedactedSecrets > 0` thì mốc là `change.TasksMDDigest` khi `SyncDigest == ""` và **không** khôi phục từ `stored` (ghi log mức warn một lần, đếm `TasksDrift` như cũ); test chứng minh `[REDACTED]` không bao giờ vào file thật.

### 2.5 Lịch sử phiên bản

`request.openspec_change_revisions` (Postgres; MySQL tương đương không RLS):

```
id uuid pk, tenant_id, request_id (FK requests), revision int, run_id uuid,
commit_sha text, tasks_md_digest text, bundle_digest text,
files_json jsonb  -- [{path,size,omitted,sha256}]
bundle_json jsonb NULL, created_at timestamptz
UNIQUE (tenant_id, request_id, revision); INDEX (tenant_id, request_id, revision DESC)
```

- `OpenSpecChangeRevisionRepository`: `Append(ctx, rev, keepText int) error` (trong transaction đang mở: chèn, rồi `UPDATE ... SET bundle_json = NULL WHERE request_id = ? AND revision <= ? - keepText AND bundle_json IS NOT NULL`), `List(ctx, requestID, before, limit)`, `Get(ctx, requestID, revision)`.
- `finish`: trong cùng `InTx` với `Upsert` và `finishPlanRun`: `revision = len(ListSucceeded(plan)) + 1` (các lượt cùng Request tuần tự nhờ unique `analysis_runs_one_running`, nên đánh số không đua), `Append`.
- Đọc: `ListPlanRevisions` hợp nhất **mọi lượt Plan thành công** của Request: với OpenSpec ghép dòng bản theo `run_id` (lấy `commit`, `files_json`, `text_kept`); với native chỉ có lượt (không có dòng bản). `status` = `current` cho bản mới nhất, `superseded` cho các bản còn lại. Phản hồi, người chỉnh và thời điểm lấy từ `analysis_runs` (lô, không N+1). Request cũ chưa có dòng bản: bản đã có `openspec_changes` được coi là bản hiện hành, `files` suy từ `Bundle`, `text_kept=true`.
- Nhãn đổi (`added/changed/removed/unchanged`) tính khi đọc từ `files_json` hai bản liền kề (`domain.DiffFileIndex`).
- `GetPlanProposal.revision > 0`: tìm lượt theo `revision`, trả `proposal` (native từ `raw_output`), `bundle.files` từ dòng bản; `checks` và `knowledge` chỉ khi là bản hiện hành. `GetPlanBundleFile.revision`: bản hiện hành đọc từ `openspec_changes`; bản cũ đọc `bundle_json` của dòng bản; `NULL` trả `REQUEST_PLAN_REVISION_TEXT_PRUNED` (`FailedPrecondition`); không có trả `REQUEST_PLAN_REVISION_NOT_FOUND` (`NotFound`).
- `AnalysisRunStore.ListSucceeded(ctx, requestID, kind)` (mới): thứ tự `started_at ASC, id`, chỉ `status = 'succeeded'`; cài ở cả hai dialect; cập nhật mọi fake.

### 2.6 Prompt

- `plan_openspec`: `Input.PreviousFiles []string` (đã sắp, tối đa 200); partial `_revise_in_place.tmpl` hiện khi rỗng-không (xem CR mục 2.7). `OpenSpecPlanPromptInput.PreviousFiles`; `RunProposalGeneration.Execute` đặt `ProposalInput.PreviousFiles` từ `existing.Bundle` khi `hasChange && existing.Status == ready` (tên file thôi, không nội dung); `runAgent` truyền xuống.
- `plan_native`: `Input.PriorPlan string`; partial `_prior_plan.tmpl`; `RunPlanGeneration.Execute` đặt từ lượt Plan thành công gần nhất (`ListSucceeded` rồi `DecodePlanProposal`/dùng `raw_output` đã sạch), cắt theo rune (hằng `priorPlanRunes = 20000`, cắt trước khi `fence`).
- Mọi chuỗi do người ngoài viết (phản hồi, đề xuất cũ, tên file) đi qua `fence`; tên file chứa thẻ đóng không thoát được khối (test thù địch).
- Golden: xem CR mục 2.7. **Golden hiện có phải không đổi.**

### 2.7 Activity

`ActivityPlanRevisionRequested = "plan.revision_requested"`; nhánh UNION mới: `analysis_runs WHERE kind = 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'`, tác nhân `user`, `actor_id` của lượt, `subject_type='plan'`, `summary` cố định "Requested changes to the plan", **không chọn cột `feedback`**; nhánh `analysis.revision_requested` thêm `AND kind <> 'plan'`. Thêm vào `AllActivityKinds`.

### 2.8 Gateway, REST, CLI, MCP, CONTRACT

`request.generatePlan` nhận `idempotencyKey`; `request.planProposal` nhận `revision`; `request.planBundleFile` nhận `revision`; `request.planRevisions` mới (`GroupRead`, `RateRead`, `AgentAllowed=false`); `channel_inventory.json`; REST `GET /{id}/plan/revisions` và tham số `revision`; CLI `orca plan revisions`, `orca plan bundle --revision`; `excluded_channels.yaml` thêm `request.planRevisions` (category `request_flow`); `CONTRACT-request-ui-api.md` mục mới.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| S1 | `AuthorizeRevisePlan` tách riêng, Go quyết định; OPA chỉ mở cửa cho `GeneratePlan` | Approver không khởi động Plan đầu; dữ liệu approver nằm ở Go |
| S2 | Kiểm quyền trước khi huỷ Approval | Sau khi huỷ, người đó không còn là approver |
| S3 | Bảng bản riêng thay vì đổi `UNIQUE` của `openspec_changes` | `openspec_changes` là dòng "hiện hành" mà `PlanArtifacts.Describe`, `TasksMdSyncer`, janitor đang dựa vào; không đụng |
| S4 | Số bản = thứ tự lượt Plan thành công, không lưu cột ở native | Một cách đánh số cho cả hai engine |
| S5 | Giữ chữ 10 bản, chỉ mục và digest giữ mãi | Chặn trần lưu trữ mà không mất khả năng biết "đổi file nào" |
| S6 | Làm sạch khi lưu và khi đọc; dấu `omitted` tính trên byte thô | Khi lưu cho dữ liệu mới; khi đọc cho dữ liệu cũ; không backfill |
| S7 | `restoreDrift` không khôi phục từ bản đã che | Không ghi `[REDACTED]` vào file thật |
| S8 | Điều kiện prompt theo `PreviousFiles`/`PriorPlan`, không theo `Feedback` | Golden cũ không đổi; "Try again" cũng được dặn |
| S9 | Không thêm MCP, không có đường ghi tay | CR mục 2.10 |
| S10 | Lỗi `NO_CHANGE` thay vì bản trùng | CR mục 3 E10 |
| S11 | Lượt chỉnh dùng job `origin = 'revision'` của 044, không đường commit thứ hai | Một worker, một chỗ thử lại commit; đóng tab không kẹt |
| S12 | Người yêu cầu thử lại được lượt thất bại của chính mình | Họ mất tư cách approver khi Approval bị huỷ |
| S13 | Không có worker commit thì approver không chỉnh được | Không để Request kẹt ở `planning` |

## 4. Phụ thuộc và thứ tự

01 (quyền) → 02 (đường sinh lại, job `revision`) song song với 03 (redaction) → 04 (bảng bản, cần 03) → 05 (đọc, cần 04) → 06 (prompt, cần 02 và 05) → 07 (activity, cần 02) → 08 (gateway, cần 05 và 07) → 09 (tích hợp và e2e, cần tất cả). BE-REQ-SOL-044 phải có trước task 02 (cần `PlanAutoJobRepository`, `ExecuteForSystem`, migration `0101`); có thể code 01, 03, 04, 05 trước.

## 5. Kiểm thử

- Quyền: bảng (reporter, admin, lead, owner, approver theo user/team/role, member không approver, chỉ đọc, người lạ) × (Approval `plan`, `task_list`, `pre_deploy`) × (có/không `feedback`, `planning`/`awaiting_plan_approval`); thứ tự trước `CancelPending`; test rego.
- Đường sinh lại: khoá idempotency, người chỉnh thứ hai, kế thừa phản hồi, hotfix, `NO_CHANGE`, dev server tra như reporter, lỗi `AI_UNAVAILABLE` không ghi gì.
- Lưu trữ hai dialect: bảng bản (ghi cùng transaction với `Upsert`, đánh số, lược chữ, Request cũ), `ListSucceeded`, `redacted_secrets`, activity; EXPLAIN trên Request có ≥ 50 lượt.
- Redaction: bí mật cắm sẵn mỗi loại, nhiều file, 256 KiB, dấu `omitted`, `restoreDrift`; che khi đọc dòng cũ.
- Prompt: golden mới, golden cũ không đổi, thù địch, ngân sách.
- Không lộ: test log/span/khung sự kiện không chứa phản hồi hay văn bản tài liệu.
- E2E: `suite_plan_revision.py` (PRV-xx) và seed; UI ở FE-REQ-SOL-045.

## 6. Rủi ro và điểm chưa kiểm chứng

- **BE-REQ-SOL-044 chưa triển khai**: tên `PlanAutoJobRepository`, `origin`, `ExecuteForSystem` lấy từ tài liệu v1.0; nếu đổi thì task 02 và 08 khớp lại. Không phát hành trước 044.
- **`RunKindPlan` bị loại khỏi `reporterIdentity`** vì lý do chưa rõ; xem task 02.
- **Số dòng, đường dẫn** có thể lệch khi triển khai (đã đọc 2026-10-20); mọi task nhắc kiểm lại.
- **Dung lượng bảng bản** chưa đo với bộ tài liệu thật.
- **Task-service** (`SupersedesPlanID`) chưa đọc.
- **Fake claude** không tác giả OpenSpec (task 09 quyết định phương án).
- Chưa chạy thật.

## 7. Câu hỏi mở

Ba câu hỏi của CR (CR-REQ-045 mục 7): chờ 044 hay dự phòng; "Keep the previous version"; bản trùng so với lỗi `NO_CHANGE`.

## 8. Tham chiếu

- Các file ở mục 1; [CR-REQ-045](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-045-plan-documents-viewer-and-revision-by-prompt.md); [BE-REQ-SOL-041](../../solution-analysis/solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md); `backend-go/prompts/README.md`; `guides/reference/git-compatibility.md`
