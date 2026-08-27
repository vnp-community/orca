package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// InitRepoInput mirrors the gRPC request 1:1.
type InitRepoInput struct {
	DevServerID   string
	DestPath      string
	DefaultBranch string
}

type InitRepoResult struct {
	Path          string
	DefaultBranch string
}

// InitRepo runs `git init` at DestPath on whichever host DevServerID
// resolves to — same dispatch shape as Clone. The caller (project-service
// context, per CloneRequest's own doc comment in gitgateway.proto) is
// responsible for then calling ProjectService.AddRepo with the returned
// path — this usecase only performs the git operation.
type InitRepo struct {
	reachability DevServerReachability
	local        GitExecutor
	relay        GitExecutor
}

func NewInitRepo(reachability DevServerReachability, local, relay GitExecutor) *InitRepo {
	return &InitRepo{reachability: reachability, local: local, relay: relay}
}

func (uc *InitRepo) Execute(ctx context.Context, in InitRepoInput) (InitRepoResult, error) {
	if in.DevServerID == "" {
		return InitRepoResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_DEV_SERVER_ID", "dev_server_id is required", nil)
	}

	reachable, err := uc.reachability.IsReachable(ctx, in.DevServerID)
	if err != nil {
		return InitRepoResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve dev server reachability", err)
	}

	executor := uc.local
	if reachable {
		executor = uc.relay
	}
	path, defaultBranch, err := executor.InitRepo(ctx, in.DestPath, in.DefaultBranch)
	if err != nil {
		return InitRepoResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_INIT_REPO_FAILED", "failed to init repository", err)
	}
	return InitRepoResult{Path: path, DefaultBranch: defaultBranch}, nil
}
