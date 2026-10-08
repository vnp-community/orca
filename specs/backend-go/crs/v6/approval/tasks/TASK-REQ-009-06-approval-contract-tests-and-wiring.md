# TASK-REQ-009-06: Bộ test hợp đồng `SubjectHandler` và wiring ở `main.go`

**From Solution:** [BE-REQ-SOL-009](../solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md) mục F và 5
**Priority:** P0
**Service/Area:** `request-service` / cmd, test hợp đồng
**File:** `backend-go/services/request-service/cmd/server/main.go` (sửa), `internal/usecase/approval_subject_handler_contract_test.go` (mới), `internal/adapter/postgres/approval_flow_integration_test.go` (mới), `internal/adapter/mysql/approval_flow_integration_test.go` (mới)
**Depends on:** TASK-REQ-009-05; CR-REQ-003
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./...` và `go test -tags integration ./internal/adapter/... ./cmd/...` với Postgres 16 và MySQL 8.0 thật)

## Context

- `main.go` là composition root duy nhất (mẫu `usage-service/cmd/server/main.go`, `mcp-service`). Task này thêm khối lắp ráp Approval vào đó.
- Rủi ro lớn nhất của solution: lệch hợp đồng `SubjectHandler` giữa 005, 007, 008, 012, 013, 014. Bộ test hợp đồng dùng chung cho mọi handler để các CR đó chạy lại.

## Việc cần làm

1. `main.go`: dựng repository theo dialect (`dbcapability.DetectDialectFromDSN`), use case, server; đăng ký `NoopSubjectHandler` cho subject chưa có handler; sau lắp ráp gọi `registry.MustCoverAll()` và thoát lỗi nếu thiếu. Nếu có handler Noop và `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS` không bật thì thoát lỗi.
2. Thêm hai biến cấu hình vào `internal/config`: `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS` (bool, mặc định false). Đăng ký `ApprovalService` trên gRPC server.
3. `approval_subject_handler_contract_test.go`: hàm xuất `RunSubjectHandlerContract(t, factory)` kiểm bất biến: `ValidateForRequest` ổn định (cùng đầu vào cùng digest), `OnApproved` chỉ ghi DB của service, không gọi mạng (fake mạng ném lỗi khi bị gọi), `OnClosedWithoutDecision` idempotent. Handler của các CR sau gọi hàm này.
4. Tích hợp hai DB: luồng `OpenApproval` -> `Approve` với handler giả ghi dòng thử trong cùng transaction + outbox; ép handler lỗi thì không còn dòng thử và Approval vẫn `pending`; `CancelPendingForRequest` đua với `Approve` (hai goroutine) không khoá chết và đúng một kết quả.
5. Test dòng golden: payload `approval.requested` và `approval.decided` khớp tệp `testdata/approval_events/*.json` (mới), không có khoá `comment`.

## Kiểm thử

- `go test ./cmd/... ./internal/usecase/... -run "Contract|Wiring"`.
- Tích hợp: `go test ./internal/adapter/... -run ApprovalFlow` với Postgres và MySQL.
- Test khởi động: thiếu handler thì `run()` trả lỗi (không `os.Exit` trong test; tách hàm `buildApprovalModule`).

## Tiêu chí hoàn thành

- [x] Service không khởi động khi thiếu handler hoặc có Noop mà chưa bật cờ.
- [ ] `RunSubjectHandlerContract` được các handler của CR 005, 007, 008 gọi (ghi vào checklist của các solution đó). Chưa: các CR đó chưa có handler riêng; hợp đồng đã sẵn và đã chạy cho handler của task này.
- [x] Không khoá chết ở test đua trên cả hai DB (chạy 50 lần).
- [x] Golden payload không chứa `comment`.

## Rủi ro và lưu ý

- Test đua cần giới hạn thời gian (`context.WithTimeout` 10s) để phát hiện khoá chết.
- Chưa kiểm chứng: hành vi `FOR UPDATE` dưới tải thật.

## Kết quả triển khai (2026-10-08)
- Wiring thật ở `cmd/server/wire_approval.go` (một lời gọi từ `main.go`): dựng use case, đăng ký 8 handler thật vào registry dùng chung với lifecycle, `ApprovalService` và `ApprovalPolicyAdminService` bằng handler thật, bộ quét expire/remind chạy theo `REQUEST_APPROVAL_SWEEP_INTERVAL`, `requireWired` làm service không lên nếu thiếu phụ thuộc. `REQUEST_APPROVAL_ENABLED` nay mặc định bật; `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS` giữ cho dev (service không lên nếu thiếu handler thật mà không bật cờ: test `FillApprovalRegistry_...`, `BuildApprovalRegistry_...`).
- `RunSubjectHandlerContract` nay là hàm xuất trong `adapter/contracttest` (không còn thân rỗng trong `_test.go`): chạy cho `TransitionSubjectHandler` (7 chủ thể) và `RequestTypeApprovalHandler`. Hợp đồng không chứng minh được "chỉ ghi DB của service, không gọi mạng"; handler có hook ngoại vi phải được duyệt tay.
- `RunApprovalFlowContract` (16 kịch bản) chạy trên cả hai DB: handler giả ghi dòng thử outbox cùng giao dịch (lỗi sau đó thì không còn dòng thử, Approval vẫn `pending`), đua `CancelPending` với `Approve` 50 lần có timeout 10 giây, đua expire với approve, 4 admin duyệt đồng thời chỉ một thắng, golden payload (`domain/testdata/approval_events/*.json`, không có `comment`).
- Tiêu chí chưa tick: `RunSubjectHandlerContract` được CR 005/007/008 gọi — các CR đó chưa triển khai handler riêng; ghi vào checklist khi chúng làm. Các chủ thể solution, findings, answer, plan, phase, task_list, pre_deploy đã có handler đúng hợp đồng và được đăng ký, nhưng `SubjectArtifacts` chưa gắn dịch vụ Solution/Plan thật (hàm `approvalSubjectArtifacts()` trả rỗng): mở approval các chủ thể này trả `REQUEST_APPROVAL_SUBJECT_UNAVAILABLE` cho tới khi CR-REQ-007/008/012/013/014 gắn cổng.
