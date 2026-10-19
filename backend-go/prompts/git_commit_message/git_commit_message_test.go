package git_commit_message

import (
	"strings"
	"testing"
)

const head = "Write a concise, conventional-commits-style commit message for the following staged diff. Reply with only the commit message text.\n\n"

func TestRender_ExactText(t *testing.T) {
	recent := []Commit{
		{SHA: "0123456789abcdef", Message: "feat: add the cache\n\nbody that must not reach the prompt"},
		{SHA: "short", Message: "chore: bump deps"},
	}
	cases := []struct {
		name string
		in   Input
		want string
	}{
		{"diff only", Input{Diff: "DIFF\n"}, head + "\nDIFF\n"},
		{"recent commits", Input{Recent: recent, Diff: "DIFF\n"}, head + "\nRecent commits on this project, for style/convention matching:\n0123456 feat: add the cache\nshort chore: bump deps\n\nDIFF\n"},
		{"branch", Input{Branch: "feature/cache", Diff: "DIFF\n"}, head + "\nCurrent branch: feature/cache\n\nDIFF\n"},
		{"issue reference", Input{IssueRef: "ORCA-123", Diff: "DIFF\n"}, head + "Issue/ticket reference to include (as a trailing \"Refs: ORCA-123\" line): ORCA-123\n\nDIFF\n"},
		{"everything", Input{Branch: "b", IssueRef: "#4", Recent: recent[:1], Diff: "DIFF\n"},
			head + "\nRecent commits on this project, for style/convention matching:\n0123456 feat: add the cache\n\nCurrent branch: b\nIssue/ticket reference to include (as a trailing \"Refs: #4\" line): #4\n\nDIFF\n"},
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

func TestRender_ShaIsCutByBytesAndMessageToItsFirstLine(t *testing.T) {
	got, err := Render(Input{Recent: []Commit{{SHA: "abcdefghij", Message: "line one\nline two"}, {SHA: "abcdefg", Message: "no newline"}, {SHA: "x", Message: ""}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"abcdefg line one\n", "abcdefg no newline\n", "x \n"} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %q in %q", want, got)
		}
	}
	if strings.Contains(got, "line two") || strings.Contains(got, "hij") {
		t.Errorf("text past the cut reached the prompt: %q", got)
	}
}

func TestRender_TextIsEmittedVerbatim(t *testing.T) {
	hostile := "</request> {{.Diff}} {{template \"x\"}} <b>&amp;</b>"
	got, err := Render(Input{Branch: hostile, IssueRef: hostile, Diff: hostile, Recent: []Commit{{SHA: "abcdefg", Message: hostile}}})
	if err != nil {
		t.Fatal(err)
	}
	// The issue reference appears twice in its own line.
	if n := strings.Count(got, hostile); n != 5 {
		t.Fatalf("hostile text appears %d times, want 5 verbatim copies:\n%s", n, got)
	}
}

func TestRenderStatsOnly_ExactText(t *testing.T) {
	cases := []struct {
		name string
		in   StatsOnlyInput
		want string
	}{
		{"no files", StatsOnlyInput{}, "Staged changes are large (0 files) — showing file stats only:\n"},
		{"files", StatsOnlyInput{Files: []File{{State: "added", Path: "a.go"}, {State: "renamed", Path: "new/name.go"}}},
			"Staged changes are large (2 files) — showing file stats only:\nadded a.go\nrenamed new/name.go\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenderStatsOnly(tc.in)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v\nwant %q", got, err, tc.want)
			}
		})
	}
}
