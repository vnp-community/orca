package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// CommitDiffInput's FilePath is required — the real git.commitDiff is a
// per-file operation (same class of fix as GetDiff's TASK-228 correction),
// not a whole-commit diff. ParentOID is optional (empty = root commit,
// diffed against the empty tree). OldPath is optional (empty = same as
// FilePath — no rename).
type CommitDiffInput struct {
	WorktreeID string
	CommitOID  string
	ParentOID  string
	FilePath   string
	OldPath    string
}

type CommitDiff struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCommitDiff(resolver ConnectionResolver, local, relay GitExecutor) *CommitDiff {
	return &CommitDiff{resolver: resolver, local: local, relay: relay}
}

func (uc *CommitDiff) Execute(ctx context.Context, in CommitDiffInput) (domain.FileDiffResult, error) {
	if in.WorktreeID == "" {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.CommitOID == "" {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_COMMIT_OID", "commit_oid is required", nil)
	}
	if in.FilePath == "" {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_FILE_PATH", "file_path is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.CommitDiff(ctx, repoPath, in.CommitOID, in.ParentOID, in.FilePath, in.OldPath)
	if err != nil {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_COMMIT_DIFF_FAILED", "failed to diff commit file", err)
	}
	return result, nil
}
