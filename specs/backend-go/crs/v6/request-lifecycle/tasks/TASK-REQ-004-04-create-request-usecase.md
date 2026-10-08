# TASK-REQ-004-04: Use case `CreateRequest` (idempotent, làm giàu ngoài giao dịch, vào `classifying`)

**From Solution:** BE-REQ-SOL-004
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/create_request.go`, `internal/usecase/create_request_test.go`, `internal/usecase/ports.go` (sửa: `IssueFetcher`) (mới/sửa)
**Depends on:** TASK-REQ-003-03, TASK-REQ-004-01, TASK-REQ-004-02
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -race ./internal/domain/... ./internal/usecase/... ./internal/adapter/... (unit) và go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql -run "Intake|Classification|Migration|Schema" (DB thật)`)

---

## Context

`RequestIdempotencyRepository.Find/Claim`, `RequestRepository.NextNumber/Create`, `TxRunner.InTx` (tham gia lồng), `TransitionRequest.Execute` (lồng được, không retry khi `InTransaction`) đều có từ các task trước. Thứ tự sự kiện `created` rồi `status_changed` phụ thuộc cột `seq` của outbox (TASK-REQ-001-03). `task-service/usecase/create_task_from_source.go` là mẫu về "Find trước, Claim sau, thua race thì đọc lại bên thắng".

## Việc cần làm

1. `ports.go`: `type IssueFetcher interface { GetIssue(ctx context.Context, provider domain.SourceProvider, ref, site string) (IssueSnapshot, error) }` và `IssueSnapshot{Title, Body, URL string; Hints domain.SourceHints}`. Lỗi sentinel `usecase.ErrIssueNotFound` để phân biệt `REQUEST_SOURCE_NOT_FOUND` với `REQUEST_SOURCE_FETCH_FAILED`.
2. `CreateRequest` struct (`repo`, `idem`, `tx`, `outbox`, `transition *TransitionRequest`, `issues IssueFetcher`), `NewCreateRequest`.
3. `Execute(ctx, in)`: (a) tenant, `reporter` (`tenant.UserID`; webhook cũng đi đường này vì gateway gắn `x-orca-user-id`), `project_id`; (b) `NormalizeSourceRef`, `NormalizeTitle` hoãn tới sau làm giàu nếu `title` rỗng và có thể làm giàu; (c) `BuildIdempotencyKey`; `ok` thì `idem.Find`: có thì `repo.Get` và trả `Created=false`; (d) làm giàu ngoài giao dịch khi (`jira` hoặc `linear`) và `ref != ""` và `title == ""`: `issues.GetIssue`, điền `Title`, `Body`, `URL`, `Hints`; lỗi `ErrIssueNotFound` thì `REQUEST_SOURCE_NOT_FOUND`; lỗi khác thì `REQUEST_SOURCE_FETCH_FAILED`; khi `title` đã có, lỗi làm giàu chỉ `slog.Warn`; (e) `NormalizeTitle`/`NormalizeBody` cuối cùng (`github`/`gitlab` thiếu title thì `REQUEST_TITLE_REQUIRED`, không gọi service ngoài); (f) `tx.InTx`: `Claim` (nếu `ok`; `claimed=false` thì đọc Request của bên thắng, trả lỗi nội bộ để rollback rồi bắt bên ngoài và trả `Created=false`) → `NextNumber` → `domain.NewRequest` (`source_*` đã chuẩn hoá, `source_hints`) → `repo.Create` → `transition.Execute(start_classification, ActorKind=system hoặc user)` → `InsertOutboxEvent` `orca.request.request.created`.
4. Lõi `CreateWithinTx(ctx, in)` (bước f, không gồm Find và làm giàu) export để TASK-REQ-006-04 dùng.
5. Bỏ qua `hints.type_hint` do `CreateRequest` công khai (đặt rỗng) trừ khi `in.AllowTypeHint` (chỉ SOL-006 đặt).
6. Payload `created`: `{request_id, project_id, number, source_provider, source_site, source_ref, reporter_id, title}` (thêm `parent_request_id`, `link_reason` ở TASK-REQ-006-04).
7. Không để lại `request_idempotency` mồ côi: mọi bước ghi nằm trong cùng `InTx`.

## Kiểm thử

Unit với repo, outbox, tx, `IssueFetcher` giả: `TestCreate_Manual_NoClientID_AlwaysNew`, `TestCreate_Manual_WithClientID_Idempotent`, `TestCreate_Jira_EnrichesTitle`, `TestCreate_Jira_EnrichFailsButTitlePresent_StillCreates`, `TestCreate_Jira_NotFound`, `TestCreate_GitHub_MissingTitle_NoExternalCall`, `TestCreate_ClaimLoser_ReturnsWinner`, `TestCreate_ClosedRequestReturnedAsIs` (Request `cancelled`: trả bản cũ, trạng thái giữ nguyên), `TestCreate_TypeHintIgnoredForPublicCall`, `TestCreate_EmitsCreatedThenStatusChanged`.
Lệnh: `go test ./services/request-service/internal/usecase/... -run CreateRequest`.

## Tiêu chí hoàn thành

- [x] Sau tạo, `status = classifying`, đúng hai sự kiện theo thứ tự `created`, `status_changed`.
- [x] `title` rỗng + `github` không gọi service ngoài.
- [x] Lỗi giữa chừng không để lại khoá hay đốt số (kiểm tích hợp ở TASK-REQ-004-07).
- [x] `CreateWithinTx` export, có test.

## Rủi ro và lưu ý

- Làm giàu ngoài giao dịch nghĩa là hai request đồng thời cùng khoá có thể cùng gọi `GetIssue`; chấp nhận (chỉ tốn một lần gọi).
- Nếu Request tồn tại nhưng `Get` thất bại ngay sau `Find` (bị xoá): không xảy ra vì không có lệnh xoá; coi là lỗi nội bộ.

## Ghi chú triển khai

- `CreateRequest` nhận cổng `RequestTransitioner` (định nghĩa ở `ports_classification.go`). Bản thật là `TransitionRequest` của agent lifecycle-a; ở nhánh này test dùng fake/`dbTransitioner`, `cmd/server/wire_transitioner_pending.go` trả `Unavailable` cho tới khi hợp nhất (xem IMPLEMENTATION-NOTES).
- Thứ tự sự kiện: `created` được ghi TRƯỚC khi gọi transition để `seq` cho đúng `created`, `status_changed` (tiêu chí của task); bước 3(f) viết ngược thứ tự.
- `RequestCreationRecorder` (no-op) là điểm móc cho revision 1 của CR-REQ-027 trong cùng giao dịch; bảng `request_revisions` chưa có nên chưa ghi revision.
- Làm giàu chạy khi còn thiếu title/body/hints (không chỉ khi title rỗng); lỗi chỉ gây thất bại nếu title vẫn rỗng.
- Thêm `REQUEST_PROJECT_INVALID` (project_id không phải UUID).
