package httpgateway

import (
	"encoding/base64"
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
			// existence. writeGRPCError maps whatever single gRPC code
			// auth-service returns for every one of these cases to the same
			// HTTP status + error body shape.
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
