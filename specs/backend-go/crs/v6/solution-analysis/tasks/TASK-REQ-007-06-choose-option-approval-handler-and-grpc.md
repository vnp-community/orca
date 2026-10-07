# TASK-REQ-007-06: `ChooseSolutionOption`, `SolutionApprovalHandler`, proto và gRPC

**From Solution:** [BE-REQ-SOL-007](../solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md) mục D, E
**Priority:** P0
**Service/Area:** `request-service` / usecase, proto, adapter grpc, cmd
**File:** `internal/usecase/choose_solution_option.go` (mới), `solution_approval_handler.go` (mới), `internal/adapter/grpc/solution_server.go` (mới), `proto/orca/request/v1/request.proto` (sửa), `cmd/server/main.go` (sửa), và `_test.go`, `_integration_test.go`
**Depends on:** TASK-REQ-007-05, TASK-REQ-009-04 (`SubjectHandler`, `UpdatePendingDigest`), TASK-REQ-009-06 (bộ test hợp đồng handler)
**Status:** [x] DONE

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
- [x] `buf breaking` xanh.
- [x] Handler qua bộ test hợp đồng.
- [x] Payload outbox không chứa `options`.

## Rủi ro và lưu ý

- Hợp đồng với frontend (CR-REQ-020) dùng `approval_digest`; đổi tên trường là phá vỡ hợp đồng.
- e2e thật cần dev server có `ai.complete` (chưa kiểm chứng): để cho CR-REQ-025.
