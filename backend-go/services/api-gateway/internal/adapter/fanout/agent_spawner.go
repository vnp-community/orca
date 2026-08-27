package fanout

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

type GRPCAgentSpawner struct {
	projectClient projectv1.ProjectServiceClient
	infraClient   infrafleetv1.InfraFleetServiceClient
}

func NewGRPCAgentSpawner(projectClient projectv1.ProjectServiceClient, infraClient infrafleetv1.InfraFleetServiceClient) *GRPCAgentSpawner {
	return &GRPCAgentSpawner{projectClient: projectClient, infraClient: infraClient}
}

// SpawnAgentTerminal composes project-service's context resolution with
// infra-fleet-service's connect+spawn — mirrors project-service.md §2's own
// prescribed "resolve context here, then call the execution-owning
// service" two-step pattern.
func (s *GRPCAgentSpawner) SpawnAgentTerminal(ctx context.Context, projectID, worktreePath, agentType string) (string, string, error) {
	proj, err := s.projectClient.GetProject(ctx, &projectv1.GetProjectRequest{Id: projectID})
	if err != nil {
		return "", "", err
	}
	conn, err := s.infraClient.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: proj.GetProject().GetDevServerId()})
	if err != nil {
		return "", "", err
	}
	resp, err := s.infraClient.SpawnTerminalSession(ctx, &infrafleetv1.SpawnTerminalSessionRequest{
		ConnectionId: conn.GetConnectionId(),
		Cwd:          worktreePath,
		Shell:        agentLaunchCommand(agentType),
	})
	if err != nil {
		return "", "", err
	}
	return resp.GetSession().GetPtyId(), conn.GetConnectionId(), nil
}

// agentLaunchCommand is a small, explicit lookup table — not part of this
// saga's coordination logic, just the CLI command each supported agent
// type launches with. Extend as new agent types are supported.
func agentLaunchCommand(agentType string) string {
	switch agentType {
	case "claude":
		return "claude"
	case "codex":
		return "codex"
	default:
		return "" // empty = agent applies its own default, per SpawnTerminalSessionRequest.shell's doc comment
	}
}
