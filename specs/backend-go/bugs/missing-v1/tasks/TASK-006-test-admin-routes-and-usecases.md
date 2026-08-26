# TASK-006: Tests for admin usecases + `/admin/api/*` routes

**From Solution:** SOL-001
**Priority:** P1
**Service:** `auth-service` + `api-gateway`
**File:** `services/auth-service/internal/usecase/*_test.go` (new), `services/api-gateway/internal/adapter/httpgateway/admin_routes_test.go` (new)
**Depends on:** TASK-002, TASK-003, TASK-004, TASK-005
**Status:** `[x]` DONE — all specified usecase tests (deactivate/reactivate round-trip, versioning + stale-version precondition failure, admin-stats counts) and all `admin_routes_test.go` route tests including the `/admin/api/audit` vs `/v1/auth/audit-log` contract-parity test are present and pass.

---

## Tests to add

### `auth-service`

- `deactivate_user_test.go` — deactivate then reactivate round-trips `is_active`.
- `access_policy_versioning_test.go` — `UpdateAccessPolicy` called twice →
  2 rows persisted, `version` goes 1 → 2, `GetAccessPolicy`/`ListAccessPolicies`
  return only the latest version per `id`. Also: calling `UpdateAccessPolicy`
  with a stale `expected_version` returns `FailedPrecondition`, not a
  silent overwrite.
- `get_admin_stats_test.go` — counts reflect seeded fixture data exactly.

### `api-gateway`

`admin_routes_test.go` — one test per route, mirroring
`auth_admin_routes_test.go`'s existing shape (fake gRPC client, assert
status code + body shape):

```go
func TestAdminRoutes_Stats(t *testing.T) {
    fake := &fakeAuthClient{statsResp: &authv1.GetAdminStatsResponse{TotalUsers: 3, ActiveSessions: 5, TotalPolicies: 2}}
    r := chi.NewRouter()
    mountAdminRoutes(r, fake)
    req := httptest.NewRequest(http.MethodGet, "/admin/api/stats", nil).WithContext(withIdentity(context.Background(), fakeIdentity))
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK { t.Fatalf("want 200, got %d", w.Code) }
    // assert body decodes to {totalUsers:3, activeSessions:5, totalPolicies:2}
}
```

Repeat for `users` (GET/POST/PATCH/DELETE), `sessions` (GET/DELETE, plus
kill-all), `policies` (GET/POST/PUT/DELETE), `audit` (GET).

### Contract regression test

Assert `/admin/api/audit` and `/v1/auth/audit-log` resolve to the same
`QueryAuditLog` RPC and return byte-identical response shapes for the same
fake client response — guards against the two REST surfaces drifting
apart later.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/auth-service/internal/usecase/... -run "Deactivate|AccessPolicy|AdminStats" -v
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestAdminRoutes -v
go build ./... && go vet ./...
```
