// Package fanout implements api-gateway's WorktreeCreator/AgentSpawner/
// PromptInjector ports (usecase/ports.go) against real gRPC clients —
// SOL-WT-02's "genuine extension is narrow" finding: every RPC these
// adapters call is already real, this package only wraps them behind the
// FanOutCreateWorktrees saga's three ports.
package fanout

import (
	"context"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

type GRPCWorktreeCreator struct {
	client gitgatewayv1.GitGatewayServiceClient
}

func NewGRPCWorktreeCreator(client gitgatewayv1.GitGatewayServiceClient) *GRPCWorktreeCreator {
	return &GRPCWorktreeCreator{client: client}
}

func (w *GRPCWorktreeCreator) CreateWorktree(ctx context.Context, projectID, repoID, branch, baseRef string) (string, string, string, error) {
	resp, err := w.client.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
		ProjectId: projectID, RepoId: repoID, Branch: branch, BaseRef: baseRef,
	})
	if err != nil {
		return "", "", "", err
	}
	return resp.GetWorktreeId(), resp.GetPath(), resp.GetHeadSha(), nil
}
