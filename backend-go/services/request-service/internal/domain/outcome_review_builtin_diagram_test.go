package domain

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func tr(b bool) *bool { return &b }

func archModel() DiagramModel {
	return DiagramModel{
		Section: SectionArchitecture, Type: DiagramArchitecture, Variant: DiagramVariantDelta, Name: "CI gọi TMS qua API key", Level: LevelContainer,
		Nodes: []DiagramNode{
			{ID: "code:ci", Label: "CI runner (Jenkins/GHA/GitLab)", Kind: "external", Tech: "bash + curl", State: StateUnmapped, InHead: true, ComponentsChanged: 3},
			{ID: "CT-02", Label: "ms-gateway", Kind: "service", State: StateSame, InBase: true, InHead: true},
			{ID: "SVC-04", Label: "ms-test-plan", Kind: "service", State: StateSame, InBase: true, InHead: true},
			{ID: "DB-1", Label: "plan-db", Kind: "database", Tech: "mysql", State: StateAdded, InHead: true},
			{ID: "SVC-09", Label: "ms-notify", Kind: "service", State: StateMissing, InBase: true},
		},
		Edges: []DiagramEdge{
			{From: "code:ci", To: "CT-02", Protocol: "http", Label: "X-Access-Key/X-Secret-Key", State: StateDeviation, Advisory: true, InHead: true},
			{From: "CT-02", To: "SVC-04", Protocol: "http", Label: "test-plan execute/result", State: StateSame, InBase: true, InHead: true, InDesign: tr(true)},
			{From: "SVC-04", To: "DB-1", Protocol: "sql", State: StateAdded, InHead: true},
			{From: "CT-02", To: "SVC-04", Protocol: "grpc", State: StateRemoved, InBase: true},
			{From: "CT-02", To: "SVC-09", Protocol: "http", State: StateMissing, InBase: true},
			{From: "SVC-09", To: "CT-02", Protocol: "http", State: StateSame},
		},
		InputDigest: "d",
	}
}

func seqModel() DiagramModel {
	return DiagramModel{
		Section: SectionFlow, Type: DiagramSequence, Variant: DiagramVariantDelta, Name: "Cổng chất lượng CI",
		Nodes: []DiagramNode{{ID: "a", Label: "CI runner", State: StateSame}, {ID: "b", Label: "ms-gateway", State: StateSame}, {ID: "c", Label: "ms-test-plan", State: StateAdded}},
		Edges: []DiagramEdge{
			{From: "a", To: "b", Label: "execute", Protocol: "http", Order: 1, State: StateSame},
			{From: "b", To: "c", Label: "run plan", Protocol: "http", Order: 2, State: StateAdded},
			{From: "c", To: "c", Label: "queue", Order: 3, State: StateDeviation},
			{From: "c", To: "a", Label: "result", Protocol: "http", Order: 4, State: StateSame},
		},
	}
}

func stateModel() DiagramModel {
	return DiagramModel{
		Section: SectionBehavior, Type: DiagramStateMachine, Variant: DiagramVariantDelta, Name: "Run",
		Nodes: []DiagramNode{{ID: "q", Label: "Queued"}, {ID: "r", Label: "Running"}, {ID: "p", Label: "Passed", State: StateAdded}, {ID: "f", Label: "Failed"}, {ID: "x", Label: "Stopped", State: StateMissing}},
		Edges: []DiagramEdge{{From: "q", To: "r", Label: "start"}, {From: "r", To: "p", Label: "ok", State: StateAdded}, {From: "r", To: "f", Label: "error"}, {From: "r", To: "r", Label: "retry"}, {From: "r", To: "x", Label: "stop", State: StateMissing}},
	}
}

func schemaModel() DiagramModel {
	return DiagramModel{
		Section: SectionData, Type: DiagramDatabase, Variant: DiagramVariantDelta, Name: "Keys",
		Nodes: []DiagramNode{
			{ID: "k", Label: "api_key", State: StateAdded, Columns: []DiagramColumn{{Name: "id", Type: "bigint"}, {Name: "secret", Type: "varchar", State: StateAdded}}},
			{ID: "u", Label: "user", Columns: []DiagramColumn{{Name: "id", Type: "bigint"}}},
			{ID: "e", Label: "empty"},
		},
		Edges: []DiagramEdge{{From: "k", To: "u", Label: "owner"}},
	}
}

func TestBuiltinDiagram_EveryTypePassesTheArtifactCheck(t *testing.T) {
	for _, m := range []DiagramModel{archModel(), seqModel(), stateModel(), schemaModel()} {
		page, err := RenderBuiltinDiagram(m)
		if err != nil {
			t.Fatalf("%s: %v", m.Type, err)
		}
		info, err := CheckArtifact(page, m)
		if err != nil {
			t.Fatalf("%s: the drawing was refused: %v", m.Type, err)
		}
		if info.ViewBox == "" || info.Title == "" || info.Desc == "" {
			t.Fatalf("%s: info %+v", m.Type, info)
		}
		if info.ForeignWords != 0 {
			t.Fatalf("%s: %.2f of the words are not the model's", m.Type, info.ForeignWords)
		}
		if bytes.Contains(page, []byte("<script")) || bytes.Contains(page, []byte("http")) && bytes.Contains(page, []byte("href=\"http")) {
			t.Fatalf("%s: unsafe content", m.Type)
		}
		again, _ := RenderBuiltinDiagram(m)
		if !bytes.Equal(page, again) {
			t.Fatalf("%s: the drawing is not deterministic", m.Type)
		}
	}
}

func TestBuiltinDiagram_SaysChangeWithMoreThanColour(t *testing.T) {
	page, _ := RenderBuiltinDiagram(archModel())
	s := string(page)
	for _, want := range []string{"+ plan-db", "? ms-notify", "not in design", "! ", ">3<", "s-removed", "s-deviation", "rv-legend"} {
		if !strings.Contains(s, want) {
			t.Errorf("the drawing lacks %q", want)
		}
	}
	if strings.Contains(s, "#") && strings.Contains(strings.ReplaceAll(s, "url(#", ""), "fill=\"#") {
		t.Errorf("a colour is written outside the role variables")
	}
}

func TestBuiltinDiagram_HostileLabelsAreTextNotMarkup(t *testing.T) {
	m := archModel()
	m.Nodes[1].Label = `x"><script>alert(1)</script>`
	page, err := RenderBuiltinDiagram(m)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(page, []byte("<script")) {
		t.Fatalf("a label became markup")
	}
	if _, err := CheckArtifact(page, m); err != nil {
		t.Fatalf("an escaped label must still pass: %v", err)
	}
}

func TestBuiltinDiagram_RefusesWhatItCannotDraw(t *testing.T) {
	if _, err := RenderBuiltinDiagram(DiagramModel{Type: DiagramArchitecture}); err == nil {
		t.Fatal("a model with no node has nothing to draw")
	}
	if _, err := RenderBuiltinDiagram(DiagramModel{Type: "pie", Nodes: []DiagramNode{{ID: "a", Label: "a"}}}); err == nil {
		t.Fatal("an unknown type has no layout")
	}
}

// BUILTIN_DIAGRAM_DUMP=<dir> writes the pages so they can be looked at in a browser.
func TestBuiltinDiagram_Dump(t *testing.T) {
	dir := os.Getenv("BUILTIN_DIAGRAM_DUMP")
	if dir == "" {
		t.Skip("set BUILTIN_DIAGRAM_DUMP to write the pages")
	}
	for name, m := range map[string]DiagramModel{"arch": archModel(), "seq": seqModel(), "state": stateModel(), "schema": schemaModel()} {
		page, _ := RenderBuiltinDiagram(m)
		if err := os.WriteFile(dir+"/"+name+".html", page, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The caller of the first message is the left-most lane, whatever the order of the nodes in the model.
func TestBuiltinDiagram_SequenceLanesFollowTheFirstMessage(t *testing.T) {
	m := seqModel()
	m.Nodes = []DiagramNode{m.Nodes[2], m.Nodes[1], m.Nodes[0]} // c, b, a: the caller "a" is last in the model
	page, err := RenderBuiltinDiagram(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if strings.Index(s, ">CI runner<") > strings.Index(s, ">ms-gateway<") || strings.Index(s, ">ms-gateway<") > strings.Index(s, ">+ ms-test-plan<") {
		t.Fatalf("lanes are not in the order of the first messages")
	}
}

func TestDiagramFindingItems_AMissingOrDeviatingArrowIsARowAndAnAdvisoryOneAsksNothing(t *testing.T) {
	items := DiagramFindingItems([]DiagramModel{archModel(), seqModel()})
	byTitle := map[string]ReviewItem{}
	for _, it := range items {
		byTitle[it.Title] = it
	}
	dev, ok := byTitle["CI runner (Jenkins/GHA/GitLab) → ms-gateway (http)"]
	if !ok || dev.Origin != OriginFact || dev.Severity != SeverityDeviation || !dev.Advisory || dev.NeedsDecision || dev.Change != ChangeAdded {
		t.Fatalf("a deviation on unapproved elements is a visible advisory row: %+v", dev)
	}
	miss, ok := byTitle["ms-gateway → ms-notify (http)"]
	if !ok || miss.Change != ChangeMissing || !miss.NeedsDecision {
		t.Fatalf("an arrow the design has and the code lacks asks for a decision: %+v", miss)
	}
	for _, it := range items {
		if it.Key == "" || it.Section == "" {
			t.Fatalf("incomplete row: %+v", it)
		}
	}
	if len(DiagramFindingItems([]DiagramModel{func() DiagramModel { m := archModel(); m.Variant = DiagramVariantDesign; return m }()})) != 0 {
		t.Fatal("the design variant has no findings")
	}
}

func TestBuildOutcomeReview_DiagramRowsKeepTheirFactOriginAndAreNotCountedTwice(t *testing.T) {
	rows := DiagramFindingItems([]DiagramModel{archModel()})
	f := Facts{Phase: ReviewPhaseDelivered}
	f.Subject.BaseRef = "main"
	built, err := BuildOutcomeReview("r1", ReviewBuildInput{Facts: f, DiagramItems: rows, KG: KGInfo{Available: true}})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, it := range built.Items {
		if it.Section == SectionArchitecture && it.Origin == OriginFact && it.Severity == SeverityDeviation {
			n++
			if it.Unverified && len(it.Evidence) > 0 {
				t.Errorf("a row with evidence must not be marked unverified: %+v", it)
			}
		}
	}
	if n != len(rows) || n == 0 {
		t.Fatalf("rows in the review: %d, derived: %d", n, len(rows))
	}
	if built.Verdict.Architecture.ReportedDeviations != 0 {
		t.Fatalf("the verdict counts arrows from the models, not from these rows: %+v", built.Verdict.Architecture)
	}
}
