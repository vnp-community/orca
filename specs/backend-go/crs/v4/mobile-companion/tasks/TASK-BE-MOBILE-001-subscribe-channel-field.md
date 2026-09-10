# TASK-BE-MOBILE-001: Proto `SubscribeRequest.channel`/`device_label` + `Subscribe` usecase + gRPC handler

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service`
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Implement đúng sketch. `impact({target: "Subscribe"})` và
> `impact({target: "Server"})` trước khi sửa: cả 2 LOW risk, đúng số liệu CR đã
> khảo sát (Subscribe: 3 impacted; Server: 3 impacted), không có gì bất ngờ.
> `buf generate` chạy từ `backend-go/proto/` (không phải từ service — `buf.gen.yaml`
> nằm ở đó) — regenerate **toàn bộ proto tree dùng chung** (không chỉ
> `notification`), vì `.proto` của `auth`/`automation`/`infrafleet`/`workflow` đã
> bị agent khác sửa trước đó và chưa regen — xác nhận build sạch cho **cả 17
> service** trong `backend-go/services/*` sau khi regen (không chỉ
> `notification-service`), không phá code-gen của ai.
>
> `TestSubscribe_EmptyChannelDefaultsToWeb` thực ra đã được cover ngầm bởi
> `TestSubscribe_SavesWebSubscription` có sẵn — vẫn thêm test riêng tên đúng
> yêu cầu để rõ ràng ý định. Cả 4 test case yêu cầu + regression test cũ đều
> PASS.
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./...` PASS 100% toàn `notification-service`.

---

## Mục tiêu

`Subscribe` RPC hiện hard-code `domain.ChannelWeb` cho mọi request
(`internal/usecase/subscribe.go:50`) — mở proto + usecase để nhận
`channel`/`device_label`, giữ nguyên hành vi cho caller hiện tại
(`useWebPushSubscription.ts`, luôn gửi channel rỗng → vẫn ra `ChannelWeb`).

## Files cần sửa

1. `backend-go/proto/orca/notification/v1/notification.proto` (MODIFY — thêm 2 field vào `SubscribeRequest`)
2. `backend-go/services/notification-service/internal/usecase/subscribe.go` (MODIFY)
3. `backend-go/services/notification-service/internal/usecase/subscribe_test.go` (MODIFY — thêm test case)
4. `backend-go/services/notification-service/internal/adapter/grpc/server.go` (MODIFY — `Subscribe` handler truyền field mới)

## Nội dung proto

```protobuf
message SubscribeRequest {
  string user_id = 1;
  string endpoint = 2;
  string p256dh_key = 3;
  string auth_key = 4;
  string channel = 5;       // "web" (mặc định nếu rỗng) | "ios" | "android"
  string device_label = 6;  // optional, hiển thị UI "Paired Devices"
}
```

Chạy `buf generate` sau khi sửa — **kiểm tra không phá `proto/gen/go` dùng
chung với service khác** trước khi commit (`git diff --stat proto/gen/go/`
chỉ nên có `notification.pb.go`/`notification_grpc.pb.go` thay đổi).

## `subscribe.go` — thêm field, giữ default an toàn

```go
type SubscribeInput struct {
	UserID      string
	Endpoint    string
	P256dhKey   string
	AuthKey     string
	Channel     string // rỗng == "web", giữ hành vi cũ cho caller hiện tại
	DeviceLabel string
}

func (uc *Subscribe) Execute(ctx context.Context, in SubscribeInput) (domain.PushSubscription, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.PushSubscription{}, apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.UserID == "" {
		return domain.PushSubscription{}, apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_NO_USER", "user_id is required", nil)
	}

	channel := domain.ChannelWeb
	if in.Channel != "" {
		channel = domain.Channel(in.Channel)
	}

	sub, err := domain.NewPushSubscription(
		uuid.NewString(), tenantID, in.UserID, channel, in.Endpoint,
		&in.P256dhKey, &in.AuthKey, in.DeviceLabel, time.Now().UTC(),
	)
	if err != nil {
		return domain.PushSubscription{}, apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_INVALID_SUBSCRIPTION", err.Error(), err)
	}
	if err := uc.repo.Save(ctx, sub); err != nil {
		return domain.PushSubscription{}, apperrors.New(apperrors.KindInternal, "NOTIFICATION_SAVE_FAILED", "failed to persist push subscription", err)
	}
	return sub, nil
}
```

`domain.Channel.Valid()` (đã tồn tại, không sửa) tự chặn `channel` không
hợp lệ qua `ErrInvalidChannel` → không cần validate riêng ở usecase.

## `adapter/grpc/server.go` — truyền field mới

```go
func (s *Server) Subscribe(ctx context.Context, req *notificationv1.SubscribeRequest) (*notificationv1.SubscribeResponse, error) {
	sub, err := s.subscribe.Execute(ctx, usecase.SubscribeInput{
		UserID:      req.GetUserId(),
		Endpoint:    req.GetEndpoint(),
		P256dhKey:   req.GetP256DhKey(),
		AuthKey:     req.GetAuthKey(),
		Channel:     req.GetChannel(),
		DeviceLabel: req.GetDeviceLabel(),
	})
	// ... không đổi phần còn lại
}
```

## Test cases cần cover

- `TestSubscribe_EmptyChannelDefaultsToWeb` (regression guard — caller cũ
  không gửi `channel` vẫn phải ra `domain.ChannelWeb`, giữ nguyên hành vi
  trước task này)
- `TestSubscribe_IOSChannelWithoutWebKeysSucceeds` (channel=`ios` không
  cần `p256dh_key`/`auth_key` — `domain.NewPushSubscription`'s
  `ErrMissingWebKeys` chỉ áp dụng khi `channel == web`, đã có sẵn ở domain,
  test này xác nhận usecase không vô tình bắt buộc 2 field đó)
- `TestSubscribe_UnknownChannelReturnsInvalidArgument` (mirror
  `domain.ErrInvalidChannel` map đúng sang `apperrors.KindInvalidArgument`)
- `TestSubscribe_DeviceLabelPersisted`

## Verify

```bash
cd backend-go && buf generate
cd backend-go/services/notification-service && go build ./... && go test ./...
gofmt -l internal/usecase/subscribe.go internal/adapter/grpc/server.go
```

## gitnexus

`impact({target: "Subscribe", direction: "upstream"})` (package
`internal/usecase`, `notification-service`) trước khi sửa — CR-MOBILE-001's
khảo sát đã ghi nhận risk LOW/3 impacted (1 direct, module `Usecase`), xác
nhận lại con số này chưa đổi trước khi commit (không tin lại số cũ, chạy
mới). Cũng chạy `impact({target: "Server", direction: "upstream"})` (package
`adapter/grpc`) trước khi sửa constructor/handler — xác nhận không phá
caller nào khác của `Server.New(...)`/`Server.Subscribe`.

## Blocking

TASK-BE-MOBILE-002 (`api-gateway`'s `notification_routes.go` passthrough)
cần `notificationv1.SubscribeRequest` đã có field `Channel`/`DeviceLabel`
(qua `buf generate`) trước khi build được. TASK-BE-MOBILE-007
(`DeliverPush` usecase) không phụ thuộc trực tiếp task này, nhưng
BE-MOBILE-SOL-002 (route mobile mới) phụ thuộc cứng.
