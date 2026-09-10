# TASK-BE-NOTIF-009: `WebPushSender` port + `SubscriptionRepository.MarkExpired`

**Solution:** BE-NOTIF-SOL-002 §2.A | **CR:** CR-NOTIF-002 (mục C, Changes Required)
**Service:** `notification-service`
**Depends on:** Không (độc lập với Track 1)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `WebPushSender` interface + `MarkExpired` thêm vào
> `SubscriptionRepository` đúng sketch. `impact({target: "SubscriptionRepository",
> direction: "upstream"})` trước khi sửa: LOW risk, 3 file import (`main.go`,
> `adapter/grpc/server.go`, `adapter/eventbus/consumer.go`) — không có gì bất ngờ.
> `go vet ./...` bắt đúng 1 implementation cần cập nhật: `fakeSubscriptionRepository`
> (`internal/usecase/subscribe_test.go`, dùng chung cho `subscribe_test.go` VÀ
> `unregister_push_subscription_test.go`) — thêm `MarkExpired` vào đó.
>
> **Phát hiện bug production có sẵn (giống bug F26 tìm thấy ở `usage-service`,
> không liên quan task này):** `push_subscriptions.{id,tenant_id,user_id}` là cột
> `UUID` thật (migration `0001_init.up.sql`), nhưng file test tích hợp CÓ SẴN TỪ
> TRƯỚC (`TestRepository_SaveSubscription_UpsertsOnEndpoint`,
> `TestRepository_ListByUser_FiltersByTenantAndUser`) dùng chuỗi thường
> (`"sub-1"`, `"tenant-1"`) — fail thật với `invalid input syntax for type uuid`
> khi chạy `-tags=integration` trên Postgres thật (chưa từng bị bắt vì
> backend-go không có CI chạy integration test). Đây là bug có sẵn, KHÔNG sửa
> (ngoài phạm vi task) — 3 test MỚI của task này (`TestRepository_MarkExpired_*`)
> dùng UUID literal hợp lệ để tự né bug, có ghi chú giải thích trong code.
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch.
> `go test ./...` (unit) PASS 100%. `go test -tags=integration
> ./internal/adapter/postgres/... -run TestRepository_MarkExpired -v` chạy
> testcontainers Postgres thật: 3/3 PASS khi chạy ổn định; gặp lại đúng loại
> flake tiền tồn tại "database system is starting up" (race khởi động
> container, không liên quan SQL) ở 1-2 lần chạy — retry luôn PASS cả 3,
> đúng loại flake đã ghi nhận ở CR-DB/CR-FLEET.

## Mục tiêu

Định nghĩa port `WebPushSender`; thêm `MarkExpired(ctx, endpoint) error` vào `SubscriptionRepository` (interface + implementation Postgres) — vòng phản hồi khi 1 Web Push endpoint trả `410 Gone`/`404`.

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/ports.go` (MODIFY)
2. `backend-go/services/notification-service/internal/adapter/postgres/repository.go` (MODIFY)

## `ports.go` — `WebPushSender` + mở rộng `SubscriptionRepository`

```go
// WebPushSender sends one already-encrypted Web Push message to one
// subscription's endpoint. Implemented by internal/adapter/external/webpush
// (TASK-BE-NOTIF-010) — this port only knows "send bytes, get back whether
// the endpoint is dead", not the RFC 8291/8292 mechanics.
type WebPushSender interface {
	// Send POSTs payload (already RFC-8291-encrypted) to sub.Endpoint with
	// vapidAuthHeader as the Authorization header. expired=true means the
	// push service returned 404/410 — the endpoint is gone, the caller
	// must not retry it and should call SubscriptionRepository.MarkExpired.
	// Any other non-2xx status or transport error is returned as err
	// (expired=false) — a transient failure, not "this subscription is dead".
	Send(ctx context.Context, sub domain.PushSubscription, vapidAuthHeader string, payload []byte) (expired bool, err error)
}
```

```go
type SubscriptionRepository interface {
	Save(ctx context.Context, sub domain.PushSubscription) error
	ListByUser(ctx context.Context, tenantID, userID string) ([]domain.PushSubscription, error)
	DeleteByEndpoint(ctx context.Context, endpoint string) error
	// MarkExpired sets status='expired' for endpoint — called when a Web
	// Push send returns 404/410, so a future event doesn't retry a dead
	// endpoint. Idempotent: marking an already-expired/nonexistent endpoint
	// affects 0 rows and is NOT an error, same rule as DeleteByEndpoint.
	MarkExpired(ctx context.Context, endpoint string) error
}
```

## `adapter/postgres/repository.go` — implementation

```go
func (r *Repository) MarkExpired(ctx context.Context, endpoint string) error {
	_, err := r.pool.Exec(ctx, `UPDATE notification.push_subscriptions SET status = 'expired', updated_at = now() WHERE endpoint = $1`, endpoint)
	if err != nil {
		return fmt.Errorf("postgres: mark push subscription expired: %w", err)
	}
	return nil
}
```

## Test cases cần cover

- `TestRepository_MarkExpired_SetsStatusExpired` — verify bằng đọc lại row qua `ListByUser` (không còn xuất hiện vì `ListByUser` lọc `status = 'active'`).
- `TestRepository_MarkExpired_UnknownEndpoint_NoError` — endpoint không tồn tại → không lỗi (idempotent).
- `TestRepository_MarkExpired_DoesNotAffectOtherEndpoints` — 2 subscription khác endpoint, mark 1 cái không ảnh hưởng cái kia.

## Verify

```bash
cd backend-go/services/notification-service
go build ./...
go test ./internal/adapter/postgres/... -run TestRepository_MarkExpired
gofmt -l internal/usecase/ports.go internal/adapter/postgres/repository.go
```

## gitnexus

`impact({target: "SubscriptionRepository", direction: "upstream"})` trước khi thêm method vào interface — xác nhận MỌI implementation hiện có của interface này (chỉ `postgres.Repository` + fake test double trong `*_test.go`) đều được cập nhật, nếu không Go compiler sẽ tự bắt lỗi thiếu method (interface satisfaction) nhưng vẫn xác nhận trước để biết đúng số lượng file cần sửa, tránh bỏ sót 1 fake trong test.

## Blocking

TASK-BE-NOTIF-010 (`webpush` adapter) phụ thuộc `WebPushSender` interface đã tồn tại để implement đúng chữ ký. TASK-BE-NOTIF-011 (`DeliverPush` usecase) phụ thuộc `MarkExpired` đã tồn tại trên `SubscriptionRepository`.
