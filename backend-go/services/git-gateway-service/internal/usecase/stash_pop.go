package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type StashPopInput struct {
	WorktreeID string
	StashRef   string
}

// StashPop calls resolver.ResolveConnection inline — see MergeBranch's doc
// comment for why (needs Mode to fail closed on relay-ssh).
type StashPop struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewStashPop(resolver ConnectionResolver, local, relay GitExecutor) *StashPop {
	return &StashPop{resolver: resolver, local: local, relay: relay}
}

func (uc *StashPop) Execute(ctx context.Context, in StashPopInput) (domain.MergeOutcome, error) {
	if in.WorktreeID == "" {
		return domain.MergeOutcome{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return domain.MergeOutcome{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return domain.MergeOutcome{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_STASH_POP_UNSUPPORTED_SSH_RELAY", "stash pop is not supported over an SSH-relay connection", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	result, err := executor.StashPop(ctx, conn.RepoPath, in.StashRef)
	if err != nil {
		return domain.MergeOutcome{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_STASH_POP_FAILED", "failed to pop stash", err)
	}
	return result, nil
}
