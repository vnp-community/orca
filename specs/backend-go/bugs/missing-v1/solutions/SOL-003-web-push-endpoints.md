# SOL-003: Fix web push endpoint paths, unauthenticated gate, and add `Unsubscribe`

**Resolves:** [BUG-003](../BUG-003-web-push-endpoints-path-and-auth-mismatch.md)
**Service:** `api-gateway` (routes) + `notification-service` (new RPC)
**Affected files (proposed):**
- `backend-go/proto/orca/notification/v1/notification.proto`
- `backend-go/services/notification-service/internal/usecase/unregister_push_subscription.go` (new)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go`
**Status:** ✅ Implemented — all 3 task(s) (TASK-009–011) DONE; see each task file's own Status/Verify section for evidence.

---

## Design

`specs/backend-go/tdd/services/notification-service.md`'s proto sketch
(line 94) already specifies the RPC this route needs:

```protobuf
rpc UnregisterPushSubscription(UnregisterPushSubscriptionRequest) returns (google.protobuf.Empty);
```

The current `notification.proto` only has `Subscribe`/`GetVapidPublicKey`/`StreamNotifications`
— no unsubscribe RPC at all, and its naming (`Subscribe`) already drifted
from the TDD's `RegisterPushSubscription`. This solution adds
`UnregisterPushSubscription` using the TDD's exact name (additive, no
breaking rename of `Subscribe` — flag the naming drift for a separate
cleanup pass, out of scope here since renaming `Subscribe` would be a
breaking proto change requiring coordinated client updates):

```protobuf
message UnregisterPushSubscriptionRequest {
  string endpoint = 1; // matches push_subscriptions.endpoint's unique index, notification-service.md:173
}
rpc UnregisterPushSubscription(UnregisterPushSubscriptionRequest) returns (google.protobuf.Empty);
```

Usecase (mirrors `register_push_subscription.go`'s existing shape per the
TDD's file layout, `notification-service.md:215`):

```go
// internal/usecase/unregister_push_subscription.go
func (uc *PushUseCase) UnregisterPushSubscription(ctx context.Context, endpoint string) error {
    return uc.repo.DeleteByEndpoint(ctx, endpoint) // idempotent — deleting an already-gone subscription is not an error
}
```

---

## Route fix — path and auth gate

`08-inter-service-communication.md`'s "API Gateway responsibilities" list
doesn't carve out an unauthenticated exception for push routes, but
`http-endpoints.md` (the frontend's real, already-shipped contract) is
explicit that these three routes have no session gate — a browser fetches
the VAPID key and subscribes *before* a session necessarily exists, for
the push-permission-prompt flow. Mount these OUTSIDE the authenticated
group, matching `mountAuthRoutes`/`mountTraceRoutes`'s existing placement
in `router.go` (both already unauthenticated, same reasoning: login/trace
routes can't depend on a session that doesn't exist yet):

```go
// router.go — move out of the authed group, next to mountAuthRoutes/mountTraceRoutes
if deps.NotificationClient != nil {
    mountPushRoutes(r, deps.NotificationClient) // NEW — /api/push-*, unauthenticated
}
```

`notification_routes.go`'s existing `mountNotificationRoutes` (the
authenticated `/v1/notifications/*` surface) stays as-is — this adds a
second, unauthenticated mount for the exact paths `http-endpoints.md`
documents, rather than moving the existing one (some other REST-first
consumer may depend on the authenticated `/v1/notifications/subscribe`
already):

```go
// notification_routes.go
func mountPushRoutes(r chi.Router, client notificationv1.NotificationServiceClient) {
    r.Get("/api/vapid-public-key", handleGetVapidPublicKey(client))   // reuse existing handler
    r.Post("/api/push-subscribe", handleSubscribe(client))            // reuse existing handler
    r.Post("/api/push-unsubscribe", handleUnsubscribe(client))        // NEW
}

func handleUnsubscribe(client notificationv1.NotificationServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var body struct{ Endpoint string `json:"endpoint"` }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
            return
        }
        // No identity available — this route is unauthenticated by design
        // (see http-endpoints.md). tenant_id comes from the subscription
        // row's own endpoint lookup server-side, not from caller context.
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

**Open question to flag, not resolved by this proposal:** an
unauthenticated `push-subscribe`/`push-unsubscribe` accepting an arbitrary
`endpoint` string with no ownership check is a spoofing surface (anyone
who learns another user's endpoint could unsubscribe them). The old TS
backend apparently accepted this same tradeoff (`http-endpoints.md`'s "No
`requireAdmin`/session gate noted at this layer" is descriptive of
existing behavior, not a security sign-off) — worth a follow-up security
review before shipping, not blocking this fix, but shouldn't be silently
carried forward without someone consciously deciding it's acceptable.

---

## Test plan

- `notification_routes_test.go` — `POST /api/push-unsubscribe` with a
  known endpoint → `204`; unknown endpoint → `204` (idempotent per usecase
  design above, not `404`).
- `unregister_push_subscription_test.go` — deletes the row; re-deleting is
  a no-op, not an error.
- Route-placement regression test: assert `/api/vapid-public-key` and
  `/api/push-subscribe` succeed with **no** `orca_session` cookie set
  (guards against accidentally remounting inside the authenticated group
  again later).

## References

- `specs/backend-go/tdd/services/notification-service.md:94,173,215` — target RPC + schema + file layout
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — API-gateway responsibilities (WS/REST routing)
- `backend-go/proto/orca/notification/v1/notification.proto` — current (thinner) RPC set
- `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go` — existing `Subscribe`/`GetVapidPublicKey` handlers to reuse
- `specs/frontend/api/http-endpoints.md` — the exact contract (paths, no auth gate) this must match
