# TASK-FLEET-03-07: `webhook.Alerter` — `FLEET_WEBHOOK_URL` POST on status change

**From Solution:** SOL-FLEET-03
**Priority:** P2
**Service:** `infra-fleet-service` (webhook adapter)
**File:** `backend-go/services/infra-fleet-service/internal/adapter/webhook/alerter.go` (new)
**Depends on:** TASK-FLEET-03-03
**Status:** `[ ]` TODO

---

## Context

Best-effort webhook delivery on a dev-server health status change, gated by
the `FLEET_WEBHOOK_URL` env var (empty = disabled). Only the service-wide
env var is implemented in this pass — a per-server webhook override (also
mentioned in BL-FLEET-03) is out of scope, flagged as a follow-up.

## Changes to make

```go
// internal/adapter/webhook/alerter.go
package webhook

type Alerter struct {
    url    string // FLEET_WEBHOOK_URL, empty = disabled
    client *http.Client
}

func NewAlerter(url string, client *http.Client) *Alerter {
    if client == nil {
        client = &http.Client{Timeout: 5 * time.Second}
    }
    return &Alerter{url: url, client: client}
}

func (a *Alerter) NotifyStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus, sample domain.DevServerHealth) {
    if a.url == "" {
        return
    }
    body, _ := json.Marshal(map[string]any{
        "event": "fleet.server.status_change", "server": ds.Host,
        "from": from, "to": to, "timestamp": time.Now().UTC().Format(time.RFC3339),
        "metrics": map[string]any{"cpu": sample.CPUPercent, "ram": sample.RAMPercent},
    })
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := a.client.Do(req)
    if err != nil {
        slog.Default().WarnContext(ctx, "webhook: status-change delivery failed", slog.Any("error", err))
        return
    }
    _ = resp.Body.Close()
}
```

Wire `usecase.WebhookAlerter` to this type at bootstrap
(`cmd/server/main.go`), reading `FLEET_WEBHOOK_URL` from config.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/webhook/... -run TestAlerter -v
```

Expected: an `httptest.Server` receives the exact JSON shape from
BL-FLEET-03; `FLEET_WEBHOOK_URL=""` sends nothing; a webhook server
returning 500 does not propagate an error to the caller.
