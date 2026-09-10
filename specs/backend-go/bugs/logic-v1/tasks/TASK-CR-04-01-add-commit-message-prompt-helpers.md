# TASK-CR-04-01: Add pure commit-message prompt helpers (issue-ref extraction, prompt composition, stats fallback)

**From Solution:** SOL-CR-04
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/commit_message_prompt.go` (new), `backend-go/services/git-gateway-service/internal/usecase/commit_message_prompt_test.go` (new)
**Depends on:** none
**Status:** `[x]` DONE — commit_message_prompt.go created (extractIssueRef, buildCommitMessagePrompt, statsOnlySummary, maxFullDiffFiles); commit_message_prompt_test.go passing

---

## Context

The one genuinely new piece of logic in BUG-CR-04 — extracting an
issue/ticket id from a branch name (BR-CR-16) — is pure string parsing with
no port dependency. Splitting it into its own file (rather than inlining
into `Execute`) makes it unit-testable without any of
`GenerateCommitMessage`'s five ports, per
`03-clean-architecture-guidelines.md`'s domain-purity goal.

## Changes to make

Create `backend-go/services/git-gateway-service/internal/usecase/commit_message_prompt.go`:

```go
package usecase

import (
	"fmt"
	"regexp"
	"strings"

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

// commitMessagePromptPrefix frames the staged diff for ai.complete — moved
// here from generate_commit_message.go so every prompt-composing constant
// lives in one file.
const commitMessagePromptPrefix = "Write a concise, conventional-commits-style commit message for the following staged diff. Reply with only the commit message text.\n\n"

// buildCommitMessagePrompt composes the full ai.complete prompt: staged
// diff (or stats-only fallback), recent-commit style context, and branch/
// issue context. All inputs are already-fetched values (Execute's job) —
// this function only formats, no I/O, no ports.
func buildCommitMessagePrompt(branch string, recent []domain.CommitRef, diffOrStats string, issueRef string) string {
	var b strings.Builder
	b.WriteString(commitMessagePromptPrefix)

	if len(recent) > 0 {
		b.WriteString("\nRecent commits on this project, for style/convention matching:\n")
		for _, c := range recent {
			sha := c.SHA
			if len(sha) > 7 {
				sha = sha[:7]
			}
			fmt.Fprintf(&b, "%s %s\n", sha, firstLine(c.Message))
		}
	}
	if branch != "" {
		fmt.Fprintf(&b, "\nCurrent branch: %s\n", branch)
	}
	if issueRef != "" {
		fmt.Fprintf(&b, "Issue/ticket reference to include (as a trailing \"Refs: %s\" line): %s\n", issueRef, issueRef)
	}
	b.WriteString("\n")
	b.WriteString(diffOrStats)
	return b.String()
}

// firstLine returns s up to its first newline — recent-commit messages may
// be multi-line, but the style-matching context only needs the summary
// line.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// statsOnlySummary is BR-CR-15's fallback body — one line per changed
// file, no diff content, when the change is too large to send in full.
func statsOnlySummary(files []domain.FileStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Staged changes are large (%d files) — showing file stats only:\n", len(files))
	for _, f := range files {
		fmt.Fprintf(&b, "%s %s\n", f.State, f.Path)
	}
	return b.String()
}
```

`generate_commit_message.go` currently defines its own
`commitMessagePromptPrefix` const — TASK-CR-04-03 removes that duplicate
when it wires this file in.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./internal/usecase/...
go test ./internal/usecase/... -run TestExtractIssueRef -v
go test ./internal/usecase/... -run TestBuildCommitMessagePrompt -v
go test ./internal/usecase/... -run TestStatsOnlySummary -v
```

Create `commit_message_prompt_test.go` with cases:
- `extractIssueRef("fix/ORCA-123-foo")` → `"ORCA-123"`;
  `extractIssueRef("feature/456-thing")` → `"#456"`;
  `extractIssueRef("main")` → `""`.
- `buildCommitMessagePrompt` output contains the recent-commit block, the
  branch line, and the issue-reference instruction line when present;
  omits each section cleanly when its input is empty (no dangling
  headers).
- `statsOnlySummary` renders one line per file, includes the file count in
  the header line.
