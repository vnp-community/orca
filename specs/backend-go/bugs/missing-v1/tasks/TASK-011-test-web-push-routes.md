# TASK-011: Tests for web push routes

**From Solution:** SOL-003
**Priority:** P1
**Service:** `api-gateway` + `notification-service`
**File:** `services/api-gateway/internal/adapter/httpgateway/notification_routes_test.go`, `services/notification-service/internal/usecase/unregister_push_subscription_test.go` (new)
**Depends on:** TASK-009, TASK-010
**Status:** `[x]` DONE — `unregister_push_subscription_test.go` and `notification_routes_test.go` (including the known-endpoint/unknown-endpoint 204 cases and the `TestPushRoutes_NoAuthRequired` route-placement regression guard) all exist and pass.

---

## Tests to add

- `unregister_push_subscription_test.go` — deletes the row for a known
  endpoint; re-deleting the same (already-gone) endpoint is a no-op, not
  an error.
- `notification_routes_test.go`:
  - `POST /api/push-unsubscribe` with a known endpoint → `204`.
  - `POST /api/push-unsubscribe` with an unknown endpoint → `204` (idempotent, not `404`).
  - **Route-placement regression test**: `GET /api/vapid-public-key` and
    `POST /api/push-subscribe` both succeed with **no** `orca_session`
    cookie set on the request — guards against these accidentally being
    remounted inside the authenticated group later.

```go
func TestPushRoutes_NoAuthRequired(t *testing.T) {
    r := chi.NewRouter()
    mountPushRoutes(r, fakeNotificationClient{})
    req := httptest.NewRequest(http.MethodGet, "/api/vapid-public-key", nil) // no cookie set
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code == http.StatusUnauthorized {
        t.Fatal("push routes must not require auth — regression against BUG-003")
    }
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/notification-service/internal/usecase/... -run TestUnregister -v
go test ./services/api-gateway/internal/adapter/httpgateway/... -run "TestPushRoutes|TestUnsubscribe" -v
```
