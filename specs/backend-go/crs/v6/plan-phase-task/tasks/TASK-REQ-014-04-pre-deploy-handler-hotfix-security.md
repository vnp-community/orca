# TASK-REQ-014-04: `SubjectHandler` `pre_deploy`, chính sách `hotfix` (cổng) và `security`

**From Solution:** BE-REQ-SOL-014
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/type_policy_pre_deploy_handler.go` (mới), `internal/domain/type_policy_hotfix.go` (mới), `internal/domain/type_policy_security.go` (mới), `internal/usecase/commit_plan.go` (nhánh mở `pre_deploy`), `cmd/server/main.go`, `*_test.go` (mới)
**Depends on:** TASK-REQ-014-03, TASK-REQ-012-06 (single_task), CR-REQ-009 (`SubjectHandler`, `OpenApproval`), CR-REQ-010 (người duyệt bắt buộc là người cho hotfix)
**Status:** `[ ] TODO`

---

## Context

- CR-REQ-003: `hotfix` và `security` có `StartGate = pre_deploy` chiếm `awaiting_plan_approval` (README v6 mục 8 điều 6); `ops_request` chặn trong `executing` (task 06). Orca không có bước deploy: đây là cổng duyệt trước khi chạy task fix.
- SOL-012 `CommitPlan`: `security` mở `pre_deploy` thay `plan` (subject là id Plan); `hotfix` mở `pre_deploy` với subject là id task fix duy nhất (task 06 của SOL-012 tạo task, không mở Approval).
- CR-REQ-009 yêu cầu mọi `subject_type` trong CHECK có `SubjectHandler`, nếu thiếu service không khởi động.
- `security`: `kind=security_recheck` (`passed`) bắt buộc trước hoàn tất; việc xác nhận mức độ là Approval `request_type` (CR-REQ-005), không ở đây.

## Việc cần làm

1. `type_policy_pre_deploy_handler.go`: 
   - `ValidateForRequest(ctx, req, subjectID)`: bảng `(type → subject)`: `hotfix`: task đúng `request_id`, không `parent_id`, không `cancelled`; `security`: task `plan` đúng `request_id`; `ops_request`: task có nhãn `gate:pre_deploy` thuộc cây Plan của Request. Digest SHA-256 của `(id, title, labels)` task (hoặc cây Plan cho `security`).
   - `OnApproved`: `hotfix`/`security` thì `TransitionRequest(plan_approved, ExpectedFrom=awaiting_plan_approval)`; `ops_request` không đổi trạng thái (consumer `approval.decided` kích `AdvanceExecution`).
   - `OnRejected`: `hotfix`/`security` thì `TransitionRequest(plan_rejected)`; `ops_request` thì `ReturnToBacklog(stage=task, category=rejected, reason=comment)`.
   - `OnClosedWithoutDecision`: không làm gì.
2. Kiểm "người duyệt bắt buộc là người" cho `hotfix`: hỏi `ApproverPolicy` (CR-REQ-010) khi `OpenApproval`; không tự cài chính sách quyền ở đây.
3. `type_policy_hotfix.go`: `PlanPreconditions` yêu cầu đúng một task, không Phase, không Plan (đã bảo đảm bởi `ShapeSingleTask`, kiểm lại); `PreExecutionGate` trả `nil` (cổng đã chiếm ở `StartGate`); `CompletionChecks` rỗng; `OnCompleted` do task 07.
4. `type_policy_security.go`: `PlanPreconditions` yêu cầu có task `test:regression` (đã kiểm ở `ValidateProposal`, ở đây không lặp); `CompletionChecks` trả `missing` khi chưa có `security_recheck` `passed`, `failed` khi bản mới nhất `failed`; `OnCompleted` rỗng.
5. Nối `CommitPlan` mở `pre_deploy` thay `plan` cho `security` và mở `pre_deploy` cho `hotfix` ngay trước trigger `analysis_ready` (CR-REQ-008 gọi). Lấy loại gate từ `FlowDefinition.StartGate`, không `switch` theo loại.
6. Đăng ký handler `pre_deploy` ở `main.go`.

## Kiểm thử

- `TestPreDeployHandler_Validate_Hotfix_TaskOfOtherRequest`, `_Security_PlanCancelled`, `_DigestChangesOnLabelEdit`.
- `TestPreDeployHandler_OnApproved_Hotfix_PlanApproved`, `_OnRejected_Security_PlanRejected`, `_OnApproved_Ops_NoStateChange`, `_OnRejected_Ops_BacklogTask`.
- `TestHotfixPolicy_NoGateDuringExecution`, `TestSecurityPolicy_Completion_MissingRecheck`, `_FailedRecheck`, `_PassedRecheck`.
- `TestCommitPlan_Security_OpensPreDeployNotPlan`, `TestCommitPlan_Hotfix_OpensPreDeployOnTask`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'PreDeploy|HotfixPolicy|SecurityPolicy|CommitPlan' -v`.

## Tiêu chí hoàn thành

- [ ] `hotfix`: Approval `pre_deploy` trên task fix duy nhất kích hoạt `executing`; người không có quyền duyệt không duyệt được; từ chối đi backlog.
- [ ] `security`: Plan được `pre_deploy` duyệt mới chạy; thiếu `security_recheck` `passed` thì Request không `completed`.
- [ ] Service từ chối khởi động nếu thiếu handler `pre_deploy`.
- [ ] Không `switch` theo loại Request trong `CommitPlan`.

## Rủi ro và lưu ý

- Q2 của CR: tên "pre_deploy" gợi ý sau khi sửa xong, nhưng cổng nằm trước khi chạy task fix (theo CR-REQ-003). Xác nhận ý nghĩa với chủ CR trước khi làm UI.
- `pre_deploy` là cổng duyệt; agent vẫn có thể chạy lệnh deploy trong task không nhãn.
- Plan của `security` không có Approval `plan` (CR-012 Q3): nội dung Plan chỉ được duyệt gián tiếp qua `pre_deploy`.
