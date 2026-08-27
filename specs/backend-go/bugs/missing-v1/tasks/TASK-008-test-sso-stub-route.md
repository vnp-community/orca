# TASK-008: Test the `/auth/sso/:provider` stub route

**From Solution:** SOL-002
**Priority:** P2
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/httpgateway/auth_routes_test.go` (new, or append to existing)
**Depends on:** TASK-007
**Status:** `[x]` DONE — `TestAuthSSORoute_Returns501` and `TestAuthSSORoute_AnyProviderReturns501` exist in `auth_routes_test.go` and pass.

---

## Test to add

```go
func TestAuthSSORoute_Returns501(t *testing.T) {
    r := chi.NewRouter()
    mountAuthRoutes(r, fakeAuthClient{}, fakeCookieValidator{})
    req := httptest.NewRequest(http.MethodGet, "/auth/sso/google", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusNotImplemented {
        t.Errorf("want 501, got %d", w.Code)
    }
    var body map[string]string
    _ = json.NewDecoder(w.Body).Decode(&body)
    if body["code"] != "NOT_IMPLEMENTED" {
        t.Errorf("want code=NOT_IMPLEMENTED, got %q", body["code"])
    }
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/httpgateway/... -run TestAuthSSORoute -v
```
