# TASK-BE-MOBILE-003: `SubscriptionRepository.MarkExpired`

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service`
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09) — đã có sẵn từ trước, không cần code thêm

> **Kết quả thực tế:** Xác nhận bằng `grep` trực tiếp: `MarkExpired` đã tồn
> tại ĐẦY ĐỦ trong `SubscriptionRepository` (`ports.go`), implement thật ở
> `postgres/repository.go`, `fakeSubscriptionRepository.MarkExpired` đã có ở
> `subscribe_test.go`, và 3 test tích hợp thật (`TestRepository_MarkExpired_SetsStatusExpired`,
> `TestRepository_MarkExpired_UnknownEndpoint_NoError`,
> `TestRepository_MarkExpired_DoesNotAffectOtherEndpoints` — tên khác 1 chút so
> với sketch của task này nhưng cùng phạm vi coverage) đã PASS thật (chạy
> `-tags=integration` trên Postgres container thật) — **do bộ CR-NOTIF-001's
> [TASK-BE-NOTIF-009](../../notification/tasks/TASK-BE-NOTIF-009-webpush-sender-port-and-mark-expired.md)
> làm trước đó cùng ngày**, độc lập nhưng trùng đúng nhu cầu của CR-MOBILE-001
> (2 CR khác nhau cùng cần `MarkExpired` trên cùng 1 interface — không phải
> trùng lặp công việc, chỉ là 1 lần code phục vụ cả 2 CR).
>
> Không có file nào bị sửa thêm cho task này — verify-only.

---

## Mục tiêu

`postgres/repository.go`'s `Save` upsert luôn ép `status = 'active'` trên
mọi conflict (`ON CONFLICT (endpoint) DO UPDATE SET ... status = 'active'`)
— không thể dùng để đánh dấu 1 subscription hết hạn. Thêm 1 phương thức
repository riêng, không đụng `Save`, để `DeliverPush` (TASK-BE-MOBILE-007)
đánh dấu token APNs/FCM hết hạn theo đúng tiêu chí chấp nhận CR-MOBILE-001.

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/ports.go` (MODIFY — thêm method vào `SubscriptionRepository`)
2. `backend-go/services/notification-service/internal/adapter/postgres/repository.go` (MODIFY — implement)
3. `backend-go/services/notification-service/internal/adapter/postgres/repository_test.go` (MODIFY — thêm test case)
4. `backend-go/services/notification-service/internal/usecase/subscribe_test.go` (MODIFY — `fakeSubscriptionRepository` cần implement method mới để tiếp tục thoả `SubscriptionRepository` interface)

## `ports.go`

```go
type SubscriptionRepository interface {
	Save(ctx context.Context, sub domain.PushSubscription) error
	ListByUser(ctx context.Context, tenantID, userID string) ([]domain.PushSubscription, error)
	DeleteByEndpoint(ctx context.Context, endpoint string) error
	// MarkExpired transitions the subscription at endpoint to
	// SubscriptionExpired. Unlike Save's upsert (which always forces
	// status back to 'active' on conflict), this never resurrects a row —
	// a device token APNs/FCM reports dead stays dead until the client
	// re-subscribes via Subscribe (which upserts status='active' again).
	// A missing endpoint affects 0 rows and is NOT an error, same
	// idempotent-by-design contract as DeleteByEndpoint.
	MarkExpired(ctx context.Context, endpoint string) error
}
```

## `postgres/repository.go`

```go
func (r *Repository) MarkExpired(ctx context.Context, endpoint string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notification.push_subscriptions
		SET status = 'expired', updated_at = now()
		WHERE endpoint = $1
	`, endpoint)
	if err != nil {
		return fmt.Errorf("postgres: mark subscription expired: %w", err)
	}
	return nil
}
```

## `fakeSubscriptionRepository` (test double, `subscribe_test.go`) — thêm method

```go
func (f *fakeSubscriptionRepository) MarkExpired(ctx context.Context, endpoint string) error {
	for i := range f.saved {
		if f.saved[i].Endpoint == endpoint {
			f.saved[i].Status = domain.SubscriptionExpired
		}
	}
	return nil
}
```

(Không sửa gì khác trong `subscribe_test.go` — chỉ thêm method này để
`fakeSubscriptionRepository` tiếp tục thoả interface `SubscriptionRepository`
sau khi ports.go đổi.)

## Test cases cần cover

- `TestRepository_MarkExpired_TransitionsStatus` (integration,
  `postgres/repository_test.go` — insert 1 row `active` (mirror
  `TestRepository_SaveSubscription_UpsertsOnEndpoint`'s setup), gọi
  `MarkExpired`, xác nhận `ListByUser` không còn trả về nó vì `ListByUser`
  lọc `status = 'active'`)
- `TestRepository_MarkExpired_UnknownEndpointIsNoop` (0 rows affected
  không phải lỗi — mirror `DeleteByEndpoint`'s test đã có)
- `TestRepository_MarkExpired_DoesNotAffectOtherEndpoints`

## Verify

```bash
cd backend-go/services/notification-service && go build ./... && go test ./...
gofmt -l internal/usecase/ports.go internal/adapter/postgres/repository.go
```

`repository_test.go` dùng testcontainers-go thật (mirror
`TestRepository_SaveSubscription_UpsertsOnEndpoint`/
`TestRepository_ListByUser_FiltersByTenantAndUser` đã có trong file, không
viết fake DB) — theo đúng
`specs/backend-go/standards/testing-strategy.md`'s quy ước cho tầng
adapter/postgres.

## gitnexus

`impact({target: "SubscriptionRepository", direction: "upstream"})` trước
khi sửa interface — xác nhận đủ danh sách implementer
(`postgres.Repository`) và mọi test double (`fakeSubscriptionRepository`)
cần cập nhật, không bỏ sót file nào ngoài 4 file đã liệt kê ở trên.

---

## ⚠️ Regression + fix lại (2026-09-11)

**Trạng thái "✅ DONE (2026-09-09)" ở trên KHÔNG còn đúng với code hiện tại
tại thời điểm phát hiện.** Commit `41442c8e7` ("resolve unresolved merge
conflict markers... reconcile duplicate CR implementations", 2026-09-10) —
khi hợp nhất 2 nhánh cùng độc lập implement CR-MOBILE-001 — đã xoá mất
`MarkExpired` khỏi cả `SubscriptionRepository` (`ports.go`) và
`postgres/repository.go` (giữ nhánh có `DeviceIDFor` nhưng không mang theo
`MarkExpired` từ nhánh kia). Xác nhận bằng `git log -S "MarkExpired"`: xuất
hiện ở `f0cc07f1e` (task này), biến mất ở `41442c8e7`. Hệ quả thật: token
APNs/FCM/Web Push chết (410/`BadDeviceToken`/`UNREGISTERED`) không bao giờ
được đánh dấu hết hạn.

**Đã fix lại (2026-09-11)**, phạm vi rộng hơn bản gốc của task này vì bản
gốc chỉ thêm `MarkExpired` mà chưa có ai gọi nó từ đâu:

- `MarkExpired` thêm lại vào `SubscriptionRepository` (`ports.go`) +
  `postgres/repository.go` — đúng như thiết kế gốc của task này.
- Thêm mới (không có trong task gốc): sentinel `usecase.ErrDeviceTokenInvalid`
  + phân loại lỗi permanent-vs-transient **thật** ở cả 3 adapter
  (`apns`, `fcm`, `webpush`) — parse `reason`/`status` JSON theo đúng tài
  liệu Apple/FCM (không chỉ đoán theo status code) — và gọi `MarkExpired`
  từ `deliver_push.go`'s `deliverOne` khi phát hiện token chết. Task gốc
  chỉ thêm method vào repository, chưa có phần này — nếu không có,
  `MarkExpired` sẽ tồn tại nhưng không ai gọi, tái tạo đúng bug đang sửa.
- Sửa luôn 2 test tích hợp cũ trong `repository_test.go`
  (`TestRepository_SaveSubscription_UpsertsOnEndpoint`,
  `TestRepository_ListByUser_FiltersByTenantAndUser`) — phát hiện khi chạy
  `-tags=integration` lần đầu thật: dùng ID không phải UUID hợp lệ
  ("sub-1", "tenant-1"...) nên **chưa từng pass thật trước đó**, dù bị merge
  báo cáo là đã verify. Cùng dạng bug đã ghi nhận ở usage-service
  (CR-DB-002/003).
- **Verify thật đã chạy** (không chỉ đọc code): `go build`/`go vet` sạch
  cho toàn bộ 17 service backend-go; unit test `internal/usecase` + 3
  adapter package (`apns`/`fcm`/`webpush`) PASS; **3 integration test
  `TestRepository_MarkExpired_*` PASS trên Postgres thật** qua
  testcontainers-go.
- Chưa sửa (ngoài phạm vi CR-MOBILE): `TestRepository_GetPublicKey_NoActiveKeyReturnsDomainError`
  cùng file cũng dùng ID không phải UUID — thuộc `VapidKeyRepository`
  (F11/CR-NOTIF-001), không phải `SubscriptionRepository`.

## Blocking

TASK-BE-MOBILE-007 (`DeliverPush` usecase) phụ thuộc cứng — cần
`MarkExpired` tồn tại trong `SubscriptionRepository` trước khi gọi được từ
usecase mới.
