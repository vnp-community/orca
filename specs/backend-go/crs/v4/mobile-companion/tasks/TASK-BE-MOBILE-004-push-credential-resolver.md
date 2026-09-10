# TASK-BE-MOBILE-004: `PushCredentialResolver` port + `credentialbroker` client adapter

**Solution:** BE-MOBILE-SOL-001 | **CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Service:** `notification-service` (client), `credential-broker-service` (không sửa — RPC đã tồn tại)
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Xác nhận trước khi code: `grep "CREDENTIAL_CATEGORY_SERVICE_SECRET"
> credential-broker-service/internal/adapter/grpc/server.go` → có xử lý (dòng 228),
> không drift so với task doc. Implement đúng sketch, mirror chính xác
> `scm-integration-service`'s `credentialbroker/client.go` (đã đọc trước khi
> viết, không đoán). Fake `CredentialBrokerServiceClient` trong test implement
> đủ 9 method của interface (8 stub `Unimplemented`, 1 configurable) — không
> dùng `New(conn)` (cần `grpc.ClientConnInterface` thật) mà build `*Resolver`
> trực tiếp qua struct literal cho test, tránh phải mock cả tầng gRPC dial.
>
> **Build/test thật**: `go build ./...`, `go vet ./...`, `gofmt -l` sạch. `go
> test ./internal/adapter/credentialbroker/... -v` — 3/3 PASS. `go test ./...`
> PASS 100% toàn `notification-service`.

---

## Mục tiêu

APNs (Team ID/Key ID/`.p8` key) và FCM (service-account JSON) credential
KHÔNG được lưu trong Vault trực tiếp hay biến môi trường trần
(`AGENTS.md`/`architecture/06-secrets-vault-architecture.md`) — phải qua
`credential-broker-service`, dùng lại `CREDENTIAL_CATEGORY_SERVICE_SECRET`
đã có sẵn trong proto (chưa từng có caller thật). Task này chỉ tạo cổng lấy
credential — KHÔNG tạo `PushSender`/APNs/FCM sender thật (xem
TASK-BE-MOBILE-005/006).

**Quan trọng — không nhầm với `VaultSigner`**: `usecase.VaultSigner`
(`SignVapidPayload`) đã tồn tại nhưng dùng cho JWT VAPID của Web Push
(RFC 8292) — giao thức khác hoàn toàn với APNs/FCM. Port mới này độc lập,
không mở rộng `VaultSigner`.

## Files cần sửa

1. `backend-go/services/notification-service/internal/usecase/ports.go` (MODIFY — thêm interface `PushCredentialResolver`)
2. `backend-go/services/notification-service/internal/adapter/credentialbroker/client.go` (MỚI)
3. `backend-go/services/notification-service/internal/adapter/credentialbroker/client_test.go` (MỚI)

## `ports.go`

```go
// PushCredentialResolver fetches this tenant's APNs/FCM credential
// material via credential-broker-service (CREDENTIAL_CATEGORY_SERVICE_SECRET
// — already defined in credentialbroker.proto and handled by that
// service's grpc/server.go, but exercised by no caller before this).
// Distinct from VaultSigner: that port signs a VAPID JWT for Web Push;
// this one returns raw credential bytes for a different protocol
// entirely (APNs JWT ES256 / FCM OAuth2 service-account) — see
// notification-service.md §9's updated credential-storage section.
type PushCredentialResolver interface {
	// Resolve returns the opaque credential blob stored for
	// (tenantID, ownerID) — ownerID is "apns" or "fcm". The blob's shape
	// is adapter-owned (see internal/adapter/pushgateway): apns_sender.go
	// JSON-decodes it as {team_id, key_id, private_key_pem}; fcm_sender.go
	// treats it as the raw FCM service-account JSON as-is.
	Resolve(ctx context.Context, tenantID, ownerID string) ([]byte, error)
}
```

## `internal/adapter/credentialbroker/client.go` (package mới)

Mirror chính xác
`backend-go/services/scm-integration-service/internal/adapter/credentialbroker/client.go`'s
`Resolver.Resolve` (cùng RPC `ResolveCredentialByOwner`, khác `Category`):

```go
// Package credentialbroker is notification-service's client to
// credential-broker-service for push credential material (APNs/FCM) —
// CREDENTIAL_CATEGORY_SERVICE_SECRET, owner_id "apns"/"fcm". Mirrors
// scm-integration-service's internal/adapter/credentialbroker/client.go
// (same RPC, different category) — see that package's doc comment for
// why ResolveCredentialByOwner (not ResolveCredential) is the right RPC
// for a caller that only ever knows (tenant_id, category, owner_id), never
// an opaque credential_id.
package credentialbroker

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

type Resolver struct {
	client credentialbrokerv1.CredentialBrokerServiceClient
}

// New wraps an already-dialed connection to credential-broker-service —
// cmd/server/main.go reuses the SAME connection already dialed for
// vaultsigner.New(...) (see that file's brokerConn), not a second dial.
func New(conn grpc.ClientConnInterface) *Resolver {
	return &Resolver{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

func (r *Resolver) Resolve(ctx context.Context, tenantID, ownerID string) ([]byte, error) {
	resp, err := r.client.ResolveCredentialByOwner(ctx, &credentialbrokerv1.ResolveCredentialByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SERVICE_SECRET,
		OwnerId:  ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("credentialbroker: resolving %s push credential: %w", ownerID, err)
	}
	return resp.GetValue(), nil
}
```

Không cần sửa `credentialbroker.proto`/`credential-broker-service` —
`ResolveCredentialByOwner` và `CREDENTIAL_CATEGORY_SERVICE_SECRET` đã tồn
tại thật (`credential-broker-service/internal/adapter/grpc/server.go:228`
xử lý category này). Xác nhận lại bằng
`grep -n "CREDENTIAL_CATEGORY_SERVICE_SECRET" backend-go/services/credential-broker-service/internal/adapter/grpc/server.go`
trước khi code — nếu handler chưa xử lý đủ category này ở thời điểm code
task (drift có thể xảy ra), ghi nhận lại, không tự sửa
`credential-broker-service` trong task này (ngoài phạm vi).

## Test cases cần cover (`client_test.go`, mock `CredentialBrokerServiceClient`)

- `TestResolver_Resolve_CallsResolveCredentialByOwnerWithServiceSecretCategory`
  (xác nhận đúng `Category`/`OwnerId`/`TenantId` gửi đi)
- `TestResolver_Resolve_PropagatesTransportError`
- `TestResolver_Resolve_ReturnsValueBytesUnmodified`

## Verify

```bash
cd backend-go/services/notification-service && go build ./... && go test ./internal/adapter/credentialbroker/... -v
gofmt -l internal/adapter/credentialbroker/client.go internal/usecase/ports.go
```

## gitnexus

`impact({target: "ResolveCredentialByOwner", direction: "downstream"})` (
package `credential-broker-service`'s `adapter/grpc`) trước khi thêm caller
mới — xác nhận RPC hiện có không có ràng buộc caller-list nào chặn
`notification-service` (mirror cách `scm-integration-service` đã dùng RPC
này thành công).

## Blocking

TASK-BE-MOBILE-005 (`apns_sender.go`) và TASK-BE-MOBILE-006
(`fcm_sender.go`) phụ thuộc cứng — cả hai cần `PushCredentialResolver` tồn
tại để lấy credential trước khi gửi.
