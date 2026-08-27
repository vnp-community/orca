package usecase

import (
	"strings"
	"testing"
)

// TestSanitize_StripsScriptAndHTMLTags is BR-PI-05's core guard: hostile
// HTML/script content in an issue field must never reach the agent prompt
// verbatim.
func TestSanitize_StripsScriptAndHTMLTags(t *testing.T) {
	in := `Before <script>alert('xss')</script> after <b>bold</b> text`
	got := sanitize(in)
	want := "Before alert('xss') after bold text"
	if got != want {
		t.Fatalf("sanitize(%q) = %q, want %q", in, got, want)
	}
}

func TestSanitize_TruncatesOversizedField(t *testing.T) {
	oversized := make([]byte, maxPromptFieldLen+500)
	for i := range oversized {
		oversized[i] = 'a'
	}
	got := sanitize(string(oversized))
	if len(got) != maxPromptFieldLen {
		t.Fatalf("expected sanitize to truncate to %d chars, got %d", maxPromptFieldLen, len(got))
	}
}

func TestSanitize_UnescapesHTMLEntities(t *testing.T) {
	got := sanitize("Tom &amp; Jerry &lt;3")
	want := "Tom & Jerry <3"
	if got != want {
		t.Fatalf("sanitize(...) = %q, want %q", got, want)
	}
}

// TestBuildAgentPrompt_OmitsEmptyAcceptanceCriteriaAndComments is BR-PI-05's
// clean-omission guard: an issue with no acceptance criteria and no
// comments must not render empty "## Acceptance Criteria" / "## Comments"
// sections.
func TestBuildAgentPrompt_OmitsEmptyAcceptanceCriteriaAndComments(t *testing.T) {
	got := buildAgentPrompt("Fix the bug", "Something is broken", "", nil)

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

func TestBuildAgentPrompt_IncludesAllSectionsWhenPopulated(t *testing.T) {
	got := buildAgentPrompt("Title", "Desc", "AC1", []string{"comment one", "comment two"})

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

func TestBuildAgentPrompt_SanitizesEveryField(t *testing.T) {
	got := buildAgentPrompt("<b>Title</b>", "<script>bad()</script>Desc", "<i>AC</i>", []string{"<script>x</script>comment"})
	if strings.Contains(got, "<b>") || strings.Contains(got, "<script>") || strings.Contains(got, "<i>") {
		t.Fatalf("expected every field to be sanitized before composing, got %q", got)
	}
}
