package usecase

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/stablyai/orca-go/prompts/git_commit_message"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// issueRefPattern matches BR-CR-16's two conventional shapes, in priority
// order: a Jira/Linear-style project-key ticket ("ORCA-123", case
// preserved from the branch as the canonical form since providers are
// case-sensitive on this), falling back to a bare numeric issue reference
// ("#123", GitHub/GitLab's own convention).
var (
	jiraStyleRef = regexp.MustCompile(`(?i)[a-z][a-z0-9]+-\d+`)
	numericRef   = regexp.MustCompile(`\d{2,}`)
)

// extractIssueRef pulls an issue/ticket id out of a branch name, per
// BR-CR-16. Returns "" if nothing matches — the caller (Execute) must NOT
// invent a Refs: line in that case.
func extractIssueRef(branch string) string {
	if m := jiraStyleRef.FindString(branch); m != "" {
		return strings.ToUpper(m)
	}
	if m := numericRef.FindString(branch); m != "" {
		return "#" + m
	}
	return ""
}

// maxFullDiffFiles is BR-CR-15's threshold — file stats only beyond this.
const maxFullDiffFiles = 50

// renderCommitMessagePrompt maps the already-fetched context (Execute's job) to the ai.complete prompt; no I/O here.
func renderCommitMessagePrompt(branch string, recent []domain.CommitRef, diffOrStats string, issueRef string) (string, error) {
	in := git_commit_message.Input{Branch: branch, IssueRef: issueRef, Diff: diffOrStats}
	for _, c := range recent {
		in.Recent = append(in.Recent, git_commit_message.Commit{SHA: c.SHA, Message: c.Message})
	}
	prompt, err := git_commit_message.Render(in)
	if err != nil {
		return "", fmt.Errorf("render commit message prompt: %w", err)
	}
	return prompt, nil
}

// renderStatsOnlySummary is BR-CR-15's fallback body: one line per changed file, no diff content.
func renderStatsOnlySummary(files []domain.FileStatus) (string, error) {
	in := git_commit_message.StatsOnlyInput{}
	for _, f := range files {
		in.Files = append(in.Files, git_commit_message.File{State: string(f.State), Path: f.Path})
	}
	summary, err := git_commit_message.RenderStatsOnly(in)
	if err != nil {
		return "", fmt.Errorf("render file stats summary: %w", err)
	}
	return summary, nil
}
