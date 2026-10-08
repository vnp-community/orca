# TASK-REQ-008-04: `SubjectHandler` cho `findings` và `answer`, và hoàn tất sau phân tích

**From Solution:** [BE-REQ-SOL-008](../solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md) mục F
**Priority:** P1
**Service/Area:** `request-service` / usecase, cmd
**File:** `internal/usecase/findings_approval_handler.go` (mới), `answer_approval_handler.go` (mới), `solution_approval_handler.go` (sửa: bộ kiểm hợp lệ cho `diagnosis`), `cmd/server/main.go` (sửa: đăng ký), và `_test.go`
**Depends on:** TASK-REQ-008-03, TASK-REQ-007-06, TASK-REQ-009-06
**Status:** [x] DONE (đã kiểm chứng 2026-10-08, sau hợp nhất với Approval thật: `go test -race ./internal/usecase/... -run "SolutionHandler|FindingsAndAnswer|FindingsHandler"`; `go test -tags integration ./internal/adapter/... -run SolutionContract` gồm `SubjectHandlerContract` thật và luồng `question` qua `ApprovalServer.Approve` → `completed`)

## Context

- Cùng khuôn `SolutionApprovalHandler` (task 007-06). `diagnosis` dùng chung `subject_type=solution` nhưng không có `chosen_option`.
- `CompletesAfterAnalysis=true` của registry (CR-REQ-003) cho `spike` và `question`: sau `analysis_approved` Request sang `completed`, không Plan, không Task. Request con tạo từ `follow_ups` qua CR-REQ-006, không tự tạo ở đây.
- Reject: `analysis_rejected` về backlog, `returned_from_stage=analysis`.

## Việc cần làm

1. `FindingsApprovalHandler`: `ValidateForRequest` (Solution đúng `kind=findings`, `proposed`; digest `DigestOptions(options,nil)`); `OnApproved` (Solution `approved`, outbox `solution.approved`, `TransitionRequest(analysis_approved)`); `OnRejected`; `OnClosedWithoutDecision`.
2. `AnswerApprovalHandler`: như trên cho `kind=answer`; "người dùng chấp nhận" là `Approve` bởi người báo cáo (chính sách của SOL-010 cho `answer` cho phép `reporter`).
3. `SolutionApprovalHandler.ValidateForRequest`: nếu `kind=diagnosis` thì không yêu cầu `chosen_option` và gọi `ValidateByKind`.
4. `main.go`: đăng ký hai handler mới cho `findings` và `answer`, thay `NoopSubjectHandler` của hai subject đó.
5. Kiểm `follow_ups` và `suggested_follow_up` chỉ là dữ liệu điền sẵn: không có đường mã nào tạo Request con hay đổi `requests.type` khi `suggest_escalate_to_change_request=true`.

## Kiểm thử

- Chạy `RunSubjectHandlerContract` cho cả hai handler.
- Unit: `answer` được chấp nhận thì Request `completed` (fake registry), không gọi bất kỳ thao tác Plan hoặc Task nào; `findings` bị từ chối thì backlog; `diagnosis` duyệt không cần `chosen_option`.
- Test chặn: `suggest_escalate_to_change_request=true` không đổi `requests.type` (fake repo không nhận lời gọi `UpdateType`).
- Lệnh: `go test ./internal/usecase/... -run "FindingsApproval|AnswerApproval|DiagnosisApproval"`.

## Tiêu chí hoàn thành

- [x] Hai `subject_type` mới có handler thật; `MustCoverAll` không còn Noop cho chúng.
- [x] Tiêu chí 9 và 10 của CR-REQ-008 có test.
- [x] Handler chỉ ghi DB của `request-service`.

## Rủi ro và lưu ý

- `answer` chấp nhận bởi chính người báo cáo trùng ý "tách nhiệm vụ" mặc định tắt cho `answer` (SOL-010 mục 2.5).

## Ghi chú triển khai (2026-10-08)

- Ba handler dùng chung thân `analysisApprovalHandler` (`solution_approval_handler.go`); `findings_approval_handler.go` và `answer_approval_handler.go` là hàm dựng. `diagnosis` không đòi `chosen_option` (chọn theo `Solution.Kind`, không theo handler). `answer`/`findings` được duyệt thì Request `completed` nhờ `CompletesAfterAnalysis`, không đụng Plan hay Task (kiểm trên DB thật cho `question`).
- Đăng ký trong `wireSolution` qua map `handlers` truyền vào `buildApprovalRegistry`: `TestWireSolution_RegistersRealHandlersAndStartsTheSweeper` chứng minh registry ba subject này nhận handler thật mà không cần `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS`. `MustCoverAll` của cả 8 subject còn phụ thuộc đợt Approval cho 5 subject còn lại.
- `suggest_escalate_to_change_request` và `follow_ups` chỉ là dữ liệu: có test khẳng định `requests.type` không đổi và không có đường mã nào tạo Request con.
- Sau hợp nhất: `wireApproval` nhận map handler của chủ sở hữu và đăng ký ba handler này thay `TransitionSubjectHandler` (bỏ phần trùng); `RunSubjectHandlerContract` thật chạy cho cả ba trên Postgres và MySQL.
