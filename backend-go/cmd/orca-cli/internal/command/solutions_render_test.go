package command

import (
	"strings"
	"testing"
)

func TestRenderSolutions_ShowsWhatTheAgentReadInTheKnowledgeGraph(t *testing.T) {
	out := renderSolutions(parseDoc(t, `{"solutions":[{"id":"s1","status":"proposed","options":{"options":[{"id":"opt-1","title":"Cache","recommended":true}],
		"design_references":{"available":true,"snapshot":"kg-1","read":["MOD-a","DEC-7"],"gaps":["no constraint on TTL"],"conflicts":["DEC-7 forbids a second cache"]}}}]}`))
	for _, want := range []string{"read 2 element(s) from snapshot kg-1", "conflict: DEC-7 forbids a second cache", "gap: no constraint on TTL"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderSolutions_SaysWhenTheGraphWasNotAvailableAndStaysQuietWithoutAnAgent(t *testing.T) {
	out := renderSolutions(parseDoc(t, `{"solutions":[{"id":"s1","status":"proposed","options":{"options":[{"id":"opt-1","title":"Cache"}],"design_references":{"available":false,"gaps":["graph missing"]}}}]}`))
	if !strings.Contains(out, "knowledge graph: not available") || !strings.Contains(out, "gap: graph missing") {
		t.Fatalf("out:\n%s", out)
	}
	plain := renderSolutions(parseDoc(t, `{"solutions":[{"id":"s1","status":"proposed","options":{"options":[{"id":"opt-1","title":"Cache"}]}}]}`))
	if strings.Contains(plain, "knowledge graph") {
		t.Fatalf("a Solution without references mentions the graph:\n%s", plain)
	}
}
