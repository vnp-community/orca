# TASK-REQ-012-06: `SubjectHandler` cho `plan` và `task_list`, đường `single_task` (hotfix)

**From Solution:** BE-REQ-SOL-012
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/plan_subject_handler.go` (mới), `plan_subject_handler_test.go` (mới), `internal/usecase/commit_single_task.go` (mới), `cmd/server/main.go` (đăng ký handler)
**Depends on:** TASK-REQ-012-05, CR-REQ-009 (`SubjectHandler`, `ApprovalRegistry`)
**Status:** `[x] DONE`

---

## Context

- CR-REQ-009 mục 2.4 định nghĩa `SubjectHandler` (`ValidateForRequest`, `OnApproved`, `OnRejected`, `OnClosedWithoutDecision`) và quy định mỗi `subject_type` trong CHECK phải có handler, thiếu thì service không khởi động. CR này sở hữu `plan` và `task_list`.
- `ValidateForRequest` cần cây thật từ `task-service`: `GetSubtree(root_id)` (`usecase/get_subtree.go`; trả task và các cạnh `depends_on`).
- Digest: nếu Plan bị sửa sau khi người duyệt xem, `Approve` với `expected_digest` cũ phải bị từ chối (cơ chế của CR-REQ-009).
- `hotfix` (`single_task`): CR-REQ-003 cho `analysis_ready` đi thẳng `awaiting_plan_approval` (không qua `planning`), nên task fix phải có trước; `requests.plan_task_id` NULL, liên kết bằng `tasks.request_id`. Approval `pre_deploy` thuộc SOL-014.

## Việc cần làm

1. `plan_subject_handler.go`: `type PlanSubjectHandler struct{ tasks TaskReader; requests RequestRepository; transitions RequestTransitioner }`; đăng ký cho hai `subject_type` bằng hai instance (`Kind: "plan"`, `"task_list"`).
2. `ValidateForRequest(ctx, req Request, subjectID string) (digest string, err error)`: `GetSubtree`; root phải `type=plan`, `request_id == req.ID`, `status != cancelled`, không rỗng; digest = SHA-256 của danh sách sắp theo `(parent_id, id)` gồm `(id, title, task_type, parent_id)` cộng các cạnh `depends_on` đã sắp, mã hoá cố định (tránh phụ thuộc thứ tự map).
3. `OnApproved`: `TransitionRequest(plan_approved, ExpectedFrom=awaiting_plan_approval)`; chỉ ghi DB `request-service` nên chạy trong cùng transaction của `Approve`.
4. `OnRejected`: `TransitionRequest(plan_rejected)` với `category=rejected` và `reason=comment` (CR-REQ-003, 006).
5. `OnClosedWithoutDecision`: không làm gì (huỷ do replan hay đổi loại đã do lệnh gây ra xử lý).
6. `commit_single_task.go`: nhánh `ShapeSingleTask` của `CommitPlan` gọi `TaskPlanWriter.CreateSingleTask` (`CreateTask` với `request_id`, `project_id`, không `parent_id`, nhãn từ đề xuất) ngay trước trigger `analysis_ready` (CR-REQ-008 gọi nội bộ); trả `task_id`; không `OpenApproval` ở đây (SOL-014 mở `pre_deploy`).
7. Đăng ký handler ở `main.go`; thêm test khởi động thất bại khi thiếu handler (đã do CR-REQ-009, ở đây chỉ kiểm hai subject này có mặt).

## Kiểm thử

- `TestPlanSubjectHandler_Validate_WrongRequest`, `_Cancelled`, `_NotPlan`, `_DigestStable_ShuffledInput`, `_DigestChanges_OnTitleEdit`, `_DigestChanges_OnNewEdge`.
- `TestPlanSubjectHandler_OnApproved_TransitionsToExecuting`, `_OnRejected_ReturnsPlanRejected`, `_OnApproved_StaleState_NoOp`.
- `TestCommitSingleTask_CreatesOneTaskNoParent_PlanTaskIDNull`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'PlanSubject|SingleTask' -v`.

## Tiêu chí hoàn thành

- [x] Duyệt Plan chuyển Request sang `executing`; từ chối ghi `plan_rejected`.
- [x] Sửa Plan sau khi xem làm `Approve` với digest cũ bị từ chối.
- [x] `task_list` trỏ vào task `plan` vỏ và duyệt được như `plan`.
- [x] `hotfix` có đúng một task không `parent_id`, `plan_task_id` NULL.

## Rủi ro và lưu ý

- Digest phụ thuộc `GetSubtree` trả đủ task và cạnh; Plan 100 task nặng hơn `GetSubtree` thường: đo khi có dữ liệu.
- Q của CR-012: `security` chiếm `awaiting_plan_approval` bằng `pre_deploy` nên không có Approval `plan`; handler `plan` không áp cho loại này.
- Handler gọi `task-service` trong `ValidateForRequest` (trước `OpenApproval`): nếu task-service sập thì `OpenApproval` thất bại, chấp nhận (không mở cổng duyệt khi không kiểm chứng được).
