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
