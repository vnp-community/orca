# TASK-REQ-012-05: Use case `GeneratePlan` (PROPOSE) và `CommitPlan` (COMMIT) kèm RPC

**From Solution:** BE-REQ-SOL-012
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/generate_plan.go` (mới), `commit_plan.go` (mới), `internal/adapter/grpc/server_plan.go` (mới), `internal/usecase/generate_plan_test.go`, `commit_plan_test.go` (mới), `internal/usecase/ports.go`
**Depends on:** TASK-REQ-012-03, TASK-REQ-012-04, CR-REQ-003 (`TransitionRequest`), CR-REQ-009 (`OpenApproval`), CR-REQ-007 (Solution `approved`), CR-REQ-002 (`requests.plan_task_id`, CAS `version`)
**Status:** `[ ] TODO`

---

## Context

- `request-service` chưa có code ngày 2026-10-06; các cổng `TransitionRequest`, `OpenApproval`, `CancelPendingForRequest`, `RequestRepository` (CAS theo `version`), `TxRunner`, `OutboxWriter` do CR-REQ-001, 002, 003, 009 định nghĩa. Đọc `internal/usecase/ports.go` thật trước khi viết; dưới đây dùng tên theo CR, cần đối chiếu.
- Trạng thái hợp lệ: `planning` (hoặc `analyzing` với `single_task`/hotfix); khác thì `REQUEST_STATE_STALE` (CR-REQ-003).
- Sau `plan_ready` Request vào `awaiting_plan_approval`. `security` mở Approval `pre_deploy` thay `plan` (SOL-014); `hotfix` không mở ở đây.
- Sự kiện: subject theo mẫu repo `orca.request.<entity>.<event>`: `orca.request.plan.generated` (README v6 mục 8 điều 4).
- Mẫu mode hai bước của task-service: `AIDecompose` (đề xuất) rồi `AIApply` (lưu).

## Việc cần làm

1. `generate_plan.go`: `GeneratePlan.Execute(ctx, in GeneratePlanInput) (GeneratePlanResult, error)`: `RequireTenantID`; đọc Request; `PlanShapeFor`; `ShapeNone` thì `REQUEST_PLAN_NOT_APPLICABLE`; kiểm trạng thái; nạp Solution `approved` (`REQUEST_PLAN_SOLUTION_NOT_APPROVED` khi thiếu); `PlanGenerator.Generate` (có `Feedback` khi sinh lại); `ValidateProposal` rồi `PlanPreconditions` (nil bỏ qua); trả đề xuất và `raw`. Không ghi gì.
2. `commit_plan.go`: `CommitPlan.Execute(ctx, in CommitPlanInput) (CommitPlanResult, error)`:
   1. Lặp lại bước kiểm 1 (tenant, trạng thái, hình dạng) và `ValidateProposal` trên `in.Proposal`.
   2. `TaskPlanWriter.CreatePlanTree` (hoặc `CreateSingleTask`), `SupersedesPlanID = request.PlanTaskID` nếu đã có (replan) và trước đó `CancelPendingForRequest` Approval cũ.
   3. `TxRunner.InTx`: CAS `UPDATE requests SET plan_task_id = ?, version = version+1 WHERE id = ? AND version = ?` (cùng giá trị khi `already_exists` nên idempotent) và `OutboxWriter` `orca.request.plan.generated` `{request_id, plan_task_id, phase_task_ids, task_ids, type, size, task_count}`.
   4. `OpenApproval(subject_type=plan|task_list, subject_id=plan_task_id, stage=plan)` ngoài tx; lỗi tạm thì retry an toàn (trả dòng `pending` cũ khi `subject_digest` trùng).
   5. `TransitionRequest(plan_ready, ExpectedFrom=planning)`; `Applied=false` coi như đã xong.
3. Thất bại giữa chừng không bù trừ: gọi lại `CommitPlan` đi tiếp từ bước còn dở nhờ `already_exists`. Ghi rõ comment.
4. `server_plan.go`: hai RPC `GeneratePlan`, `CommitPlan`. Tự kiểm quyền (README v6 mục 8 điều 13: gateway không kiểm OPA): người gọi phải là chủ Request hoặc có quyền ghi theo chính sách của CR-REQ-010; chưa có `Grant` ở mức Request, nên để cổng `RequestAuthorizer` và ghi vào Câu hỏi mở của solution.
5. `actor` lấy từ `tenant.UserID(ctx)`; truyền `creator_id`.
6. Metrics và log: log độ dài proposal, `task_count`, không log nội dung.

## Kiểm thử

- `TestGeneratePlan_WritesNothing` (fake `TaskPlanWriter` không bị gọi), `_NotApplicable_Spike`, `_StateStale`, `_SolutionNotApproved`, `_PassesFeedback`, `_InvalidJSON`.
- `TestCommitPlan_HappyPath_OrderOfSteps` (fake ghi thứ tự: tạo cây, CAS, outbox, approval, transition), `_Twice_AlreadyExists_NoSecondApproval`, `_ApprovalFails_RetryContinues`, `_TransitionApplied false`, `_Replan_CancelsOldApprovalAndSupersedes`, `_CASConflict_ReturnsStale`, `_Hotfix_NoApprovalOpened`.
- Integration hai dialect của repository CAS đã có ở CR-REQ-002; ở đây chỉ test usecase với fake.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'GeneratePlan|CommitPlan' -v`.

## Tiêu chí hoàn thành

- [ ] PROPOSE không ghi gì ở cả hai service.
- [ ] COMMIT hai lần liên tiếp hoặc đồng thời: một Plan, một Approval `pending`, một sự kiện `plan.generated`.
- [ ] Sau COMMIT Request ở `awaiting_plan_approval`; `requests.plan_task_id` đã đặt.
- [ ] `spike`, `question` bị `REQUEST_PLAN_NOT_APPLICABLE`.
- [ ] Replan huỷ Approval cũ và Plan cũ (qua `supersedes_plan_id`).

## Rủi ro và lưu ý

- Hai lệnh ngoài tx (task-service, approval) và một tx nội bộ không nguyên tử chung: cơ chế `already_exists` là điều duy nhất bảo vệ; test đủ điểm lỗi.
- Quyền ghi ở mức Request chưa chốt (CR-REQ-003/010): không tự nghĩ ra; để cổng và test với fake.
- Tên cổng `TransitionRequest`, `OpenApproval` chưa kiểm chứng với code đã merge.
