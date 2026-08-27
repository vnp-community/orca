# TASK-AG-03-03: Route `agent.hook` notifications — `StreamAgentHooks` in `adapter/devserveragent`

**From Solution:** SOL-AG-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`, `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`
**Depends on:** TASK-AG-01-02
**Status:** `[ ]` TODO

---

## Context

`session.go`'s notification demux (`routeNotification`) today only routes `pty.data`/`pty.exit`/`pty.replay` (by `ptyID`) and `browser.screencastReady/Frame/Ended/Error` (by `subscriptionID`). `agent.hook` is a third, already-real notification (`agent/src/relay/agent-hook-server.ts`'s `forwardEvent`) this client has no handler for. `agent.hook` currently carries no `ptyId`/Orca session id (SOL-AG-03's "genuine gap", see TASK-AG-03-07) — so, unlike `pty.*`/`browser.*`, it cannot be demuxed by a caller-supplied key; this task fans it out to every subscriber on the session, unkeyed, mirroring `routeScreencastNotification`'s shape without its correlation key.

## Changes to make

In `session.go`, extend the demux switch:

```go
func (s *session) routeNotification(n JSONRPCNotification) {
	switch n.Method {
	case "pty.data", "pty.exit", "pty.replay":
		s.routePtyNotification(n)
	case "browser.screencastReady", "browser.screencastFrame", "browser.screencastEnded", "browser.screencastError":
		s.routeScreencastNotification(n)
	case "agent.hook":
		s.routeAgentHookNotification(n)
	default:
		return
	}
}
```

Add the hook-specific types, fan-out, and subscribe/unsubscribe (session-wide, not keyed — there is no correlation key yet, see TASK-AG-03-07):

```go
// rawAgentHookNotification is session.go's internal decoding of one
// agent.hook notification — RecordAgentHookProviderSession (usecase layer)
// wraps this into AgentHookEvent. providerSession fields are empty when the
// hook event carried none (not every hook fires one).
type rawAgentHookNotification struct {
	WorktreeID              string
	ProviderSessionKey      string
	ProviderSessionID       string
}

// agentHookNotificationParams mirrors agent-hook-server.ts's
// AgentHookRelayEnvelope's fields this client cares about — worktreeId for
// the correlation fallback (TASK-AG-03-05), providerSession.{key,id} for
// what to persist. ptyId does NOT exist on the wire yet — see TASK-AG-03-07.
type agentHookNotificationParams struct {
	WorktreeID      string `json:"worktreeId"`
	ProviderSession *struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	} `json:"providerSession"`
}

func (s *session) routeAgentHookNotification(n JSONRPCNotification) {
	var p agentHookNotificationParams
	if len(n.Params) > 0 {
		_ = json.Unmarshal(n.Params, &p)
	}
	raw := rawAgentHookNotification{WorktreeID: p.WorktreeID}
	if p.ProviderSession != nil {
		raw.ProviderSessionKey = p.ProviderSession.Key
		raw.ProviderSessionID = p.ProviderSession.ID
	}

	s.hookMu.Lock()
	subs := append([]chan rawAgentHookNotification(nil), s.hookSubs...)
	s.hookMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- raw:
		default: // slow/gone consumer — drop rather than block the read loop
		}
	}
}

// subscribeAgentHooks registers a new listener for every agent.hook
// notification on this session (unkeyed — see routeAgentHookNotification's
// doc comment). Exactly one long-lived subscriber per devServer connection
// is expected (usecase.RecordAgentHookProviderSession), not one per
// AgentSession.
func (s *session) subscribeAgentHooks() chan rawAgentHookNotification {
	ch := make(chan rawAgentHookNotification, 64)
	s.hookMu.Lock()
	s.hookSubs = append(s.hookSubs, ch)
	s.hookMu.Unlock()
	return ch
}

func (s *session) unsubscribeAgentHooks(ch chan rawAgentHookNotification) {
	s.hookMu.Lock()
	for i, c := range s.hookSubs {
		if c == ch {
			s.hookSubs = append(s.hookSubs[:i], s.hookSubs[i+1:]...)
			break
		}
	}
	s.hookMu.Unlock()
	close(ch)
}
```

Add the two new fields to the `session` struct, alongside `ptyMu`/`ptySubs`:

```go
	hookMu   sync.Mutex
	hookSubs []chan rawAgentHookNotification
```

In `usecase/ports.go`, add the exported event type next to `PtyEvent`, and
extend `DevServerAgentClient` with the new method:

```go
// AgentHookEvent is one agent.hook notification, decoded from
// agent-hook-server.ts's AgentHookRelayEnvelope (only the fields this
// service consumes) — see DevServerAgentClient.StreamAgentHooks.
type AgentHookEvent struct {
	WorktreeID         string
	ProviderSessionKey string
	ProviderSessionID  string
}
```

```go
	// StreamAgentHooks subscribes to every agent.hook notification on
	// devServer's persistent session — ONE long-lived subscription per
	// devServer connection (not per AgentSession, unlike StreamPty),
	// consumed by RecordAgentHookProviderSession.
	StreamAgentHooks(ctx context.Context, devServer domain.DevServer) (<-chan AgentHookEvent, func(), error)
```

In `client.go`, add the exported wrapper (mirrors `StreamPty`'s shape):

```go
// StreamAgentHooks subscribes to every agent.hook notification on
// devServer's persistent session — one long-lived subscription per
// devServer connection (not per AgentSession), consumed by
// usecase.RecordAgentHookProviderSession.
func (c *Client) StreamAgentHooks(ctx context.Context, devServer domain.DevServer) (<-chan usecase.AgentHookEvent, func(), error) {
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return nil, nil, err
	}
	raw := sess.subscribeAgentHooks()
	out := make(chan usecase.AgentHookEvent)
	go func() {
		defer close(out)
		for r := range raw {
			select {
			case out <- usecase.AgentHookEvent{WorktreeID: r.WorktreeID, ProviderSessionKey: r.ProviderSessionKey, ProviderSessionID: r.ProviderSessionID}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, func() { sess.unsubscribeAgentHooks(raw) }, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -run TestStreamAgentHooks -v
```

Add a test asserting: an `agent.hook` notification with a `providerSession`
payload is decoded and delivered to a subscriber; a notification with no
`providerSession` still delivers with empty `ProviderSessionKey/ID` (the
usecase layer, not this adapter, decides that's a no-op — see
TASK-AG-03-05).
