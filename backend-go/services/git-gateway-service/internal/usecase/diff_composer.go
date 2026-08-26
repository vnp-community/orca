package usecase

import (
	"context"
	"strings"
)

// gatherFullDiff composes a whole-worktree unified diff by listing changed
// files via GetStatus and concatenating one per-file GetDiff call per file
// — GetDiff itself is per-file only (TASK-228: the real Dev Server Agent's
// git.diff has no whole-repo mode), but GenerateCommitMessage/
// GeneratePullRequestFields genuinely need full context for a useful
// prompt, so that composition happens here, once, entirely inside
// git-gateway-service rather than threading a "give me everything" mode
// through to the wire channel the frontend never asked for (see TASK-228's
// own "Frontend impact check" note).
func gatherFullDiff(ctx context.Context, getStatus *GetStatus, getDiff *GetDiff, worktreeID string, staged bool) (string, error) {
	status, err := getStatus.Execute(ctx, GetStatusInput{WorktreeID: worktreeID})
	if err != nil {
		return "", err
	}

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
