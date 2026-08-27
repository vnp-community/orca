# TASK-AG-02-05: Register `agent.stop`/`agent.kill` wscompat channels

**From Solution:** SOL-AG-02
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go`
**Depends on:** TASK-AG-01-08, TASK-AG-02-04
**Status:** `[ ]` TODO

---

## Context

Extends `channels_agent.go` (TASK-AG-01-08) with the two teardown channels. Neither needs `RegisterStreamChannel` (no new stream to open) — plain `Registry.Register` is enough, same as any other one-shot RPC-backed channel.

## Changes to make

In `channels_agent.go`, extend `registerAgentChannels` and add the two handlers:

```go
func registerAgentChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	registerAgentStartChannel(r, client)
	registerAgentStopChannel(r, client)
	registerAgentKillChannel(r, client)
}

type agentStopArgs struct {
	SessionID string `json:"sessionId"`
}

func registerAgentStopChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("agent.stop", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[agentStopArgs](args, 0)
		if err != nil {
			return nil, err
		}
		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := client.StopAgentSession(invokeCtx, &infrafleetv1.StopAgentSessionRequest{SessionId: in.SessionID}); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
}

type agentKillArgs struct {
	SessionID string `json:"sessionId"`
	Signal    string `json:"signal"` // "" -> agent.kill defaults to SIGKILL server-side
}

func registerAgentKillChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("agent.kill", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[agentKillArgs](args, 0)
		if err != nil {
			return nil, err
		}
		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := client.KillAgentSession(invokeCtx, &infrafleetv1.KillAgentSessionRequest{SessionId: in.SessionID, Signal: in.Signal}); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
}
```

Confirm the exact `Registry.Register` signature (plain `ChannelHandler`, per
this package's doc comment distinguishing it from `RegisterStreamChannel`)
matches an existing one-shot channel elsewhere in `wscompat` (e.g.
`terminal.close`/`terminal.resize` in `channels_terminal.go`) and mirror
that signature exactly rather than the sketch above if they differ.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run 'TestAgentStop|TestAgentKill' -v
```
