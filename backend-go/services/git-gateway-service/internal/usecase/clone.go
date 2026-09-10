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
	reachability DevServerReachability
	local        GitExecutor
	relay        GitExecutor
}

func NewClone(reachability DevServerReachability, local, relay GitExecutor) *Clone {
	return &Clone{reachability: reachability, local: local, relay: relay}
}

func (uc *Clone) Execute(ctx context.Context, in CloneInput) (CloneResult, error) {
	if in.DevServerID == "" {
		return CloneResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_DEV_SERVER_ID", "dev_server_id is required", nil)
	}

	reachable, err := uc.reachability.IsReachable(ctx, in.DevServerID)
	if err != nil {
		return CloneResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve dev server reachability", err)
	}

	executor := uc.local
	if reachable {
		ctx = WithDevServerID(ctx, in.DevServerID)
		executor = uc.relay
	}
	worktreePath, defaultBranch, err := executor.Clone(ctx, in.URL, in.DestPath)
	if err != nil {
		return CloneResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CLONE_FAILED", "failed to clone repository", err)
	}
	return CloneResult{WorktreePath: worktreePath, DefaultBranch: defaultBranch}, nil
}
