package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// maxWaitTerminalSessionTimeout caps WaitTerminalSessionRequest.timeout_ms
// server-side, per that message's proto doc comment ("capped server-side,
// default max 30s").
const maxWaitTerminalSessionTimeout = 30 * time.Second

// WaitTerminalSessionInput mirrors the gRPC request 1:1 by design, see
// register_dev_server.go's comment for the rationale.
type WaitTerminalSessionInput struct {
	PtyID     string
	TimeoutMs int32
}

// WaitTerminalSessionResult mirrors WaitTerminalSessionResponse.
type WaitTerminalSessionResult struct {
	Exited   bool
	ExitCode int32
	TimedOut bool
}

// WaitTerminalSession is a bounded blocking poll for process exit — there is
// no dedicated "wait" primitive in DevServerAgentClient (see TASK-181's
// seven-method list), so this usecase reuses StreamPty: it subscribes the
// same way AttachPty does and blocks until either a pty.exit event arrives
// or the (capped) timeout elapses. Deliberately does NOT surface Output
// events to the caller — WaitTerminalSessionResponse has no data field, only
// exited/exit_code/timed_out — they are read and discarded so the loop
// keeps waiting for the exit event instead of returning early on the first
// unrelated output chunk.
type WaitTerminalSession struct {
	sessions   TerminalSessionRepository
	resolver   ConnectionResolver
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewWaitTerminalSession(sessions TerminalSessionRepository, resolver ConnectionResolver, devServers DevServerRepository, agent DevServerAgentClient) *WaitTerminalSession {
	return &WaitTerminalSession{sessions: sessions, resolver: resolver, devServers: devServers, agent: agent}
}

func (uc *WaitTerminalSession) Execute(ctx context.Context, in WaitTerminalSessionInput) (WaitTerminalSessionResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return WaitTerminalSessionResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	_, devServer, err := resolveTerminalSession(ctx, tenantID, in.PtyID, uc.sessions, uc.resolver, uc.devServers)
	if err != nil {
		return WaitTerminalSessionResult{}, err
	}

	timeout := time.Duration(in.TimeoutMs) * time.Millisecond
	if timeout <= 0 || timeout > maxWaitTerminalSessionTimeout {
		timeout = maxWaitTerminalSessionTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	events, unsubscribe, err := uc.agent.StreamPty(waitCtx, devServer, in.PtyID)
	if err != nil {
		return WaitTerminalSessionResult{}, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STREAM_PTY_FAILED", "failed to subscribe to pty output", err)
	}
	defer unsubscribe()

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return WaitTerminalSessionResult{TimedOut: true}, nil
			}
			if ev.Exited {
				return WaitTerminalSessionResult{Exited: true, ExitCode: ev.ExitCode}, nil
			}
			// Output event — keep waiting for exit, see type doc comment.
		case <-waitCtx.Done():
			return WaitTerminalSessionResult{TimedOut: true}, nil
		}
	}
}
