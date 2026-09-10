# TASK-BE-MOBILE-005: `PushSender` port + `pushgateway.APNsSender`

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service`
**Depends on:** TASK-BE-MOBILE-004 (`PushCredentialResolver`)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Implement đúng sketch. `golang.org/x/net/http2` **không
> cần** — `APNsSender` chỉ nhận `*http.Client` đã dựng sẵn từ caller (đúng
> thiết kế sketch), không tự import `http2` trong file này — việc đó thuộc
> `main.go` (TASK-BE-MOBILE-009). Thêm `apnsBaseURL` field (không có trong
> sketch) để test được với `httptest.Server` — production luôn dùng
> `apnsProductionBaseURL` (hardcode, unexported field, không route nào ngoài
> `NewAPNsSender` set được giá trị khác).
>
> Thêm `internal/adapter/pushgateway/jws.go` (`derECDSASignatureToRawJWS`) —
> **cố ý duplicate**, không import lại `internal/usecase`'s helper cùng tên
> (khác package, unexported, và về kiến trúc đây là crypto cục bộ của
> adapter, không phải usecase concern — khác VAPID's trường hợp, ở đây
> không có Vault Transit call nào, ký trực tiếp bằng private key PEM đã
> resolve). Sẽ dùng lại cho FCM (TASK-006) nếu cần.
>
> `impact({target: "PushSender", direction: "downstream"})` trả "not found"
> (symbol mới, index chưa bắt kịp) — xác nhận thủ công bằng grep: đúng 1
> implementer (`APNsSender`), không có consumer/tên trùng bất ngờ.
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./internal/adapter/pushgateway/... -v` — **5/5 PASS**, gồm
> `TestSignAPNsProviderToken_ProducesValidES256JWT` verify bằng `ecdsa.Verify`
> thật (không chỉ kiểm tra không lỗi). `go test ./...` PASS 100% toàn
> `notification-service`.

---

## Mục tiêu

Sender thật gửi push tới APNs cho subscription `channel == ios`. Định
nghĩa `PushSender` port ở đây (dùng chung với FCM, TASK-BE-MOBILE-006) —
task đầu tiên trong 2 task chạm `ports.go` cho port này thắng, task kia chỉ
implement.

**Quyết định khoá trước khi code** (để 1 AI agent không phải tự đoán):
dùng `crypto/ecdsa` + `encoding/json`/`encoding/base64` từ stdlib để tự dựng
JWT ES256 (APNs's provider token, [Apple docs "Establishing a token-based
connection to APNs"]) — KHÔNG thêm dependency `golang-jwt/jwt` mới (repo
chưa dùng thư viện này ở bất kỳ đâu, `grep -rln "golang-jwt" backend-go`
cho 0 kết quả — thêm 1 dependency mới cho 3 dòng ký JWT ES256 không đáng,
`crypto/ecdsa.SignASN1` + base64url tự ráp header.payload.signature là đủ).
HTTP/2 dùng `net/http`'s `http.Client` với `http2.Transport`
(`golang.org/x/net/http2` — SDK-adjacent, kiểm tra đã có trong
`go.work.sum` hay cần `go get` trước khi code).

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/ports.go` (MODIFY — thêm `PushSender` interface + `ErrDeviceTokenInvalid`)
2. `backend-go/services/notification-service/internal/adapter/pushgateway/apns_sender.go` (MỚI)
3. `backend-go/services/notification-service/internal/adapter/pushgateway/apns_sender_test.go` (MỚI)

## `ports.go` — thêm

```go
// PushSender delivers event to one ios/android push subscription — the
// sole difference between APNsSender and FCMSender is which third-party
// wire protocol they speak; DeliverPush (TASK-BE-MOBILE-007) picks one by
// subscription.Channel via a map, not a type switch.
type PushSender interface {
	Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error
}

// ErrDeviceTokenInvalid is returned by PushSender.Send when the
// third-party service reports the device token/endpoint no longer
// accepts pushes (APNs 410/BadDeviceToken, FCM 404/UNREGISTERED).
// DeliverPush treats this as "call MarkExpired", never a generic error.
var ErrDeviceTokenInvalid = errors.New("usecase: device token no longer valid")
```

## `internal/adapter/pushgateway/apns_sender.go`

```go
// Package pushgateway implements usecase.PushSender against APNs/FCM —
// notification-service.md §6's deliver_push.go design, adapter half.
// Credential material never touches this package's own storage: every
// Send call resolves fresh via usecase.PushCredentialResolver
// (internal/adapter/credentialbroker), per architecture/06's
// "credential-broker-service mediates all tenant secret material" rule.
package pushgateway

type apnsCredential struct {
	TeamID         string `json:"team_id"`
	KeyID          string `json:"key_id"`
	PrivateKeyPEM  string `json:"private_key_pem"`
}

type APNsSender struct {
	credentials usecase.PushCredentialResolver
	httpClient  *http.Client // constructed by caller with an http2.Transport
	bundleID    string       // APNs apns-topic header — mobile/app.json's ios.bundleIdentifier ("com.stably.orca.mobile")
}

func NewAPNsSender(credentials usecase.PushCredentialResolver, httpClient *http.Client, bundleID string) *APNsSender {
	return &APNsSender{credentials: credentials, httpClient: httpClient, bundleID: bundleID}
}

var _ usecase.PushSender = (*APNsSender)(nil)

func (s *APNsSender) Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error {
	raw, err := s.credentials.Resolve(ctx, sub.TenantID, "apns")
	if err != nil {
		return fmt.Errorf("pushgateway: resolving apns credential: %w", err)
	}
	var cred apnsCredential
	if err := json.Unmarshal(raw, &cred); err != nil {
		return fmt.Errorf("pushgateway: decoding apns credential: %w", err)
	}

	token, err := signAPNsProviderToken(cred)
	if err != nil {
		return fmt.Errorf("pushgateway: signing apns provider token: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"aps": map[string]any{"alert": map[string]string{"title": event.Title, "body": event.Body}},
	})
	if err != nil {
		return fmt.Errorf("pushgateway: encoding apns payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.push.apple.com/3/device/"+sub.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pushgateway: building apns request: %w", err)
	}
	req.Header.Set("authorization", "bearer "+token)
	req.Header.Set("apns-topic", s.bundleID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pushgateway: apns request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone { // 410 == BadDeviceToken
		return usecase.ErrDeviceTokenInvalid
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushgateway: apns returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

// signAPNsProviderToken builds the ES256 JWT APNs requires on the
// authorization header — stdlib crypto/ecdsa only, no new JWT dependency
// (see this task's "quyết định khoá trước khi code").
func signAPNsProviderToken(cred apnsCredential) (string, error) { /* ... */ }
```

## Test cases cần cover

- `TestAPNsSender_Send_Success200` (httptest.Server trả 200)
- `TestAPNsSender_Send_410ReturnsErrDeviceTokenInvalid`
- `TestAPNsSender_Send_OtherErrorStatusIsGenericError` (không phải
  `ErrDeviceTokenInvalid`, để `DeliverPush` không vô tình mark-expired sai)
- `TestAPNsSender_Send_CredentialResolveFailurePropagates`
- `TestSignAPNsProviderToken_ProducesValidES256JWT` (verify chữ ký bằng
  public key tương ứng, không chỉ kiểm tra không lỗi)

## Verify

```bash
cd backend-go/services/notification-service && go build ./... && go test ./internal/adapter/pushgateway/... -run TestAPNs -v
gofmt -l internal/adapter/pushgateway/apns_sender.go internal/usecase/ports.go
```

Nếu cần thêm `golang.org/x/net/http2` vào `go.mod`:
`cd backend-go/services/notification-service && go get golang.org/x/net/http2 && go mod tidy` —
xác nhận `go.work.sum` ở gốc `backend-go/` không bị phá cho service khác
(`go build ./...` từ `backend-go/` sau khi tidy).

## gitnexus

`impact({target: "PushSender", direction: "downstream"})` sau khi thêm
interface — vì đây là symbol MỚI, `impact()` sẽ trả về rỗng/không tồn tại
lúc khảo sát trước khi code (đúng ghi chú của CR-MOBILE-001) — chạy lại
NGAY SAU khi thêm interface, trước khi thêm implementer thứ 2
(TASK-BE-MOBILE-006), để xác nhận không có consumer bất ngờ nào khác đã
tồn tại tên trùng.

## Blocking

TASK-BE-MOBILE-007 (`DeliverPush` usecase) phụ thuộc cứng vào cả
`PushSender` (interface, task này) và `APNsSender` (implementer cụ thể cho
channel `ios`).
