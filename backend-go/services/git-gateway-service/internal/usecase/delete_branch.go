package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type DeleteBranchInput struct {
	WorktreeID string
	Branch     string
}

// DeleteBranch (soft, -d) calls resolver.ResolveConnection inline — see
// MergeBranch's doc comment for why (needs Mode to fail closed on
// relay-ssh). ForceDeleteBranch (existing) stays the -D path and is
// unaffected by this task.
type DeleteBranch struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewDeleteBranch(resolver ConnectionResolver, local, relay GitExecutor) *DeleteBranch {
	return &DeleteBranch{resolver: resolver, local: local, relay: relay}
}

func (uc *DeleteBranch) Execute(ctx context.Context, in DeleteBranchInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" || in.Branch == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_ARGS", "worktree_id and branch are required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_DELETE_BRANCH_UNSUPPORTED_SSH_RELAY", "branch delete is not supported over an SSH-relay connection", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	if err := executor.DeleteBranch(ctx, conn.RepoPath, in.Branch); err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_DELETE_BRANCH_FAILED", "failed to delete branch", err)
	}
	return domain.SimpleResult{Success: true}, nil
}
