# TASK-REQ-043-01: Miền `ApprovalExecutionMode`, hai mã lỗi mới, trường proto `execution_mode`

**From Solution:** [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md)
**Priority:** P1
**Service:** `request-service` · `proto/orca/request/v1`
**File:** `backend-go/proto/orca/request/v1/approval.proto` (sửa) + mã sinh, `services/request-service/internal/domain/{approval_execution_mode,approval_execution_mode_test}.go` (mới), `domain/approval_events.go` (sửa)
**Depends on:** không
**Status:** [x] DONE (2026-10-20)

---

## Context

- `ApproveRequest` có trường 1 đến 4 (`approval.proto:65-70`), `ApproveResponse` có trường 1 đến 2 (`:106-109`). `RejectRequest` giữ nguyên.
- `domain/execution_mode.go` đã có `ParseExecutionMode` (nhận cả `step_by_step`, dùng cho cài đặt dự án) và `ErrExecutionModeInvalid` (`REQUEST_EXECUTION_MODE_INVALID`). Lỗi kiểu này nằm ở `domain/execution_errors.go` (mẫu `ErrExecutionActorNotAllowed`, dòng 33).
- `ApprovalDecidedPayload` (`approval_events.go:27-41`) chỉ có các khoá cũ.
- `ExecutionStartPrecheck` hiện tự lọc subject và stage bằng một `switch` (`approval_execution_precheck.go:22-29`); hàm dùng chung ở task này thay phần lọc đó ở task 03.

## Việc cần làm

1. Proto: `ApproveRequest.execution_mode = 5` (chuỗi; chú thích: rỗng hoặc `continuous`, chỉ cho Plan, task list, pre_deploy ở `awaiting_plan_approval`); `ApproveResponse.execution_mode = 3` (chế độ đang ghim trên Request sau quyết định, rỗng nếu chưa ghim). Kiểm số trường còn trống bằng `buf`/`protoc` theo quy trình của repo; sinh lại mã, không sửa tay file sinh; diff mã sinh chỉ gồm phần thêm.
2. `domain.ParseApprovalExecutionMode(raw string) (ExecutionMode, error)`: cắt khoảng trắng, hạ chữ thường; rỗng → `""`; `continuous` → `ExecutionContinuous`; còn lại (kể cả `step_by_step`) → `ErrExecutionModeInvalid(raw)`.
3. `domain.ExecutionModeChoiceApplies(a Approval) bool`: `a.Stage == string(RequestStatusAwaitingPlanApproval)` và `a.SubjectType` ∈ {`SubjectPlan`, `SubjectTaskList`, `SubjectPreDeploy`}.
4. `ErrExecutionModeNotApplicable()` (`FailedPrecondition`, `REQUEST_EXECUTION_MODE_NOT_APPLICABLE`) và `ErrExecutionModeAlreadyPinned(pinned ExecutionMode)` (`FailedPrecondition`, `REQUEST_EXECUTION_MODE_ALREADY_PINNED`, thông điệp nêu chế độ đang ghim, không lộ dữ liệu khác).
5. `ApprovalDecidedPayload.ExecutionMode string` với tag `json:"execution_mode,omitempty"`; `NewApprovalDecidedPayload` giữ nguyên chữ ký.
6. Ghi hai mã lỗi mới vào danh mục mã lỗi của service nếu có (kiểm `domain/rpc_catalog.go` và test danh mục lỗi); mã không có trong danh mục làm test hiện có đỏ thì thêm vào.

## Kiểm thử

- Bảng `ParseApprovalExecutionMode`: `""`, `"  "`, `continuous`, `Continuous`, `" CONTINUOUS "`, `step_by_step`, `continuous;x`, chuỗi 1000 ký tự; mỗi ca kiểm giá trị trả và mã lỗi.
- `ExecutionModeChoiceApplies`: từng `SubjectType` (kể cả `solution`, `phase`, `request_type`) × stage (`awaiting_plan_approval` và hai stage khác).
- Marshal `ApprovalDecidedPayload`: không có khoá `execution_mode` khi rỗng; có khi đặt.
- `go test ./services/request-service/internal/domain/... -run 'ApprovalExecutionMode|ApprovalDecidedPayload' -v`; `buf lint`/`buf breaking` (hoặc lệnh tương đương của repo) xanh.

## Tiêu chí hoàn thành

- [x] Proto thêm đúng hai trường, mã sinh cập nhật, `buf breaking` không báo phá vỡ. Bằng chứng: `ApproveRequest.execution_mode = 5`, `ApproveResponse.execution_mode = 3`; `git diff --stat` chỉ đụng `approval.proto` và `approval.pb.go` (+28/-6, chỉ phần thêm). `buf` không cài trong môi trường này nên `buf lint`/`buf breaking` chưa chạy; hai trường thêm mới với số trường chưa dùng nên không thể là thay đổi phá vỡ.
- [x] Hàm phân tích từ chối `step_by_step`; hai lỗi mới có đúng loại gRPC và mã. Bằng chứng: `TestParseApprovalExecutionMode`, `TestExecutionModeErrors`.
- [x] Payload không đổi byte khi không chọn chế độ (test so JSON). Bằng chứng: `TestApprovalDecidedPayload_ExecutionModeKey` và golden `TestApprovalEvents_DecidedPayloadMatchesGolden` giữ nguyên.

## Rủi ro và lưu ý

- Số trường 5 và 3 có thể bị CR khác giữ; nếu va chạm thì đổi số và cập nhật CR/solution.
- Chạy `gitnexus_impact` cho `NewApprovalDecidedPayload` và `ApprovalDecidedPayload` (nhiều người tiêu thụ); báo blast radius.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai (2026-10-20)

- Proto: sửa `backend-go/proto/orca/request/v1/approval.proto`, sinh lại bằng `protoc 33.4` + `protoc-gen-go v1.36.11` (`--go_opt=paths=source_relative`), đặt lại hai dòng header cũ (`protoc-gen-go v1.36.12`, `protoc (unknown)`) để diff chỉ còn phần thêm. `approval_grpc.pb.go` không đổi (không có RPC mới).
- Miền: `domain/approval_execution_mode.go` (`ParseApprovalExecutionMode`, `ExecutionModeChoiceApplies`, `ErrExecutionModeNotApplicable`, `ErrExecutionModeAlreadyPinned`), `domain/approval_events.go` (`ApprovalDecidedPayload.ExecutionMode`, `omitempty`), test `domain/approval_execution_mode_test.go`.
- Lệnh: `go test ./internal/domain/... -run 'ApprovalExecutionMode|ApprovalDecidedPayload|ExecutionModeChoice|ExecutionModeErrors|ApprovalEvents' -v` xanh (6 test); `go test ./internal/domain/` xanh; `go build ./...` và `go test ./...` trong `backend-go/proto` xanh.
- Danh mục mã lỗi: service không có danh mục mã lỗi tập trung (kiểm `rpc_catalog.go`: chỉ là danh mục RPC); không có test danh mục nào cần thêm. Hai mã mới được liệt kê trong CONTRACT (task 04).
- Blast radius (không có GitNexus, dùng grep): `NewApprovalDecidedPayload` được gọi ở `usecase/decide_approval.go`, `usecase/cancel_approval.go`, `usecase/expire_approvals.go`, `usecase/cancel_pending_approvals_for_request.go`, `usecase/approval_request_type_recorder.go` và test; chữ ký không đổi, trường mới `omitempty` nên payload của các đường kia không đổi byte.
- Không kiểm chứng: `buf lint`/`buf breaking` (không có `buf`).
