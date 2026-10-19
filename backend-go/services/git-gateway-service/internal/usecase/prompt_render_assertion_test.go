package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func mustAgentPrompt(t *testing.T, title, description, acceptanceCriteria string, comments []string) string {
	t.Helper()
	got, err := renderAgentPrompt(title, description, acceptanceCriteria, comments)
	if err != nil {
		t.Fatalf("renderAgentPrompt: %v", err)
	}
	return got
}

func mustCommitMessagePrompt(t *testing.T, branch string, recent []domain.CommitRef, diffOrStats, issueRef string) string {
	t.Helper()
	got, err := renderCommitMessagePrompt(branch, recent, diffOrStats, issueRef)
	if err != nil {
		t.Fatalf("renderCommitMessagePrompt: %v", err)
	}
	return got
}

func mustStatsOnlySummary(t *testing.T, files []domain.FileStatus) string {
	t.Helper()
	got, err := renderStatsOnlySummary(files)
	if err != nil {
		t.Fatalf("renderStatsOnlySummary: %v", err)
	}
	return got
}

func mustPullRequestPrompt(t *testing.T, diff string) string {
	t.Helper()
	got, err := renderPullRequestPrompt(diff)
	if err != nil {
		t.Fatalf("renderPullRequestPrompt: %v", err)
	}
	return got
}
