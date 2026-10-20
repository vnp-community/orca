# TASK-REQ-043-02: `DecideApproval` ghim chế độ trong cùng transaction, payload và audit

**From Solution:** [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/{decide_approval,decide_approval_execution_mode,audit_event_tap}.go` (sửa/mới), `internal/adapter/grpc/approval_server.go` (sửa), `cmd/server/wire_approval.go` (sửa), test tương ứng
**Depends on:** TASK-REQ-043-01
**Status:** [ ] TODO

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

- [ ] Duyệt Plan kèm `continuous` ghim chế độ và chuyển Request sang `executing` trong một transaction; handler lỗi để lại cột `NULL`.
- [ ] Quyết định không kèm chế độ hành xử y hệt trước (so sánh test hiện có, không sửa kỳ vọng).
- [ ] Payload `approval.decided` có `execution_mode` đúng điều kiện; audit có metadata.
- [ ] Thiếu `Pinner` khi khởi động thì lỗi.

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
