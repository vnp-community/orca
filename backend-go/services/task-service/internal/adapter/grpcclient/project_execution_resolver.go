// This file implements usecase.ProjectExecutionResolver, mirroring
// git-gateway-service's ConnectionResolver
// (internal/adapter/grpcclient/resolver.go) exactly: project_id is passed
// through verbatim as infra-fleet-service's connection_id, per that file's
// confirmed convention (ResolveConnectionRequest has no separate
// project/worktree field, only connection_id).
package grpcclient

import (
	"context"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// ProjectExecutionResolver implements usecase.ProjectExecutionResolver by
// calling infra-fleet-service's ResolveConnection RPC.
type ProjectExecutionResolver struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewProjectExecutionResolver(client infrafleetv1.InfraFleetServiceClient) *ProjectExecutionResolver {
	return &ProjectExecutionResolver{client: client}
}

// ResolveConnection asks infra-fleet-service which host owns projectID.
// Like git-gateway-service's worktreeID, task-service's projectID IS the
// infra-fleet-service connectionId — passed through verbatim, and echoed
// back as the connectionID on a successful resolve. worktreePath is
// resp.GetRepoPath() verbatim — the same field git-gateway-service's own
// ConnectionResolver (internal/adapter/grpcclient/resolver.go:82) reads for
// its ResolvedConnection.RepoPath, per that response message's own doc
// comment (proto/orca/infrafleet/v1/infrafleet.proto's
// ResolveConnectionResponse.repo_path: "Callers like git-gateway-service's
// RelayExecutor need repo_path alongside dev_server to know which path to
// operate on").
func (p *ProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, string, bool, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", "", false, err
	}
	resp, err := p.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{ConnectionId: projectID})
	if err != nil {
		return "", "", false, fmt.Errorf("grpcclient: ResolveConnection(%q): %w", projectID, err)
	}
	if !resp.GetConnected() {
		return "", "", false, nil
	}
	return projectID, resp.GetRepoPath(), true, nil
}
