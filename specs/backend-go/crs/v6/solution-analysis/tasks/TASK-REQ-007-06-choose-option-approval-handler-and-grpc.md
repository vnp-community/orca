# TASK-REQ-007-06: `ChooseSolutionOption`, `SolutionApprovalHandler`, proto và gRPC

**From Solution:** [BE-REQ-SOL-007](../solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md) mục D, E
**Priority:** P0
**Service/Area:** `request-service` / usecase, proto, adapter grpc, cmd
**File:** `internal/usecase/choose_solution_option.go` (mới), `solution_approval_handler.go` (mới), `internal/adapter/grpc/solution_server.go` (mới), `proto/orca/request/v1/request.proto` (sửa), `cmd/server/main.go` (sửa), và `_test.go`, `_integration_test.go`
**Depends on:** TASK-REQ-007-05, TASK-REQ-009-04 (`SubjectHandler`, `UpdatePendingDigest`), TASK-REQ-009-06 (bộ test hợp đồng handler)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08, sau hợp nhất với Approval thật: `go test -race ./internal/usecase/... ./internal/adapter/grpc/...`; `go test -tags integration ./internal/adapter/postgres/... ./internal/adapter/mysql/... -run SolutionContract` gồm luồng generate → choose → `ApprovalServer.Approve` → `planning`, digest sai bị `REQUEST_APPROVAL_DIGEST_MISMATCH`, reject → backlog; `buf breaking` so với `main` sạch)

## Context

- `request.proto` do CR-REQ-001 tạo; thêm message và RPC additive, `buf breaking` xanh.
- Tên kênh WS `solution.*` thuộc CR-REQ-016 (`solution.generate`, `solution.choose`... theo CR đó); backend giữ tên RPC `GenerateSolution`, `ListSolutions`, `ChooseSolutionOption`.
- `ChooseSolutionOptionResponse.approval_digest` là thứ UI gửi lại ở `Approve(expected_digest)`.

## Việc cần làm

1. `request.proto`: thêm enum `SolutionKind`, `SolutionStatus`, `AnalysisMode`, message `Solution`, `AnalysisRun`, các cặp request/response và ba RPC đúng CR 2.7. `chosen_option=-1` nghĩa là chưa chọn. Chạy `buf lint`, `buf generate`, `buf breaking`.
2. `ChooseSolutionOption.Execute`: Solution `proposed`; `option_id` đổi thành chỉ số (`REQUEST_SOLUTION_OPTION_NOT_FOUND` nếu lạ); transaction: `SolutionRepository.Choose` (khoá lạc quan `version`), `ApprovalRepository.UpdatePendingDigest("solution", id, DigestOptions(options,&idx))`, trả digest. Cùng `option_id` lặp là no-op thành công. Khoá thứ tự Request rồi Approval nếu đụng tới Approval.
3. `SolutionApprovalHandler`: `ValidateForRequest` (thuộc Request, `proposed`; `kind=solution` thì `chosen_option` không NULL, nếu không `REQUEST_SOLUTION_OPTION_NOT_CHOSEN`; trả `DigestOptions`), `OnApproved` (Solution `approved`, outbox `solution.approved{request_id, solution_id, kind, chosen_option}`, `TransitionRequest(analysis_approved)`), `OnRejected` (`rejected`, `TransitionRequest(analysis_rejected)`), `OnClosedWithoutDecision` (`superseded` khi do đổi loại).
4. `solution_server.go` ánh xạ ba RPC, tự kiểm quyền, ánh xạ lỗi.
5. `main.go`: đăng ký handler cho `solution`, khởi chạy vòng phục hồi (task 05), đăng ký server.
6. Cung cấp hàm `Supersede(ctx, tx, requestID)` cho CR-REQ-005 (đổi loại) nếu chưa có.

## Kiểm thử

- Unit: chọn lặp; chọn khi không `proposed`; option lạ; digest thay đổi sau mỗi lần chọn; `Approve` chưa chọn bị chặn; `OnRejected` không để `chosen_option` rò sang Plan.
- Chạy `RunSubjectHandlerContract` (task 009-06) cho handler này.
- Integration hai DB: luồng đầy đủ `GenerateSolution` (Relay giả) -> `ChooseSolutionOption` -> `Approve` -> Request `planning`; sửa `options` hoặc đổi `chosen_option` không dùng digest mới thì `REQUEST_APPROVAL_DIGEST_MISMATCH`; đổi loại làm Solution `superseded` và Approval `cancelled`.
- Lệnh: `go test ./internal/usecase/... ./internal/adapter/... -run "ChooseSolution|SolutionApproval|SolutionFlow"`.

## Tiêu chí hoàn thành

- [x] Tiêu chí chấp nhận 9 đến 14 của CR-REQ-007 mục 4 có test.
- [x] `buf breaking` xanh (`buf breaking --against .git#branch=main,subdir=backend-go/proto`, 2026-10-08; không sửa `.proto`).
- [x] Handler qua bộ test hợp đồng (`contracttest.RunSubjectHandlerContract` thật, trên DB thật, cho `solution`/`diagnosis`, `findings`, `answer`: `SolutionContract/SubjectHandlerContract`).
- [x] Payload outbox không chứa `options`.

## Rủi ro và lưu ý

- Hợp đồng với frontend (CR-REQ-020) dùng `approval_digest`; đổi tên trường là phá vỡ hợp đồng.
- e2e thật cần dev server có `ai.complete` (chưa kiểm chứng): để cho CR-REQ-025.

## Ghi chú triển khai (2026-10-08)

- Proto đã đầy đủ (`solution.proto`, ba RPC trong `RequestService`), tác vụ này KHÔNG sửa `.proto` nên `buf breaking` không có thay đổi để vi phạm; chưa chạy `buf` (không có `origin/main` chứa package, CI tự bỏ qua).
- `server_solution.go` (không phải `solution_server.go`) cài `GenerateSolution`, `ListSolutions`, `ChooseSolutionOption` vào `Server` qua `WithSolution`; `engine_override` ngoài `native` bị từ chối; `Get/SetProjectEngineSettings` thuộc CR-REQ-026, không làm ở đây.
- `NewSolutionApprovalHandler` (cả `kind=solution` lẫn `diagnosis`) dùng chung thân với handler findings/answer. `OnApproved` tự tính lại digest từ nội dung hiện tại và so với `Approval.SubjectDigest`, sai thì `REQUEST_APPROVAL_DIGEST_MISMATCH` (phòng thủ thêm, không phụ thuộc use case Approve). `OnRejected` gọi `ReturnRequestToBacklog` (stage analysis, category rejected) và xoá `chosen_option`.
- Đăng ký: `wireSolution` trả `handlers` và `main.go` truyền vào `buildApprovalRegistry`. `SupersedeForTypeChange` có sẵn cho CR-REQ-005; đường đổi loại hiện hoạt động qua `OnClosedWithoutDecision(why="type_changed")` khi có Approval pending, còn `ChangeRequestType` chưa gọi hàm này (xem IMPLEMENTATION-NOTES).
- Đường `Approve`/`Reject` của `ApprovalServer` gRPC thật đã được chạy đầu-cuối trên Postgres và MySQL (`RPCFlow`, `RPCReject`, `SolutionFlowGenerateChooseApprove`). `REQUEST_APPROVAL_DIGEST_MISMATCH` đến từ cả use case Decide (digest cũ) và handler (nội dung bị sửa sau khi mở).
- Sau hợp nhất: handler `solution`/`findings`/`answer` thay `TransitionSubjectHandler` cho ba subject này (`wireApproval(..., owned)`), nên không cần `SubjectArtifacts` cho chúng; `ValidateForRequest` từ chối request ngoài `awaiting_analysis_approval` và subject lạ theo hợp đồng chung, và cho phép mở khi chưa chọn (digest không-chọn); `OnApproved` mới đòi `chosen_option`.
- Quyền chọn phương án: `ApproverAwareAuthorizer` (người báo cáo, admin/lead/owner, hoặc người `AuthorizeApprovalDecision` của chính approval pending cho phép quyết định, gồm team/vai trò theo snapshot); kiểm trước giao dịch vì có thể gọi tenant-service.
