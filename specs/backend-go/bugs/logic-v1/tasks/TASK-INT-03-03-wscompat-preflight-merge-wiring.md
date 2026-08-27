# TASK-INT-03-03: Rewrite `preflight.check` to merge local + relay checks

**From Solution:** SOL-INT-03
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-INT-03-01
**Status:** `[ ]` TODO

---

## Context

Replaces `registerPreflightChannels`'s hardcoded local-only stub
(`channels.go:566-573`) with real relay checks fanned out over 3
already-wired RPCs (`GetFleetHealth`, `ScanWorkspacePorts`, `Relay` for
`github.auth.status`/`gitlab.auth.status`) merged via
`usecase.MergePreflightStatuses` (TASK-INT-03-01). Deliberately does NOT
call `preflight.check` on the agent itself — see SOL-INT-03's "genuine gap"
section for why (Part A/Part B shape divergence).

**Correction to SOL-INT-03's code sample**: `GetFleetHealthRequest` has a
required `tenant_id` field (`infrafleet.proto:222-224`) that the
solution's sample omits — this task's version fills it from
`id.TenantID`.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`,
replace `registerPreflightChannels` (`:566-573`) and its stale doc comment
(`:551-565`) with:

```go
// ── preflight.check ──────────────────────────────────────────────────────
//
// Merges a fixed local check set with relay checks fanned out over
// infra-fleet-service's already-wired GetFleetHealth/ScanWorkspacePorts/
// Relay RPCs — see SOL-INT-03. Deliberately does NOT call the agent's own
// preflight.check RPC: Part A and Part B disagree on that method's
// response shape (infra-fleet-service.md §10), so narrower, unambiguous
// RPCs are used instead.
func registerPreflightChannels(r *Registry, infraClient infrafleetv1.InfraFleetServiceClient) {
	r.Register("preflight.check", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type preflightArgs struct {
			ConnectionID string `json:"connectionId"` // empty = local-only, no relay attempted
		}
		in, _ := decodeArg[preflightArgs](args, 0)
		local := localPreflightChecks()
		if in.ConnectionID == "" {
			return usecase.MergePreflightStatuses(local, nil, nil), nil
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		relay, relayErr := runRelayPreflightChecks(rpcCtx, infraClient, id.TenantID, in.ConnectionID, id.UserID)
		return usecase.MergePreflightStatuses(local, relay, relayErr), nil
	})
}

// localPreflightChecks — unchanged from today's hardcoded literal:
// git-gateway-service's local executor requires the real git binary in its
// own container image, an infra guarantee, not a per-request probe; now
// explicitly labeled source:"local" rather than an undifferentiated map
// key.
func localPreflightChecks() []usecase.PreflightCheckResult {
	return []usecase.PreflightCheckResult{
		{ID: "git", Status: usecase.PreflightOK, Source: usecase.PreflightSourceLocal},
	}
}

// runRelayPreflightChecks fans out the 3 already-wired relay calls. A hard
// failure on GetFleetHealth (the connectivity-defining call) is treated as
// relay-unreachable for the whole batch — an auth-status-specific failure
// (e.g. domain.ErrAgentMethodNotFound on a relay-ssh Dev Server, see
// SOL-INT-01) is NOT connectivity-wide and instead produces one
// skip-status result for just that check, so a relay-ssh Dev Server still
// gets disk/port results merged in.
func runRelayPreflightChecks(ctx context.Context, client infrafleetv1.InfraFleetServiceClient, tenantID, connectionID, userID string) ([]usecase.PreflightCheckResult, error) {
	health, err := client.GetFleetHealth(ctx, &infrafleetv1.GetFleetHealthRequest{TenantId: tenantID})
	if err != nil {
		return nil, err // relay-connectivity warning, no partial results
	}
	var results []usecase.PreflightCheckResult
	for _, h := range health.GetStatuses() {
		results = append(results, usecase.PreflightCheckResult{
			ID: "disk-space", Source: usecase.PreflightSourceRelay,
			Status:  diskStatus(h.GetDiskPercent()),
			Message: fmt.Sprintf("%.0f%% disk used", h.GetDiskPercent()),
		})
	}

	if _, err := client.ScanWorkspacePorts(ctx, &infrafleetv1.ScanWorkspacePortsRequest{ConnectionId: connectionID}); err == nil {
		results = append(results, usecase.PreflightCheckResult{
			ID: "port-availability", Source: usecase.PreflightSourceRelay, Status: usecase.PreflightOK,
		})
	}

	for _, c := range []struct{ id, method string }{
		{"github-cli-auth", "github.auth.status"},
		{"gitlab-cli-auth", "gitlab.auth.status"},
	} {
		paramsJSON, _ := json.Marshal(map[string]any{"userId": userID})
		resp, err := client.Relay(ctx, &infrafleetv1.RelayRequest{ConnectionId: connectionID, Method: c.method, ParamsJson: string(paramsJSON)})
		if err != nil {
			// Per-check skip, not connectivity-wide — see this function's
			// doc comment. Covers SOL-INT-01's relay-ssh gap honestly.
			results = append(results, usecase.PreflightCheckResult{
				ID: c.id, Status: usecase.PreflightSkip, Message: "not available on this connection mode", Source: usecase.PreflightSourceRelay,
			})
			continue
		}
		var out struct {
			OK bool `json:"ok"`
		}
		_ = json.Unmarshal([]byte(resp.GetResultJson()), &out)
		status := usecase.PreflightError
		if out.OK {
			status = usecase.PreflightOK
		}
		results = append(results, usecase.PreflightCheckResult{ID: c.id, Status: status, Source: usecase.PreflightSourceRelay})
	}
	return results, nil
}

// diskStatus maps a raw disk-usage percentage to a PreflightStatus —
// thresholds are a starting point (>90% error, >75% warning), not tuned
// against a real fleet; revisit if these prove noisy in practice.
func diskStatus(percent float64) usecase.PreflightStatus {
	switch {
	case percent >= 90:
		return usecase.PreflightError
	case percent >= 75:
		return usecase.PreflightWarning
	default:
		return usecase.PreflightOK
	}
}
```

Update `RegisterRealChannels`'s call site (`:90`) from
`registerPreflightChannels(r)` to
`registerPreflightChannels(r, infraFleetClient)` — `infraFleetClient` is
already a `RegisterRealChannels` parameter, no new client needed.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/...
```

Expected: clean build/tests. Add to `channels_test.go` per SOL-INT-03's
test plan: `preflight.check` with no `connectionId` returns local-only
results, no RPC calls made; `GetFleetHealth` failing produces the
`relay-connectivity` warning and the local `git` check still present; a
`github.auth.status` relay failure (simulating a relay-ssh Dev Server)
produces a `skip`-status `github-cli-auth` result while
`disk-space`/`port-availability` still come back `ok`; every
`PreflightCheckResult` in a merged response has a non-empty `Source`
(regression guard against a future relay check forgetting to tag itself).
