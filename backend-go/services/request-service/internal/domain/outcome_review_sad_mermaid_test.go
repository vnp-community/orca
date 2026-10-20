package domain

import (
	"errors"
	"os"
	"testing"
)

func readSADBlock(t *testing.T, name string) MermaidDiagram {
	t.Helper()
	raw, err := os.ReadFile("testdata/sad/" + name)
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseMermaid(string(raw))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return d
}

func edgeSet(d MermaidDiagram) map[string]MermaidEdge {
	out := map[string]MermaidEdge{}
	for _, e := range d.Edges {
		out[e.From+">"+e.To] = e
	}
	return out
}

// Fixtures are sections 5.5, 8.1 and 8.4 of the SAD of vnp-qa-platform, copied byte for byte.
func TestParseMermaid_SAD55ContainerFlowchart(t *testing.T) {
	d := readSADBlock(t, "sad-5-5-container-flowchart.mmd")
	if d.Kind != MermaidFlowchart || len(d.Nodes) != 17 || len(d.Edges) != 40 || len(d.Warnings) != 0 {
		t.Fatalf("kind=%s nodes=%d edges=%d warnings=%v", d.Kind, len(d.Nodes), len(d.Edges), d.Warnings)
	}
	es := edgeSet(d)
	for _, want := range []string{"SPA>GW", "GW>BG", "AT>MY", "CM>KF", "KF>IN", "PM>S3", "TP>AT", "IN>EXT", "IN>BG", "SS>RD"} {
		if _, ok := es[want]; !ok {
			t.Errorf("missing edge %s", want)
		}
	}
	if es["IN>EXT"].Label != "Jira, Google" || es["IN>BG"].Label != "/internal/bug" {
		t.Errorf("labels: %+v %+v", es["IN>EXT"], es["IN>BG"])
	}
	labels := map[string]MermaidNode{}
	for _, n := range d.Nodes {
		labels[n.ID] = n
	}
	if labels["GW"].Label != "ms-gateway route table, auth, MCP" || labels["MY"].Shape != "datastore" || labels["KF"].Shape != "queue" || labels["SPA"].Label != "SPA Vue 3" {
		t.Errorf("node labels/shapes: %+v %+v %+v %+v", labels["GW"], labels["MY"], labels["KF"], labels["SPA"])
	}
}

func TestParseMermaid_SAD81Sequence(t *testing.T) {
	d := readSADBlock(t, "sad-8-1-sequence.mmd")
	if d.Kind != MermaidSequence || len(d.Nodes) != 8 || len(d.Messages) != 9 {
		t.Fatalf("kind=%s participants=%d messages=%d", d.Kind, len(d.Nodes), len(d.Messages))
	}
	if len(d.Warnings) != 1 || d.Warnings[0] != "mermaid_self_message_skipped:17" {
		t.Errorf("the self call IN->>IN must be skipped with a warning: %v", d.Warnings)
	}
	first, reply := d.Messages[0], d.Messages[3]
	if first.From != "U" || first.To != "GW" || first.Order != 1 || first.Label != "POST /bug/update/status" {
		t.Errorf("first message: %+v", first)
	}
	if !reply.Reply || reply.From != "BM" || reply.To != "U" || reply.Order != 4 {
		t.Errorf("reply: %+v", reply)
	}
	if d.Nodes[0].Label != "Người dùng" {
		t.Errorf("participant alias: %+v", d.Nodes[0])
	}
	for i, m := range d.Messages {
		if m.Order != i+1 {
			t.Errorf("order must follow the kept messages: %+v", m)
		}
	}
}

func TestParseMermaid_SAD84SubgraphFlowchart(t *testing.T) {
	d := readSADBlock(t, "sad-8-4-bug-jira-flowchart.mmd")
	if len(d.Nodes) != 10 || len(d.Edges) != 11 || len(d.Warnings) != 0 {
		t.Fatalf("nodes=%d edges=%d warnings=%v", len(d.Nodes), len(d.Edges), d.Warnings)
	}
	es := edgeSet(d)
	if !es["W>JR"].Both || es["W>JR"].Label != "REST v2/v3" || es["OB>C"].Label != "tms.bug.v1" || es["N>SS"].Label != "tms.integration.v1" {
		t.Errorf("edges: %+v %+v %+v", es["W>JR"], es["OB>C"], es["N>SS"])
	}
	groups := map[string]string{}
	for _, n := range d.Nodes {
		groups[n.ID] = n.Group
	}
	if groups["BM"] != "TMS" || groups["W"] != "ms-integration" || groups["JR"] != "" {
		t.Errorf("subgraph groups: %v", groups)
	}
}

func TestParseMermaid_UnknownBlocksAreIgnoredNotFatal(t *testing.T) {
	for _, text := range []string{"erDiagram\n  A ||--o{ B : has", "stateDiagram-v2\n  [*] --> A", "", "pie title x\n \"a\": 1"} {
		if _, err := ParseMermaid(text); !errors.Is(err, ErrUnsupportedMermaid) {
			t.Errorf("%q: got %v, want ErrUnsupportedMermaid", text, err)
		}
	}
	// Garbage inside a supported block is skipped line by line with a warning, never a panic.
	d, err := ParseMermaid("flowchart LR\n  A --> B\n  ??? !!!\n  B -.-> C\n  C -- hop --> D\n  %% comment\n  classDef x fill:#fff")
	if err != nil {
		t.Fatal(err)
	}
	es := edgeSet(d)
	if len(d.Warnings) != 1 || es["A>B"].From == "" || es["B>C"].From == "" || es["C>D"].Label != "hop" {
		t.Errorf("edges=%v warnings=%v", es, d.Warnings)
	}
}

func TestParseMermaid_SizeLimits(t *testing.T) {
	if _, err := ParseMermaid("flowchart LR\n" + string(make([]byte, maxMermaidBytes+1))); err == nil {
		t.Error("an oversized block must be refused")
	}
}

func FuzzParseMermaid(f *testing.F) {
	for _, s := range []string{"flowchart LR\nA-->B", "sequenceDiagram\nA->>B: x", "flowchart TD\nsubgraph X[\"y\"]\nA[(db)]-->|l|B\nend"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) { _, _ = ParseMermaid(s) })
}
