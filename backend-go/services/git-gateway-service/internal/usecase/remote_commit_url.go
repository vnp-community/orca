package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type RemoteCommitURLInput struct {
	WorktreeID string
	SHA        string
}

// RemoteCommitURL is pure local string-construction (resolve origin's URL,
// pattern-match the host, build a permalink) — no agent method on either
// side, see TASK-210's Contract correction section.
type RemoteCommitURL struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewRemoteCommitURL(resolver ConnectionResolver, local, relay GitExecutor) *RemoteCommitURL {
	return &RemoteCommitURL{resolver: resolver, local: local, relay: relay}
}

func (uc *RemoteCommitURL) Execute(ctx context.Context, in RemoteCommitURLInput) (string, error) {
	if in.WorktreeID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.SHA == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_SHA", "sha is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	url, err := executor.RemoteCommitURL(ctx, repoPath, in.SHA)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_REMOTE_URL_FAILED", "failed to resolve remote commit url", err)
	}
	return url, nil
}
