# TASK-BE-MOBILE-006: `pushgateway.FCMSender`

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service`
**Depends on:** TASK-BE-MOBILE-004 (`PushCredentialResolver`), TASK-BE-MOBILE-005 (`PushSender` interface đã tồn tại trong `ports.go`)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `grep -rln "x/oauth2/google" backend-go` xác nhận đúng
> 0 kết quả trước khi thêm — không drift. `golang.org/x/oauth2` KHÔNG có mặt
> gián tiếp trong `go.mod` của `notification-service` như task doc dự đoán
> (chỉ có ở `go.work.sum` gốc) — phải `go get` thật.
>
> **Rủi ro thật gặp phải khi `go get`**: lần đầu `go get golang.org/x/oauth2/google`
> tự động nâng `golang.org/x/oauth2` lên v0.37.0, kéo theo yêu cầu Go
> **1.26.0** (bump `go` directive trong `go.mod` từ 1.25.0) — không chấp
> nhận vì phá tính nhất quán baseline Go của toàn `backend-go`. Đã pin cứng
> `v0.36.0` (đúng version task doc dự kiến ban đầu) + `go mod edit -go=1.25.0`
> — build lại sạch, xác nhận Go 1.26 không thực sự cần thiết. `go mod tidy`
> (chạy tự động, không riêng lệnh gõ tay) kéo thêm hàng loạt dependency
> KHÔNG liên quan (`cenkalti/backoff/v5`, `grpc-ecosystem/grpc-gateway/v2`,
> `otel/exporters/otlp/*`, bump `klauspost/compress`) — do MVS resolution
> bị ảnh hưởng bởi thay đổi CỦA AGENT KHÁC ở service khác trong cùng
> `go.work`. Đã **revert về `go.mod` gốc + tự tay thêm đúng 2 dòng cần
> thiết** (`golang.org/x/oauth2 v0.36.0` direct, `cloud.google.com/go/compute/metadata
> v0.9.0` indirect — transitive thật của `x/oauth2/google`) thay vì chạy
> `go mod tidy`, để không mang theo bloat không liên quan. `go build ./...`
> tự bổ sung đúng `go.sum` cần thiết (0 dòng go.sum mới — checksum đã có sẵn
> từ lần thử trước). Xác nhận build sạch cho **toàn bộ 17 service** trong
> `backend-go/services/*` sau thay đổi.
>
> Test dùng **RSA key thật + luồng OAuth2 JWT-bearer thật** (không fake
> `TokenSource`): service-account JSON giả có `token_uri` trỏ vào
> `httptest.Server` riêng — `x/oauth2/google`'s `TokenSource.Token()` tự ký
> JWT bằng key thật rồi POST tới URL đó thật sự, xác nhận toàn bộ luồng
> credential-parse → JWT-sign → token-exchange → API-call hoạt động đúng,
> không chỉ mock `TokenSource` (rủi ro fake sai giả định).
> `google.CredentialsFromJSON` bị đánh dấu `Deprecated` trong version SDK
> hiện tại (khuyến nghị `CredentialsFromJSONWithParams`) nhưng vẫn hoạt
> động đúng — giữ nguyên theo đúng sketch của task, không tự ý đổi API
> ngoài phạm vi.
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./internal/adapter/pushgateway/... -run TestFCM -v` — **5/5 PASS**. `go
> test ./...` PASS 100% toàn `notification-service`.

---

## Mục tiêu

Sender thật gửi push tới FCM (HTTP v1 API) cho subscription
`channel == android`. Không sửa `ports.go` — `PushSender` interface đã có
từ TASK-BE-MOBILE-005, task này chỉ thêm 1 implementer thứ hai.

**Quyết định khoá trước khi code**: dùng
`golang.org/x/oauth2/google` (`google.FindDefaultCredentials`/
`ConfigFromJSON` lấy access token từ service-account JSON) — package này
đã có mặt gián tiếp trong `go.work.sum` (`golang.org/x/oauth2 v0.36.0`)
nhưng **chưa được import trực tiếp ở bất kỳ đâu trong `backend-go`**
(`grep -rln "x/oauth2/google" backend-go` → 0 kết quả tại thời điểm khảo
sát) — xác nhận lại bằng đúng lệnh grep này trước khi code, vì có thể đã
đổi giữa lúc viết task và lúc thực thi.

## Files cần sửa

1. `backend-go/services/notification-service/internal/adapter/pushgateway/fcm_sender.go` (MỚI)
2. `backend-go/services/notification-service/internal/adapter/pushgateway/fcm_sender_test.go` (MỚI)

## `internal/adapter/pushgateway/fcm_sender.go`

```go
package pushgateway

// fcmCredential is the FCM service-account JSON as-is (project_id,
// private_key, client_email, ...) — google.CredentialsFromJSON parses
// it directly, no custom struct needed for the OAuth2 fields.
type FCMSender struct {
	credentials usecase.PushCredentialResolver
	httpClient  *http.Client
}

func NewFCMSender(credentials usecase.PushCredentialResolver, httpClient *http.Client) *FCMSender {
	return &FCMSender{credentials: credentials, httpClient: httpClient}
}

var _ usecase.PushSender = (*FCMSender)(nil)

func (s *FCMSender) Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error {
	raw, err := s.credentials.Resolve(ctx, sub.TenantID, "fcm")
	if err != nil {
		return fmt.Errorf("pushgateway: resolving fcm credential: %w", err)
	}

	creds, err := google.CredentialsFromJSON(ctx, raw, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return fmt.Errorf("pushgateway: parsing fcm service account: %w", err)
	}
	token, err := creds.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("pushgateway: fetching fcm oauth2 token: %w", err)
	}

	var projectID struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(raw, &projectID); err != nil {
		return fmt.Errorf("pushgateway: reading fcm project_id: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"token":        sub.Endpoint,
			"notification": map[string]string{"title": event.Title, "body": event.Body},
		},
	})
	if err != nil {
		return fmt.Errorf("pushgateway: encoding fcm payload: %w", err)
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pushgateway: building fcm request: %w", err)
	}
	req.Header.Set("authorization", "Bearer "+token.AccessToken)
	req.Header.Set("content-type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pushgateway: fcm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound { // FCM UNREGISTERED
		return usecase.ErrDeviceTokenInvalid
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushgateway: fcm returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
```

## Test cases cần cover

- `TestFCMSender_Send_Success200`
- `TestFCMSender_Send_404ReturnsErrDeviceTokenInvalid`
- `TestFCMSender_Send_OtherErrorStatusIsGenericError`
- `TestFCMSender_Send_CredentialResolveFailurePropagates`
- `TestFCMSender_Send_MalformedServiceAccountJSONFails` (không panic khi
  credential blob không parse được — trả lỗi thường, KHÔNG
  `ErrDeviceTokenInvalid`, vì đây là lỗi cấu hình, không phải token hết
  hạn)

## Verify

```bash
cd backend-go/services/notification-service && go get golang.org/x/oauth2/google && go mod tidy
go build ./... && go test ./internal/adapter/pushgateway/... -run TestFCM -v
gofmt -l internal/adapter/pushgateway/fcm_sender.go
```

Sau `go mod tidy`, chạy `cd backend-go && go build ./...` từ workspace gốc
để xác nhận `go.work.sum` không bị phá cho service khác dùng chung
`golang.org/x/oauth2` gián tiếp.

## gitnexus

`impact({target: "PushSender", direction: "downstream"})` trước khi thêm
implementer thứ 2 — xác nhận `APNsSender` (TASK-BE-MOBILE-005) đã tồn tại
đúng như kỳ vọng và không có implementer nào khác bất ngờ xuất hiện giữa 2
task.

## Blocking

TASK-BE-MOBILE-007 (`DeliverPush` usecase) phụ thuộc cứng — cần cả
`APNsSender` và `FCMSender` tồn tại để xây map `channel -> PushSender` đầy
đủ 2 channel.
