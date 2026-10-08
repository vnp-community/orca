# TASK-REQ-014-06: Chính sách `ops_request`: runbook, rollback, cổng `pre_deploy` từng bước, ghi kết quả

**From Solution:** BE-REQ-SOL-014
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/type_policy_ops_request.go` (mới), `internal/domain/runbook_rollback.go` (mới), `internal/domain/type_policy_ops_request_test.go` (mới), `internal/usecase/advance_execution.go` (kiểm thử nối cổng)
**Depends on:** TASK-REQ-014-03, TASK-REQ-014-04 (handler `pre_deploy`), TASK-REQ-013-04 (`AdvanceExecution`)
**Status:** [ ] TODO

---

## Context

- `ops_request` chặn trong `executing` bằng Approval `pre_deploy` trước bước không đảo ngược (README v6 mục 8 điều 6; Orca không có bước deploy, `pre_deploy` gắn với task có nhãn `gate:pre_deploy`).
- Đề xuất Plan (SOL-012): `TaskProposal.irreversible=true` thêm nhãn `gate:pre_deploy` khi lưu; có ít nhất một task nhãn `rollback`.
- `UpdateTask` thay cả danh sách nhãn (`update_task.go`): nhãn có thể bị xoá tay; `PreExecutionGate` đọc nhãn tại thời điểm chạy từ `TaskRef.Labels` do `ListTasks` trả về.
- Rollback không tự chạy; bước lỗi đi backlog stage `task`, lý do nêu tên task `rollback` liên quan để người quyết định (SOL-014 2.5).

## Việc cần làm

1. `runbook_rollback.go`: hàm thuần `CheckRunbook(p PlanProposal) error`:
   - mọi `TaskProposal.Irreversible` có nhãn hiệu lực chứa `gate:pre_deploy` (`EffectiveLabels`);
   - có ít nhất một task nhãn `rollback` (`REQUEST_RUNBOOK_ROLLBACK_MISSING`);
   - mỗi bước `irreversible` có một task `rollback` đứng **sau** nó theo `depends_on_indices`, hoặc `description` của nó có `rollback_note` khác rỗng (đề xuất định dạng dòng `rollback_note: ...`); nếu không, `REQUEST_RUNBOOK_IRREVERSIBLE_STEP_UNGATED` hoặc `..._ROLLBACK_MISSING` cho bước đó.
2. `type_policy_ops_request.go`:
   - `PlanPreconditions` gọi `CheckRunbook`.
   - `PreExecutionGate(req, task)`: nếu `task.Labels` chứa `gate:pre_deploy` và chưa có Approval `pre_deploy` `approved` cho `task.ID` thì trả `GateRequirement{SubjectType:"pre_deploy", SubjectID:task.ID, Stage:"task"}`; có `approved` thì `nil`; task không nhãn thì `nil`. Cổng đọc Approval qua `ApprovalReader` truyền khi dựng policy.
   - `CompletionChecks`: cần `ops_result` `passed` với `summary` khác rỗng; thiếu là `missing`; `failed` là `failed`.
   - `OnCompleted`: rỗng.
3. Bước lỗi: quy tắc đưa vào `ReportTaskOutcome` qua cổng (không rẽ nhánh theo loại): `TypePolicy` cung cấp `FailureHint(req, task) string` tuỳ chọn (interface mở rộng nhỏ `FailureHinter`) trả tên task `rollback` liên quan để thêm vào `reason`. Nếu không muốn đổi interface, `ReportTaskOutcome` kiểm kiểu `policy.(FailureHinter)`.
4. Kiểm `AdvanceExecution` không chạy task bị chặn, giữ `open`, Request vẫn `executing`, Approval `pending` hiện ở hộp duyệt (CR-REQ-022) và Execute backlog (SOL-015).
5. Đăng ký policy vào registry.

## Kiểm thử

- `TestCheckRunbook_Table`: không `rollback`; bước `irreversible` thiếu nhãn; `rollback` đứng trước chứ không sau; `rollback_note` thay thế; hợp lệ.
- `TestOpsPolicy_PreExecutionGate_LabelledTaskWithoutApproval`, `_WithApproved_Nil`, `_UnlabelledTask_Nil`.
- `TestOpsPolicy_Completion_RequiresOpsResult`.
- `TestAdvanceExecution_OpsRunbook_GateOpensOnceAndSkipsTask` (chạy lặp, một Approval `pending`), `_AfterApproval_Dispatches`.
- `TestReportTaskOutcome_OpsStepFails_BacklogMentionsRollbackTask`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'Runbook|OpsPolicy|OpsRunbook' -v`.

## Tiêu chí hoàn thành

- [ ] Task nhãn `gate:pre_deploy` không `Execute` cho đến khi có Approval `approved`; task không nhãn chạy bình thường.
- [ ] Approval được tạo đúng một lần dù `AdvanceExecution` chạy lặp.
- [ ] Đề xuất thiếu rollback hoặc bước không đảo ngược thiếu cổng bị đúng mã lỗi.
- [ ] `ops_result` thiếu thì không hoàn tất.
- [ ] Bước lỗi không tự rollback; lý do backlog nêu task `rollback` liên quan.

## Rủi ro và lưu ý

- Nhãn bị xoá tay làm mất cổng; chưa có khoá nhãn (ghi ở SOL-014 mục 6).
- Định dạng `rollback_note` là đề xuất của task này, chưa có trong CR: xác nhận với chủ CR-REQ-014 trước khi cố định.
- `ops_request` cần runbook có rollback do AI sinh; chất lượng chưa đo.

## Tiến độ

Đã làm: `CheckRunbook`, `opsRequestPolicy` (`PreExecutionGate` mở Approval một lần, `CompletionChecks`, `FailureHint` nêu task rollback), test `AdvanceExecution` cổng, lý do backlog. Còn thiếu: lời gọi `PlanPreconditions` từ `GeneratePlan` (CR-REQ-012 chưa có).

Lệch so với task và điểm chưa kiểm chứng: xem `IMPLEMENTATION-NOTES.md` mục "Đợt 3, phần request-service (exec)".
