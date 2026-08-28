// Package webhook implements usecase.WebhookAlerter — best-effort webhook
// delivery on a dev-server health status change, gated by the
// FLEET_WEBHOOK_URL env var (empty = disabled). BL-FLEET-03 also mentions a
// per-server webhook override; that's out of scope for this pass (only the
// service-wide env var is implemented) — flagged as a follow-up.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// Alerter implements usecase.WebhookAlerter.
type Alerter struct {
	url    string // FLEET_WEBHOOK_URL, empty = disabled
	client *http.Client
}

// NewAlerter builds an Alerter. A nil client defaults to a 5s-timeout
// http.Client — webhook delivery must never hang PollFleetHealth's poll
// tick indefinitely.
func NewAlerter(url string, client *http.Client) *Alerter {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Alerter{url: url, client: client}
}

// NotifyStatusChange POSTs BL-FLEET-03's status_change webhook shape.
// Best-effort: a delivery failure (including a non-2xx response) is
// logged, never returned — this must not fail the poll tick that
// triggered it (see usecase.WebhookAlerter's doc comment).
func (a *Alerter) NotifyStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus, sample domain.DevServerHealth) {
	if a.url == "" {
		return
	}
	body, err := json.Marshal(map[string]any{
		"event": "fleet.server.status_change", "server": ds.Host,
		"from": from, "to": to, "timestamp": time.Now().UTC().Format(time.RFC3339),
		"metrics": map[string]any{"cpu": sample.CPUPercent, "ram": sample.RAMPercent},
	})
	if err != nil {
		slog.Default().WarnContext(ctx, "webhook: marshal payload failed", slog.Any("error", err))
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(body))
	if err != nil {
		slog.Default().WarnContext(ctx, "webhook: building request failed", slog.Any("error", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		slog.Default().WarnContext(ctx, "webhook: status-change delivery failed", slog.Any("error", err))
		return
	}
	_ = resp.Body.Close()
	// A non-2xx response (e.g. 500) is logged, not treated as an error the
	// caller must handle — best-effort delivery, matching the "webhook
	// server returning 500 does not propagate an error to the caller"
	// contract.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Default().WarnContext(ctx, "webhook: status-change delivery returned non-2xx", slog.Int("statusCode", resp.StatusCode))
	}
}
