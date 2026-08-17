package httpgateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

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
		sub.Post("/subscribe", handleSubscribe(client))
		sub.Get("/vapid-public-key", handleGetVapidPublicKey(client))
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

func handleSubscribe(client notificationv1.NotificationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

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

func handleGetVapidPublicKey(client notificationv1.NotificationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetVapidPublicKey(ctx, &notificationv1.GetVapidPublicKeyRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
