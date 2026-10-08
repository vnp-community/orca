package grpcclient

import (
	"context"
	"fmt"
	"sort"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RepoStateProbe reads branch and file states through git-gateway GetStatus. It adds no Git command of its own;
// GetStatus takes a worktree id, so without one there is nothing to ask and the caller records repo_check=skipped.
type RepoStateProbe struct {
	git gitgatewayv1.GitGatewayServiceClient
}

var _ usecase.RepoStateProbe = (*RepoStateProbe)(nil)

func NewRepoStateProbe(git gitgatewayv1.GitGatewayServiceClient) *RepoStateProbe {
	return &RepoStateProbe{git: git}
}

func (p *RepoStateProbe) Snapshot(ctx context.Context, worktreeID string) (usecase.RepoSnapshot, error) {
	if worktreeID == "" {
		return usecase.RepoSnapshot{}, usecase.ErrProbeUnavailable
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return usecase.RepoSnapshot{}, err
	}
	resp, err := p.git.GetStatus(tctx, &gitgatewayv1.GetStatusRequest{WorktreeId: worktreeID})
	if err != nil {
		return usecase.RepoSnapshot{}, fmt.Errorf("grpcclient: git GetStatus: %w", err)
	}
	snap := usecase.RepoSnapshot{Branch: resp.GetBranch()}
	for _, f := range resp.GetFiles() {
		snap.Files = append(snap.Files, usecase.RepoFileState{Path: f.GetPath(), State: f.GetState()})
	}
	sort.Slice(snap.Files, func(i, j int) bool {
		if snap.Files[i].Path != snap.Files[j].Path {
			return snap.Files[i].Path < snap.Files[j].Path
		}
		return snap.Files[i].State < snap.Files[j].State
	})
	return snap, nil
}
