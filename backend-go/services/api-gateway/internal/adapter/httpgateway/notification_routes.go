package httpgateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

// mountNotificationRoutes wires the two real unary RPCs of
// notification-service's REST surface — Subscribe and GetVapidPublicKey.
// StreamNotifications is deliberately NOT mounted here: it's a
// server-streaming RPC already served for real at GET /v1/notifications/stream
// by a dedicated WS bridge (see router.go's mounting order, which makes that
// literal path win over anything registered under this prefix).
func mountNotificationRoutes(r chi.Router, client notificationv1.NotificationServiceClient) {
	r.Route("/v1/notifications", func(sub chi.Router) {
		sub.Post("/subscribe", handleSubscribe(client, nil))
		sub.Get("/vapid-public-key", handleGetVapidPublicKey(client, nil))
	})
}

// subscribeRequestBody is the REST request shape for POST
// /v1/notifications/subscribe — user_id is deliberately absent: it comes
// from the validated Identity, never trusted from the request body, per
// api-gateway.md §9.
type subscribeRequestBody struct {
	Endpoint  string `json:"endpoint"`
	P256dhKey string `json:"p256dh_key"`
	AuthKey   string `json:"auth_key"`
}

// resolveSoftIdentity reads the identity a prior authMiddleware run already
// attached (the /v1/notifications/* mount); when nothing is attached (the
// unauthenticated /api/push-* mount — see mountPushRoutes's doc comment for
// why that mount MUST stay outside authMiddleware) it falls back to
// validating the orca_session cookie directly, IF one is present, without
// ever failing the request when it isn't. This is the fix for the live bug
// where GetVapidPublicKey/Subscribe are tenant-scoped usecases
// (VapidKeyRepository.GetPublicKey/Subscribe both call
// tenant.RequireTenantID) but the unauthenticated mount never gave them a
// tenant to resolve, even for a genuinely logged-in browser whose fetch()
// sends the session cookie same-origin by default — NOTIFICATION_NO_TENANT
// fired for every caller, logged in or not. A caller with no cookie at all
// (BUG-003's original "before session" scenario) still degrades to an
// empty identity here, same as before this fix.
func resolveSoftIdentity(r *http.Request, cookieValidator CookieSessionValidator) usecase.Identity {
	if identity, ok := identityFromContext(r.Context()); ok {
		return identity
	}
	if cookieValidator == nil {
		return usecase.Identity{}
	}
	id, err := cookieValidator.ValidateCookie(r.Context(), r)
	if err != nil {
		return usecase.Identity{}
	}
	return usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role}
}

func handleSubscribe(client notificationv1.NotificationServiceClient, cookieValidator CookieSessionValidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := resolveSoftIdentity(r, cookieValidator)

		var body subscribeRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Subscribe(ctx, &notificationv1.SubscribeRequest{
			UserId:    identity.UserID,
			Endpoint:  body.Endpoint,
			P256DhKey: body.P256dhKey,
			AuthKey:   body.AuthKey,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func handleGetVapidPublicKey(client notificationv1.NotificationServiceClient, cookieValidator CookieSessionValidator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := resolveSoftIdentity(r, cookieValidator)

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetVapidPublicKey(ctx, &notificationv1.GetVapidPublicKeyRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// mountPushRoutes wires the web-push REST surface at the literal,
// UNAUTHENTICATED paths specs/frontend/api/http-endpoints.md documents
// (`/api/vapid-public-key`, `/api/push-subscribe`, `/api/push-unsubscribe`)
// — distinct from mountNotificationRoutes's `/v1/notifications/*` mount of
// (mostly) the same RPCs behind authMiddleware. Per http-endpoints.md these
// routes are unauthenticated by design: a browser registering for push
// notifications does so before/independent of any session, and the
// service-worker's push-subscribe call has no session cookie to present
// either. See router.go's mounting order — this MUST be called outside the
// authed route group, never inside it (regression guard: BUG-003,
// TestPushRoutes_NoAuthRequired in notification_routes_test.go).
//
// cookieValidator is passed through to handleGetVapidPublicKey/handleSubscribe
// so a genuinely logged-in browser (the common real case: push permission is
// requested from inside the app, session cookie already set) gets its real
// tenant/user resolved via resolveSoftIdentity — without that, both
// usecases' tenant.RequireTenantID call always failed
// (NOTIFICATION_NO_TENANT), for every caller, logged in or not. A caller
// with no cookie at all still degrades to an empty identity, never a 401 —
// BUG-003's guarantee is unchanged.
func mountPushRoutes(r chi.Router, client notificationv1.NotificationServiceClient, cookieValidator CookieSessionValidator) {
	r.Get("/api/vapid-public-key", handleGetVapidPublicKey(client, cookieValidator))
	r.Post("/api/push-subscribe", handleSubscribe(client, cookieValidator))
	r.Post("/api/push-unsubscribe", handleUnsubscribe(client))
}

// unsubscribeRequestBody is the REST request shape for POST
// /api/push-unsubscribe.
type unsubscribeRequestBody struct {
	Endpoint string `json:"endpoint"`
}

func handleUnsubscribe(client notificationv1.NotificationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body unsubscribeRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		// Unauthenticated by design (see http-endpoints.md) — no identity
		// to attach. Tenant/user scoping comes from the endpoint row's own
		// lookup server-side, not from caller identity.
		if _, err := client.UnregisterPushSubscription(r.Context(), &notificationv1.UnregisterPushSubscriptionRequest{
			Endpoint: body.Endpoint,
		}); err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
