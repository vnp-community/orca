// channels_agent.go registers the agent.* wscompat channels (AI-CLI agent
// session control-plane) against infra-fleet-service's agent-session gRPC
// surface (TASK-AG-01..05). agent.start/agent.resume/agent.switchAccount's
// output/exit notifications reuse the SAME AttachPty stream mechanism
// terminal.create already sets up (agent.spawn's ptyId is attachable
// exactly like a plain PTY's, per agent-spawner.ts's agent.output/
// agent.exited notifications) — this file reuses
// terminalStreamRegistry/drainAttachPtyOutput from channels_terminal.go
// rather than introducing a second push path. New file per this repo's
// naming discipline — agent sessions are a distinct concept from generic
// terminals.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// registerAgentChannels wires every agent.* channel this pass gives real
// backend-go logic to. Called from RegisterRealChannels (channels.go). bus
// is nil-able — see registerAgentStatusSubscribeChannel's doc comment for
// what happens when NATS isn't configured.
func registerAgentChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient, bus *commoneventbus.Consumer) {
	registerAgentStartChannel(r, client)
	registerAgentStopChannel(r, client)
	registerAgentKillChannel(r, client)
	registerAgentResumeChannel(r, client)
	registerAgentSwitchAccountChannel(r, client)
	registerAgentStatusSubscribeChannel(r, bus)
}

// agentSessionView is the JSON shape sent back to the renderer on
// agent.start/agent.resume/agent.switchAccount's ack frame.
type agentSessionView struct {
	ID                 string `json:"id"`
	PtyID              string `json:"ptyId"`
	WorktreeID         string `json:"worktreeId"`
	DevServerID        string `json:"devServerId"`
	UserID             string `json:"userId"`
	ModelID            string `json:"modelId"`
	AccountID          string `json:"accountId"`
	Status             string `json:"status"`
	StartedAtUnixMs    int64  `json:"startedAtUnixMs"`
	LastActiveAtUnixMs int64  `json:"lastActiveAtUnixMs"`
}

func toAgentSessionView(s *infrafleetv1.AgentSession) agentSessionView {
	return agentSessionView{
		ID: s.GetId(), PtyID: s.GetPtyId(), WorktreeID: s.GetWorktreeId(), DevServerID: s.GetDevServerId(),
		UserID: s.GetUserId(), ModelID: s.GetModelId(), AccountID: s.GetAccountId(), Status: s.GetStatus(),
		StartedAtUnixMs: s.GetStartedAtUnixMs(), LastActiveAtUnixMs: s.GetLastActiveAtUnixMs(),
	}
}

// attachAgentPtyStream opens the same long-lived AttachPty stream
// terminal.create uses (see attachContext's doc comment in
// channels_terminal.go for why it's NOT derived from the invoke's own ctx)
// for an agent session's ptyId, and starts drainAttachPtyOutput to forward
// its output/exit as push events. Shared by agent.start/agent.resume/
// agent.switchAccount — the three channels that mint a NEW agent pty and
// must attach to it.
func attachAgentPtyStream(client infrafleetv1.InfraFleetServiceClient, id Identity, streams *terminalStreamRegistry, ptyID, opDesc string) (<-chan PushEvent, error) {
	streamCtx, cancel := attachContext(id)
	stream, err := client.AttachPty(streamCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("wscompat: opening AttachPty stream for %s agent pty %q: %w", opDesc, ptyID, err)
	}
	if err := stream.Send(&infrafleetv1.PtyClientFrame{
		Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: ptyID}},
	}); err != nil {
		cancel()
		return nil, fmt.Errorf("wscompat: sending AttachPty's initial attach frame for %s agent pty %q: %w", opDesc, ptyID, err)
	}

	entry := &terminalStreamEntry{stream: stream, cancel: cancel}
	streams.put(ptyID, entry)

	events := make(chan PushEvent)
	go drainAttachPtyOutput(streamCtx, ptyID, entry, streams, events)
	return events, nil
}

// ── agent.start ─────────────────────────────────────────────────────────

type agentStartArgs struct {
	ConnectionID string `json:"connectionId"`
	WorktreeID   string `json:"worktreeId"`
	UserID       string `json:"userId"`
	Cwd          string `json:"cwd"`
	ModelID      string `json:"modelId"`
	AccountID    string `json:"accountId"`
	TrustPreset  string `json:"trustPreset"`
	Cols         int32  `json:"cols"`
	Rows         int32  `json:"rows"`
}

func registerAgentStartChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStreamChannel("agent.start", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
		in, err := decodeArg[agentStartArgs](args, 0)
		if err != nil {
			return nil, nil, err
		}
		streams := terminalStreamsFromContext(ctx)
		if streams == nil {
			return nil, nil, errNoTerminalStreamRegistry
		}

		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		session, err := client.StartAgentSession(invokeCtx, &infrafleetv1.StartAgentSessionRequest{
			ConnectionId: in.ConnectionID,
			WorktreeId:   in.WorktreeID,
			UserId:       in.UserID,
			Cwd:          in.Cwd,
			ModelId:      in.ModelID,
			AccountId:    in.AccountID,
			TrustPreset:  in.TrustPreset,
			Cols:         in.Cols,
			Rows:         in.Rows,
		})
		if err != nil {
			return nil, nil, err
		}

		events, err := attachAgentPtyStream(client, id, streams, session.GetPtyId(), "new")
		if err != nil {
			return nil, nil, err
		}
		return toAgentSessionView(session), events, nil
	})
}

// ── agent.stop / agent.kill ─────────────────────────────────────────────
//
// Neither needs RegisterStreamChannel (no new stream to open) — plain
// Registry.Register is enough, same as any other one-shot RPC-backed
// channel.

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

// ── agent.resume ────────────────────────────────────────────────────────

type agentResumeArgs struct {
	ConnectionID string `json:"connectionId"`
	WorktreeID   string `json:"worktreeId"`
	UserID       string `json:"userId"`
	Cwd          string `json:"cwd"`
	Cols         int32  `json:"cols"`
	Rows         int32  `json:"rows"`
}

func registerAgentResumeChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStreamChannel("agent.resume", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
		in, err := decodeArg[agentResumeArgs](args, 0)
		if err != nil {
			return nil, nil, err
		}
		streams := terminalStreamsFromContext(ctx)
		if streams == nil {
			return nil, nil, errNoTerminalStreamRegistry
		}
		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		session, err := client.ResumeAgentSession(invokeCtx, &infrafleetv1.ResumeAgentSessionRequest{
			ConnectionId: in.ConnectionID, WorktreeId: in.WorktreeID, UserId: in.UserID,
			Cwd: in.Cwd, Cols: in.Cols, Rows: in.Rows,
		})
		if err != nil {
			return nil, nil, err
		}
		// Same AttachPty-stream setup as agent.start — a resumed session's
		// ptyId is attachable exactly like a fresh one's.
		events, err := attachAgentPtyStream(client, id, streams, session.GetPtyId(), "resumed")
		if err != nil {
			return nil, nil, err
		}
		return toAgentSessionView(session), events, nil
	})
}

// ── agent.switchAccount ─────────────────────────────────────────────────

type agentSwitchAccountArgs struct {
	ConnectionID string `json:"connectionId"`
	WorktreeID   string `json:"worktreeId"`
	UserID       string `json:"userId"`
	ProjectID    string `json:"projectId"`
	Cwd          string `json:"cwd"`
}

func registerAgentSwitchAccountChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStreamChannel("agent.switchAccount", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
		in, err := decodeArg[agentSwitchAccountArgs](args, 0)
		if err != nil {
			return nil, nil, err
		}
		streams := terminalStreamsFromContext(ctx)
		if streams == nil {
			return nil, nil, errNoTerminalStreamRegistry
		}
		invokeCtx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		session, err := client.SwitchAgentAccount(invokeCtx, &infrafleetv1.SwitchAgentAccountRequest{
			ConnectionId: in.ConnectionID, WorktreeId: in.WorktreeID, UserId: in.UserID,
			ProjectId: in.ProjectID, Cwd: in.Cwd,
		})
		if err != nil {
			return nil, nil, err
		}
		// Same AttachPty-stream setup as agent.start/agent.resume — the new
		// session's ptyId is attachable exactly like any other.
		events, err := attachAgentPtyStream(client, id, streams, session.GetPtyId(), "switched")
		if err != nil {
			return nil, nil, err
		}
		return toAgentSessionView(session), events, nil
	})
}

// ── agent.subscribeStatus ───────────────────────────────────────────────
//
// Pure push-only channel (no ack payload the caller needs), same shape as
// notifications.subscribe — see channels_push.go's precedent and
// registry.go's StreamHandler doc comment for why this uses
// Registry.RegisterStream rather than RegisterStreamChannel.
//
// bus is nil when api-gateway's NATS connection failed at startup (see
// main.go's degrade-not-crash handling, mirroring every other
// optional-NATS service in this codebase) — subscribing then simply
// returns a channel that is closed immediately, so a renderer that calls
// agent.subscribeStatus just never receives push events instead of the
// whole invoke erroring or panicking.
func registerAgentStatusSubscribeChannel(r *Registry, bus *commoneventbus.Consumer) {
	r.RegisterStream("agent.subscribeStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		events := make(chan PushEvent)
		if bus == nil {
			close(events)
			return events, nil
		}
		go func() {
			defer close(events)
			_ = bus.SubscribeEphemeral(ctx, "INFRA", "orca.infra.agent.statusChanged", func(ctx context.Context, ev commoneventbus.Event) error {
				if ev.TenantID != id.TenantID {
					return nil // tenant isolation — see tenant-service's consumer.go for the established pattern
				}
				select {
				case events <- PushEvent{Channel: "agent.statusChanged", Args: []any{ev.Payload}}:
				case <-ctx.Done():
				}
				return nil
			})
		}()
		go func() {
			_ = bus.SubscribeEphemeral(ctx, "INFRA", "orca.infra.agent.rateLimited", func(ctx context.Context, ev commoneventbus.Event) error {
				if ev.TenantID != id.TenantID {
					return nil
				}
				select {
				case events <- PushEvent{Channel: "agent:rateLimited", Args: []any{ev.Payload}}:
				case <-ctx.Done():
				}
				return nil
			})
		}()
		return events, nil
	})
}
