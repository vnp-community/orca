# BUG-003: Web push endpoints at the wrong path, wrong auth gate, and missing `unsubscribe`

**Service:** `api-gateway` (proxies to `notification-service`)
**File:** `internal/adapter/httpgateway/notification_routes.go`, `router.go`
**Severity:** Medium — breaks browser push subscribe/unsubscribe for any client still calling the documented `/api/push-*` contract
**Symptom:** `fetch('/api/vapid-public-key')`, `fetch('/api/push-subscribe')`, `fetch('/api/push-unsubscribe')` all 404; the two that do exist require session auth the spec says they don't
**Status:** ❌ Open

---

## Description

`specs/frontend/api/http-endpoints.md` documents three unauthenticated
(no `requireAdmin`/session gate noted) routes under `/api/push-*`:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/vapid-public-key` | Fetch the VAPID public key |
| `POST` | `/api/push-subscribe` | Register a push subscription |
| `POST` | `/api/push-unsubscribe` | Remove a push subscription |

`notification_routes.go`'s `mountNotificationRoutes` registers instead:

```go
r.Route("/v1/notifications", func(sub chi.Router) {
    sub.Post("/subscribe", handleSubscribe(client))
    sub.Get("/vapid-public-key", handleGetVapidPublicKey(client))
})
```

— i.e. `POST /v1/notifications/subscribe` and
`GET /v1/notifications/vapid-public-key`. Three concrete gaps:

1. **Wrong paths.** Neither matches the spec's `/api/*` prefix, so a client
   built against the documented contract 404s on both.
2. **Wrong auth gate.** `mountNotificationRoutes` is called from inside
   `router.go`'s authenticated `r.Group` (behind `authMiddleware` +
   `rateLimitMiddleware`, see `router.go`'s `NewRouter`) — but
   `http-endpoints.md` explicitly notes push routes have no such gate in
   the old backend (`backend/src/server/push-api-routes.ts`, hand-rolled,
   no `requireAdmin`/session check). A browser fetching the VAPID key
   *before* establishing a session (the normal bootstrap order for
   push-permission prompts) would get a `401` here instead of the key.
3. **`/api/push-unsubscribe` has no backend at all.** There is no
   `Unsubscribe` handler in `notification_routes.go`, and no matching RPC
   call anywhere in `services/api-gateway`:
   ```
   $ grep -rn "nsubscribe" backend-go/services/api-gateway --include="*.go"
   (no matches outside "Subscribe")
   ```
   Check whether `notification-service`'s proto
   (`proto/orca/notification/v1/`) even has an `Unsubscribe` RPC before
   fixing this — if it doesn't, this is a service-level gap, not just a
   missing route.

---

## References

- `specs/frontend/api/http-endpoints.md` — `## Web push (/api/push-*)`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/notification_routes.go` — `mountNotificationRoutes`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go` — `NewRouter` (auth group placement)
- `backend-go/proto/orca/notification/v1/` — check for an `Unsubscribe` RPC
