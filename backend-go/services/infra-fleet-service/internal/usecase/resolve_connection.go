package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ResolveConnectionOutput is the resolved dispatch record: whether a dev
// server owns the given connectionId (Connected) and, if so, which one, plus
// the per-connection metadata (RepoPath, WorktreeID) callers like
// git-gateway-service's RelayExecutor need alongside DevServer.
type ResolveConnectionOutput struct {
	Connected bool
	DevServer domain.DevServer
	RepoPath  string
	// ConnectionID is the resolved infra.connections.id — the value Relay's
	// RelayInput.ConnectionID expects. Populated regardless of which
	// ResolveConnectionInput key resolved this row (TASK-025), so a
	// dev-server- or worktree-keyed caller has a valid Relay connection id
	// without conflating it with DevServer.ID, a different id space.
	ConnectionID string
	WorktreeID   string
	// NodeVersion is the connected session's self-reported Node.js version
	// (TASK-INT-03-02) — empty when Connected is false, when Sessions is
	// nil, or when the live session predates this field. Never populated
	// by any resolver other than Sessions (see HandshakeInfoProvider's doc
	// comment).
	NodeVersion string
}

// ResolveConnectionInput mirrors ResolveConnectionRequest 1:1 — exactly one
// of ConnectionID/DevServerID/WorktreeID is expected to be set (see
// infrafleet.proto's ResolveConnectionRequest doc comment).
type ResolveConnectionInput struct {
	ConnectionID string
	DevServerID  string
	WorktreeID   string
}

// ResolveConnection is THE core coordination/execution dispatch primitive of
// this service — see specs/backend-go/services/infra-fleet-service.md §7.
// git-gateway-service calls this on every git.* dispatch to decide
// local-exec vs. relay; project-service calls it to validate a dev-server
// binding; any connectionId-bound feature in the system reduces to this call.
type ResolveConnection struct {
	resolver ConnectionResolver
	// Sessions is optional (nil by default) — set directly by the
	// composition root when a live-session Node-version enrichment is
	// available (TASK-INT-03-02). See HandshakeInfoProvider's doc comment.
	Sessions HandshakeInfoProvider
}

func NewResolveConnection(resolver ConnectionResolver) *ResolveConnection {
	return &ResolveConnection{resolver: resolver}
}

func (uc *ResolveConnection) Execute(ctx context.Context, in ResolveConnectionInput) (ResolveConnectionOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ResolveConnectionOutput{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	var (
		connected bool
		devServer domain.DevServer
		conn      domain.Connection
	)
	switch {
	case in.DevServerID != "":
		connected, devServer, conn, err = uc.resolver.ResolveConnectionByDevServer(ctx, tenantID, in.DevServerID)
	case in.WorktreeID != "":
		connected, devServer, conn, err = uc.resolver.ResolveConnectionByWorktree(ctx, tenantID, in.WorktreeID)
	case in.ConnectionID == "":
		// No key at all is not an error — it's the caller's own signal
		// that there's nothing to resolve (a connectionless, local-only
		// worktree or session). Short-circuit before the repository
		// round-trip.
		return ResolveConnectionOutput{Connected: false}, nil
	default:
		connected, devServer, conn, err = uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	}
	if err != nil {
		return ResolveConnectionOutput{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	out := ResolveConnectionOutput{Connected: connected, DevServer: devServer, ConnectionID: conn.ID, RepoPath: conn.RepoPath, WorktreeID: conn.WorktreeID}
	if connected && uc.Sessions != nil {
		if v, ok := uc.Sessions.NodeVersionFor(devServer.ID); ok {
			out.NodeVersion = v
		}
	}
	return out, nil
}
