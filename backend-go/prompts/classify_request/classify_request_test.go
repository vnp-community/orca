package classify_request

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
)

var boundaryRe = regexp.MustCompile(`<<<REQUEST_DATA_[0-9a-f]{16}>>>`)

func TestRender_FullInputUsesOneBoundaryTwice(t *testing.T) {
	out, err := Render(Input{Title: "Slow page", IssueType: "Bug", Labels: []string{"a", "b"}, Body: "It takes four seconds."})
	if err != nil {
		t.Fatal(err)
	}
	found := boundaryRe.FindAllString(out, -1)
	if len(found) != 2 || found[0] != found[1] {
		t.Fatalf("want one boundary twice, got %v", found)
	}
	want := found[0] + "\ntitle: Slow page\nissue_type: Bug\nlabels: a, b\nbody:\nIt takes four seconds.\n" + found[0] + "\n\nReply with exactly one JSON object"
	if !strings.Contains(out, want) {
		t.Errorf("data block malformed:\n%s", out)
	}
	if strings.HasSuffix(out, "\n") {
		t.Error("the prompt ends with the JSON line and no newline")
	}
}

func TestRender_BoundaryIsRandomPerCall(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		out, err := Render(Input{Title: "t", Body: "b"})
		if err != nil {
			t.Fatal(err)
		}
		seen[boundaryRe.FindString(out)] = true
	}
	if len(seen) < 15 {
		t.Fatalf("saw %d distinct boundaries in 20 calls", len(seen))
	}
}

func TestRender_BoundaryAvoidsTextAlreadyInTheContent(t *testing.T) {
	// The first draw equals a boundary the body already contains, so a second draw must be used.
	first := bytes.Repeat([]byte{0x00}, 8)
	second := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	decoy := "<<<REQUEST_DATA_0000000000000000>>>"
	out, err := renderWith(Input{Title: "t", Body: decoy}, bytes.NewReader(append(first, second...)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<<<REQUEST_DATA_0102030405060708>>>\ntitle: t") || strings.Count(out, decoy) != 1 {
		t.Errorf("the decoy must stay content and not become the boundary:\n%s", out)
	}
}

func TestRender_EntropyFailureIsAnError(t *testing.T) {
	_, err := renderWith(Input{Title: "t"}, bytes.NewReader([]byte{1, 2}))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("a failing entropy source must fail the render, got %v", err)
	}
}

func TestRender_BodyIsCutAtTheCap(t *testing.T) {
	out, err := Render(Input{Title: "t", Body: strings.Repeat("ệ", 30000)})
	if err != nil || strings.Count(out, "ệ") != BodyRunes {
		t.Fatalf("body runes = %d, %v", strings.Count(out, "ệ"), err)
	}
}

func TestRender_NoLabelsAndNoIssueType(t *testing.T) {
	out, err := Render(Input{Title: "Only a title"})
	if err != nil || !strings.Contains(out, "issue_type: \nlabels: \nbody:\n") {
		t.Fatalf("%q, %v", out, err)
	}
}
