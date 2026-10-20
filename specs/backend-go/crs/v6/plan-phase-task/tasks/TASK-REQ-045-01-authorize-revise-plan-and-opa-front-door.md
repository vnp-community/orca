# TASK-REQ-045-01: `AuthorizeRevisePlan`: approver của cổng Plan được chỉnh, và cửa trước OPA cho `GeneratePlan`

**From Solution:** [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md)
**Priority:** P1
**Service:** `request-service`, `policy/orca-authz`
**File:** `internal/usecase/{ports_solution,solution_authorization,plan_ports,plan_revision_authorization,generate_plan}.go` (sửa/mới; `plan_revision_authorization.go` chứa luật thử lại của chính mình), mọi fake của `SolutionActorAuthorizer` và `PlanActorAuthorizer` (`solution_fakes_test.go`, `plan_fakes_test.go`), `cmd/server/wire_plan.go` (sửa), `internal/domain/plan_errors.go` (sửa thông điệp `ErrPlanForbidden`), `backend-go/policy/orca-authz/{request.rego,request_test.rego}` (sửa)
**Depends on:** Không
**Status:** [ ] TODO

---

## Context

- `GeneratePlan.Execute` gọi `uc.auth.AuthorizePlan` (`generate_plan.go:86`) = reporter hoặc `admin|lead|owner`; approver của Approval Plan không chỉnh được (`plan_ports.go:41-61`).
- Mẫu có sẵn: `ApproverAwareAuthorizer.AuthorizeRevise` + `orPendingApprover` (`solution_authorization.go`) của CR-REQ-041; lọc Approval `pending` rồi `Decider.CanDecide`.
- Cổng Plan có ba kiểu Approval: `plan`, `task_list`, `pre_deploy` (`domain/approval_subject_stage.go`: `ApprovalStatusesFor`).
- OPA `request.rego`: nhóm `plan` chỉ cho `owner|reporter` (`group_roles`); `member_steering_rpcs` (dòng 43-47, 76-81) mới chỉ có `GenerateSolution`, `ChooseSolutionOption` cho hành động `analyze`. Bài học TASK-REQ-041-04: approver là member thường bị chặn ở cửa trước nếu không mở rộng luật này.

## Việc cần làm

1. `SolutionActorAuthorizer` thêm `AuthorizeRevisePlan(ctx, req) error`; `ReporterOrAdminAuthorizer.AuthorizeRevisePlan` = `AuthorizeGenerate`; `ApproverAwareAuthorizer.AuthorizeRevisePlan` = `a.orPendingApprover(ctx, req, a.Base.AuthorizeGenerate(ctx, req), planGateSubjects...)` với `planGateSubjects = []domain.SubjectType{SubjectPlan, SubjectTaskList, SubjectPreDeploy}` đặt cạnh `analysisGateSubjects`. Cập nhật mọi fake và nơi dựng literal (`wire_plan.go` truyền `stores.analysisRuns` và `approversMayRevise` = repository job của 044 đã nối, `contracttest`).
2. `PlanActorAuthorizer` thêm `AuthorizePlanRevise`; `planActorFromSolution` (đổi hàm dựng thành `NewPlanActorAuthorizer(inner, runs AnalysisRunStore, approversMayRevise bool)`) hiện thực:
   - nhánh Base: `inner.AuthorizeGenerate` (reporter/admin/lead/owner) qua thì qua;
   - nhánh approver: chỉ khi `approversMayRevise` (có worker commit phía server, xem CR mục 2.3) thì gọi `inner.AuthorizeRevisePlan`; khi `false` approver nhận `REQUEST_PLAN_FORBIDDEN`;
   - nhánh thử lại của chính mình: qua khi `req.Status == planning`, `req.PlanTaskID != ""`, lượt Plan gần nhất (`runs.ListRecent`) có `ActorID` bằng người gọi, `Status == failed` và `Feedback != ""`; chỉ khi `approversMayRevise`;
   - đổi `REQUEST_SOLUTION_FORBIDDEN` thành `REQUEST_PLAN_FORBIDDEN` (cùng cách `AuthorizePlan`); lỗi hạ tầng trả nguyên.
3. `GeneratePlan.Execute`: sau `requests.Get`, chọn `AuthorizePlanRevise` khi `req.PlanTaskID != ""` và (`req.Status == awaiting_plan_approval && in.Feedback != ""`, hoặc `req.Status == planning`); còn lại `AuthorizePlan` (lần sinh Plan đầu). Giữ chạy trước `InTx` (trước `CancelPending`). Đường hệ thống `ExecuteForSystem` của BE-REQ-TASK-044-03 bỏ kiểm quyền và không đi qua đây.
4. `ErrPlanForbidden` đổi thông điệp: "only the reporter, an administrator or an approver of the pending decision may ask for a Plan or changes to it" (mã không đổi); `CommitPlan.Execute` giữ `AuthorizePlan` (không nới); lượt chỉnh được commit bởi worker 044 (task 02).
5. `request.rego`: thêm `member_plan_steering_rpcs := {"GeneratePlan"}` và luật `allow` cho `human`, `input.action == "plan"`, `input.rpc in member_plan_steering_rpcs`, `input.caller_project_role == "member"`, kèm chú thích "Go decides: revision by the approver of the pending Plan gate (CR-REQ-045); first generation stays with the reporter and admins". Không đụng `agent_rpcs`.
6. `request_test.rego`: member qua `GeneratePlan`; member không qua `CommitPlan`, `StartPhase`; owner/reporter như cũ; không phải member (`caller_project_role == ""`) bị chặn; agent không qua `GeneratePlan`.

## Kiểm thử

- Bảng quyền `TestAuthorizePlanRevise_*`: reporter, admin, lead, owner, approver theo user, theo team, theo role, member không approver, người chỉ đọc, người lạ; × Approval `plan`, `task_list`, `pre_deploy`; × (có/không `feedback`, `planning`/`awaiting_plan_approval`); × `approversMayRevise` đúng/sai (sai: chỉ nhánh Base); luật thử lại của chính mình (người gọi là actor lượt `failed` có phản hồi → qua; actor lượt khác, lượt `succeeded`, lượt không phản hồi, Request không ở `planning`, Request chưa có Plan → không).
- `TestGeneratePlan_Revise_ApproverNotReporter_Succeeds`, `_ApproverStartingFirstPlan_Forbidden`, `_AuthorizeRunsBeforeCancelPending` (fake ghi thứ tự: `authorize_revise_plan@outside` rồi `cancel_pending@tx`), `_InfraErrorNotMaskedAsForbidden`.
- `go test ./services/request-service/internal/usecase/... -run 'AuthorizePlan|RevisePlan|GeneratePlan' -v`; `make opa-test` (hoặc `opa test backend-go/policy/orca-authz`).

## Tiêu chí hoàn thành

- [ ] Approver (user, team, role) của Approval `plan`/`task_list`/`pre_deploy` đang chờ chỉnh được; người không có quyền bị `REQUEST_PLAN_FORBIDDEN` và không đổi dữ liệu.
- [ ] Approver không khởi động được lần sinh Plan đầu; người đã yêu cầu chỉnh thử lại được lượt thất bại của chính mình, người khác thì không.
- [ ] Khi không có worker commit, approver bị `REQUEST_PLAN_FORBIDDEN` (reporter/admin vẫn chỉnh được).
- [ ] Kiểm quyền xảy ra trước khi Approval bị huỷ (test thứ tự).
- [ ] Member dự án qua cửa OPA cho `GeneratePlan`, không qua cho RPC khác của nhóm `plan`; người lạ vẫn bị chặn.
- [ ] Mọi fake biên dịch; test hiện có xanh.

## Rủi ro và lưu ý

- `CanDecide` có thể áp ràng buộc tách nhiệm (`self_approval_allowed`); không áp lại cho bước chỉnh (reporter đã qua nhánh Base).
- Chạy `gitnexus_impact` cho `GeneratePlan.Execute`, `PlanActorAuthorizer`, `SolutionActorAuthorizer` và báo blast radius (`NewPlanActorAuthorizer` còn dùng ở `CommitPlan`).
- Commit Plan của approver do worker của BE-REQ-SOL-044 (task 02); task này chỉ lo quyền.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
