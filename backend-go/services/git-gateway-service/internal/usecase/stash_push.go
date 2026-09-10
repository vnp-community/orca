package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type StashPushInput struct {
	WorktreeID       string
	Message          string
	IncludeUntracked bool
}

// StashPush calls resolver.ResolveConnection inline — see MergeBranch's
// doc comment for why (needs Mode to fail closed on relay-ssh).
type StashPush struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewStashPush(resolver ConnectionResolver, local, relay GitExecutor) *StashPush {
	return &StashPush{resolver: resolver, local: local, relay: relay}
}

func (uc *StashPush) Execute(ctx context.Context, in StashPushInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_STASH_PUSH_UNSUPPORTED_SSH_RELAY", "stash push is not supported over an SSH-relay connection", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	result, err := executor.StashPush(ctx, conn.RepoPath, in.Message, in.IncludeUntracked)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_STASH_PUSH_FAILED", "failed to stash changes", err)
	}
	return result, nil
}
