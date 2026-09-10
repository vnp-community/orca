package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// gatherFullDiff composes a whole-worktree unified diff by listing changed
// files via GetStatus and concatenating one per-file GetDiff call per file
// — GetDiff itself is per-file only (TASK-228: the real Dev Server Agent's
// git.diff has no whole-repo mode), but GenerateCommitMessage/
// GeneratePullRequestFields genuinely need full context for a useful
// prompt, so that composition happens here, once, entirely inside
// git-gateway-service.
func gatherFullDiff(ctx context.Context, getStatus *GetStatus, getDiff *GetDiff, worktreeID string, staged bool) (string, error) {
	status, err := getStatus.Execute(ctx, GetStatusInput{WorktreeID: worktreeID})
	if err != nil {
		return "", err
	}
	return gatherFullDiffFromStatus(ctx, getDiff, worktreeID, status, staged)
}

// gatherFullDiffFromStatus is gatherFullDiff's body minus its own
// GetStatus.Execute call, so a caller that already has a GitStatus (e.g.
// GenerateCommitMessage.Execute, which needs status.Files' count for
// BR-CR-15's threshold check before deciding whether to call this at all)
// doesn't pay for a second resolve-and-dispatch round trip.
func gatherFullDiffFromStatus(ctx context.Context, getDiff *GetDiff, worktreeID string, status domain.GitStatus, staged bool) (string, error) {
	var b strings.Builder
	for _, f := range status.Files {
		diff, err := getDiff.Execute(ctx, GetDiffInput{WorktreeID: worktreeID, FilePath: f.Path, Staged: staged})
		if err != nil {
			return "", err
		}
		b.WriteString(diff.UnifiedDiff)
	}
	return b.String(), nil
}
