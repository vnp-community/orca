# TASK-PI-02-03: `generateBranchName` (BR-PI-04) + `buildAgentPrompt` (BR-PI-05) pure functions

**From Solution:** SOL-PI-02
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/branch_name.go` (new), `backend-go/services/git-gateway-service/internal/usecase/agent_prompt.go` (new)
**Depends on:** none
**Status:** `[x] DONE — branch_name.go (generateBranchName/BR-PI-04) + agent_prompt.go (buildAgentPrompt/BR-PI-05) added, no adapter deps.`

---

## Context

Branch-name generation and prompt sanitization are stateless string
transforms with no external call — SOL-PI-02's two genuinely new pieces of
logic (no TDD precedent, small enough not to need one). Kept as their own
files so they're unit-testable without any port fakes, ahead of
`create_worktree_from_issue.go` (TASK-PI-02-05) which calls them.

## Changes to make

`internal/usecase/branch_name.go` (new):

```go
package usecase

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// generateBranchName implements BR-PI-04: "type/description-issueId".
// type is inferred from issue labels ("bug"->"fix", "enhancement"/"feature"->"feat",
// no match->"chore"); description is the title kebab-cased and truncated to
// 40 chars; issueId is the provider's issue number/key.
func generateBranchName(title string, labels []string, externalRef string) string {
	kind := inferBranchType(labels)
	desc := kebabCase(truncate(title, 40))
	return fmt.Sprintf("%s/%s-%s", kind, desc, sanitizeRefForBranch(externalRef))
}

func inferBranchType(labels []string) string {
	for _, l := range labels {
		switch strings.ToLower(l) {
		case "bug":
			return "fix"
		case "enhancement", "feature":
			return "feat"
		}
	}
	return "chore"
}

func kebabCase(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// sanitizeRefForBranch strips characters git branch names can't contain
// (e.g. "#") from a provider issue ref like "owner/repo#123" or "ENG-123".
func sanitizeRefForBranch(ref string) string {
	return nonAlnum.ReplaceAllString(strings.ToLower(ref), "-")
}
```

`internal/usecase/agent_prompt.go` (new):

```go
package usecase

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

const maxPromptFieldLen = 4000

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

// buildAgentPrompt implements BR-PI-05: sanitize before composing — strip
// HTML/script content, truncate any single field so one hostile/huge issue
// body can't blow the agent's context budget.
func buildAgentPrompt(title, description, acceptanceCriteria string, comments []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", sanitize(title))
	if description != "" {
		fmt.Fprintf(&b, "## Description\n%s\n\n", sanitize(description))
	}
	if acceptanceCriteria != "" {
		fmt.Fprintf(&b, "## Acceptance Criteria\n%s\n\n", sanitize(acceptanceCriteria))
	}
	if len(comments) > 0 {
		b.WriteString("## Comments\n")
		for _, c := range comments {
			fmt.Fprintf(&b, "- %s\n", sanitize(c))
		}
	}
	return b.String()
}

func sanitize(s string) string {
	s = htmlTagPattern.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return truncate(s, maxPromptFieldLen)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go vet ./services/git-gateway-service/...
```
