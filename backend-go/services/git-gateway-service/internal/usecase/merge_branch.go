package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type MergeBranchInput struct {
	WorktreeID string
	Branch     string
	NoFF       bool
}

// MergeBranch calls resolver.ResolveConnection inline (rather than going
// through dispatchExecutor) because it needs ResolvedConnection.Mode
// directly, to fail closed on relay-ssh BEFORE ever attempting the relay
// call — see SOL-PW-03.
type MergeBranch struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewMergeBranch(resolver ConnectionResolver, local, relay GitExecutor) *MergeBranch {
	return &MergeBranch{resolver: resolver, local: local, relay: relay}
}

func (uc *MergeBranch) Execute(ctx context.Context, in MergeBranchInput) (domain.MergeResult, error) {
	if in.WorktreeID == "" || in.Branch == "" {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_ARGS", "worktree_id and branch are required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return domain.MergeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_MERGE_UNSUPPORTED_SSH_RELAY", "merge is not supported over an SSH-relay connection", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	result, err := executor.MergeBranch(ctx, conn.RepoPath, in.Branch, in.NoFF)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_MERGE_FAILED", "failed to merge branch", err)
	}
	return result, nil
}
