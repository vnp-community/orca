# TASK-CLI-03-01: `apiclient.GetHealth` — remote health-check client

**From Solution:** SOL-CLI-03
**Priority:** P0 — `orca daemon status`'s remote mode (TASK-CLI-03-04) depends on this
**Service:** `orca-cli`
**File:** `backend-go/cmd/orca-cli/internal/apiclient/health.go`
**Depends on:** TASK-CLI-01-06 (scaffold — `apiclient.Client`)
**Status:** `[ ]` TODO

---

## Context

`orca daemon status` in remote/GitOps mode is a read-only health check against `api-gateway`'s already-real `/healthz`/`/readyz` (`backend-go/common/health/health.go`), the same signal Kubernetes liveness/readiness probes use — not a new health concept invented for this CLI.

## Changes to make

`common/health/health.go` confirms both endpoints are status-code-only, no body worth parsing: `/healthz` always `200` while the process is up; `/readyz` is `200` when every registered `Checker` passes, `503` (with a `{name: "ok"|err.Error()}` JSON body, not needed here) when any fails. `probeOK` below reads only the status code, matching that contract exactly.

`backend-go/cmd/orca-cli/internal/apiclient/health.go`:

```go
package apiclient

import "context"

// HealthResult is GetHealth's answer — Healthy reflects /healthz's 200/non-200
// status, Ready reflects /readyz's (a readyz checker failing, e.g. DB
// unreachable, is Ready=false, distinct from Healthy=false/unreachable).
type HealthResult struct {
	Healthy bool
	Ready   bool
}

// GetHealth calls {api-url}/healthz then {api-url}/readyz. A network
// failure (gateway completely unreachable) returns an error — distinct
// from a reachable gateway reporting unhealthy/not-ready, which returns a
// HealthResult with Healthy/Ready false and a nil error.
func (c *Client) GetHealth(ctx context.Context) (HealthResult, error) {
	healthy, err := c.probeOK(ctx, "/healthz")
	if err != nil {
		return HealthResult{}, err
	}
	ready, err := c.probeOK(ctx, "/readyz")
	if err != nil {
		return HealthResult{}, err
	}
	return HealthResult{Healthy: healthy, Ready: ready}, nil
}

// probeOK issues an unauthenticated GET (health endpoints are not behind
// authMiddleware) and reports whether the response was 2xx — a
// non-2xx/non-network-error response is a valid "not healthy/not ready"
// answer, not a Go error.
func (c *Client) probeOK(ctx context.Context, path string) (bool, error) {
	req, err := newGetRequest(ctx, c.baseURL+path)
	if err != nil {
		return false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err // genuine network failure — gateway unreachable
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}
```

Add a small `newGetRequest` helper (or inline `http.NewRequestWithContext(ctx, "GET", url, nil)`) alongside `client.go`'s existing `do` method if one doesn't already exist there.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./cmd/orca-cli/...
go test ./cmd/orca-cli/internal/apiclient/... -run TestGetHealth -v
```

Expected new test `health_test.go` against an `httptest.Server` fake `/healthz`/`/readyz` pair: `200`/`200` -> `{Healthy:true, Ready:true}`; `200`/`503` (a `readyz` checker failing) -> `{Healthy:true, Ready:false}`, a distinct outcome from the gateway being fully unreachable (connection refused -> non-nil error, not a `HealthResult`).
