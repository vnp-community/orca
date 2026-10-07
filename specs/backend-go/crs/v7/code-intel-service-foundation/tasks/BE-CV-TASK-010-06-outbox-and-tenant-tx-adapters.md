# BE-CV-TASK-010-06: Adapter Postgres/MySQL: `withTenantTx`, `withRelayTx`, `outbox.Store`, `insertOutboxEvents`, test AST

**From Solution:** BE-CV-SOL-010-scaffold-code-intel-service
**Priority:** P0
**Service:** `code-intel-service`
**File:** `internal/domain/outbox_record.go`, `internal/domain/outbox_subjects.go`, `internal/usecase/ports.go`, `internal/adapter/postgres/{repository.go,tenant_tx.go,outbox.go,tenant_scope_guard_test.go}`, `internal/adapter/mysql/{repository.go,outbox.go,tenant_scope_guard_test.go}` (đều mới, tiền tố `backend-go/services/code-intel-service/`)
**Depends on:** BE-CV-TASK-010-05
**Status:** [x] DONE

---

## Context

`common/outbox.Store` có `FetchUnpublished(ctx, limit) ([]outbox.Record, error)` và `MarkPublished(ctx, ids []string) error` (`common/outbox/outbox.go`). Mẫu: `mcp-service/internal/adapter/postgres/{tenant_tx.go,repository.go,tenant_scope_guard_test.go}` và `mcp-service/internal/domain/outbox.go`. SOL-011 sẽ thêm nhiều file repository, nên test AST phải quét theo glob chứ không chỉ `repository.go` như bản `mcp-service`.

## Việc cần làm

1. `domain/outbox_record.go`: `OutboxRecord{ID, Subject string; OccurredAt time.Time; Version int; PayloadJSON []byte}`. `domain/outbox_subjects.go`: hằng `SubjectIndexChanged = "orca.codeintel.index.changed"`, `SubjectReindexStarted`, `SubjectReindexFinished`, `SubjectReviewSaved` (bốn subject, hợp đồng §5; ba subject còn lại do CR chủ sở hữu thêm).
2. `usecase/ports.go`: cổng `OutboxStore` (mô tả hợp đồng ghi outbox; phương thức cụ thể do SOL-011 thêm theo từng repository) và `Clock` (`Now() time.Time`) cho test.
3. Postgres `tenant_tx.go`: `withTenantTx(ctx, tenantID, fn func(pgx.Tx) error)` (`SELECT set_config('app.tenant_id', $1, true)`), `withRelayTx` (`set_config('app.relay','on',true)`), `withTx` (begin/rollback/commit). `outbox.go`: `FetchUnpublished` (`ORDER BY created_at LIMIT $1`, trong `withRelayTx`), `MarkPublished` (`UPDATE … SET published_at = now() WHERE id = ANY($1::uuid[])`), và `insertOutboxEvents(ctx, tx, tenantID, events)` (hàm nội bộ cho mọi phương thức ghi). Mọi truy vấn có tiền tố schema `codeintel.`.
4. MySQL `outbox.go`: cùng ba hàm dùng `*sql.DB`/`*sql.Tx`; `FetchUnpublished` `WHERE published_at IS NULL ORDER BY created_at LIMIT ?`; `MarkPublished` dùng placeholder động `IN (?,?…)` (≤ `BatchSize` 100 id). Hai phương thức relay xuyên tenant **có chủ ý**; ghi chú. `insertOutboxEvents` nhận `tenantID` từ `tenant.RequireTenantID`.
5. `repository.go` mỗi dialect: `type Repository struct{ pool/db }`, `New(...)`, kiểm `var _ outbox.Store = (*Repository)(nil)`.
6. Test AST: duyệt mọi file khớp `repository.go` và `*_repository.go` trong package; mọi phương thức xuất của `Repository` (trừ `FetchUnpublished`, `MarkPublished` ghi chú ngoại lệ có `withRelayTx`) phải gọi `withTenantTx`/`withRelayTx`/`withMaintenanceTx` (Postgres) hoặc `tenant.RequireTenantID` (MySQL). Test phải đỏ khi thêm một phương thức mẫu thiếu phạm vi (viết một case âm trong test bằng cách parse chuỗi nguồn).

## Kiểm thử

- Unit: `go test ./services/code-intel-service/internal/adapter/...` (AST guard).
- Integration hai dialect: `InsertOutboxEvent` trong transaction rồi rollback → không còn dòng; `FetchUnpublished` theo `created_at`; `MarkPublished`; hai relay giả đồng thời không mất sự kiện (giao lặp chấp nhận); Postgres role `NOBYPASSRLS`: `FetchUnpublished` ngoài `withRelayTx` trả rỗng.

## Tiêu chí hoàn thành

- [x] Sự kiện ghi cùng transaction; rollback không để lại dòng.
- [x] RLS: tenant A không thấy/ghi dòng tenant B; chỉ relay đọc chéo tenant, không `INSERT`.
- [x] Test AST bắt phương thức thiếu phạm vi, quét mọi file repository.

## Rủi ro và lưu ý

- `ORDER BY created_at` không ổn định khi hai sự kiện cùng transaction (Q2 SOL-010).
- Không bắt chước `task-service` (không `set_config`).
