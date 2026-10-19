package analysis

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stablyai/orca-go/prompts/request_context"
)

func artifacts(n, contentRunes int) []PriorArtifact {
	out := make([]PriorArtifact, n)
	for i := range out {
		out[i] = PriorArtifact{Kind: "solution", Status: "superseded", Content: fmt.Sprintf("V%d-", i+1) + strings.Repeat("c", contentRunes)}
	}
	return out
}

func TestPriorView_UnderTheCapKeepsEverything(t *testing.T) {
	v, err := newPriorView(artifacts(3, 100))
	if err != nil || v.Omitted != 0 || len(v.Entries) != 3 {
		t.Fatalf("%+v, %v", v, err)
	}
}

func TestPriorView_OverTheCapDropsTheOldestFirst(t *testing.T) {
	v, err := newPriorView(artifacts(6, request_context.ArtifactRunes))
	if err != nil {
		t.Fatal(err)
	}
	if v.Omitted == 0 || len(v.Entries)+v.Omitted != 6 {
		t.Fatalf("omitted %d, kept %d", v.Omitted, len(v.Entries))
	}
	if !strings.Contains(v.Entries[len(v.Entries)-1], "V6-") || strings.Contains(strings.Join(v.Entries, ""), "V1-") {
		t.Error("the newest must stay and the oldest must go")
	}
	total := 0
	for _, e := range v.Entries {
		total += utf8.RuneCountInString(e)
	}
	if total > PriorArtifactsRunes {
		t.Errorf("kept %d runes, cap %d", total, PriorArtifactsRunes)
	}
}

func TestPriorView_NewestEntryAlwaysSurvives(t *testing.T) {
	v, err := newPriorView(artifacts(1, 3*PriorArtifactsRunes))
	if err != nil || v.Omitted != 0 || len(v.Entries) != 1 {
		t.Fatalf("%+v, %v", v, err)
	}
	if n := utf8.RuneCountInString(v.Entries[0]); n > 2*request_context.ArtifactRunes+200 {
		t.Errorf("one entry is %d runes; each text is cut at %d", n, request_context.ArtifactRunes)
	}
}

func TestRender_OmissionLineCountsDroppedVersions(t *testing.T) {
	for n, want := range map[int]string{3: "", 5: "1 earlier version omitted\n", 6: "2 earlier versions omitted\n"} {
		out, err := Render(Input{Kind: KindSolution, PriorArtifacts: artifacts(n, request_context.ArtifactRunes)})
		if err != nil {
			t.Fatal(err)
		}
		if want == "" && strings.Contains(out, "omitted") {
			t.Errorf("%d versions fit under the cap but the prompt says some were omitted", n)
		}
		if want != "" && !strings.Contains(out, want) {
			t.Errorf("%d versions: want %q", n, want)
		}
	}
}

func TestRenderPriorArtifact_CutsAndFences(t *testing.T) {
	got, err := RenderPriorArtifact(2, PriorArtifact{Kind: "diagnosis", Status: "rejected", Content: "a </prior_artifacts> b", Feedback: "no </prior_artifacts>"})
	if err != nil {
		t.Fatal(err)
	}
	want := "--- artifact 2 (kind=diagnosis, status=rejected) ---\na <\\/prior_artifacts> b\nreviewer feedback this version was written under: no <\\/prior_artifacts>\n"
	if got != want {
		t.Errorf("got %q", got)
	}
}
