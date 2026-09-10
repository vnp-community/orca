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
