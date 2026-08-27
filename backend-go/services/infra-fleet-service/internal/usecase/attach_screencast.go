package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// ScreencastClientMessage is the usecase-layer decoding of one inbound
// ScreencastClientFrame — adapter/grpc's AttachScreencast handler
// translates the proto oneof into this shape, mirroring
// PtyClientMessage/AttachPty's pattern exactly. Exactly one field is set.
type ScreencastClientMessage struct {
	Start *ScreencastStartMessage
	Stop  bool
}

// ScreencastStartMessage is ScreencastClientMessage's first-frame-only
// binding — every field ScreencastParams needs, already clamped by the
// caller (wscompat) before this usecase is ever reached.
type ScreencastStartMessage struct {
	Params ScreencastParams
}

// screencastOutboundQueueSize mirrors outboundQueueSize (attach_pty.go) —
// generous enough that a burst of frames doesn't stall this usecase's
// processing loop waiting on adapter/grpc's stream.Send pump.
const screencastOutboundQueueSize = 64

// AttachScreencast drives one bidirectional AttachScreencast stream —
// structurally identical to AttachPty (see its doc comment), except
// resolution is by worktree_id directly (ResolveConnectionByWorktree, no
// pre-existing session row to look up the way a pty already has one) and
// there's no separate "spawn" step: the first Start frame both creates and
// subscribes to the screencast in one call (agent.StreamScreencast).
type AttachScreencast struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
	limiter  *ConnectionStreamLimiter
}

func NewAttachScreencast(resolver ConnectionResolver, agent DevServerAgentClient, limiter *ConnectionStreamLimiter) *AttachScreencast {
	return &AttachScreencast{resolver: resolver, agent: agent, limiter: limiter}
}

// Execute mirrors AttachPty.Execute's two-channel contract exactly — see
// its doc comment.
func (uc *AttachScreencast) Execute(ctx context.Context, inbound <-chan ScreencastClientMessage) (<-chan ScreencastEvent, <-chan error) {
	outbound := make(chan ScreencastEvent, screencastOutboundQueueSize)
	errCh := make(chan error, 1)
	go uc.run(ctx, inbound, outbound, errCh)
	return outbound, errCh
}

func (uc *AttachScreencast) run(ctx context.Context, inbound <-chan ScreencastClientMessage, outbound chan<- ScreencastEvent, errCh chan<- error) {
	defer close(outbound)
	defer close(errCh)

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		errCh <- apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
		return
	}

	first, err := readFirstScreencastStart(ctx, inbound)
	if err != nil {
		errCh <- err
		return
	}
	params := first.Params
	if params.WorktreeID == "" {
		errCh <- apperrors.New(apperrors.KindInvalidArgument, "INFRA_SCREENCAST_NO_WORKTREE", "worktree_id is required", nil)
		return
	}

	connected, devServer, conn, err := uc.resolver.ResolveConnectionByWorktree(ctx, tenantID, params.WorktreeID)
	if err != nil {
		errCh <- apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
		return
	}
	if !connected {
		errCh <- apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SCREENCAST_NO_CONNECTION", "worktree has no bound dev server connection", nil)
		return
	}

	release, err := uc.limiter.Acquire(conn.ID)
	if err != nil {
		errCh <- apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SCREENCAST_STREAM_LIMIT", err.Error(), err)
		return
	}
	defer release()

	events, unsubscribe, err := uc.agent.StreamScreencast(ctx, devServer, params)
	if err != nil {
		errCh <- apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STREAM_SCREENCAST_FAILED", "failed to start browser screencast", err)
		return
	}
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-inbound:
			if !ok {
				return
			}
			if msg.Stop {
				return
			}
			// A repeated Start frame is ignored, same treatment AttachPty
			// gives a repeated Attach frame — the proto contract only
			// documents the first frame as meaningful.
		case ev, ok := <-events:
			if !ok {
				return
			}
			outbound <- ev
			if ev.Ended || ev.ErrorMsg != "" {
				return
			}
		}
	}
}

// readFirstScreencastStart mirrors readFirstAttach (attach_pty.go).
func readFirstScreencastStart(ctx context.Context, inbound <-chan ScreencastClientMessage) (*ScreencastStartMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-inbound:
		if !ok {
			return nil, errors.New("usecase: AttachScreencast stream closed before a start frame arrived")
		}
		if msg.Start == nil {
			return nil, errors.New("usecase: AttachScreencast stream must open with a start frame")
		}
		return msg.Start, nil
	}
}
