package usecase

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestExtractIssueRef(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"fix/ORCA-123-foo", "ORCA-123"},
		{"feature/456-thing", "#456"},
		{"main", ""},
	}
	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := extractIssueRef(tt.branch)
			if got != tt.want {
				t.Errorf("extractIssueRef(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}

func TestBuildCommitMessagePrompt(t *testing.T) {
	t.Run("includes recent commits, branch, and issue ref when present", func(t *testing.T) {
		recent := []domain.CommitRef{
			{SHA: "abcdef1234567", Message: "feat: add thing\n\nlonger body"},
		}
		prompt := buildCommitMessagePrompt("fix/ORCA-123-foo", recent, "diff content", "ORCA-123")

		if !strings.Contains(prompt, "Recent commits on this project") {
			t.Error("expected recent-commit block header")
		}
		if !strings.Contains(prompt, "abcdef1") {
			t.Error("expected truncated sha in recent-commit block")
		}
		if !strings.Contains(prompt, "feat: add thing") {
			t.Error("expected first line of commit message")
		}
		if strings.Contains(prompt, "longer body") {
			t.Error("expected only first line of commit message, not the full body")
		}
		if !strings.Contains(prompt, "Current branch: fix/ORCA-123-foo") {
			t.Error("expected branch line")
		}
		if !strings.Contains(prompt, "ORCA-123") || !strings.Contains(prompt, "Refs:") {
			t.Error("expected issue-reference instruction line")
		}
		if !strings.Contains(prompt, "diff content") {
			t.Error("expected diff content included")
		}
	})

	t.Run("omits sections cleanly when empty", func(t *testing.T) {
		prompt := buildCommitMessagePrompt("", nil, "diff content", "")
		if strings.Contains(prompt, "Recent commits") {
			t.Error("expected no recent-commit header when recent is empty")
		}
		if strings.Contains(prompt, "Current branch:") {
			t.Error("expected no branch line when branch is empty")
		}
		if strings.Contains(prompt, "Refs:") {
			t.Error("expected no issue-reference line when issueRef is empty")
		}
	})
}

func TestStatsOnlySummary(t *testing.T) {
	files := []domain.FileStatus{
		{Path: "a.go", State: domain.FileStateModified},
		{Path: "b.go", State: domain.FileStateAdded},
	}
	summary := statsOnlySummary(files)

	if !strings.Contains(summary, "2 files") {
		t.Errorf("expected file count in header, got %q", summary)
	}
	if !strings.Contains(summary, "modified a.go") {
		t.Errorf("expected one line per file, got %q", summary)
	}
	if !strings.Contains(summary, "added b.go") {
		t.Errorf("expected one line per file, got %q", summary)
	}
}
