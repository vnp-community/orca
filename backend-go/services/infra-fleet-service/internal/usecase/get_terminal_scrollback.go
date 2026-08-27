package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// scrollbackDrainWindow bounds how long GetTerminalScrollback waits to
// collect already-buffered output before returning — the agent's
// StreamPty replay delivers buffered chunks immediately on subscribe, so
// this window only needs to be long enough to drain them, not to observe
// new output.
const scrollbackDrainWindow = 500 * time.Millisecond

// GetTerminalScrollbackResult mirrors GetTerminalScrollbackResponse.
type GetTerminalScrollbackResult struct {
	Text      string
	Truncated bool
}

// GetTerminalScrollback subscribes to ptyID's replay buffer via StreamPty
// and concatenates every buffered output chunk delivered within
// scrollbackDrainWindow — a read-only view over the same replay mechanism
// AttachPty/WaitTerminalSession already subscribe to, not a new capture
// path. Truncated is always false today: the agent-side replay buffer's
// own retention bound is not currently surfaced through PtyEvent, so this
// is an honest "we don't know" rather than a fabricated true/false — see
// PtyEvent's doc comment for the gap.
type GetTerminalScrollback struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient

	drainWindow time.Duration // overridable by tests; defaults to scrollbackDrainWindow
}

func NewGetTerminalScrollback(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *GetTerminalScrollback {
	return &GetTerminalScrollback{sessions: sessions, resolver: resolver, agent: agent, drainWindow: scrollbackDrainWindow}
}

func (uc *GetTerminalScrollback) Execute(ctx context.Context, ptyID string) (GetTerminalScrollbackResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GetTerminalScrollbackResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return GetTerminalScrollbackResult{}, err
	}

	window := uc.drainWindow
	if window <= 0 {
		window = scrollbackDrainWindow
	}
	drainCtx, cancel := context.WithTimeout(ctx, window)
	defer cancel()

	events, unsubscribe, err := uc.agent.StreamPty(drainCtx, devServer, ptyID)
	if err != nil {
		return GetTerminalScrollbackResult{}, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STREAM_PTY_FAILED", "failed to subscribe to pty output", err)
	}
	defer unsubscribe()

	var buf []byte
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return GetTerminalScrollbackResult{Text: string(buf)}, nil
			}
			if !ev.Exited {
				buf = append(buf, ev.Data...)
			}
		case <-drainCtx.Done():
			return GetTerminalScrollbackResult{Text: string(buf)}, nil
		}
	}
}
