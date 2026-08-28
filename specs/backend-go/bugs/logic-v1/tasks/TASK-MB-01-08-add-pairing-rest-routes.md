# TASK-MB-01-08: Add `api-gateway` REST routes for device pairing (1 unauthenticated, 3 authenticated)

**From Solution:** SOL-MB-01
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/pairing_routes.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go`
**Depends on:** TASK-MB-01-07
**Status:** `[ ]` TODO

---

## Context

`CompleteDevicePairing` is the one REST endpoint on the entire gateway
surface that bypasses `authMiddleware` by design (there is no session yet).
`router.go`'s existing `mountPushRoutes` (mounted outside the authed
`r.Group`, see its doc comment referencing `BUG-003`) is the exact
unauthenticated-mount precedent this task follows — never move this route
inside the authed group.

## Changes to make

`backend-go/services/api-gateway/internal/adapter/httpgateway/pairing_routes.go`:

```go
package httpgateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// mountPairingRoutes wires the 3 authenticated device-pairing endpoints —
// call from inside router.go's authed r.Group, never for /complete (see
// mountUnauthenticatedPairingRoutes below).
func mountPairingRoutes(r chi.Router, client authv1.AuthServiceClient) {
	r.Route("/v1/users/me/paired-devices", func(sub chi.Router) {
		sub.Post("/pairing-sessions", handleInitiateDevicePairing(client))
		sub.Get("/", handleListPairedDevices(client))
		sub.Delete("/{deviceId}", handleUnpairDevice(client))
	})
}

// mountUnauthenticatedPairingRoutes wires CompleteDevicePairing — the ONE
// REST endpoint on the entire gateway surface that bypasses authMiddleware
// by design (there is no session yet; guarded solely by pairing_token
// possession + one-time-use + 5-minute expiry, enforced server-side by
// auth-service). Mounted OUTSIDE the authed r.Group in router.go, following
// mountPushRoutes's identical precedent — a regression here (this route
// moving inside authMiddleware) breaks pairing entirely, and MUST be
// covered by a NoAuthRequired test mirroring TestPushRoutes_NoAuthRequired.
//
// Rate-limited more tightly than the default rateLimitMiddleware tier — a
// brute-force pairing-token guesser is the realistic threat model here,
// defense in depth alongside BR-MB-01/02's server-side expiry+one-time-use.
func mountUnauthenticatedPairingRoutes(r chi.Router, client authv1.AuthServiceClient, pairingRateLimiter func(http.Handler) http.Handler) {
	r.With(pairingRateLimiter).Post("/v1/paired-devices/pairing-sessions/{token}/complete", handleCompleteDevicePairing(client))
}

type completeDevicePairingRequestBody struct {
	MobilePublicKeyB64 string `json:"mobilePublicKey"` // base64
	DeviceLabel        string `json:"deviceLabel"`
}

func handleCompleteDevicePairing(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		var body completeDevicePairingRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		mobilePub, err := base64.StdEncoding.DecodeString(body.MobilePublicKeyB64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "mobilePublicKey must be base64")
			return
		}
		// No identity to attach — this is the intentional unauthenticated
		// exception. Do not call gatewaygrpc.AttachIdentity here.
		resp, err := client.CompleteDevicePairing(r.Context(), &authv1.CompleteDevicePairingRequest{
			PairingToken: token, MobilePublicKey: mobilePub, DeviceLabel: body.DeviceLabel,
		})
		if err != nil {
			// Generic error shape only — no distinguishing "expired" vs.
			// "wrong token" vs. "already consumed" in the HTTP response, so
			// an unauthenticated prober learns nothing about pairing-session
			// existence. writeGRPCError already maps auth-service's single
			// AUTH_PAIRING_TOKEN_INVALID code to one generic 404 shape.
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleInitiateDevicePairing(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.InitiateDevicePairing(ctx, &authv1.InitiateDevicePairingRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func handleListPairedDevices(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListPairedDevices(ctx, &authv1.ListPairedDevicesRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleUnpairDevice(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		if _, err := client.UnpairDevice(ctx, &authv1.UnpairDeviceRequest{DeviceId: chi.URLParam(r, "deviceId")}); err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
```

Add `"encoding/base64"` to the import block.

In `router.go`, mount the unauthenticated route next to `mountPushRoutes`
(outside `r.Group`), and the authenticated 3 inside the existing
`if deps.AuthClient != nil { ... }` block alongside `mountAuthAdminRoutes`:

```go
// outside the authed group, next to mountPushRoutes:
if deps.AuthClient != nil {
	mountUnauthenticatedPairingRoutes(r, deps.AuthClient, pairingRateLimitMiddleware(deps.RateLimiter))
}
// ...
r.Group(func(authed chi.Router) {
	// ...
	if deps.AuthClient != nil {
		mountAuthAdminRoutes(authed, deps.AuthClient)
		mountAdminRoutes(authed, deps.AuthClient)
		mountPairingRoutes(authed, deps.AuthClient)
	}
```

`pairingRateLimitMiddleware` can be a stricter-limit wrapper around the
same `rateLimitMiddleware`/`deps.RateLimiter` primitive `router.go` already
uses — check `rateLimitMiddleware`'s signature before adding a new limiter
type.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... && go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run Pairing
```

Add `TestCompleteDevicePairing_NoAuthRequired` mirroring
`TestPushRoutes_NoAuthRequired` — a request with no `Authorization`
header/session cookie must reach the handler (not `authMiddleware`'s 401).
Add a test asserting an invalid/expired/already-used token all produce the
identical HTTP status + error body shape.
