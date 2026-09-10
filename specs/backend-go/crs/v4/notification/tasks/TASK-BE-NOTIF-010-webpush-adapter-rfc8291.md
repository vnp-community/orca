# TASK-BE-NOTIF-010: `internal/adapter/external/webpush/` — RFC 8291 encrypt + RFC 8292 VAPID JWT qua Vault

**Solution:** BE-NOTIF-SOL-002 §2.B (+ "Quyết định khác" mục 1) | **CR:** CR-NOTIF-002 (mục B)
**Service:** `notification-service`
**Depends on:** TASK-BE-NOTIF-009
**Status:** ✅ DONE (2026-09-09) — kèm 1 bug production nghiêm trọng phát hiện + đã vá ở service KHÁC

> **⚠️ Phát hiện quan trọng nhất của cả bộ CR-NOTIF-002: `credential-broker-service`'s
> `SignVapidPayload` gọi SAI Vault operation.** Đọc trực tiếp
> `services/credential-broker-service/internal/usecase/sign_vapid_payload.go`
> (theo đúng yêu cầu "PHẢI xác nhận trước khi khoá code" của task) phát hiện:
> usecase gọi `store.TransitEncrypt` (`transit/encrypt/<key>` — trả về ciphertext
> đối xứng, KHÔNG PHẢI chữ ký) thay vì `store.TransitSign` (`transit/sign/<key>` —
> chữ ký ECDSA thật). Test có sẵn (`TestSignVapidPayload_SignsViaTransit`) chỉ
> assert đúng hành vi SAI đó (fake trả `"vault:v1:<key>:<plaintext>"`, không phải
> chữ ký thật) — không ai từng viết test để bắt lỗi này. Nếu không sửa, MỌI lần
> gửi Web Push thật sẽ bị push service (Chrome/Firefox/...) từ chối vì chữ ký
> JWT không hợp lệ — tính năng "hoạt động" ở tầng code nhưng **không bao giờ gửi
> được push thật**.
>
> **Đã vá (ngoài phạm vi ban đầu của task, nhưng là phụ thuộc chặn trực tiếp —
> không sửa thì TASK-010 vô nghĩa):**
> - `credential-broker-service/internal/usecase/ports.go`: thêm `TransitSign` vào
>   `SecretStore` interface.
> - `credential-broker-service/internal/adapter/vault/secret_store.go`: thêm
>   passthrough `TransitSign` → `common/secrets.Client.TransitSign` (đã có sẵn,
>   đúng chuẩn, chỉ chưa được expose qua port này).
> - `sign_vapid_payload.go`: đổi gọi `store.TransitSign`.
> - `fakes_test.go`/`sign_vapid_payload_test.go`: fake `TransitSign` trả giá trị
>   CỐ Ý khác định dạng `TransitEncrypt` (để test fail rõ ràng nếu ai đó vô tình
>   đổi lại gọi nhầm method), cập nhật assertion theo hành vi đúng.
> - `impact({target: "SecretStore", direction: "upstream"})` trước khi sửa: LOW
>   risk, 6 impacted (thêm method vào interface, không đổi method có sẵn).
> - `go build`/`go vet`/`go test ./...` của `credential-broker-service` PASS 100%
>   sau khi sửa.
>
> **Rủi ro vận hành còn lại (KHÔNG tự sửa được, cần xác nhận ngoài code)**: key
> Vault `"vapid-signing-<tenant_id>"` phải được provision là khoá ký bất đối
> xứng (vd. `ecdsa-p256`) — usecase này không tự tạo key (`vapidKeyName`'s doc
> comment: provisioning ở ngoài/ops), nên nếu key thật hiện tại được tạo cho mục
> đích encrypt (kiểu đối xứng), `TransitSign` sẽ lỗi ở tầng Vault. Đã ghi rõ cảnh
> báo này trong code (`sign_vapid_payload.go`'s doc comment).
>
> ---
>
> **Kết quả implement thật của TASK-010** (`notification-service/internal/adapter/external/webpush/`):
>
> - `encrypt.go`: RFC 8291 §3.1-3.4 implement trực tiếp — ECDH P-256 ephemeral
>   keypair mỗi lần gửi (`crypto/ecdh`), HKDF tự viết bằng `crypto/hmac`+
>   `crypto/sha256` theo đúng RFC 5869 (KHÔNG dùng `golang.org/x/crypto/hkdf` —
>   quyết định khác sketch: tự viết ~20 dòng HKDF rõ ràng, dễ audit theo từng
>   bước RFC, tránh phụ thuộc thêm; xác nhận `go mod tidy` không cần sửa
>   `go.mod`/`go.sum` — package hoàn toàn stdlib).
> - `TestEncrypt_RoundTripDecryptsCorrectly` — **decrypt độc lập thật** (không tái
>   dùng state nội bộ của `encrypt`): tạo UA keypair thật, gọi `encrypt`, rồi tự
>   viết lại toàn bộ luồng giải mã từ phía receiver (ECDH bằng `ua_private` +
>   `as_public` lấy từ header message, HKDF, AES-GCM open) — chứng minh
>   ciphertext thật sự đúng theo RFC, không chỉ "tự nhất quán với chính nó".
> - `vapid.go` **(cập nhật 2026-09-09 khi làm TASK-011): di chuyển sang
>   `internal/usecase/vapid.go`** — giữ nguyên tại `webpush` package lúc viết
>   task này ban đầu là SAI vị trí kiến trúc (`usecase` không được import
>   `internal/adapter/*`, và `webpush` đã import ngược `usecase` để lấy
>   `VaultSigner` — giữ `vapid.go` ở `webpush` sẽ tạo import cycle thật khi
>   TASK-011's `deliver_push.go` cần gọi nó). Xem TASK-BE-NOTIF-011's "Kết quả
>   thực tế" cho lý do đầy đủ. Nội dung kỹ thuật bên dưới (thuật toán, test)
>   không đổi, chỉ đổi package/vị trí file.** Phát hiện thêm 1 vấn đề encoding
>   task đã cảnh báo trước — Vault
>   Transit's `marshaling_algorithm` mặc định là `asn1` (DER), nhưng JWS ES256
>   cần raw `r||s` 64 byte. Thêm `derECDSASignatureToRawJWS` (parse ASN.1 bằng
>   `encoding/asn1`, pad `r`/`s` về 32 byte mỗi phần) — **cố ý KHÔNG** đổi
>   `common/secrets.Client.TransitSign` để tự thêm `marshaling_algorithm=jws`
>   (dù đơn giản hơn), vì hàm đó có doc comment gợi ý được thiết kế chung cho
>   cả trường hợp RSA JWT-signing khác — chuyển đổi DER ở đúng call site VAPID
>   này an toàn hơn, không đổi hành vi 1 primitive dùng chung.
>   `TestDerECDSASignatureToRawJWS_RoundTripsAndVerifies` xác nhận bằng
>   `ecdsa.Verify` thật (không chỉ so sánh độ dài byte).
> - `sender.go`: đúng sketch, bỏ field `signer` không dùng khỏi `New(...)` (khác
>   1 chi tiết nhỏ so với sketch — `BuildVapidAuthHeader` là hàm độc lập, `Sender`
>   không cần giữ `signer`).
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./internal/adapter/external/webpush/... -v` — **13/13 PASS** (nhiều hơn 8
> test tối thiểu task yêu cầu — thêm test invalid-key, nil-web-keys, và
> DER-conversion). `go test ./...` toàn `notification-service` PASS 100%.
> `impact({target: "Sender"...})` trả "not found" (index chưa bắt kịp package
> mới) — xác nhận thủ công bằng grep: đúng 0 caller ngoài package (TASK-011 sẽ
> wire).

## Mục tiêu

Implement `usecase.WebPushSender` thật: mã hoá payload theo RFC 8291, ký VAPID JWT (RFC 8292) qua `VaultSigner.SignVapidPayload` (Vault Transit — **không** ký cục bộ bằng private key trong process này, đúng §9's headline property), gửi HTTP POST tới endpoint.

## Files cần sửa

1. `backend-go/services/notification-service/internal/adapter/external/webpush/sender.go` (MỚI)
2. `backend-go/services/notification-service/internal/adapter/external/webpush/encrypt.go` (MỚI)
3. `backend-go/services/notification-service/internal/adapter/external/webpush/vapid.go` (MỚI)
4. `backend-go/services/notification-service/go.mod` (MODIFY nếu cần thêm `golang.org/x/crypto/hkdf` như dependency trực tiếp — đã có gián tiếp qua `go.sum`, xác nhận `go mod tidy` không kéo thêm gì ngoài dự kiến)

## `sender.go` — cấu trúc chính

```go
// Package webpush implements usecase.WebPushSender: RFC 8291 message
// encryption + RFC 8292 VAPID auth header, POSTed to a Web Push endpoint.
// VAPID JWT signing is delegated to usecase.VaultSigner (Vault Transit) —
// this package never holds a VAPID private key, per
// notification-service.md §9.
package webpush

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

type Sender struct {
	signer usecase.VaultSigner
	http   *http.Client
}

func New(signer usecase.VaultSigner, httpClient *http.Client) *Sender {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Sender{signer: signer, http: httpClient}
}

func (s *Sender) Send(ctx context.Context, sub domain.PushSubscription, vapidAuthHeader string, payload []byte) (bool, error) {
	encrypted, err := encrypt(payload, sub.P256dhKey, sub.AuthKey)
	if err != nil {
		return false, fmt.Errorf("webpush: encrypt payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(encrypted))
	if err != nil {
		return false, fmt.Errorf("webpush: build request: %w", err)
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "86400")
	req.Header.Set("Authorization", vapidAuthHeader)

	resp, err := s.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("webpush: post to endpoint: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return false, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return true, nil
	default:
		return false, fmt.Errorf("webpush: push service responded %d", resp.StatusCode)
	}
}
```

`vapidAuthHeader` xây ở tầng gọi (`usecase.DeliverPush`, TASK-BE-NOTIF-011, vì cần `tenantID` để gọi `VaultSigner` — `Sender` KHÔNG tự giữ `tenantID`) qua 1 hàm export riêng:

```go
// BuildVapidAuthHeader (vapid.go) — exported so DeliverPush can call it
// once per subscription's audience (aud claim = endpoint's origin) before
// calling Sender.Send.
func BuildVapidAuthHeader(ctx context.Context, signer usecase.VaultSigner, tenantID, endpointOrigin, subject string) (string, error) {
	header := `{"typ":"JWT","alg":"ES256"}`
	claims := fmt.Sprintf(`{"aud":%q,"exp":%d,"sub":%q}`, endpointOrigin, time.Now().Add(12*time.Hour).Unix(), subject)
	signingInput := base64url(header) + "." + base64url(claims)

	sig, err := signer.SignVapidPayload(ctx, tenantID, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("webpush: sign vapid jwt: %w", err)
	}
	jwt := signingInput + "." + sig // sig assumed already base64url from VaultSigner — confirm exact return shape against credential-broker-service's SignVapidPayloadResponse.signature before locking this line
	publicKey := "" // caller must supply the tenant's VAPID public key (from VapidKeyRepository) for the k= param
	return fmt.Sprintf("vapid t=%s, k=%s", jwt, publicKey), nil
}
```

**Điểm PHẢI xác nhận trước khi khoá code thật** (không suy đoán, đọc source thật trước khi implement, đúng nguyên tắc "TUYỆT ĐỐI không bịa"): `credential-broker-service`'s `SignVapidPayloadResponse.signature`'s encoding thật (base64 chuẩn hay base64url, có padding hay không) — đọc `backend-go/services/credential-broker-service/internal/usecase/sign_vapid_payload.go` và `credentialbroker.proto`'s `SignVapidPayloadResponse` trước khi viết dòng ghép `jwt`. Nếu encoding khác base64url-no-pad (chuẩn JWT), phải tự re-encode ở đây trước khi ghép vào JWT — không giả định khớp sẵn.

## `encrypt.go` — RFC 8291 message encryption

Dùng `crypto/ecdh` (P-256 ephemeral keypair mỗi lần gửi — **không tái sử dụng giữa các lần gửi**, kể cả cùng subscription) + `golang.org/x/crypto/hkdf` (2 lần derive: PRK từ ECDH shared secret + `auth_key`, rồi Content Encryption Key + nonce từ PRK) + `crypto/aes`/`crypto/cipher` (AES-128-GCM). Salt (16 byte) cũng PHẢI random mới mỗi lần gửi (`crypto/rand`). Không chép lại toàn bộ byte layout ở đây — implement bám sát RFC 8291 §3.1-3.4 trực tiếp khi code, vì đây là thuật toán đã chuẩn hoá, sai 1 byte offset là hỏng toàn bộ (không phải chỗ để diễn giải lại bằng lời).

## Test cases cần cover

- `TestEncrypt_DifferentSaltEachCall` — gọi `encrypt` 2 lần với CÙNG payload/keys → 2 ciphertext KHÁC NHAU (xác nhận salt/ephemeral key không bị tái sử dụng — rủi ro mật mã nghiêm trọng nhất nếu sai, ghi rõ ở solution's bảng rủi ro).
- `TestEncrypt_RoundTripDecryptsCorrectly` — dùng chính công thức RFC 8291 để giải mã lại trong test (hoặc 1 implementation tham chiếu độc lập, ví dụ 1 helper Python/Node dùng thư viện `web-push` chuẩn chạy offline một lần để tạo golden vector — KHÔNG bắt buộc CI phụ thuộc runtime ngoài Go, chỉ dùng để tạo test vector cố định) — assert giải mã ra đúng payload gốc.
- `TestBuildVapidAuthHeader_CallsSignerExactlyOnce` — fake `VaultSigner` assert `SignVapidPayload` được gọi đúng 1 lần với `tenantID` đúng.
- `TestBuildVapidAuthHeader_SignerError_Propagates`.
- `TestSender_Send_2xxReturnsNotExpired`.
- `TestSender_Send_410ReturnsExpiredTrue`.
- `TestSender_Send_404ReturnsExpiredTrue`.
- `TestSender_Send_5xxReturnsError_NotExpired` — phân biệt rõ lỗi tạm thời (không mark expired) với 404/410 (mark expired) — dùng `httptest.Server` giả lập push endpoint, KHÔNG cần trình duyệt/push service thật, đúng tiêu chí chấp nhận của CR-NOTIF-002.

## Verify

```bash
cd backend-go/services/notification-service
go build ./...
go test ./internal/adapter/external/webpush/... -v
gofmt -l internal/adapter/external/webpush/*.go
go vet ./internal/adapter/external/webpush/...
```

## gitnexus

Package hoàn toàn mới — không có blast radius để đo trước khi tạo. Sau khi tạo, chạy `impact({target: "Sender", direction: "upstream"})` để xác nhận caller duy nhất là `cmd/server/main.go` (TASK-BE-NOTIF-011) trước khi merge.

## Blocking

TASK-BE-NOTIF-011 (`DeliverPush` usecase) phụ thuộc `Sender`/`BuildVapidAuthHeader` đã tồn tại và test pass.
