package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ResolveDirectWebSocketDevServer finds or creates the SQL-backed DevServer
// row a direct-websocket agent's external devServerID (e.g. "dev-01",
// caller-supplied at /api/agent-token mint time) maps to.
//
// Why this exists: the agent-token/handshake pathway (agentwsserver package)
// predates CR-DS-006/007/008's approval/grouping model and only ever
// tracked live connections in-memory (agentwsserver.Registry keyed by a raw
// string devServerID, devserveragent.Client.sessions keyed the same way) —
// it never called RegisterDevServer, so no infra.dev_servers row backed a
// connected agent, and the Admin Console's Approvals tab stayed empty no
// matter how many agents were live. Live-verified: 3 systemd-managed agents
// connected and handshaking successfully (confirmed via agent logs) while
// `SELECT * FROM infra.dev_servers` returned zero rows.
//
// TenantID is NOT pulled from context here (unlike RegisterDevServer) —
// the token-mint HTTP endpoint authenticates via a single shared
// ORCA_AGENT_API_SECRET, not a per-user session, so there is no caller
// identity to extract a tenant from. The composition root (main.go) passes
// a configured default tenant (ORCA_AGENT_DEFAULT_TENANT_ID, falling back
// to the bootstrap tenant) as an explicit input field instead.
type ResolveDirectWebSocketDevServer struct {
	repo DevServerRepository
}

func NewResolveDirectWebSocketDevServer(repo DevServerRepository) *ResolveDirectWebSocketDevServer {
	return &ResolveDirectWebSocketDevServer{repo: repo}
}

type ResolveDirectWebSocketDevServerInput struct {
	TenantID string
	// DevServerID is the agent's external identifier (e.g. "dev-01") —
	// reused as domain.DevServer.Host, which direct-websocket mode
	// otherwise leaves unused (the agent dials Orca, not the other way
	// around — see devserveragent.Client.getInboundSession's doc comment).
	DevServerID string
}

// Execute reuses the same row across agent restarts/reconnects (looked up
// by TenantID+DevServerID+direct-websocket mode) so approval_status/group_id
// set by an admin survive the agent's own reconnect cycle — only the very
// first connection for a given DevServerID creates a new row.
func (uc *ResolveDirectWebSocketDevServer) Execute(ctx context.Context, in ResolveDirectWebSocketDevServerInput) (domain.DevServer, error) {
	existing, found, err := uc.repo.FindByHostAndMode(ctx, in.TenantID, in.DevServerID, domain.ConnectionModeDirectWebSocket)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_RESOLVE_FAILED", "failed to resolve dev server for agent token", err)
	}
	if found {
		return existing, nil
	}

	devServer, err := domain.NewDevServer(uuid.NewString(), in.TenantID, in.DevServerID, domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_CONSTRUCT_FAILED", "failed to construct dev server for agent token", err)
	}
	saved, err := uc.repo.Register(ctx, devServer)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_REGISTER_FAILED", "failed to register dev server for agent token", err)
	}
	return saved, nil
}
