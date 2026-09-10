// infrafleet_client.go implements usecase.AgentSpawner by wrapping
// infra-fleet-service.SpawnTerminalSession (BL-AG-01's agent.spawn) —
// git-gateway-service does not implement PTY spawn itself.
package grpcclient

import (
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	"context"
	"log/slog"
)

// InfraFleetAgentSpawner implements usecase.AgentSpawner against
// infra-fleet-service's SpawnTerminalSession RPC.
type InfraFleetAgentSpawner struct {
	client infrafleetv1.InfraFleetServiceClient
	logger *slog.Logger
}

func NewInfraFleetAgentSpawner(client infrafleetv1.InfraFleetServiceClient, logger *slog.Logger) *InfraFleetAgentSpawner {
	if logger == nil {
		logger = slog.Default()
	}
	return &InfraFleetAgentSpawner{client: client, logger: logger}
}

// SpawnAndInject spawns a real PTY session at cwd (empty connection_id —
// this saga's caller has no resolved dev-server connection for a
// freshly-created worktree today, so this always targets git-gateway-service's
// own host; a follow-up would thread ConnectionResolver's answer through
// once CreateWorktreeFromIssue also supports remote dev servers). The
// follow-up prompt-injection write once the PTY reports idle is a
// CONFIRMED GAP: infra-fleet-service's proto has no write/inject RPC yet
// (BL-AG-01:127-142's wait-for-idle-then-write contract has no backing RPC
// — see ports.go's AgentSpawner doc comment) — spawn still succeeds and
// returns a real session id; injection is logged as unimplemented rather
// than silently pretended to have happened.
func (s *InfraFleetAgentSpawner) SpawnAndInject(ctx context.Context, worktreeID, cwd, prompt string) (string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	resp, err := s.client.SpawnTerminalSession(ctx, &infrafleetv1.SpawnTerminalSessionRequest{Cwd: cwd})
	if err != nil {
		return "", err
	}
	sessionID := resp.GetSession().GetPtyId()
	if prompt != "" {
		s.logger.WarnContext(ctx, "agent prompt injection is not yet implemented — infra-fleet-service has no PTY write/inject RPC",
			"worktree_id", worktreeID, "session_id", sessionID)
	}
	return sessionID, nil
}
