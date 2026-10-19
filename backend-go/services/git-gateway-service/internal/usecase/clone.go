package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// CloneInput mirrors the gRPC request 1:1.
type CloneInput struct {
	DevServerID string
	URL         string
	DestPath    string
}

type CloneResult struct {
	WorktreePath  string
	DefaultBranch string
}

// Clone dispatches to whichever GitExecutor answers for DevServerID —
// same resolve-then-dispatch shape as every other usecase in this package,
// keyed by DevServerReachability instead of ConnectionResolver since no
// worktree/connectionId exists yet (see ports.go's doc comment on this
// port).
type Clone struct {
	reachability RepoHostRouting
	local        GitExecutor
	relay        GitExecutor
}

func NewClone(reachability RepoHostRouting, local, relay GitExecutor) *Clone {
	return &Clone{reachability: reachability, local: local, relay: relay}
}

func (uc *Clone) Execute(ctx context.Context, in CloneInput) (CloneResult, error) {
	if in.DevServerID == "" {
		return CloneResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_DEV_SERVER_ID", "dev_server_id is required", nil)
	}

	// Never fall back to uc.local when the dev server is unreachable: git
	// would run in this container, not on the host the caller named.
	if _, err := RouteRepoHost(ctx, uc.reachability, in.DevServerID); err != nil {
		return CloneResult{}, err
	}
	ctx = WithDevServerID(ctx, in.DevServerID)
	executor := uc.relay
	worktreePath, defaultBranch, err := executor.Clone(ctx, in.URL, in.DestPath)
	if err != nil {
		return CloneResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CLONE_FAILED", "failed to clone repository", err)
	}
	return CloneResult{WorktreePath: worktreePath, DefaultBranch: defaultBranch}, nil
}
