package usecase

import (
	"strings"
	"testing"
)

// TestRenderAgentPrompt_OmitsEmptyAcceptanceCriteriaAndComments is BR-PI-05's
// clean-omission guard: an issue with no acceptance criteria and no
// comments must not render empty "## Acceptance Criteria" / "## Comments"
// sections.
func TestRenderAgentPrompt_OmitsEmptyAcceptanceCriteriaAndComments(t *testing.T) {
	got := mustAgentPrompt(t, "Fix the bug", "Something is broken", "", nil)

	if want := "# Fix the bug\n\n"; got[:len(want)] != want {
		t.Fatalf("expected the title heading first, got %q", got)
	}
	if !strings.Contains(got, "## Description\nSomething is broken\n\n") {
		t.Fatalf("expected the description section, got %q", got)
	}
	if strings.Contains(got, "## Acceptance Criteria") {
		t.Errorf("expected no Acceptance Criteria section when empty, got %q", got)
	}
	if strings.Contains(got, "## Comments") {
		t.Errorf("expected no Comments section when there are no comments, got %q", got)
	}
}

func TestRenderAgentPrompt_IncludesAllSectionsWhenPopulated(t *testing.T) {
	got := mustAgentPrompt(t, "Title", "Desc", "AC1", []string{"comment one", "comment two"})

	for _, want := range []string{
		"# Title\n\n",
		"## Description\nDesc\n\n",
		"## Acceptance Criteria\nAC1\n\n",
		"## Comments\n",
		"- comment one\n",
		"- comment two\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected prompt to contain %q, got %q", want, got)
		}
	}
}

func TestRenderAgentPrompt_SanitizesEveryField(t *testing.T) {
	got := mustAgentPrompt(t, "<b>Title</b>", "<script>bad()</script>Desc", "<i>AC</i>", []string{"<script>x</script>comment"})
	if strings.Contains(got, "<b>") || strings.Contains(got, "<script>") || strings.Contains(got, "<i>") {
		t.Fatalf("expected every field to be sanitized before composing, got %q", got)
	}
}
