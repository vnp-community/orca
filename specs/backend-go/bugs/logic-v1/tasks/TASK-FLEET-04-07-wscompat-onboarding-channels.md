# TASK-FLEET-04-07: `devServer.detectAgents`/`devServer.preflightCheck` WS channels + real `toDevServerView` fields

**From Solution:** SOL-FLEET-04
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-FLEET-04-06, TASK-FLEET-02-06 (real `DevServer.status`)
**Status:** `[ ]` TODO

---

## Context

Closes the field gap this file's own doc comment flags: "none of the
frontend's ... platform/arch/nodeVersion ... fields exist server-side yet"
(`channels.go:334-337`). `toDevServerView` stops hardcoding `Platform`/
`Arch`/`NodeVersion` to `nil` and `status` to `"disconnected"`
(`channels.go:382`), and two new WS channels expose the two new RPCs from
TASK-FLEET-04-06. The existing `preflight.check` channel
(`channels.go:565-573`) is unrelated and untouched — it answers a different
question ("can the browser's own backend host run gh/glab").

## Changes to make

In `toDevServerView` (`channels.go:377-388`), replace the hardcoded
`nil`/`"disconnected"` placeholders with the real proto fields:

```go
func toDevServerView(ds *infrafleetv1.DevServer) devServerView {
    return devServerView{
        // ... existing fields unchanged ...
        Platform:    nullableString(ds.GetPlatform()),    // was: nil, hardcoded
        Arch:        nullableString(ds.GetArch()),        // was: nil, hardcoded
        NodeVersion: nullableString(ds.GetNodeVersion()),  // was: nil, hardcoded
        Status:      ds.GetStatus(),                       // was: "disconnected", hardcoded
    }
}

// nullableString returns nil for an empty string so a DevServer that
// predates this migration (zero-value fields) still renders as an honest
// nil placeholder, not an empty string.
func nullableString(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}
```

Add two new channel handlers, following this file's existing
`preflight.check`/`fleet.health.checkAll` handler shape:

```go
// "devServer.detectAgents" — calls InfraFleetServiceClient.DetectDevServerAgents
func handleDevServerDetectAgents(client infrafleetv1.InfraFleetServiceClient) channelHandler {
    return func(ctx context.Context, params json.RawMessage) (any, error) {
        var req struct{ DevServerID string `json:"devServerId"` }
        if err := json.Unmarshal(params, &req); err != nil {
            return nil, err
        }
        resp, err := client.DetectDevServerAgents(ctx, &infrafleetv1.DetectDevServerAgentsRequest{DevServerId: req.DevServerID})
        if err != nil {
            return nil, err
        }
        return map[string]any{"agents": resp.GetAgents(), "platform": resp.GetPlatform()}, nil
    }
}

// "devServer.preflightCheck" — calls InfraFleetServiceClient.CheckDevServerPreflight
func handleDevServerPreflightCheck(client infrafleetv1.InfraFleetServiceClient) channelHandler {
    return func(ctx context.Context, params json.RawMessage) (any, error) {
        var req struct {
            DevServerID string `json:"devServerId"`
            ProbePort   int32  `json:"probePort"`
        }
        if err := json.Unmarshal(params, &req); err != nil {
            return nil, err
        }
        resp, err := client.CheckDevServerPreflight(ctx, &infrafleetv1.CheckDevServerPreflightRequest{DevServerId: req.DevServerID, ProbePort: req.ProbePort})
        if err != nil {
            return nil, err
        }
        return resp, nil
    }
}
```

Register both handlers in this file's channel-dispatch table alongside the
existing `devServer.*`/`preflight.*` entries.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run 'TestToDevServerView|TestDevServerDetectAgents|TestDevServerPreflightCheck' -v
```

Expected: `toDevServerView` maps real (non-nil) platform/arch/nodeVersion/
status fields when present, falls back to the existing honest-nil
placeholders when a `DevServer` predates this migration (zero-value
fields); the two new channels marshal request/response correctly against a
fake gRPC client.
