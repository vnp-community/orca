package usecase

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/prompts/git_agent_prompt"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// asciiFiller is ASCII on purpose: sanitize cuts by bytes, and a multi-byte cut would pin invalid UTF-8 into a golden.
func asciiFiller(tag string, n int) string {
	var b strings.Builder
	for i := 1; b.Len() < n; i++ {
		fmt.Fprintf(&b, "%s line %04d: the quick brown fox jumps over the lazy dog\n", tag, i)
	}
	return b.String()[:n]
}

// The agent prompt is the prompt injected into the agent session that is spawned in a worktree created from an issue.
func TestPromptGolden_buildAgentPrompt(t *testing.T) {
	rendered := map[string]string{
		"title_only":                       mustAgentPrompt(t, "Fix the login redirect", "", "", nil),
		"title_and_description":            mustAgentPrompt(t, "Fix the login redirect", "Users land on a blank page after login.", "", nil),
		"with_acceptance_criteria":         mustAgentPrompt(t, "Fix the login redirect", "Users land on a blank page.", "- The user lands on the dashboard\n- The redirect keeps the query string", nil),
		"with_comments":                    mustAgentPrompt(t, "Fix the login redirect", "", "", []string{"Seen on staging too.", "Started after the last release."}),
		"all_sections":                     mustAgentPrompt(t, "Fix the login redirect", "Users land on a blank page.", "- The user lands on the dashboard", []string{"Seen on staging too.", "Started after the last release."}),
		"html_stripped_entities_unescaped": mustAgentPrompt(t, "<b>Fix</b> &amp; ship", "Before <script>alert('xss')</script> after &lt;3", "<ul><li>one</li></ul>", []string{"<i>note</i> Tom &amp; Jerry"}),
		"fields_cut_at_4000_bytes":         mustAgentPrompt(t, "Long fields", asciiFiller("description", git_agent_prompt.FieldBytes+200), asciiFiller("criteria", git_agent_prompt.FieldBytes+200), []string{asciiFiller("comment", git_agent_prompt.FieldBytes+200)}),
	}
	checkPromptGoldens(t, "buildAgentPrompt", rendered)
}

// The commit message prompt and the stats-only summary feed the ai.complete relay that writes the commit message.
func TestPromptGolden_buildCommitMessagePrompt(t *testing.T) {
	recent := []domain.CommitRef{
		{SHA: "0123456789abcdef0123456789abcdef01234567", Message: "feat(cache): add the landing page cache\n\nLonger body that must not reach the prompt."},
		{SHA: "abc1234", Message: "fix(login): keep the query string"},
		{SHA: "short", Message: "chore: bump deps"},
	}
	diff := "diff --git a/cache.go b/cache.go\n--- a/cache.go\n+++ b/cache.go\n@@ -1 +1,2 @@\n+package cache\n"
	rendered := map[string]string{
		"diff_only":                    mustCommitMessagePrompt(t, "", nil, diff, ""),
		"with_recent_commits":          mustCommitMessagePrompt(t, "", recent, diff, ""),
		"with_branch":                  mustCommitMessagePrompt(t, "feature/cache", nil, diff, ""),
		"with_issue_reference":         mustCommitMessagePrompt(t, "", nil, diff, "ORCA-123"),
		"with_numeric_issue_reference": mustCommitMessagePrompt(t, "", nil, diff, "#456"),
		"all_sections":                 mustCommitMessagePrompt(t, "feature/ORCA-123-cache", recent, diff, "ORCA-123"),
		"stats_only_fallback":          mustCommitMessagePrompt(t, "feature/cache", recent[:1], mustStatsOnlySummary(t, []domain.FileStatus{{Path: "cache.go", State: domain.FileStateAdded}, {Path: "main.go", State: domain.FileStateModified}, {Path: "old.go", State: domain.FileStateDeleted}}), ""),
	}
	checkPromptGoldens(t, "buildCommitMessagePrompt", rendered)
}

func TestPromptGolden_statsOnlySummary(t *testing.T) {
	rendered := map[string]string{
		"no_files": mustStatsOnlySummary(t, nil),
		"mixed_states": mustStatsOnlySummary(t, []domain.FileStatus{
			{Path: "cache.go", State: domain.FileStateAdded}, {Path: "main.go", State: domain.FileStateModified},
			{Path: "old.go", State: domain.FileStateDeleted}, {Path: "new/name.go", State: domain.FileStateRenamed, OldPath: "old/name.go"},
		}),
	}
	checkPromptGoldens(t, "statsOnlySummary", rendered)
}

// The pull request prompt used to be a prefix that Execute concatenated with the fetched diff; the golden pins the joined text.
func TestPromptGolden_prFieldsPromptPrefix(t *testing.T) {
	diff := "diff --git a/cache.go b/cache.go\n+package cache\n"
	checkPromptGoldens(t, "prFieldsPromptPrefix", map[string]string{
		"prefix_with_diff": mustPullRequestPrompt(t, diff),
	})
}
