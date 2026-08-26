# TASK-186: Wire all 10 `terminal.*` `wscompat` channels — `AttachPty` piped into the push bridge

**From Solution:** SOL-029 (design part 4: "`wscompat` wiring including piping into the push bridge")
**Priority:** P1
**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_terminal.go` (new), `internal/adapter/wscompat/registry.go`, `internal/adapter/wscompat/handler.go`, `cmd/server/main.go`
**Depends on:** TASK-185, TASK-012 (`add-push-bridge-primitives.md` — `StreamHandler`/`PushEvent`/`pipePush`, not yet implemented at time of writing this task)
**Status:** `[partial]` — downgraded from a prior `[x]` DONE, see the correction note below: the ack+push infrastructure this task built is real and correct, but its wire format (`terminal.output`/`terminal.exited` JSON push channels) doesn't match the real frontend's `terminal.multiplex` binary protocol — `terminal.create` is now a real push-capable channel: `registry.go` adds `StreamChannelHandler`/`Registry.RegisterStreamChannel`/`Registry.DispatchStreamChannel` (a parallel registration path alongside the existing `ChannelHandler`/`Register`/`Dispatch` and the pre-existing ack-less `StreamHandler`/`RegisterStream`/`StreamHandlerFor` used by `notifications.subscribe` — none of those three are removed or altered in behavior), and `handler.go`'s `handleInvoke` checks `DispatchStreamChannel` first: it acks with the invoke's own result (e.g. `terminal.create`'s `TerminalSession`) exactly like an ordinary invoke, then — only after that ack is written, matching SOL-035/TASK-014's ack-before-stream ordering — starts `pipePush(ctx, conn, writeMu, events)` on the connection's own lifetime `ctx` (not the 25s-bounded `dispatchCtx`). `drainAttachPtyOutput` (`channels_terminal.go`) now forwards every `PtyServerFrame` instead of discarding it: `PtyServerFrame_Out` → `PushEvent{Channel:"terminal.output", Args:[{ptyId,data}]}`, `PtyServerFrame_Exited` → a final `PushEvent{Channel:"terminal.exited", Args:[{ptyId,exitCode}]}` then returns; the events channel is closed (via `defer`) on ANY stream end — explicit exit, `terminal.close`'s `cancel()`, or a transport error — so `pipePush`'s read loop always terminates instead of hanging, and a blocked event send unblocks via the stream's own `ctx.Done()`. `terminalStreamRegistry` is now genuinely per-WS-connection: threaded via `terminalStreamsContext`/`terminalStreamsFromContext` (a context key, exactly as this task's Step 5 specifies), constructed fresh once per connection in `Handler.ServeHTTP` (`handler.go`) — `registerTerminalChannels`'s signature is unchanged (`(r *Registry, client infrafleetv1.InfraFleetServiceClient)`), so `channels.go`/`main.go` needed no edits, matching this task's Step 6 prediction. Verified end-to-end (not just build-passes) by `channels_terminal_test.go`, including a real `Handler.ServeHTTP`/`pipePush` integration test and a same-`ptyId`-two-connections isolation regression test — see TASK-187 for the full list. `go build ./... && go vet ./... && go test ./... -race` clean across the whole `api-gateway` module.
>
> ⚠️ **Correction — downgraded from DONE, protocol mismatch confirmed:** a follow-up trace through the actual old backend (`backend/src/main/runtime/rpc/methods/terminal.ts`) and shipped frontend (`frontend/src/renderer/src/runtime/remote-runtime-terminal-multiplexer.ts`, `web-preload-api.ts`, `web-runtime-client.ts`/`web-session-client.ts`) found that `terminal.output`/`terminal.exited` as named JSON push channels are **not the real contract** — SOL-029's design (lines 590-592) was written against `specs/frontend/api/rpc-catalog.md`'s `terminal.*` table, which never lists the method the real frontend actually uses: `terminal.multiplex`, a *streaming RPC* (`defineStreamingMethod`, not a push-channel subscribe) whose payloads are raw **binary**, opcode-tagged frames (`TerminalStreamOpcode.Output/Resized/Error/Metadata/...`, `backend/src/shared/terminal-stream-protocol.ts`) demultiplexed client-side by a client-assigned `streamId` — not `{"type":"push","channel":"terminal.output",...}` JSON consumed via `client.on(channel,...)`. A repo-wide grep for the literal strings `"terminal.output"`/`"terminal.exited"` across `backend/`, `frontend/`, and `desktop/` returns zero matches anywhere outside backend-go's own (now-corrected-to-flag) implementation and tests.
>
> What this means: the ack+push registration mechanism this task built (`StreamChannelHandler`/`RegisterStreamChannel`/`DispatchStreamChannel`, per-connection `terminalStreamRegistry` via `terminalStreamsContext`, `pipePush` on the connection-lifetime ctx) is sound infrastructure and should be kept — but `drainAttachPtyOutput`'s actual wire format (JSON push events named `terminal.output`/`terminal.exited`) needs to be replaced with a `terminal.multiplex` handler that speaks the binary `TerminalStreamOpcode` framing over the same connection, the same way `runtime.subscribeToTerminalData`/`sendFrame`/`sendBinary` do in the old backend. That is real, unstarted implementation work (a new binary-framing wire layer), not a wiring gap — leaving this **`[partial]`**, not DONE, until that lands.
>
> See the investigation's full file list for follow-up: `backend/src/main/runtime/rpc/methods/terminal.ts` (terminal.create:1287, terminal.multiplex:1510), `backend/src/shared/terminal-stream-protocol.ts`, `frontend/src/renderer/src/runtime/remote-runtime-terminal-multiplexer.ts`, `frontend/src/renderer/src/web/web-preload-api.ts:1156`, `frontend/src/renderer/src/web/web-runtime-client.ts`, `web-session-client.ts`, `specs/frontend/tdd/v5/03-runtime-client-layer.md` §6, and `specs/frontend/api/rpc-catalog.md`'s `terminal.*` table (which itself needs a `terminal.multiplex` row added — a doc gap, not just a backend-go gap).
>
> Everything else this task built (ack-before-stream ordering, per-connection registry fix closing the same-`ptyId`-cross-connection-collision bug, `drainAttachPtyOutput` no longer silently discarding frames, clean stream-end/error propagation) is real, verified, and worth keeping regardless of the wire-format fix above — this is a "wrong last-mile format," not a wasted pass.

---

## Context

`terminal.create` is shaped like TASK-014's `notifications.subscribe`
precedent (SOL-035): an `invoke` call whose job is to both return a value
(the new `ptyId`) **and** start a stream. TASK-012 defines
`StreamHandler`/`PushEvent`/`pipePush` in `push_bridge.go` — this task
reuses those primitives verbatim, adding only the one thing `AttachPty`
needs beyond `StreamNotifications`'s server-only-streaming shape: a
per-WS-connection registry of open `AttachPty` client streams, so
`terminal.send`/`terminal.resize`'s follow-up `invoke` calls on the SAME
connection can find and write to the stream `terminal.create` opened.

`registry.go`'s `ChannelHandler`/`Register` (invoke/send) are untouched —
`RegisterStream` is a new, parallel registration path alongside them, not a
replacement.

## Changes to make

### Step 1 — `internal/adapter/wscompat/registry.go`: add `RegisterStream`

Add near the existing `ChannelHandler` type and `Register` method:

```go
// StreamChannelHandler is a channel whose invoke ALSO opens a push
// subscription — e.g. terminal.create both acks with {ptyId} and starts
// terminal.output/terminal.exited push frames. ack is the invoke's own
// ResultMessage.Result (same as an ordinary ChannelHandler's return); events
// is piped by pipePush (push_bridge.go, TASK-012) until ctx is cancelled or
// the channel closes.
type StreamChannelHandler func(ctx context.Context, id Identity, args []json.RawMessage) (ack any, events <-chan PushEvent, err error)

// streamHandlers holds every StreamChannelHandler, parallel to handlers —
// added as a second map (not folded into ChannelHandler's signature) so
// every existing invoke/send handler is untouched by this change.
//
// Add a `streamHandlers map[string]StreamChannelHandler` field to Registry
// and initialize it in NewRegistry alongside `handlers`.

// RegisterStream adds or replaces the stream handler for channel.
func (r *Registry) RegisterStream(channel string, h StreamChannelHandler) {
	r.streamHandlers[channel] = h
}

// DispatchStream resolves and invokes the stream handler for channel, if
// one is registered. ok=false means channel has no StreamChannelHandler —
// the caller should fall back to the ordinary Dispatch path.
func (r *Registry) DispatchStream(ctx context.Context, id Identity, channel string, args []json.RawMessage) (ack any, events <-chan PushEvent, ok bool, err error) {
	h, found := r.streamHandlers[channel]
	if !found {
		return nil, nil, false, nil
	}
	ack, events, err = h(ctx, id, args)
	return ack, events, true, err
}
```

`PushEvent` is defined by TASK-012 in `push_bridge.go` (same package,
`wscompat`) — no import needed, just don't redefine it here.

### Step 2 — `internal/adapter/wscompat/handler.go`: check `DispatchStream` before `Dispatch`

Find `handleInvoke`'s dispatch line:

```go
	result, err := h.Registry.Dispatch(dispatchCtx, identity, msg.Channel, msg.Args)
```

Replace with:

```go
	ack, events, isStream, err := h.Registry.DispatchStream(dispatchCtx, identity, msg.Channel, msg.Args)
	var result any
	if isStream {
		result = ack
		if err == nil && events != nil {
			// pipePush (push_bridge.go, TASK-012) runs for the lifetime of
			// the WS connection (ctx, not dispatchCtx — the stream must
			// outlive this single invoke's 25s dispatch window). Started
			// AFTER the ack path below writes the result frame, matching
			// TASK-014's "ack first, then start pipePush" ordering — see
			// that task for why ack-before-stream avoids a race where a
			// push frame could arrive before the client has even seen the
			// ptyId it needs to associate the push with.
			go pipePush(ctx, conn, writeMu, events)
		}
	} else {
		result, err = h.Registry.Dispatch(dispatchCtx, identity, msg.Channel, msg.Args)
	}
```

This introduces `ctx`/`conn`/`writeMu` references inside `handleInvoke` —
confirm they're already in scope (they are: `handleInvoke`'s existing
signature is `(ctx context.Context, conn *websocket.Conn, writeMu
*sync.Mutex, identity Identity, msg InboundMessage)`), so no signature
change is needed here, only the body edit above. The rest of
`handleInvoke` (the `err != nil` / success write-back) is untouched — it
already writes `result` into `ResultMessage.Result`.

### Step 3 — New file `internal/adapter/wscompat/channels_terminal.go`

```go
// ── terminal.* (infra-fleet-service) ─────────────────────────────────────
//
// terminal.create is a StreamChannelHandler: it both acks with the new
// TerminalSession AND opens infra-fleet-service's AttachPty stream,
// translating PtyServerFrame into PushEvent via pipeAttachPtyToPush —
// SOL-035's pipePush (TASK-012) is reused verbatim for actually writing
// those as `push` frames back to the client.
//
// r.terminalStreams is a per-WS-connection registry (map[ptyId]
// infrafleetv1.InfraFleetService_AttachPtyClient, mutex-guarded) — created
// once per connection (see terminalStreamRegistry below) so terminal.send/
// terminal.resize, arriving as separate invoke calls on the SAME
// connection, can find the stream terminal.create opened. Scoped to one
// WS connection's lifetime, same principle push_bridge.go establishes for
// its own push-piping — this is the minimal addition beyond that shape,
// not a cross-connection registry.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// terminalStreamRegistry is one WS connection's open AttachPty streams.
// Constructed once in ServeHTTP (handler.go) alongside writeMu, passed to
// registerTerminalChannels' closures via Identity-independent capture (see
// wiring note at the bottom of this file for exactly where it's
// constructed — this type intentionally has no package-level instance).
type terminalStreamRegistry struct {
	mu      sync.Mutex
	streams map[string]infrafleetv1.InfraFleetService_AttachPtyClient
}

func newTerminalStreamRegistry() *terminalStreamRegistry {
	return &terminalStreamRegistry{streams: make(map[string]infrafleetv1.InfraFleetService_AttachPtyClient)}
}

func (r *terminalStreamRegistry) register(ptyID string, stream infrafleetv1.InfraFleetService_AttachPtyClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams[ptyID] = stream
}

func (r *terminalStreamRegistry) get(ptyID string) (infrafleetv1.InfraFleetService_AttachPtyClient, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.streams[ptyID]
	return s, ok
}

func (r *terminalStreamRegistry) close(ptyID string) {
	r.mu.Lock()
	stream, ok := r.streams[ptyID]
	delete(r.streams, ptyID)
	r.mu.Unlock()
	if ok {
		_ = stream.CloseSend()
	}
}

func registerTerminalChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient, streams *terminalStreamRegistry) {
	type createArgs struct {
		ConnectionID string `json:"connectionId"`
		Cwd          string `json:"cwd"`
		Shell        string `json:"shell"`
		Cols         int32  `json:"cols"`
		Rows         int32  `json:"rows"`
	}

	r.RegisterStream("terminal.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.SpawnTerminalSession(ctx, &infrafleetv1.SpawnTerminalSessionRequest{
			ConnectionId: in.ConnectionID, Cwd: in.Cwd, Shell: in.Shell, Cols: in.Cols, Rows: in.Rows,
		})
		if err != nil {
			return nil, nil, err
		}
		ptyID := resp.GetSession().GetPtyId()

		// Open AttachPty now, not lazily on first send — output must start
		// flowing immediately even before the caller's first terminal.send.
		stream, err := client.AttachPty(ctx)
		if err != nil {
			return nil, nil, err
		}
		if err := stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: ptyID}}}); err != nil {
			return nil, nil, err
		}
		streams.register(ptyID, stream)

		events := make(chan PushEvent)
		go pipeAttachPtyToPush(stream, ptyID, events)
		return resp.GetSession(), events, nil
	})

	r.Register("terminal.send", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type sendArgs struct {
			PtyID string `json:"ptyId"`
			Data  []byte `json:"data"` // JSON string is base64-decoded into []byte automatically by encoding/json
		}
		in, err := decodeArg[sendArgs](args, 0)
		if err != nil {
			return nil, err
		}
		stream, ok := streams.get(in.PtyID)
		if !ok {
			return nil, fmt.Errorf("terminal session %q has no open stream on this connection", in.PtyID)
		}
		if err := stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: in.Data}}}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("terminal.resize", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resizeArgs struct {
			PtyID string `json:"ptyId"`
			Cols  int32  `json:"cols"`
			Rows  int32  `json:"rows"`
		}
		in, err := decodeArg[resizeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		// Prefer the in-stream resize frame if a stream is open (lower
		// latency); fall back to the unary RPC otherwise (e.g. a resize
		// that races terminal.create's ack).
		if stream, ok := streams.get(in.PtyID); ok {
			if err := stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Resize{Resize: &infrafleetv1.PtyResize{Cols: in.Cols, Rows: in.Rows}}}); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := client.ResizeTerminalSession(ctx, &infrafleetv1.ResizeTerminalSessionRequest{PtyId: in.PtyID, Cols: in.Cols, Rows: in.Rows}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("terminal.close", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type closeArgs struct {
			PtyID string `json:"ptyId"`
		}
		in, err := decodeArg[closeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.KillTerminalSession(ctx, &infrafleetv1.KillTerminalSessionRequest{PtyId: in.PtyID})
		streams.close(in.PtyID)
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("terminal.stop", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[struct {
			PtyID string `json:"ptyId"`
		}](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.StopTerminalProcess(ctx, &infrafleetv1.StopTerminalProcessRequest{PtyId: in.PtyID})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("terminal.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, _ := decodeArg[struct {
			ConnectionID string `json:"connectionId"`
		}](args, 0)
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ListTerminalSessions(ctx, &infrafleetv1.ListTerminalSessionsRequest{ConnectionId: in.ConnectionID})
		if err != nil {
			return nil, err
		}
		return resp.GetSessions(), nil
	})

	r.Register("terminal.wait", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[struct {
			PtyID     string `json:"ptyId"`
			TimeoutMs int32  `json:"timeoutMs"`
		}](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.WaitTerminalSession(ctx, &infrafleetv1.WaitTerminalSessionRequest{PtyId: in.PtyID, TimeoutMs: in.TimeoutMs})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("terminal.focus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[struct {
			PtyID string `json:"ptyId"`
		}](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.FocusTerminalSession(ctx, &infrafleetv1.FocusTerminalSessionRequest{PtyId: in.PtyID})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("terminal.agentStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[struct {
			PtyID string `json:"ptyId"`
		}](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetTerminalAgentStatus(ctx, &infrafleetv1.GetTerminalAgentStatusRequest{PtyId: in.PtyID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("terminal.isRunningAgent", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[struct {
			PtyID string `json:"ptyId"`
		}](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetTerminalAgentStatus(ctx, &infrafleetv1.GetTerminalAgentStatusRequest{PtyId: in.PtyID})
		if err != nil {
			return nil, err
		}
		// Projects the SAME RPC's response down to one bool — see this
		// file's package doc comment for why isRunningAgent isn't a
		// separate RPC.
		return map[string]bool{"running": resp.GetAgentRunning()}, nil
	})

	r.Register("terminal.inspectProcess", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[struct {
			PtyID string `json:"ptyId"`
		}](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.InspectTerminalProcess(ctx, &infrafleetv1.InspectTerminalProcessRequest{PtyId: in.PtyID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// pipeAttachPtyToPush reads PtyServerFrame from stream and forwards each as
// a PushEvent until the stream ends — the thin translator feeding pipePush
// (push_bridge.go, TASK-012), which does the actual `push`-frame writing
// serialized through writeMu. Closes events when done.
func pipeAttachPtyToPush(stream infrafleetv1.InfraFleetService_AttachPtyClient, ptyID string, events chan<- PushEvent) {
	defer close(events)
	for {
		frame, err := stream.Recv()
		if err != nil {
			return
		}
		switch f := frame.Frame.(type) {
		case *infrafleetv1.PtyServerFrame_Out:
			events <- PushEvent{Channel: "terminal.output", Args: map[string]any{"ptyId": ptyID, "data": f.Out.GetData()}}
		case *infrafleetv1.PtyServerFrame_Exited:
			events <- PushEvent{Channel: "terminal.exited", Args: map[string]any{"ptyId": ptyID, "exitCode": f.Exited.GetExitCode()}}
			return
		}
	}
}
```

### Step 4 — `channels.go`: grow `RegisterRealChannels`

Add a `*terminalStreamRegistry` parameter and the
`registerTerminalChannels` call:

```go
func RegisterRealChannels(
	r *Registry,
	// ... existing params ...
	terminalStreams *terminalStreamRegistry,
) {
	// ... existing register*Channels calls ...
	registerTerminalChannels(r, infraFleetClient, terminalStreams)
}
```

### Step 5 — `handler.go`: construct `terminalStreamRegistry` per connection

In `ServeHTTP`, alongside `var writeMu sync.Mutex`, add:

```go
	terminalStreams := newTerminalStreamRegistry()
```

This means `RegisterRealChannels` can no longer be called once at
`main.go`'s composition-root time with a single shared registry — it needs
a **per-connection** registry. Restructure: `RegisterRealChannels` continues
to register every OTHER channel once, globally, exactly as today; only
`registerTerminalChannels` needs a fresh `terminalStreamRegistry` per
connection. The straightforward fix: keep `registerTerminalChannels`
registered once per `Registry` as `RegisterRealChannels` already does
(passing a shared `*terminalStreamRegistry` is wrong — it would leak
`ptyId`s across unrelated users' connections), and instead thread the
per-connection registry through `Identity`... **do not do this** — Identity
is the wrong place for connection-scoped mutable state.

Correct fix: `terminalStreamRegistry` must be looked up from `ctx`, not
captured in the handler closure. In `ServeHTTP`, before the message loop,
wrap `ctx`:

```go
	ctx := terminalStreamsContext(r.Context(), newTerminalStreamRegistry())
```

Add to `channels_terminal.go`:

```go
type terminalStreamsCtxKey struct{}

func terminalStreamsContext(ctx context.Context, streams *terminalStreamRegistry) context.Context {
	return context.WithValue(ctx, terminalStreamsCtxKey{}, streams)
}

func terminalStreamsFromContext(ctx context.Context) *terminalStreamRegistry {
	streams, _ := ctx.Value(terminalStreamsCtxKey{}).(*terminalStreamRegistry)
	return streams
}
```

And change `registerTerminalChannels`'s handlers to call
`terminalStreamsFromContext(ctx)` instead of closing over a `streams`
parameter — drop the `streams *terminalStreamRegistry` parameter from
`registerTerminalChannels`'s signature and from Step 4's
`RegisterRealChannels` call entirely; each handler resolves it from `ctx`
at call time instead. This keeps `RegisterRealChannels` a one-time,
global-`Registry` call (matching every other channel in this file) while
still giving `terminal.*` genuinely per-connection stream state.

### Step 6 — `cmd/server/main.go`: no signature change needed

Since Step 5's context-based fix removes the extra `RegisterRealChannels`
parameter, `main.go`'s existing
`wscompat.RegisterRealChannels(wsCompatRegistry, ...)` call only needs
`infraFleetClient` already passed (it already is, for
`devServer.*`/`fleet.*`) — no further edit here.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
