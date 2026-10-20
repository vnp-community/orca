# TASK-REQ-043-02: `DecideApproval` ghim chế độ trong cùng transaction, payload và audit

**From Solution:** [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/{decide_approval,decide_approval_execution_mode,audit_event_tap}.go` (sửa/mới), `internal/adapter/grpc/approval_server.go` (sửa), `cmd/server/wire_approval.go` (sửa), test tương ứng
**Depends on:** TASK-REQ-043-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- `DecideApproval.Execute` (`decide_approval.go:45-170`): khoá Request (`:84`) rồi Approval (`:88`), kiểm quyền/phiên bản/digest/stage (`:111-121`), `UpdateDecision` (`:133`), handler (`:143`), outbox (`:158`). Gọi lại cùng quyết định trả kết quả cũ (`:93-96`).
- `PinExecutionModeIfUnset` đã có ở hai dialect (`adapter/postgres/request_repository.go:171`, `adapter/mysql/request_repository.go:164`) và dùng transaction trong `ctx` qua `scoped()`; hiện chỉ `StartExecution` gọi nó, ngoài transaction.
- `store_wiring.go:35,87,136` có `stores.executionModes` (cùng repository với Request).
- `AuditTap` chỉ đưa `subject_type` và `stage` vào metadata (`audit_event_tap.go:117-122`).

## Việc cần làm

1. `DecideApprovalInput.ExecutionMode string`; `DecideApprovalResult.ExecutionMode domain.ExecutionMode`; `DecideApproval.Pinner ExecutionModePinner`.
2. Đầu `Execute`: `ParseApprovalExecutionMode`; có chế độ mà `Decision == reject` → `ErrExecutionModeInvalid`. Sau khi đọc `probe`: có chế độ mà `!ExecutionModeChoiceApplies(probe)` → `ErrExecutionModeNotApplicable` (trước precheck, tránh gọi task-service vô ích). Truyền chế độ vào `StartPrecheck.Check` (chữ ký đổi ở task 03; làm hai task cùng nhánh hoặc task 03 trước nếu cần biên dịch).
3. `decide_approval_execution_mode.go` (file mới, để `decide_approval.go` không phình): `pinRequestedMode(ctx, pinner, req, mode) (domain.ExecutionMode, error)`: `mode == ""` → trả `req.ExecutionMode` không gọi; `pinner == nil` → lỗi `Internal` rõ ràng; `got != mode` → `ErrExecutionModeAlreadyPinned(got)`.
4. Trong transaction, sau stage mismatch và **trước** `a.Approve`: gọi `pinRequestedMode`. Ca "đã quyết định" (`sameDecision`) không ghim, trả `ExecutionMode: req.ExecutionMode`.
5. Payload: sau `NewApprovalDecidedPayload`, `p.ExecutionMode = string(mode)` chỉ khi `Decision == approve`; kết quả `res.ExecutionMode = after.ExecutionMode` (đọc sau handler).
6. `audit_event_tap.go`: `auditFields.ExecutionMode string `json:"execution_mode"``; `approvalAudit` thêm `md["execution_mode"]` khi khác rỗng. Không đổi hành vi với `rejected`, `cancelled`, `expired`.
7. `approval_server.go`: `decide(...)` nhận `executionMode`; `Approve` truyền `req.GetExecutionMode()` và trả `ApproveResponse.ExecutionMode`; `Reject` truyền `""`.
8. `wire_approval.go`: `Decide.Pinner = stores.executionModes`; thêm vào danh sách `requireWired` để thiếu thì khởi động lỗi (không tắt im lặng).

## Kiểm thử

- Unit với fake: chế độ hợp lệ ghim một lần, payload và kết quả mang `continuous`; `reject`+chế độ; subject/stage sai; đã ghim `step_by_step` (không ghi quyết định, Approval vẫn `pending`); đã ghim `continuous` (idempotent); handler lỗi (fake pinner thấy không còn ghim vì transaction cuộn lại); nhấp đúp (một sự kiện, một lần ghim, kết quả thứ hai mang chế độ đang ghim); `Pinner == nil`; hai quyết định đồng thời (một thắng, người kia `REQUEST_APPROVAL_ALREADY_DECIDED`).
- `AuditTap`: có `execution_mode` khi payload có; không khi vắng; không rò tiêu đề (test audit hiện có giữ).
- `go test ./services/request-service/internal/usecase/... -run 'DecideApproval|AuditTap|ExecutionMode' -v`; `go test ./services/request-service/internal/adapter/grpc/...`.

## Tiêu chí hoàn thành

- [x] Duyệt Plan kèm `continuous` ghim chế độ và chuyển Request sang `executing` trong một transaction; handler lỗi để lại cột `NULL`. Bằng chứng: `TestDecideApproval_ContinuousPinsTheModeWithTheDecision`, `TestDecideApproval_HandlerFailureRollsTheModeBack`; contract thật trên Postgres 16 và MySQL 8: `RunAllPinsTheModeInTheDecisionsTransaction`, `RunAllRollsBackWithTheSubjectHandler`.
- [x] Quyết định không kèm chế độ hành xử y hệt trước (so sánh test hiện có, không sửa kỳ vọng). Bằng chứng: toàn bộ `TestApproval_*` cũ xanh không sửa kỳ vọng; thêm `TestDecideApproval_NoChoiceBehavesAsBefore`, `...NoChoiceReportsTheModeAlreadyPinned`, contract `ApproveWithoutModeLeavesTheModeUnpinned`.
- [x] Payload `approval.decided` có `execution_mode` đúng điều kiện; audit có metadata. Bằng chứng: `TestDecideApproval_*` kiểm khoá có/vắng; `TestAudit_EveryEventDerivedActionWritesExactlyOneEntry/approve_and_run_all_steps`; contract đọc hàng outbox thật.
- [x] Thiếu `Pinner` khi khởi động thì lỗi. Bằng chứng: `requireWired` duyệt mọi trường xuất khẩu của `DecideApproval` trừ `Log, Confirm, StartPrecheck`, nên `Pinner` nil làm khởi động lỗi `wiring: DecideApproval.Pinner is nil`; `go test ./cmd/...` xanh. `TestDecideApproval_ModeWithoutAPinnerIsAnExplicitError` giữ lỗi `REQUEST_EXECUTION_MODE_PINNER_MISSING` (Internal) cho đường dùng trực tiếp.

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `DecideApproval.Execute`, `approvalAudit`, `ApprovalServer.decide`; đây là đường duyệt của mọi Approval, blast radius lớn.
- Chưa từng gọi `PinExecutionModeIfUnset` trong transaction của quyết định: contract test ở task 05 phải chứng minh cuộn lại trên cả hai dialect.
- Thứ tự khoá giữ nguyên Request rồi Approval; không thêm khoá thứ ba.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

## Ghi chú triển khai (2026-10-20)

- Mã: `usecase/decide_approval.go` (Input/Result/`Pinner`, phân tích sớm, kiểm `ExecutionModeChoiceApplies` trước precheck, ghim sau stage mismatch và trước `a.Approve`, payload, kết quả đọc từ `after`), `usecase/decide_approval_execution_mode.go` (mới, `pinRequestedMode`), `usecase/audit_event_tap.go` (`auditFields.ExecutionMode`, `md["execution_mode"]`), `adapter/grpc/approval_server.go` (`decide(...)` thêm `executionMode`, `Approve` trả `ExecutionMode`, `Reject` truyền `""`), `cmd/server/wire_approval.go` (`Pinner: stores.executionModes`).
- Test mới: `usecase/decide_approval_execution_mode_test.go` (hợp lệ, không chọn, đã ghim `step_by_step`/`continuous`, handler lỗi cuộn lại, nhấp đúp, `Pinner == nil`, hai quyết định đồng thời, subject/stage sai, `reject` kèm chế độ, precheck thấy chế độ yêu cầu), thêm ca vào `audit_test.go`, `adapter/grpc/approval_server_test.go` (`Approve` chuyển `execution_mode` xuống use case).
- Lệnh: `go test ./internal/usecase/... -run 'DecideApproval|Audit|ExecutionMode'`, `go test ./internal/adapter/grpc/...`, `go test ./...` trong request-service: xanh. Contract: `go test -tags=integration -p 1 ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'Execution|Approval'` xanh (Postgres 23s, MySQL 62s).
- Chứng minh ghim cuộn lại cùng transaction trên hai DB: `RunAllRollsBackWithTheSubjectHandler` đọc lại cột `execution_mode` rỗng, Approval `pending` v1, Request ở `awaiting_plan_approval`, không có hàng outbox.
- Blast radius (grep): `DecideApproval.Execute` được gọi duy nhất từ `adapter/grpc/approval_server.go`; `DecideApprovalInput`/`Result` dùng trong `contracttest/approval_flow_contract.go`, `e2e/*` (tag `e2e`, `go vet -tags=e2e` xanh) và test usecase. `approvalAudit` chỉ đọc thêm một khoá, các nhánh `rejected/cancelled/expired` không đổi. `ApprovalServer.decide` đổi chữ ký nội bộ (hai chỗ gọi).
- Sai khác so với spec: thứ tự kiểm `NotApplicable` chạy cả khi Approval đã được quyết định (gọi lại), vì dựa vào `probe`; gọi lại một quyết định đã xong với `continuous` trả chế độ đang ghim, không ghim lại.
- Không kiểm chứng: chạy trên stack thật (WS, UI).
