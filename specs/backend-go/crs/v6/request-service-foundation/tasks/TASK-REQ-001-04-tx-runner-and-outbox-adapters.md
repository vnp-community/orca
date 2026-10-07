# TASK-REQ-001-04: `TxRunner`, `OutboxWriter` và `outbox.Store` cho hai dialect

**From Solution:** BE-REQ-SOL-001
**Priority:** P0
**Service:** `request-service`
**File:** `internal/domain/outbox_event.go`, `internal/domain/outbox_subjects.go`, `internal/usecase/ports.go`, `internal/adapter/postgres/{repository.go,tx.go,outbox.go}`, `internal/adapter/mysql/{repository.go,tx.go,outbox.go}` (tất cả mới); test tích hợp cùng thư mục
**Depends on:** TASK-REQ-001-01, TASK-REQ-001-03
**Status:** `[x] DONE`

---

## Context

`common/outbox.Store` có `FetchUnpublished(ctx, limit) ([]outbox.Record, error)` và `MarkPublished(ctx, ids []string) error`. `task-service/internal/adapter/{postgres,mysql}/outbox.go` đã cài hai hàm này (Postgres `id = ANY($1)`, MySQL dựng `IN (?,...)` và chặn `len(ids)==0`). Điểm khác cần làm: `task-service` ghi outbox **ngoài** giao dịch nghiệp vụ (ghi chú ở `postgres/outbox.go`), còn `request-service` bắt buộc ghi **cùng giao dịch** (L3 của feature request-lifecycle). Mẫu RLS và relay: `mcp-service/internal/adapter/postgres/tenant_tx.go` (`withTenantTx`, `withRelayTx`).

## Việc cần làm

1. `domain/outbox_event.go`: `type OutboxEvent struct { ID, TenantID, Subject string; OccurredAt time.Time; Version int; Payload []byte }`. `domain/outbox_subjects.go`: hằng subject cho các sự kiện sẽ dùng (ví dụ `SubjectRequestCreated = "orca.request.request.created"`, `SubjectRequestStatusChanged`, `SubjectRequestClassified`, `SubjectRequestTypeConfirmed`, `SubjectRequestTypeChanged`, `SubjectRequestReturned`, `SubjectRequestCompleted`); chỉ khai báo hằng, chưa phát.
2. `usecase/ports.go`: `TxRunner` (`InTx(ctx, fn func(context.Context) error) error`, tham gia giao dịch của ctx khi lồng nhau) và `OutboxWriter` (`InsertOutboxEvent(ctx, domain.OutboxEvent) error`). Thêm hàm `usecase.NewOutboxEvent(subject string, payload any) (domain.OutboxEvent, error)` sinh `ID=uuid.NewString()`, `OccurredAt=time.Now().UTC()`, `Version=1`, tenant từ ctx (`tenant.RequireTenantID`).
3. Postgres `tx.go`: khoá ctx riêng `txKey`, `InTx` như SOL-001 mục 2.D (`Begin`, `set_config('app.tenant_id', $1, true)`, commit, rollback bằng `defer`); `exec(ctx)` trả `pgx.Tx` của ctx hoặc, ngoài giao dịch, chạy từng câu trong một giao dịch ngắn có `set_config` (`withTenantTx`). `withRelayTx` cho `outbox.Store`.
4. Postgres `outbox.go`: `InsertOutboxEvent` (`INSERT INTO request.outbox_events (id, tenant_id, subject, occurred_at, version, payload) VALUES ($1,$2,$3,$4,$5,$6::jsonb)` qua `exec(ctx)`), `FetchUnpublished` (`ORDER BY created_at, seq LIMIT $1`, trong `withRelayTx`; `seq` để hai sự kiện cùng giao dịch giữ thứ tự chèn), `MarkPublished` (`UPDATE ... SET published_at = now() WHERE id = ANY($1)`, trong `withRelayTx`).
5. MySQL `tx.go` và `outbox.go`: `InTx` với `*sql.Tx` trong ctx, tham gia khi lồng; `InsertOutboxEvent` (`created_at` do DEFAULT lo, `seq` tự tăng); `FetchUnpublished` `ORDER BY created_at, seq`; `MarkPublished` dựng `IN (?,...)`, trả sớm khi `len(ids)==0`. Mọi `INSERT` mang `tenant_id`.
6. `repository.go` mỗi dialect: `type Repository struct`, `New(pool)`; chỉ khai báo hiện tại, repo nghiệp vụ thêm ở TASK-REQ-002-04, 002-05.
7. Compile-time assert: `var _ outbox.Store = (*Repository)(nil)`, `var _ usecase.TxRunner = ...`, `var _ usecase.OutboxWriter = ...` ở mỗi adapter.

## Kiểm thử

Tên test (build tag `integration`, chạy từng dialect):
- `TestInTx_RollbackLeavesNoOutboxRow`: ghi một dòng trong `InTx` rồi trả lỗi, không còn dòng.
- `TestInTx_NestedJoinsOuterTransaction`: `InTx` lồng ghi hai dòng, lỗi ở lớp ngoài thì cả hai biến mất.
- `TestFetchUnpublished_OrderedByCreatedAtThenSeq`, `TestOutbox_OrderPreservedWithinOneTransaction` (hai `InsertOutboxEvent` trong một `InTx`, `FetchUnpublished` trả đúng thứ tự chèn, lặp 50 lần), `TestMarkPublished_ExcludesFromFetch`, `TestMarkPublished_Empty`.
- `TestTwoRelaysConcurrentlyNoLoss`: hai `outbox.Relay` cùng store với publisher giả, mọi dòng được publish ít nhất một lần.
- Postgres: `TestRLS_TenantIsolation` với role `NOBYPASSRLS` (đặt tenant A, đọc bằng tenant B thấy 0 dòng); `TestRelayCanReadAcrossTenants`.
- MySQL: `TestQueriesFilterByTenant` (chèn hai tenant, `ListUnpublished` nội bộ chỉ tính theo cách repo lọc; kiểm bằng kiểm tra SQL hoặc dữ liệu).
Lệnh: `go test -tags=integration ./services/request-service/internal/adapter/postgres/... ./services/request-service/internal/adapter/mysql/... -v`.

## Tiêu chí hoàn thành

- [x] Giao dịch hủy thì không có dòng outbox; commit thì có (cả hai dialect).
- [x] `FetchUnpublished` đúng thứ tự; `MarkPublished` rỗng không lỗi.
- [x] Postgres: tenant A không đọc được dòng B bằng repository và bằng SQL trực tiếp.
- [x] Ba assert `var _` biên dịch.

## Rủi ro và lưu ý

- `InTx` lồng nhau: lỗi ở lớp trong không rollback sớm; chỉ lớp ngoài cùng commit hoặc rollback. Use case dùng retry CAS (CR-REQ-003) chỉ được thử lại khi nó là lớp ngoài cùng; cần hàm `txscope.Active(ctx)` hoặc tương đương, thêm ở TASK-REQ-003-04 nếu chưa có.
- `pgx` thực thi `set_config(..., true)` cục bộ giao dịch; không dùng `SET` thường (rò rỉ tenant giữa kết nối pool).
