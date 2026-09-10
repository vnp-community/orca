package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// TeardownConnection now serves two purposes over one code path — see
// infrafleet.proto's TeardownConnectionRequest doc comment:
//   - BR-SSH-13's "Cancel" action: stop any in-flight relaySSHReconnect/
//     backgroundReconnect backoff loop for the dev server.
//   - BE-SOL-STORAGE-003 §5's confirmed-logout explicit-close path
//     (TASK-BE-STORAGE-012) — the one deliberate exception to the
//     reconnect-resume grace period (BE-SOL-STORAGE-003 §2):
//     established|degraded -> closed immediately, no waiting for
//     grace_period_seconds to elapse.
//
// This is a thin composition of already-tested pieces, not new business
// logic: domain.Connection.CloseExplicitly (TASK-BE-STORAGE-009) and
// CloseTerminalSessionsForConnection (TASK-BE-STORAGE-010), plus BR-SSH-13's
// CancelReconnect call so a torn-down connection's dev server doesn't keep
// trying to re-establish it behind the caller's back.
type TeardownConnection struct {
	resolver ConnectionResolver
	conns    ConnectionRepository
	sessions TerminalSessionRepository
	agent    DevServerAgentClient
}

func NewTeardownConnection(resolver ConnectionResolver, conns ConnectionRepository, sessions TerminalSessionRepository, agent DevServerAgentClient) *TeardownConnection {
	return &TeardownConnection{resolver: resolver, conns: conns, sessions: sessions, agent: agent}
}

// Execute closes connectionID immediately, bypassing any grace period, and
// closes every terminal_sessions row bound to it. A connectionID unknown
// within the caller's tenant is a not-found error, not a silent no-op —
// unlike ResolveConnection's "not found = execute locally" convention, an
// explicit teardown request names a connection the caller believes exists.
func (uc *TeardownConnection) Execute(ctx context.Context, connectionID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	connected, devServer, conn, err := uc.resolver.ResolveConnection(ctx, tenantID, connectionID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	if !connected {
		return apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "connection not found for this tenant", nil)
	}

	conn.CloseExplicitly()
	if err := uc.conns.UpdateStatus(ctx, tenantID, conn); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_UPDATE_CONNECTION_STATUS_FAILED", "failed to persist closed connection status", err)
	}

	if err := NewCloseTerminalSessionsForConnection(uc.sessions).Execute(ctx, tenantID, conn.ID); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_CLOSE_TERMINAL_SESSIONS_FAILED", "connection closed, but failed to close its terminal sessions", err)
	}

	// BR-SSH-13: stop any in-flight relaySSHReconnect/backgroundReconnect
	// loop now that this connection is explicitly torn down — a no-op if no
	// reconnect loop is running (see Client.CancelReconnect).
	uc.agent.CancelReconnect(devServer.ID)

	// Best-effort notify the live agent, if reachable, to run its own
	// immediate teardown (kill agent.spawn PTYs now, bypassing its own
	// local grace period — see TASK-AG-STORAGE-007/009's agent-side half of
	// this CR). Deliberately NOT the KillWorkspacePort/ScanWorkspacePorts
	// pattern of propagating agent.Exec's error as a real failure: the whole
	// point of an explicit teardown is that server-side state (this
	// connection, its terminal_sessions) must end up closed even when the
	// dev server is unreachable right now (laptop closed, agent crashed,
	// network down) — that is exactly the case a confirmed logout most
	// needs to succeed in. A failed/unreachable notify here is not an error
	// for the caller; the agent's own reconnect-resume grace period
	// (SOL-AG-STORAGE-003) already treats a connection it can't reach as
	// eventually torn down on its own timeline.
	_, _ = uc.agent.Exec(ctx, devServer, "connection.teardown", map[string]any{"connectionId": conn.ID})
	return nil
}
