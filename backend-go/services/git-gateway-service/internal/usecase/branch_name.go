package usecase

import (
	"fmt"
	"regexp"
	"strings"
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
