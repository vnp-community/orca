package grpcclient

import (
	"context"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// AIConnection says where ai.complete runs: through an infra connection, or straight to a
// dev server when no infra.connections row exists for the project.
type AIConnection struct {
	ConnectionID string
	DevServerID  string
	// RepoPath and WorktreeID are only known for a resolved infra connection (the dev-server fallback has neither).
	RepoPath   string
	WorktreeID string
}

// AIConnectionResolver finds the dev server for a project. connectionID = projectID almost
// never resolves (BUG-025 in task-service: infra.connections ids are server-generated), so the
// default-repo dev server is the normal path, not an edge case.
type AIConnectionResolver struct {
	infra    infrafleetv1.InfraFleetServiceClient
	projects projectv1.ProjectServiceClient
}

func NewAIConnectionResolver(infra infrafleetv1.InfraFleetServiceClient, projects projectv1.ProjectServiceClient) *AIConnectionResolver {
	return &AIConnectionResolver{infra: infra, projects: projects}
}

func (r *AIConnectionResolver) ResolveForProject(ctx context.Context, projectID string) (AIConnection, error) {
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return AIConnection{}, err
	}
	resp, err := r.infra.ResolveConnection(tctx, &infrafleetv1.ResolveConnectionRequest{ConnectionId: projectID})
	if err != nil {
		return AIConnection{}, fmt.Errorf("grpcclient: ResolveConnection: %w", err)
	}
	if resp.GetConnected() {
		return AIConnection{ConnectionID: projectID, RepoPath: resp.GetRepoPath(), WorktreeID: resp.GetWorktreeId()}, nil
	}

	pctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return AIConnection{}, err
	}
	repos, err := r.projects.ListRepos(pctx, &projectv1.ListReposRequest{ProjectId: projectID})
	if err != nil {
		return AIConnection{}, fmt.Errorf("grpcclient: ListRepos: %w", err)
	}
	if len(repos.GetRepos()) == 0 || repos.GetRepos()[0].GetDevServerId() == "" {
		return AIConnection{}, usecase.ErrNoDevServer
	}
	devServerID := repos.GetRepos()[0].GetDevServerId()
	reachable, err := r.reachable(tctx, devServerID)
	if err != nil {
		return AIConnection{}, err
	}
	if !reachable {
		return AIConnection{}, usecase.ErrNoDevServer
	}
	return AIConnection{DevServerID: devServerID}, nil
}

// reachable treats a dev server with no health sample as unreachable rather than an error.
func (r *AIConnectionResolver) reachable(ctx context.Context, devServerID string) (bool, error) {
	resp, err := r.infra.GetFleetHealth(ctx, &infrafleetv1.GetFleetHealthRequest{})
	if err != nil {
		return false, fmt.Errorf("grpcclient: GetFleetHealth: %w", err)
	}
	for _, h := range resp.GetStatuses() {
		if h.GetDevServerId() == devServerID {
			return h.GetReachable(), nil
		}
	}
	return false, nil
}
