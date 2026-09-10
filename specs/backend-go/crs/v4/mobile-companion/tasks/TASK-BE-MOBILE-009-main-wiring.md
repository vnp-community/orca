# TASK-BE-MOBILE-009: `cmd/server/main.go` — wire `DeliverPush`/`PushSender`/`PushCredentialResolver`

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service`
**Depends on:** TASK-BE-MOBILE-004, TASK-BE-MOBILE-005, TASK-BE-MOBILE-006, TASK-BE-MOBILE-007, TASK-BE-MOBILE-008
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Implement đúng sketch, đổi tên theo TASK-007
> (`NewDeliverMobilePush`, không phải `NewDeliverPush` — tên đó đã dùng cho
> Web Push). `NewHandleIncomingEvent` cập nhật đủ 6 tham số thật (khác 4
> tham số trong sketch — sketch không biết `notifications`/`deliverPush`
> Web Push đã tồn tại từ trước). `pushCredentials` tái dùng đúng `brokerConn`
> có sẵn (không dial mới) theo đúng yêu cầu. `golang.org/x/net/http2` build
> sạch ngay, không cần sửa `go.mod` (subpackage của module `golang.org/x/net`
> đã có sẵn dạng indirect).
>
> `impact({target: "run", direction: "downstream"})` như task ghi: trả về
> CRITICAL/50 impacted — nhưng đây chỉ là cây phụ thuộc tự nhiên của 1
> composition root (mọi thứ `run()` gọi tới), không phải tín hiệu rủi ro thật
> cho việc "thêm tham số mới". Chạy thêm `impact({target: "run", direction:
> "upstream"})` (câu hỏi task thực sự quan tâm — "có caller nào khác ngoài
> `main()` không"): **LOW risk, đúng 1 caller** (`main()`) — an toàn.
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./...` PASS 100% toàn `notification-service`. Build lại **toàn bộ 17
> service** trong `backend-go/services/*` từ workspace gốc — sạch, không service
> nào bị phá bởi thay đổi chung (`golang.org/x/oauth2`, `cloud.google.com/go/compute/metadata`
> thêm vào `go.mod`).

---

## Mục tiêu

Composition root cuối cùng — nối toàn bộ symbol mới của
BE-MOBILE-SOL-001 vào `main.go`, tái sử dụng đúng `brokerConn` đã dial sẵn
cho `vaultsigner.New(...)` (§1.1 của BE-MOBILE-SOL-001 — KHÔNG dial kết nối
thứ hai tới `credential-broker-service`).

## Files cần sửa

1. `backend-go/services/notification-service/cmd/server/main.go` (MODIFY)

## Thay đổi cụ thể

```go
// Sau dòng "signer := notificationvaultsigner.New(brokerConn)" đã có:
pushCredentials := notificationcredentialbroker.New(brokerConn) // reuse — không dial connection mới

// http2-enabled client dùng chung cho cả APNs và FCM sender (thư viện
// thêm ở TASK-BE-MOBILE-005/006 — golang.org/x/net/http2).
pushHTTPClient := &http.Client{
	Transport: &http2.Transport{},
	Timeout:   10 * time.Second,
}

apnsSender := notificationpushgateway.NewAPNsSender(pushCredentials, pushHTTPClient, "com.stably.orca.mobile")
fcmSender := notificationpushgateway.NewFCMSender(pushCredentials, pushHTTPClient)

deliverPushUC := usecase.NewDeliverPush(repo, map[domain.Channel]usecase.PushSender{
	domain.ChannelIOS:     apnsSender,
	domain.ChannelAndroid: fcmSender,
}, logger)

// handleIncomingEventUC — đổi lời gọi hiện có, thêm tham số deliverPushUC:
handleIncomingEventUC := usecase.NewHandleIncomingEvent(broadcast, repo, deliverPushUC, logger)
```

Import thêm:
`notificationcredentialbroker "github.com/stablyai/orca-go/services/notification-service/internal/adapter/credentialbroker"`,
`notificationpushgateway "github.com/stablyai/orca-go/services/notification-service/internal/adapter/pushgateway"`,
`"github.com/stablyai/orca-go/services/notification-service/internal/domain"`,
`"golang.org/x/net/http2"`.

**Không sửa** `notificationgrpc.New(...)`'s lời gọi — `Server`'s `signer`
field (VAPID) không liên quan tới `DeliverPush`, giữ nguyên chữ ký hiện
tại.

## Test cases cần cover

Không có unit test riêng cho `main.go` (theo convention hiện có của service
này — `main_test.go` không tồn tại). Việc "test" cho task này LÀ build
sạch + tất cả test package khác vẫn pass sau khi đổi composition root.

## Verify

```bash
cd backend-go/services/notification-service && go build ./... && go vet ./... && go test ./...
gofmt -l cmd/server/main.go
```

Chạy thêm từ workspace gốc để xác nhận không phá service khác dùng chung
`proto/gen/go`/`go.work.sum`:

```bash
cd backend-go && go build ./...
```

## gitnexus

`impact({target: "run", direction: "downstream"})` (package `main`,
`notification-service`) trước khi sửa — xác nhận composition root này
không có caller nào khác ngoài `main()` (rủi ro thấp, nhưng đây là điểm
cuối nối MỌI symbol mới của solution, sai 1 chữ ký làm cả service không
build được).

## Blocking

Không task nào trong BE-MOBILE-SOL-001 phụ thuộc ngược task này — đây là
task cuối của solution. BE-MOBILE-SOL-002 (route mobile mới, service khác
— `api-gateway`) không phụ thuộc trực tiếp vào `main.go` này.
