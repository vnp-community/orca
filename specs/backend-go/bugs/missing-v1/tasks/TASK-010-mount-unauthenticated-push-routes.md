# TASK-010: Mount `/api/push-*` unauthenticated, add `push-unsubscribe`

**From Solution:** SOL-003
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/httpgateway/notification_routes.go`, `router.go`
**Depends on:** TASK-009
**Status:** `[ ]` TODO

---

## Changes to make

### `notification_routes.go` — add a new, unauthenticated mount function

```go
func mountPushRoutes(r chi.Router, client notificationv1.NotificationServiceClient) {
    r.Get("/api/vapid-public-key", handleGetVapidPublicKey(client)) // reuse existing handler
    r.Post("/api/push-subscribe", handleSubscribe(client))           // reuse existing handler
    r.Post("/api/push-unsubscribe", handleUnsubscribe(client))       // NEW
}

func handleUnsubscribe(client notificationv1.NotificationServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var body struct{ Endpoint string `json:"endpoint"` }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
            return
        }
        // Unauthenticated by design (see http-endpoints.md) — no identity
        // to attach. tenant scoping comes from the endpoint row's own
        // lookup server-side.
        if _, err := client.UnregisterPushSubscription(r.Context(), &notificationv1.UnregisterPushSubscriptionRequest{
            Endpoint: body.Endpoint,
        }); err != nil {
            writeGRPCError(w, err)
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}
```

### `router.go` — mount OUTSIDE the authenticated group

```go
// Unauthenticated group, next to mountAuthRoutes/mountTraceRoutes:
if deps.NotificationClient != nil {
    mountPushRoutes(r, deps.NotificationClient)
}
```

Leave the existing authenticated `mountNotificationRoutes` call
(`/v1/notifications/subscribe`, `/v1/notifications/vapid-public-key`)
untouched — this adds a second, unauthenticated mount at the paths
`http-endpoints.md` actually documents, it doesn't replace the existing one.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
