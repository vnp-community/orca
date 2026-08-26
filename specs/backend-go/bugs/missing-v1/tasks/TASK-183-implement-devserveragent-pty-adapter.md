# TASK-183: Implement PTY methods on `adapter/devserveragent.Client` — the adapter over the existing agent wire protocol

**From Solution:** SOL-029 (design part 2: "the adapter-over-existing-agent-wire-protocol implementation")
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `internal/adapter/devserveragent/methods.go` (new), `internal/adapter/devserveragent/client.go`, `internal/adapter/devserveragent/session.go`
**Depends on:** TASK-181
**Status:** `[partial]` — PTY adapter implemented and tested (30/30 passing in `internal/adapter/devserveragent`, incl. new coverage for: SpawnPty failing loudly on a pty.create response missing `id`, WritePty/ResizePty/KillPty sending the exact `{id,...}` param shape over the wire, StreamPty's notification demux correctly isolating two concurrently-subscribed ptyIds from each other, StreamPty's forwarding goroutine closing its output channel on ctx cancellation, and session-level `routeNotification`/`subscribePty`/`unsubscribePty` unit tests — matching-ptyId-only routing, channel closed + map entry removed after unsubscribe, and non-blocking drop when a subscriber's channel is full). AgentStatus/InspectProcess remain a best-effort heuristic layered on the confirmed `pty.listProcesses` RPC (no dedicated per-pty status/inspect primitive exists in the confirmed catalog; `ReadyForInput` is naively `== AgentRunning`, `Pid` is always 0) and StopTerminalProcess-via-Ctrl-C-over-WritePty-style gaps remain unverified against the real agent RPC surface (`agent/src/relay/pty-agent-bridge.ts`, `pty-handler.ts`) — out of scope for this backend-go-only pass, `agent/` changes excluded per architecture docs.

---

## Context

Per `08-inter-service-communication.md`'s "Talking to the Dev Server Agent"
section, **Option A is the explicit default**: preserve the existing wire
protocol, no `agent/` changes. Every new `DevServerAgentClient` PTY method
is implemented as a call through the **same** 13-byte-framed JSON-RPC
transport `client.go`'s existing `Exec`/`Health` already use — not a new
protocol. `StreamPty` is the one genuinely new capability: today
`session.go`'s `readLoop` (lines 226-229) silently discards any inbound
JSON-RPC message that isn't a response to a pending request ("a
notification or malformed payload — this client issues no
onRequest/onNotification handlers yet"). This task adds a notification-demux
layer so PTY output/exit notifications route to per-`ptyId` subscriber
channels instead of being dropped — the one place this task touches
`session.go`'s read loop, and it is additive (a new branch, not a change to
the existing request/response path).

## Changes to make

### Step 1 — `internal/adapter/devserveragent/session.go`: add notification routing

Add fields to `session` (near `pending map[uint32]*pendingCall`):

```go
	// notifSubs routes inbound JSON-RPC notifications (no response expected)
	// to subscribers keyed by the value of their params.ptyId field — used
	// by subscribeNotifications/StreamPty (methods.go). A notification for
	// a ptyId with no subscriber is dropped, same as before this field
	// existed.
	notifSubs map[string]chan json.RawMessage
```

Initialize it in `newSession`:

```go
func newSession(host string, cfg Config, logger *slog.Logger) *session {
	return &session{
		cfg:         cfg,
		host:        host,
		logger:      logger,
		pending:     make(map[uint32]*pendingCall),
		notifSubs:   make(map[string]chan json.RawMessage),
		nextFrameID: 1,
		closeCh:     make(chan struct{}),
	}
}
```

In `readLoop`, find:

```go
		resp, ok, err := ParseJSONRPCResponse(decoded.Payload)
		if err != nil || !ok {
			continue // a notification or malformed payload — this client issues no onRequest/onNotification handlers yet (see README "Known gaps")
		}
```

Replace with:

```go
		resp, ok, err := ParseJSONRPCResponse(decoded.Payload)
		if err != nil || !ok {
			// Not a response to a pending call — try routing it as a
			// notification (e.g. pty output/exit) before dropping it.
			s.routeNotification(decoded.Payload)
			continue
		}
```

Add the new method near `readLoop`:

```go
// notificationEnvelope is the minimal shape needed to route a JSON-RPC
// notification by its params.ptyId — mirrors the agent's pty output/exit
// notification shape (agent/src/relay/pty-handler.ts's output-forwarding
// side). Confirm the real field name against that file before finalizing
// (flagged the same way methods.go's ptyMethodName lookup is).
type notificationEnvelope struct {
	Method string `json:"method"`
	Params struct {
		PtyID string `json:"ptyId"`
	} `json:"params"`
}

// routeNotification demuxes an inbound JSON-RPC notification to its
// subscriber by params.ptyId, if one is registered. Silently drops
// anything with no subscriber or that doesn't parse as a notification —
// same as the pre-TASK-183 behavior, just no longer unconditional.
func (s *session) routeNotification(payload json.RawMessage) {
	var env notificationEnvelope
	if err := json.Unmarshal(payload, &env); err != nil || env.Params.PtyID == "" {
		return
	}
	s.mu.Lock()
	ch, ok := s.notifSubs[env.Params.PtyID]
	s.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- payload:
	default:
		// Subscriber's channel is full (StreamPty consumer stalled) —
		// drop rather than block readLoop, the same backpressure posture
		// call()'s resultCh (buffered 1) takes for responses.
	}
}

// subscribeNotifications registers ch to receive every notification whose
// params.ptyId == ptyID until ctx is cancelled, then unregisters and closes
// it. Returns immediately; the caller reads from the returned channel.
func (s *session) subscribeNotifications(ctx context.Context, ptyID string) <-chan json.RawMessage {
	ch := make(chan json.RawMessage, 32) // buffered: PTY output can burst faster than a slow consumer drains
	s.mu.Lock()
	s.notifSubs[ptyID] = ch
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		delete(s.notifSubs, ptyID)
		s.mu.Unlock()
		close(ch)
	}()
	return ch
}
```

### Step 2 — `internal/adapter/devserveragent/client.go`: add `StreamPty`

Add near `Exec`/`Health`:

```go
// StreamPty subscribes to output/exit notifications for one ptyId — see
// session.go's subscribeNotifications/routeNotification for the
// notification-demux layer this depends on.
func (c *Client) StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan usecase.PtyEvent, error) {
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return nil, err
	}
	raw := sess.subscribeNotifications(ctx, ptyID)
	out := make(chan usecase.PtyEvent)
	go func() {
		defer close(out)
		for payload := range raw {
			ev, ok := decodePtyNotification(ptyID, payload)
			if !ok {
				continue
			}
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
```

Add the `usecase` import to `client.go` (it does not import `usecase`
today — this is the first method to need it):

```go
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
```

Double check this does not create an import cycle: `usecase` must not
import `adapter/devserveragent` anywhere (it should not — `usecase` only
defines the `DevServerAgentClient` port, per this package's own doc
comment on why the port lives there). If `go build` reports a cycle,
`PtyEvent` needs to move to a small shared package instead — flag and
resolve before proceeding, do not silently restructure without noting it
in the PR.

### Step 3 — `internal/adapter/devserveragent/methods.go` (new file)

Typed wrappers for the specific agent RPC methods this service calls —
`pty.spawn`/`write`/`resize`/`kill`, `pty.agentStatus`, `pty.inspect` (or
whatever the real agent handler names are — see the flagged note below).
This file is also where Stack A vs Stack B method-name divergence
(`infra-fleet-service.md` §10: "`pty.create` vs `pty.spawn`") is absorbed
in one place, not per call site.

```go
// Package devserveragent — methods.go: typed wrappers over Client.Exec for
// the pty.* JSON-RPC methods, per infra-fleet-service.md §6's package-
// layout note. No call site outside this file should hardcode a "pty.*"
// method name string — see ptyMethodName's doc comment for why.
package devserveragent

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ptyMethodName resolves verb ("spawn"/"write"/"resize"/"kill") to the
// real JSON-RPC method name for devServer's negotiated Stack (A: local
// WS-connected dispatcher, B: SSH-deployed RelayDispatcher) —
// handshake.go already knows which Stack a session negotiated
// (infra-fleet-service.md:332). A single lookup table closes the class of
// bug §10 flags (gaps-and-findings.md's TS-side divergence bugs) by
// construction: no call site below hardcodes "pty.spawn" or "pty.create"
// directly.
//
// FLAGGED: the exact Stack A vs Stack B method names (and whether
// agentStatus/inspect have any agent-side equivalent at all) are not
// verified against agent/src/relay/pty-agent-bridge.ts /
// pty-handler.ts in this task — confirm against those files and correct
// this table before considering this method complete. The names below are
// this solution's best-effort default (Stack A == Stack B == "pty.<verb>"),
// deliberately naive until confirmed.
func (c *Client) ptyMethodName(devServer domain.DevServer, verb string) string {
	// TODO(confirm-against-agent): both stacks currently map to the same
	// name. Branch on devServer's negotiated stack here once §10's real
	// divergence is confirmed — do not let call sites below branch instead.
	return "pty." + verb
}

func (c *Client) SpawnPty(ctx context.Context, devServer domain.DevServer, cwd, shell string, cols, rows int32) (string, error) {
	result, err := c.Exec(ctx, devServer, c.ptyMethodName(devServer, "spawn"), map[string]any{
		"cwd": cwd, "shell": shell, "cols": cols, "rows": rows,
	})
	if err != nil {
		return "", err
	}
	ptyID, _ := result["ptyId"].(string)
	if ptyID == "" {
		return "", fmt.Errorf("devserveragent: pty.spawn response missing ptyId")
	}
	return ptyID, nil
}

func (c *Client) WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	_, err := c.Exec(ctx, devServer, c.ptyMethodName(devServer, "write"), map[string]any{
		"ptyId": ptyID, "data": base64.StdEncoding.EncodeToString(data),
	})
	return err
}

func (c *Client) ResizePty(ctx context.Context, devServer domain.DevServer, ptyID string, cols, rows int32) error {
	_, err := c.Exec(ctx, devServer, c.ptyMethodName(devServer, "resize"), map[string]any{
		"ptyId": ptyID, "cols": cols, "rows": rows,
	})
	return err
}

func (c *Client) KillPty(ctx context.Context, devServer domain.DevServer, ptyID string) error {
	_, err := c.Exec(ctx, devServer, c.ptyMethodName(devServer, "kill"), map[string]any{
		"ptyId": ptyID,
	})
	return err
}

// AgentStatus/InspectProcess degrade honestly when the agent can't answer:
// an "unknown method" style error from Exec is treated as (false/unknown,
// nil error), not propagated as a gRPC failure — matching channels.go's
// "best-effort... honest placeholder, not fabricated data" convention
// elsewhere in this codebase. FLAGGED: confirm the real method name and
// error shape against agent/src/relay/pty-agent-bridge.ts before removing
// this comment; isUnsupportedMethodError's implementation below is a
// placeholder string-match, not a typed sentinel, until that's confirmed.
func (c *Client) AgentStatus(ctx context.Context, devServer domain.DevServer, ptyID string) (running bool, kind string, ready bool, err error) {
	result, execErr := c.Exec(ctx, devServer, c.ptyMethodName(devServer, "agentStatus"), map[string]any{"ptyId": ptyID})
	if execErr != nil {
		if isUnsupportedMethodError(execErr) {
			return false, "", false, nil
		}
		return false, "", false, execErr
	}
	running, _ = result["running"].(bool)
	kind, _ = result["kind"].(string)
	ready, _ = result["readyForInput"].(bool)
	return running, kind, ready, nil
}

func (c *Client) InspectProcess(ctx context.Context, devServer domain.DevServer, ptyID string) (known bool, pid int32, command, cwd string, err error) {
	result, execErr := c.Exec(ctx, devServer, c.ptyMethodName(devServer, "inspect"), map[string]any{"ptyId": ptyID})
	if execErr != nil {
		if isUnsupportedMethodError(execErr) {
			return false, 0, "", "", nil
		}
		return false, 0, "", "", execErr
	}
	pidFloat, _ := result["pid"].(float64) // JSON numbers decode as float64
	command, _ = result["command"].(string)
	cwd, _ = result["cwd"].(string)
	return true, int32(pidFloat), command, cwd, nil
}

// isUnsupportedMethodError is a placeholder heuristic — FLAGGED for
// confirmation against the agent's real JSON-RPC error code for "unknown
// method" (JSONRPCError.Code, jsonrpc.go) before this is trustworthy.
func isUnsupportedMethodError(err error) bool {
	type coded interface{ Code() int }
	if c, ok := err.(coded); ok {
		return c.Code() == -32601 // JSON-RPC 2.0's standard "Method not found"
	}
	return false
}

// decodePtyNotification parses one raw pty output/exit notification into
// usecase.PtyEvent — FLAGGED: the exact notification shape (method name,
// params field names for output bytes vs exit code) is not verified
// against agent/src/relay/pty-handler.ts's output-forwarding side. Best
// guess below: method "pty.output" carries base64 data, method "pty.exit"
// carries exitCode.
func decodePtyNotification(ptyID string, payload []byte) (out usecase.PtyEvent, ok bool) {
	var env struct {
		Method string `json:"method"`
		Params struct {
			Data     string `json:"data"`
			ExitCode int32  `json:"exitCode"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return usecase.PtyEvent{}, false
	}
	switch env.Method {
	case "pty.output":
		data, err := base64.StdEncoding.DecodeString(env.Params.Data)
		if err != nil {
			return usecase.PtyEvent{}, false
		}
		return usecase.PtyEvent{PtyID: ptyID, Output: data}, true
	case "pty.exit":
		return usecase.PtyEvent{PtyID: ptyID, Exited: true, ExitCode: env.Params.ExitCode}, true
	default:
		return usecase.PtyEvent{}, false
	}
}
```

`decodePtyNotification` needs `"encoding/json"` and the `usecase` import —
add both to `methods.go`'s import block (it currently only has
`context`/`encoding/base64`/`fmt`/`domain` in the sketch above).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go build ./internal/adapter/devserveragent/... ./internal/usecase/...
go vet ./internal/adapter/devserveragent/...
```

Expected: `*Client` now satisfies `usecase.DevServerAgentClient` in full —
`go build` on any package constructing a `usecase.DevServerAgentClient`
from `*devserveragent.Client` (there are none yet outside tests/composition
root) should compile cleanly once TASK-185 wires it into `cmd/server/main.go`.

**Before merging:** resolve every `FLAGGED` comment above against
`agent/src/relay/pty-agent-bridge.ts` / `agent/src/relay/pty-handler.ts`'s
actual RPC surface and notification shape — this task's code is a
best-effort default per SOL-029's own explicit flag, not verified against
the real agent.
