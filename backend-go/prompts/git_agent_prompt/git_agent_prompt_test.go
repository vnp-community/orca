package git_agent_prompt

import (
	"strings"
	"testing"
)

func TestRender_ExactText(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want string
	}{
		{"title only", Input{Title: "T"}, "# T\n\n"},
		{"description", Input{Title: "T", Description: "D"}, "# T\n\n## Description\nD\n\n"},
		{"acceptance criteria", Input{Title: "T", AcceptanceCriteria: "- A"}, "# T\n\n## Acceptance Criteria\n- A\n\n"},
		{"comments", Input{Title: "T", Comments: []string{"one", "two"}}, "# T\n\n## Comments\n- one\n- two\n"},
		{"all sections", Input{Title: "T", Description: "D", AcceptanceCriteria: "A", Comments: []string{"c"}}, "# T\n\n## Description\nD\n\n## Acceptance Criteria\nA\n\n## Comments\n- c\n"},
		{"html is stripped and entities unescaped", Input{Title: "<b>Fix</b> &amp; ship", Description: "x <script>alert('xss')</script> y &lt;3"}, "# Fix & ship\n\n## Description\nx alert('xss') y <3\n\n"},
		// Presence is decided on the raw value, so a field that is only markup keeps its empty section.
		{"markup-only description keeps its heading", Input{Title: "T", Description: "<br>"}, "# T\n\n## Description\n\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tc.in)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v\nwant %q", got, err, tc.want)
			}
		})
	}
}

func TestRender_EmptyInputStillRenders(t *testing.T) {
	got, err := Render(Input{})
	if err != nil || got != "# \n\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestSanitize(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"script and tags", `Before <script>alert('xss')</script> after <b>bold</b> text`, "Before alert('xss') after bold text"},
		{"entities", "Tom &amp; Jerry &lt;3", "Tom & Jerry <3"},
		// Tags go first and entities after, so an escaped tag survives as text.
		{"escaped tag is not stripped", "&lt;b&gt;x&lt;/b&gt;", "<b>x</b>"},
	}
	for _, tc := range cases {
		if got := Sanitize(tc.in); got != tc.want {
			t.Errorf("%s: Sanitize(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestSanitize_CutsAtFieldBytes(t *testing.T) {
	if got := Sanitize(strings.Repeat("a", FieldBytes+500)); len(got) != FieldBytes {
		t.Fatalf("len = %d, want %d", len(got), FieldBytes)
	}
	exact := strings.Repeat("b", FieldBytes)
	if Sanitize(exact) != exact {
		t.Error("a field of exactly FieldBytes must be left alone")
	}
}

// The cut counts bytes, as the original did; the golden files pin that, so a change here is a prompt change.
func TestSanitize_CutIsByBytesNotRunes(t *testing.T) {
	got := Sanitize(strings.Repeat("a", FieldBytes-1) + "é")
	if len(got) != FieldBytes {
		t.Fatalf("len = %d, want %d", len(got), FieldBytes)
	}
}

func TestRender_EveryFieldIsSanitized(t *testing.T) {
	got, err := Render(Input{Title: "<b>T</b>", Description: "<script>bad()</script>D", AcceptanceCriteria: "<i>AC</i>", Comments: []string{"<script>x</script>c"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(got, "<>") {
		t.Fatalf("markup reached the prompt: %q", got)
	}
}

func TestRender_TemplateSyntaxInAFieldIsNotEvaluated(t *testing.T) {
	hostile := `{{.Title}} {{template "x"}}`
	got, err := Render(Input{Title: hostile, Description: hostile, AcceptanceCriteria: hostile, Comments: []string{hostile}})
	if err != nil || strings.Count(got, hostile) != 4 {
		t.Fatalf("got %q, %v", got, err)
	}
}
