// channels_terminal.go registers the terminal.* wscompat channels
// (SSH/dev-server PTY control-plane) against infra-fleet-service's
// Terminal/PTY gRPC surface added by TASK-180..185.
//
// # What's MISSING here: the push bridge (TASK-012)
//
// terminal.create opens infra-fleet-service's bidirectional AttachPty
// stream, which pushes PtyServerFrame values (terminal output bytes, exit)
// for as long as the session lives — there is no request/response boundary
// to hang those on. wscompat's ONLY dispatch primitive today is the plain
// ChannelHandler (Registry.Register/Dispatch, see registry.go): one invoke
// in, one result out, nothing more. The push-capable machinery this file
// would need — StreamChannelHandler, PushEvent, pipePush, and
// Registry.RegisterStream — is TASK-012's deliverable and was CONFIRMED
// ABSENT from this worktree before writing this file (grepped registry.go
// and handler.go for all four names: no hits). Per this pass's explicit
// instructions, registry.go/handler.go/channels.go/cmd/server/main.go are
// not edited here, and none of those four names are invented in this file
// either.
//
// Until TASK-012 lands, drainAttachPtyOutput (below) reads every
// PtyServerFrame the stream produces and DISCARDS it — see that function's
// doc comment. terminal.create's synchronous ack (the spawned
// TerminalSession) still works, and terminal.send/resize/close/etc. still
// drive the session server-side, but no terminal OUTPUT reaches the browser
// through this pass.
//
// ONE-LINE CHANGE ONCE TASK-012 LANDS: inside drainAttachPtyOutput's for
// loop, replace the discard with a call to whatever push helper TASK-012
// adds (this doc comment assumes `pipePush(id, "terminal.output", view)`,
// per this pass's own instructions) — the stream-reading loop itself does
// not need to change.
//
// # Per-pty-id stream registry — deviation from the (missing) task spec
//
// This file's instructions called for keying terminalStreamRegistry
// per-WebSocket-connection "via a context key". That is not reachable from
// here: Handler.ServeHTTP (which this pass may not edit) never attaches any
// such key to the context it hands to Registry.Dispatch, and a plain
// ChannelHandler has no other way to learn which WS connection it is being
// called on. Since every terminal.* channel call after terminal.create
// already carries pty_id (the id terminal.create's ack returns), this file
// keys terminalStreamRegistry by pty_id instead: globally unique, always
// present on every subsequent call, sufficient for correct routing without
// fabricating a fake per-connection identity. The trade-off: a stream is
// only ever cleaned up by an explicit terminal.close, never automatically
// on WS disconnect (there is no connection-lifecycle hook reachable from
// this package) — tracked as a known gap alongside the push-bridge one
// above, both to be revisited once TASK-012's primitives (and whatever
// connection-lifecycle hook ships with them) land.
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

// terminalStreamEntry is one live AttachPty stream — sendMu serializes
// stream.Send calls, since a grpc.ClientStream forbids concurrent senders
// (mirrors Handler.ServeHTTP's own writeMu discipline for the same
// underlying reason, see handler.go).
type terminalStreamEntry struct {
	stream infrafleetv1.InfraFleetService_AttachPtyClient
	cancel context.CancelFunc

	sendMu sync.Mutex
}

func (e *terminalStreamEntry) send(frame *infrafleetv1.PtyClientFrame) error {
	e.sendMu.Lock()
	defer e.sendMu.Unlock()
	return e.stream.Send(frame)
}

// terminalStreamRegistry maps pty_id -> its live AttachPty stream — see this
// file's package doc comment for why pty_id, not a per-connection context
// key.
type terminalStreamRegistry struct {
	mu      sync.Mutex
	streams map[string]*terminalStreamEntry
}

func newTerminalStreamRegistry() *terminalStreamRegistry {
	return &terminalStreamRegistry{streams: make(map[string]*terminalStreamEntry)}
}

func (r *terminalStreamRegistry) put(ptyID string, entry *terminalStreamEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams[ptyID] = entry
}

func (r *terminalStreamRegistry) get(ptyID string) (*terminalStreamEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.streams[ptyID]
	return e, ok
}

// remove deletes and returns ptyID's entry, if any — terminal.close's
// cleanup path.
func (r *terminalStreamRegistry) remove(ptyID string) (*terminalStreamEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.streams[ptyID]
	delete(r.streams, ptyID)
	return e, ok
}

// registerTerminalChannels wires every terminal.* channel this pass gives
// real backend-go logic to. Called from main.go's composition root
// alongside RegisterRealChannels — NOT called from within this file (per
// this pass's instructions, cmd/server/main.go is not edited here); wiring
// the one extra call site is a one-line follow-up.
func registerTerminalChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	streams := newTerminalStreamRegistry()

	registerTerminalCreateChannel(r, client, streams)
	registerTerminalSendChannel(r, streams)
	registerTerminalResizeChannel(r, client)
	registerTerminalCloseChannel(r, client, streams)
	registerTerminalStopChannel(r, client)
	registerTerminalListChannel(r, client)
	registerTerminalWaitChannel(r, client)
	registerTerminalFocusChannel(r, client)
	registerTerminalAgentStatusChannels(r, client)
	registerTerminalInspectProcessChannel(r, client)
}

// terminalSessionView is the wire shape terminal.create/terminal.list
// return — mirrors TerminalSession's proto fields with unix-millisecond
// timestamps (JSON-friendlier than a Timestamp message).
type terminalSessionView struct {
	PtyID        string `json:"ptyId"`
	ConnectionID string `json:"connectionId"`
	Cwd          string `json:"cwd"`
	CreatedAt    int64  `json:"createdAt"`
	LastActiveAt int64  `json:"lastActiveAt"`
}

func toTerminalSessionView(s *infrafleetv1.TerminalSession) terminalSessionView {
	return terminalSessionView{
		PtyID:        s.GetPtyId(),
		ConnectionID: s.GetConnectionId(),
		Cwd:          s.GetCwd(),
		CreatedAt:    s.GetCreatedAtUnixMs(),
		LastActiveAt: s.GetLastActiveAtUnixMs(),
	}
}

// attachContext builds the long-lived, identity-stamped context an AttachPty
// stream needs. Deliberately NOT derived from the invoke's own ctx: every
// wscompat invoke is bounded by handler.go's 25s invokeTimeout
// (handleInvoke wraps ctx in context.WithTimeout before calling
// Registry.Dispatch) — an AttachPty stream inherited from that ctx would be
// killed 25 seconds after terminal.create, mid-session, for every terminal
// pane. context.Background() plus the same identity metadata
// gatewaygrpc.AttachIdentity stamps on every other outbound call keeps the
// stream alive for the pty's real lifetime instead.
func attachContext(id Identity) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}), cancel
}

// ── terminal.create ─────────────────────────────────────────────────────

type terminalCreateArgs struct {
	ConnectionID string `json:"connectionId"`
	Cwd          string `json:"cwd"`
	Shell        string `json:"shell"`
	Cols         int32  `json:"cols"`
	Rows         int32  `json:"rows"`
}

func registerTerminalCreateChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient, streams *terminalStreamRegistry) {
	r.Register("terminal.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalCreateArgs](args, 0)
		if err != nil {
			return nil, err
		}

		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		spawnResp, err := client.SpawnTerminalSession(invokeCtx, &infrafleetv1.SpawnTerminalSessionRequest{
			ConnectionId: in.ConnectionID,
			Cwd:          in.Cwd,
			Shell:        in.Shell,
			Cols:         in.Cols,
			Rows:         in.Rows,
		})
		if err != nil {
			return nil, err
		}
		session := spawnResp.GetSession()

		// See attachContext's doc comment: the stream MUST outlive this
		// invoke's own 25s deadline.
		streamCtx, cancel := attachContext(id)
		stream, err := client.AttachPty(streamCtx)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("wscompat: opening AttachPty stream for pty %q: %w", session.GetPtyId(), err)
		}
		if err := stream.Send(&infrafleetv1.PtyClientFrame{
			Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: session.GetPtyId()}},
		}); err != nil {
			cancel()
			return nil, fmt.Errorf("wscompat: sending AttachPty's initial attach frame for pty %q: %w", session.GetPtyId(), err)
		}

		entry := &terminalStreamEntry{stream: stream, cancel: cancel}
		streams.put(session.GetPtyId(), entry)
		go drainAttachPtyOutput(session.GetPtyId(), entry)

		return toTerminalSessionView(session), nil
	})
}

// drainAttachPtyOutput reads every PtyServerFrame the agent pushes for one
// pty session until the stream ends (terminal.close, the agent process
// exiting, or a transport error) and — TODO(push-bridge): forward via
// pipePush once TASK-012 lands, see this file's package doc comment for the
// exact one-line change — for now just discards each frame so the stream is
// still drained (a gRPC client stream that nobody reads from can eventually
// back up/stall server-side sends).
func drainAttachPtyOutput(ptyID string, entry *terminalStreamEntry) {
	for {
		frame, err := entry.stream.Recv()
		if err != nil {
			return // stream ended — io.EOF on a clean close, or a real transport error either way
		}
		switch frame.GetFrame().(type) {
		case *infrafleetv1.PtyServerFrame_Out:
			// TODO(push-bridge): forward via pipePush once TASK-012 primitives land.
			_ = frame.GetOut().GetData()
		case *infrafleetv1.PtyServerFrame_Exited:
			// TODO(push-bridge): forward via pipePush once TASK-012 primitives land.
			_ = frame.GetExited().GetExitCode()
		}
		_ = ptyID
	}
}

// ── terminal.send ───────────────────────────────────────────────────────

type terminalSendArgs struct {
	PtyID string `json:"ptyId"`
	Data  string `json:"data"`
}

func registerTerminalSendChannel(r *Registry, streams *terminalStreamRegistry) {
	r.Register("terminal.send", func(_ context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalSendArgs](args, 0)
		if err != nil {
			return nil, err
		}
		entry, ok := streams.get(in.PtyID)
		if !ok {
			return nil, fmt.Errorf("wscompat: no live AttachPty stream for pty %q — call terminal.create first", in.PtyID)
		}
		if err := entry.send(&infrafleetv1.PtyClientFrame{
			Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: []byte(in.Data)}},
		}); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

// ── terminal.resize ─────────────────────────────────────────────────────
//
// Uses the unary ResizeTerminalSession RPC rather than AttachPty's in-stream
// PtyResize frame — simpler (no dependency on terminalStreamRegistry state)
// and the proto explicitly documents the in-stream frame as "the low-latency
// ALTERNATIVE to the unary RPC", i.e. the unary path is the primary one.

type terminalResizeArgs struct {
	PtyID string `json:"ptyId"`
	Cols  int32  `json:"cols"`
	Rows  int32  `json:"rows"`
}

func registerTerminalResizeChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.resize", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalResizeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.ResizeTerminalSession(ctx, &infrafleetv1.ResizeTerminalSessionRequest{PtyId: in.PtyID, Cols: in.Cols, Rows: in.Rows})
		return nil, err
	})
}

// ── terminal.close ──────────────────────────────────────────────────────

type terminalPtyIDArg struct {
	PtyID string `json:"ptyId"`
}

func registerTerminalCloseChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient, streams *terminalStreamRegistry) {
	r.Register("terminal.close", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalPtyIDArg](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, killErr := client.KillTerminalSession(ctx, &infrafleetv1.KillTerminalSessionRequest{PtyId: in.PtyID})

		if entry, ok := streams.remove(in.PtyID); ok {
			entry.cancel() // ends drainAttachPtyOutput's Recv loop and the underlying stream
		}
		return nil, killErr
	})
}

// ── terminal.stop ───────────────────────────────────────────────────────

func registerTerminalStopChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.stop", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalPtyIDArg](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.StopTerminalProcess(ctx, &infrafleetv1.StopTerminalProcessRequest{PtyId: in.PtyID})
		return nil, err
	})
}

// ── terminal.list ───────────────────────────────────────────────────────

type terminalListArgs struct {
	ConnectionID string `json:"connectionId"`
}

func registerTerminalListChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, _ := decodeArg[terminalListArgs](args, 0) // no args ("list everything") is valid, so ignore a decode error

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ListTerminalSessions(ctx, &infrafleetv1.ListTerminalSessionsRequest{ConnectionId: in.ConnectionID})
		if err != nil {
			return nil, err
		}
		views := make([]terminalSessionView, 0, len(resp.GetSessions()))
		for _, s := range resp.GetSessions() {
			views = append(views, toTerminalSessionView(s))
		}
		return views, nil
	})
}

// ── terminal.wait ───────────────────────────────────────────────────────

type terminalWaitArgs struct {
	PtyID     string `json:"ptyId"`
	TimeoutMs int32  `json:"timeoutMs"`
}

type terminalWaitView struct {
	Exited   bool  `json:"exited"`
	ExitCode int32 `json:"exitCode"`
	TimedOut bool  `json:"timedOut"`
}

func registerTerminalWaitChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.wait", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalWaitArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.WaitTerminalSession(ctx, &infrafleetv1.WaitTerminalSessionRequest{PtyId: in.PtyID, TimeoutMs: in.TimeoutMs})
		if err != nil {
			return nil, err
		}
		return terminalWaitView{Exited: resp.GetExited(), ExitCode: resp.GetExitCode(), TimedOut: resp.GetTimedOut()}, nil
	})
}

// ── terminal.focus ──────────────────────────────────────────────────────

func registerTerminalFocusChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.focus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalPtyIDArg](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.FocusTerminalSession(ctx, &infrafleetv1.FocusTerminalSessionRequest{PtyId: in.PtyID})
		return nil, err
	})
}

// ── terminal.agentStatus / terminal.isRunningAgent ──────────────────────
//
// Both channels back the same GetTerminalAgentStatus RPC (see that
// message's proto doc comment) — terminal.isRunningAgent just projects the
// one boolean field terminal.agentStatus's richer view also carries.

type terminalAgentStatusView struct {
	AgentRunning  bool   `json:"agentRunning"`
	AgentKind     string `json:"agentKind"`
	ReadyForInput bool   `json:"readyForInput"`
}

func registerTerminalAgentStatusChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	getStatus := func(ctx context.Context, id Identity, ptyID string) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		return client.GetTerminalAgentStatus(ctx, &infrafleetv1.GetTerminalAgentStatusRequest{PtyId: ptyID})
	}

	r.Register("terminal.agentStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalPtyIDArg](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := getStatus(ctx, id, in.PtyID)
		if err != nil {
			return nil, err
		}
		return terminalAgentStatusView{AgentRunning: resp.GetAgentRunning(), AgentKind: resp.GetAgentKind(), ReadyForInput: resp.GetReadyForInput()}, nil
	})

	r.Register("terminal.isRunningAgent", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalPtyIDArg](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := getStatus(ctx, id, in.PtyID)
		if err != nil {
			return nil, err
		}
		return resp.GetAgentRunning(), nil
	})
}

// ── terminal.inspectProcess ─────────────────────────────────────────────

type terminalInspectProcessView struct {
	Known   bool   `json:"known"`
	Pid     int32  `json:"pid"`
	Command string `json:"command"`
	Cwd     string `json:"cwd"`
}

func registerTerminalInspectProcessChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.inspectProcess", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalPtyIDArg](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.InspectTerminalProcess(ctx, &infrafleetv1.InspectTerminalProcessRequest{PtyId: in.PtyID})
		if err != nil {
			return nil, err
		}
		return terminalInspectProcessView{Known: resp.GetKnown(), Pid: resp.GetPid(), Command: resp.GetCommand(), Cwd: resp.GetCwd()}, nil
	})
}
