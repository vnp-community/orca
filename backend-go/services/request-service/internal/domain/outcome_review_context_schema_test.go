package domain

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compileContextSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile("../../schemas/outcome-review/outcome-review-context-1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	const url = "https://orca.local/schemas/outcome-review/outcome-review-context-1.schema.json"
	if err := c.AddResource(url, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(url)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// withoutNulls drops the members a Go struct writes as null for a slice or pointer that was left unset: an agent leaves them out.
func withoutNulls(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if e == nil {
				delete(x, k)
				continue
			}
			x[k] = withoutNulls(e)
		}
		return x
	case []any:
		for i := range x {
			x[i] = withoutNulls(x[i])
		}
		return x
	}
	return v
}

func validateAgainstSchema(t *testing.T, s *jsonschema.Schema, v any) error {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	return s.Validate(withoutNulls(generic))
}

// The Go types and the schema are two statements of one contract; this keeps them from drifting apart.
func TestContextSchema_TheGoContractConformsToTheSchema(t *testing.T) {
	s := compileContextSchema(t)
	full := pilotResult(t)
	full.Items = []AgentReviewItem{{Subject: "s", Section: SectionContracts, Change: ChangeAdded, Title: "t", Severity: SeverityBreaking, NeedsDecision: true,
		Source: SourceKG, ReviewState: KGApproved, Evidence: []ReviewEvidence{{Kind: EvidenceFile, Ref: "a.go", Line: 3}, {Kind: EvidenceSource, Doc: sadDoc, Section: "5.5"}}}}
	full.Diagrams = []DiagramModel{
		archDelta([]DiagramNode{kgNode("CT-02", "ms-gateway", true, true), {ID: "x", Source: SourceCode, Label: "x", Kind: "service", Parent: "CT-02", Columns: []DiagramColumn{{Name: "c", Type: "t"}}}},
			[]DiagramEdge{{From: "CT-02", To: "x", InHead: true, Order: 1, BaseOrder: 2, Evidence: fileEv("a.go")}}),
	}
	full.Warnings = []string{"budget_exhausted"}
	if err := validateAgainstSchema(t, s, full); err != nil {
		t.Errorf("a result built from every field of the Go types must conform: %v", err)
	}
	if err := validateAgainstSchema(t, s, ContextResult{Schema: ContextSchema}); err != nil {
		t.Errorf("the smallest result: %v", err)
	}
}

func TestContextSchema_RefusesWhatTheContractForbids(t *testing.T) {
	s := compileContextSchema(t)
	base := func() map[string]any {
		return map[string]any{"schema": ContextSchema}
	}
	cases := map[string]func(m map[string]any){
		"another schema": func(m map[string]any) { m["schema"] = "outcome-review-context/2" },
		"an unknown key": func(m map[string]any) { m["verdict"] = "pass" },
		"a state on an edge": func(m map[string]any) {
			m["diagrams"] = []any{map[string]any{"section": "architecture", "type": "architecture-delta", "variant": "delta", "edges": []any{map[string]any{"from": "a", "to": "b", "state": "added"}}}}
		},
		"a section that is not one": func(m map[string]any) {
			m["items"] = []any{map[string]any{"section": "ui", "change": "added", "title": "t"}}
		},
		"a bad graph id":     func(m map[string]any) { m["kg"] = map[string]any{"read": []any{"fr-1"}} },
		"a bad document sum": func(m map[string]any) { m["design"] = map[string]any{"docSha256": map[string]any{"a.md": "abc"}} },
		"a bad container kind": func(m map[string]any) {
			m["design"] = map[string]any{"containers": []any{map[string]any{"label": "x", "kind": "class", "source": "kg"}}}
		},
		"a code source in a design": func(m map[string]any) {
			m["design"] = map[string]any{"containers": []any{map[string]any{"label": "x", "kind": "service", "source": "code"}}}
		},
		"an unknown warning": func(m map[string]any) { m["warnings"] = []any{"<script>"} },
		"too many items": func(m map[string]any) {
			items := make([]any, 121)
			for i := range items {
				items[i] = map[string]any{"section": "flow", "change": "added", "title": "t"}
			}
			m["items"] = items
		},
		"an evidence kind that does not exist": func(m map[string]any) {
			m["items"] = []any{map[string]any{"section": "flow", "change": "added", "title": "t", "evidence": []any{map[string]any{"kind": "hunch"}}}}
		},
	}
	for name, mutate := range cases {
		m := base()
		mutate(m)
		if err := s.Validate(m); err == nil {
			t.Errorf("%s: the schema accepted it", name)
		}
	}
	if err := s.Validate(base()); err != nil {
		t.Fatalf("control: %v", err)
	}
}
