package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type RemoteFileURLInput struct {
	WorktreeID string
	Path       string
	Ref        string
}

// RemoteFileURL mirrors RemoteCommitURL — pure local string-construction,
// see TASK-210's Contract correction section.
type RemoteFileURL struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewRemoteFileURL(resolver ConnectionResolver, local, relay GitExecutor) *RemoteFileURL {
	return &RemoteFileURL{resolver: resolver, local: local, relay: relay}
}

func (uc *RemoteFileURL) Execute(ctx context.Context, in RemoteFileURLInput) (string, error) {
	if in.WorktreeID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.Path == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_PATH", "path is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	url, err := executor.RemoteFileURL(ctx, repoPath, in.Path, in.Ref)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_REMOTE_URL_FAILED", "failed to resolve remote file url", err)
	}
	return url, nil
}
