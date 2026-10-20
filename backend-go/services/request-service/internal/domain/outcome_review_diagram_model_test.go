package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func lockedPilotDesign(t *testing.T) DesignModel {
	t.Helper()
	d, viol, _ := NormalizeDesign(ContextKG{Available: true, Snapshot: "snap-000004"}, pilotDesign(t), nil)
	if len(viol) != 0 {
		t.Fatal(viol)
	}
	return d
}

func deliveredInput(t *testing.T) ContextValidationInput {
	d := lockedPilotDesign(t)
	return ContextValidationInput{
		Mode: ContextModeDelivered, LockedDesign: &d,
		Evidence: EvidenceContext{ChangedFiles: map[string]bool{"internal/x.go": true}, BaseHasFile: func(p string) bool { return p == "cmd/main.go" }},
	}
}

func fileEv(p string) []ReviewEvidence { return []ReviewEvidence{{Kind: EvidenceFile, Ref: p}} }

func kgNode(id, label string, inBase, inHead bool) DiagramNode {
	return DiagramNode{ID: id, KGID: id, Source: SourceKG, Label: label, Kind: "service", InBase: inBase, InHead: inHead}
}

func archDelta(nodes []DiagramNode, edges []DiagramEdge) DiagramModel {
	return DiagramModel{Section: SectionArchitecture, Type: DiagramArchitecture, Variant: DiagramVariantDelta, Level: LevelContainer, Nodes: nodes, Edges: edges}
}

func delivered(t *testing.T, in ContextValidationInput, diagrams ...DiagramModel) (ValidatedContext, error) {
	t.Helper()
	return ValidateContext(ContextResult{Schema: ContextSchema, KG: ContextKG{Available: true}, Diagrams: diagrams}, in)
}

func findDiagram(t *testing.T, v ValidatedContext, section ReviewSection, variant string) DiagramModel {
	t.Helper()
	for _, m := range v.Diagrams {
		if m.Section == section && m.Variant == variant {
			return m
		}
	}
	t.Fatalf("no %s/%s diagram in %d diagrams", section, variant, len(v.Diagrams))
	return DiagramModel{}
}

func edgeOf(m DiagramModel, from, to string) (DiagramEdge, bool) {
	for _, e := range m.Edges {
		if e.From == from && e.To == to {
			return e, true
		}
	}
	return DiagramEdge{}, false
}

func TestValidateDiagram_StatesAreDerivedByOrcaNotReported(t *testing.T) {
	in := deliveredInput(t)
	nodes := []DiagramNode{kgNode("CT-02", "ms-gateway", true, true), kgNode("SVC-05", "ms-bug-management", true, true), kgNode("SVC-03", "ms-case-management", true, true), kgNode("SVC-06", "ms-api-test", true, true),
		{ID: "sad:DS-03", Source: SourceSADText, Label: "Kafka", Kind: "queue", InBase: true, InHead: true, Evidence: srcEv("5.1")}}
	edges := []DiagramEdge{
		{From: "CT-02", To: "SVC-05", InBase: true, InHead: true, State: StateAdded, Change: StateAdded},  // the agent lies; it is unchanged
		{From: "SVC-05", To: "SVC-03", InHead: true, Protocol: "GRPC", Evidence: fileEv("internal/x.go")}, // new and not designed
		{From: "SVC-03", To: "sad:DS-03", InBase: true, InHead: true},                                     // designed and unchanged
		{From: "CT-02", To: "SVC-03", InBase: true, Evidence: fileEv("cmd/main.go")},                      // designed, removed by the code
		{From: "SVC-05", To: "SVC-06", InHead: true},                                                      // new, not designed, no evidence
	}
	v, err := delivered(t, in, archDelta(nodes, edges))
	if err != nil {
		t.Fatal(err)
	}
	m := findDiagram(t, v, SectionArchitecture, DiagramVariantDelta)

	same, _ := edgeOf(m, "CT-02", "SVC-05")
	if same.State != StateSame || same.Change != StateSame {
		t.Errorf("an agent cannot set the state: %+v", same)
	}
	dev, _ := edgeOf(m, "SVC-05", "SVC-03")
	if dev.State != StateDeviation || dev.Change != StateAdded || dev.Protocol != "grpc" || dev.Unverified {
		t.Errorf("new edge the design does not have: %+v", dev)
	}
	if !dev.Advisory {
		t.Errorf("a deviation resting on an element nobody approved is advisory: %+v", dev)
	}
	missing, _ := edgeOf(m, "CT-02", "SVC-03")
	if missing.State != StateMissing || missing.Change != StateRemoved {
		t.Errorf("designed edge the code no longer has: %+v", missing)
	}
	if bare, _ := edgeOf(m, "SVC-05", "SVC-06"); !bare.Unverified || bare.State != StateDeviation {
		t.Errorf("a finding with no evidence is unverified: %+v", bare)
	}
	if ok, _ := edgeOf(m, "SVC-03", "sad:DS-03"); ok.State != StateSame || ok.Unverified {
		t.Errorf("unchanged edge: %+v", ok)
	}
	if !v.Arch.DesignChecked || v.Arch.NewEdges != 2 || v.Arch.RemovedEdges != 1 || v.Arch.UnplannedEdges != 2 || v.Arch.ReportedDeviations != 3 {
		t.Errorf("architecture verdict: %+v", v.Arch)
	}
	if v.Arch.WithEvidence != 2 { // the deviation with a file and the missing edge with a base file; the bare one does not count
		t.Errorf("only findings with verified evidence count: %+v", v.Arch)
	}
	if v.Arch.Advisory != 3 {
		t.Errorf("advisory = %d", v.Arch.Advisory)
	}
}

func TestValidateDiagram_ApprovedBasisAsksForADecision(t *testing.T) {
	// SVC-05 is approved in the pilot design; a node that is not in a design (new container) has no basis, so it is advisory.
	d := lockedPilotDesign(t)
	for i := range d.Elements {
		if d.Elements[i].ID == "SVC-03" {
			d.Elements[i].ReviewState = KGApproved
		}
	}
	in := deliveredInput(t)
	in.LockedDesign = &d
	nodes := []DiagramNode{kgNode("SVC-05", "ms-bug-management", true, true), kgNode("SVC-03", "ms-case-management", true, true)}
	v, err := delivered(t, in, archDelta(nodes, []DiagramEdge{{From: "SVC-05", To: "SVC-03", InHead: true, Evidence: fileEv("internal/x.go")}}))
	if err != nil {
		t.Fatal(err)
	}
	e, _ := edgeOf(findDiagram(t, v, SectionArchitecture, DiagramVariantDelta), "SVC-05", "SVC-03")
	if e.State != StateDeviation || e.Advisory {
		t.Errorf("a deviation between approved elements is not advisory: %+v", e)
	}
}

func TestValidateDiagram_Rejections(t *testing.T) {
	base := func() []DiagramNode {
		return []DiagramNode{kgNode("CT-02", "ms-gateway", true, true), kgNode("SVC-05", "ms-bug-management", true, true)}
	}
	cases := map[string]func() DiagramModel{
		"a component by its name": func() DiagramModel {
			return archDelta(append(base(), DiagramNode{Source: SourceCode, Label: "BugRepository", Kind: "service", InHead: true}), nil)
		},
		"a file as a node": func() DiagramModel {
			return archDelta(append(base(), DiagramNode{Source: SourceCode, Label: "internal/bug/store.go", Kind: "service", InHead: true}), nil)
		},
		"a kg id that was not read": func() DiagramModel {
			return archDelta(append(base(), DiagramNode{KGID: "SVC-77", Source: SourceKG, Label: "ghost", Kind: "service", InHead: true}), nil)
		},
		"sad-text with no source evidence": func() DiagramModel {
			return archDelta(append(base(), DiagramNode{ID: "sad:DS-77", Source: SourceSADText, Label: "Cache", Kind: "datastore", InHead: true}), nil)
		},
		"a bad kind": func() DiagramModel {
			n := base()
			n[0].Kind = "class"
			return archDelta(n, nil)
		},
		"no source": func() DiagramModel {
			return archDelta(append(base(), DiagramNode{Label: "Thing", Kind: "service", InHead: true}), nil)
		},
		"a self call": func() DiagramModel {
			return archDelta(base(), []DiagramEdge{{From: "SVC-05", To: "SVC-05", InHead: true}})
		},
		"an edge to a node not drawn": func() DiagramModel {
			return archDelta(base(), []DiagramEdge{{From: "SVC-05", To: "SVC-02", InHead: true}})
		},
		"component level": func() DiagramModel {
			m := archDelta(base(), nil)
			m.Level = "component"
			return m
		},
		"the wrong type for the section": func() DiagramModel {
			m := archDelta(base(), nil)
			m.Type = DiagramSequence
			return m
		},
		"malformed evidence": func() DiagramModel {
			return archDelta(base(), []DiagramEdge{{From: "SVC-05", To: "CT-02", InHead: true, Evidence: []ReviewEvidence{{Kind: EvidenceKG, Ref: "not an id"}}}})
		},
		"an unknown parent": func() DiagramModel {
			return archDelta(append(base(), DiagramNode{Source: SourceKG, KGID: "SVC-BUG-09", Label: "bug core", Kind: "service", Parent: "SVC-404"}), nil)
		},
	}
	for name, mk := range cases {
		v, err := delivered(t, deliveredInput(t), mk())
		if err == nil {
			t.Errorf("%s: accepted: %d diagrams", name, len(v.Diagrams))
			continue
		}
		if _, ok := AsContextViolations(err); !ok {
			t.Errorf("%s: %T is not a violation list", name, err)
		}
	}
}

func TestValidateDiagram_ComponentsFoldIntoTheirContainer(t *testing.T) {
	nodes := []DiagramNode{
		kgNode("SVC-05", "ms-bug-management", true, true), {ID: "sad:DS-01", Source: SourceSADText, Label: "MySQL 8", Kind: "datastore", InBase: true, InHead: true, Evidence: srcEv("5.1")},
		{ID: "SVC-BUG-09", KGID: "SVC-BUG-09", Source: SourceKG, Label: "bug core", Kind: "service", Parent: "SVC-05", InHead: true},
		{ID: "bug-store", Source: SourceCode, Label: "bug store", Kind: "service", Parent: "ms-bug-management", InHead: true},
		{ID: "old-part", Source: SourceCode, Label: "old part", Kind: "service", Parent: "SVC-05", InBase: true, InHead: true},
	}
	edges := []DiagramEdge{
		{From: "SVC-BUG-09", To: "sad:DS-01", InHead: true, InBase: true},
		{From: "SVC-BUG-09", To: "bug-store", InHead: true}, // between two parts of one container: not drawn
	}
	v, err := delivered(t, deliveredInput(t), archDelta(nodes, edges))
	if err != nil {
		t.Fatal(err)
	}
	m := findDiagram(t, v, SectionArchitecture, DiagramVariantDelta)
	if len(m.Nodes) != 2 {
		t.Fatalf("components must not be nodes: %+v", m.Nodes)
	}
	var bug DiagramNode
	for _, n := range m.Nodes {
		if n.ID == "SVC-05" {
			bug = n
		}
	}
	if bug.ComponentsChanged != 2 || bug.Drilldown == nil || bug.Drilldown.ContainerID != "SVC-05" || bug.Drilldown.Lens != "architecture" {
		t.Errorf("two parts changed (added), the unchanged one is not counted: %+v", bug)
	}
	if e, ok := edgeOf(m, "SVC-05", "sad:DS-01"); !ok || e.State != StateSame {
		t.Errorf("a call of a part is a call of its container: %+v", m.Edges)
	}
	if len(m.Edges) != 1 {
		t.Errorf("edges: %+v", m.Edges)
	}
	if v.Arch.ContainersChanged != 1 {
		t.Errorf("a container with changed parts counts as changed: %+v", v.Arch)
	}
}

func TestValidateDiagram_TooManyNodesAreGathered(t *testing.T) {
	var nodes []DiagramNode
	var edges []DiagramEdge
	for i := 0; i < 15; i++ {
		nodes = append(nodes, DiagramNode{Source: SourceCode, Label: fmt.Sprintf("svc %02d", i), Kind: "service", InBase: true, InHead: i%5 != 0})
		if i > 0 {
			edges = append(edges, DiagramEdge{From: fmt.Sprintf("svc %02d", i-1), To: fmt.Sprintf("svc %02d", i), InBase: true, InHead: true})
		}
	}
	for i := range nodes {
		nodes[i].ID = nodes[i].Label
	}
	in := deliveredInput(t)
	in.LockedDesign = &DesignModel{} // no design: nodes keep their own states
	v, err := delivered(t, in, archDelta(nodes, edges))
	if err != nil {
		t.Fatal(err)
	}
	m := findDiagram(t, v, SectionArchitecture, DiagramVariantDelta)
	if len(m.Nodes) != MaxDiagramNodes || !m.Truncated {
		t.Fatalf("nodes=%d truncated=%v", len(m.Nodes), m.Truncated)
	}
	var more *DiagramNode
	for i := range m.Nodes {
		if m.Nodes[i].ID == "more" {
			more = &m.Nodes[i]
		}
	}
	if more == nil || more.Label != "+4 more" {
		t.Fatalf("the rest is one node: %+v", more)
	}
	for _, e := range m.Edges {
		if e.From == e.To {
			t.Errorf("an edge inside the gathered nodes is dropped: %+v", e)
		}
	}
	// The changed nodes are kept ahead of the unchanged ones.
	removed := 0
	for _, n := range m.Nodes {
		if n.State == StateRemoved {
			removed++
		}
	}
	if removed != 3 {
		t.Errorf("all three removed nodes must stay visible, got %d", removed)
	}
}

func TestValidateDiagram_UnmappedOnlyForCodeAndOnlyWithADesign(t *testing.T) {
	nodes := []DiagramNode{
		kgNode("SVC-05", "ms-bug-management", true, true),
		{Source: SourceCode, Label: "Mystery Worker", Kind: "service", InHead: true},
		{Source: SourceCode, Label: "ms-bug-management", Kind: "service", InHead: true, InBase: true}, // the design knows this one
	}
	v, err := delivered(t, deliveredInput(t), archDelta(nodes, nil))
	if err != nil {
		t.Fatal(err)
	}
	m := findDiagram(t, v, SectionArchitecture, DiagramVariantDelta)
	states := map[string]EdgeState{}
	for _, n := range m.Nodes {
		states[n.ID] = n.State
	}
	if states["code:mystery-worker"] != StateUnmapped {
		t.Errorf("a node in neither the graph nor the SAD is unmapped: %v", states)
	}
	if states["SVC-05"] != StateSame || len(m.Nodes) != 2 {
		t.Errorf("a code node the design knows is that container, not unmapped: %v", states)
	}
	for _, n := range m.Nodes {
		if n.ID == "SVC-05" && n.KGID != "SVC-05" {
			t.Error("the container keeps the graph id")
		}
		if n.Source == SourceCode && n.KGID != "" {
			t.Error("a code node never gets an invented graph id")
		}
	}

	// Without a design nothing is "not in the design".
	in := deliveredInput(t)
	in.LockedDesign = &DesignModel{}
	v2, err := delivered(t, in, archDelta([]DiagramNode{{Source: SourceCode, Label: "Mystery Worker", Kind: "service", InHead: true}}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if n := findDiagram(t, v2, SectionArchitecture, DiagramVariantDelta).Nodes[0]; n.State == StateUnmapped || n.State != StateAdded {
		t.Errorf("no design, no unmapped: %+v", n)
	}
}

func TestValidateContext_KGUnavailableHasNoDesignChecks(t *testing.T) {
	raw := ContextResult{Schema: ContextSchema, KG: ContextKG{Available: false}, Design: pilotDesign(t), Diagrams: []DiagramModel{
		archDelta([]DiagramNode{{Source: SourceCode, Label: "Billing", Kind: "service", InHead: true}, {Source: SourceCode, Label: "Ledger", Kind: "service", InHead: true}},
			[]DiagramEdge{{From: "Billing", To: "Ledger", InHead: true, InBase: false, Evidence: fileEv("internal/x.go")}}),
	}}
	for i := range raw.Diagrams[0].Nodes {
		raw.Diagrams[0].Nodes[i].ID = raw.Diagrams[0].Nodes[i].Label
	}
	v, err := ValidateContext(raw, ContextValidationInput{Mode: ContextModeDelivered, Evidence: EvidenceContext{ChangedFiles: map[string]bool{"internal/x.go": true}}})
	if err != nil {
		t.Fatal(err)
	}
	if v.KG.Available || v.Design.Available || v.Arch.DesignChecked || v.Flow.DesignChecked {
		t.Errorf("kg=%+v design=%v arch=%+v", v.KG, v.Design.Available, v.Arch)
	}
	for _, m := range v.Diagrams {
		if m.Variant == DiagramVariantDesign {
			t.Errorf("no design diagram without a design: %+v", m)
		}
		for _, e := range m.Edges {
			if e.State == StateDeviation || e.State == StateMissing || e.InDesign != nil {
				t.Errorf("no design, no deviation or missing: %+v", e)
			}
		}
		for _, n := range m.Nodes {
			if n.State == StateUnmapped || n.KGID != "" {
				t.Errorf("no unmapped without a design and no graph ids: %+v", n)
			}
		}
	}
	if v.Arch.UnplannedEdges != 0 || v.Arch.ReportedDeviations != 0 || !contains(v.Warnings, "kg_unavailable") || !contains(v.Warnings, "design_ignored:kg_unavailable") {
		t.Errorf("verdict=%+v warnings=%v", v.Arch, v.Warnings)
	}
}

func TestValidateContext_DesignModeBuildsDesignDiagramsAndIgnoresDelta(t *testing.T) {
	raw := pilotResult(t)
	raw.Diagrams = []DiagramModel{archDelta([]DiagramNode{{Source: SourceCode, Label: "x", Kind: "service"}}, nil)}
	v, err := ValidateContext(raw, ContextValidationInput{Mode: ContextModeDesign})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(v.Warnings, "diagram_ignored:architecture:delta_in_design_mode") {
		t.Errorf("a delta diagram has no code to come from in design mode: %v", v.Warnings)
	}
	arch := findDiagram(t, v, SectionArchitecture, DiagramVariantDesign)
	if len(arch.Nodes) > MaxDiagramNodes || len(arch.Edges) == 0 || arch.InputDigest == "" || arch.Level != LevelContainer {
		t.Errorf("design architecture: nodes=%d edges=%d", len(arch.Nodes), len(arch.Edges))
	}
	seq := findDiagram(t, v, SectionFlow, DiagramVariantDesign)
	if seq.Name != "Flow 8.1" || len(seq.Edges) == 0 || len(seq.Nodes) > MaxDiagramLanes {
		t.Errorf("design flow: %+v", seq)
	}
	for _, e := range arch.Edges {
		if e.State != StateSame || e.InDesign == nil || !*e.InDesign {
			t.Errorf("everything in the design view is the design: %+v", e)
		}
	}
	if v.Arch.ReviewedRatio <= 0 || !v.Arch.DesignChecked {
		t.Errorf("verdict: %+v", v.Arch)
	}
}

func TestValidateContext_DesignDiagramDigestDoesNotDependOnCommit(t *testing.T) {
	d := lockedPilotDesign(t)
	a, _ := BuildDesignDiagrams(d)
	// The delivered phase builds them again from the same locked design after any amount of code changed.
	in := deliveredInput(t)
	v, err := delivered(t, in, archDelta([]DiagramNode{kgNode("SVC-05", "ms-bug-management", true, true)}, nil))
	if err != nil {
		t.Fatal(err)
	}
	var b []DiagramModel
	for _, m := range v.Diagrams {
		if m.Variant == DiagramVariantDesign {
			b = append(b, m)
		}
	}
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("design diagrams: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].InputDigest != b[i].InputDigest || a[i].InputDigest == "" {
			t.Errorf("design diagram %d changed between phases: %s %s", i, a[i].InputDigest, b[i].InputDigest)
		}
	}
}

func TestValidateDiagram_SequenceReorderAndOrder(t *testing.T) {
	lanes := []DiagramNode{kgNode("CT-02", "ms-gateway", true, true), kgNode("SVC-05", "ms-bug-management", true, true), kgNode("SVC-01", "ms-system-setting", true, true)}
	msgs := []DiagramEdge{
		{From: "CT-02", To: "SVC-05", Order: 5, BaseOrder: 1, InBase: true, InHead: true, Label: "update"},
		{From: "SVC-05", To: "SVC-01", Order: 9, BaseOrder: 9, InBase: true, InHead: true, Label: "notify"},
	}
	m := DiagramModel{Section: SectionFlow, Type: DiagramSequence, Variant: DiagramVariantDelta, Level: LevelContainer, Nodes: lanes, Edges: msgs}
	v, err := delivered(t, deliveredInput(t), m)
	if err != nil {
		t.Fatal(err)
	}
	got := findDiagram(t, v, SectionFlow, DiagramVariantDelta)
	if got.Edges[0].Order != 1 || got.Edges[1].Order != 2 || !got.Edges[0].Reordered || got.Edges[1].Reordered {
		t.Errorf("messages are numbered 1..n and reorder is derived from the base order: %+v", got.Edges)
	}
	if v.Flow.Reordered != 1 {
		t.Errorf("flow verdict: %+v", v.Flow)
	}

	// A flow with more than six lanes keeps the lanes with changes and gathers the rest.
	var many []DiagramNode
	for i := 0; i < 9; i++ {
		many = append(many, DiagramNode{Source: SourceCode, Label: fmt.Sprintf("lane %d", i), Kind: "service", InBase: true, InHead: true})
		many[i].ID = many[i].Label
	}
	big := DiagramModel{Section: SectionFlow, Type: DiagramSequence, Variant: DiagramVariantDelta, Level: LevelContainer, Nodes: many,
		Edges: []DiagramEdge{{From: "lane 0", To: "lane 1", Order: 1, InBase: true, InHead: true}}}
	in := deliveredInput(t)
	in.LockedDesign = &DesignModel{}
	v2, err := delivered(t, in, big)
	if err != nil {
		t.Fatal(err)
	}
	if g := findDiagram(t, v2, SectionFlow, DiagramVariantDelta); len(g.Nodes) != MaxDiagramLanes || !g.Truncated {
		t.Errorf("lanes: %d truncated=%v", len(g.Nodes), g.Truncated)
	}
}

func TestValidateDiagram_StateMachineAndSchema(t *testing.T) {
	sm := DiagramModel{Section: SectionBehavior, Type: DiagramStateMachine, Variant: DiagramVariantDelta,
		Nodes: []DiagramNode{{Label: "Open", InBase: true, InHead: true}, {Label: "Closed", InBase: true, InHead: true}, {Label: "Reopened", InHead: true}},
		Edges: []DiagramEdge{{From: "Open", To: "Closed", InBase: true, InHead: true}, {From: "Closed", To: "Reopened", InHead: true, Label: "reopen"}}}
	db := DiagramModel{Section: SectionData, Type: DiagramDatabase, Variant: DiagramVariantDelta,
		Nodes: []DiagramNode{{Label: "bugs", InBase: true, InHead: true, Columns: []DiagramColumn{{Name: "id", Type: "uuid", InBase: true, InHead: true}, {Name: "closed_at", Type: "timestamp", InHead: true}}}}}
	v, err := delivered(t, deliveredInput(t), sm, db)
	if err != nil {
		t.Fatal(err)
	}
	b := findDiagram(t, v, SectionBehavior, DiagramVariantDelta)
	states := map[string]EdgeState{}
	for _, n := range b.Nodes {
		states[n.ID] = n.State
	}
	if states["reopened"] != StateAdded || states["open"] != StateSame {
		t.Errorf("states: %v", states)
	}
	if e, _ := edgeOf(b, "closed", "reopened"); e.State != StateAdded {
		t.Errorf("transition: %+v", e)
	}
	d := findDiagram(t, v, SectionData, DiagramVariantDelta)
	if d.Nodes[0].Columns[1].State != StateAdded || d.Nodes[0].Columns[0].State != StateSame {
		t.Errorf("columns: %+v", d.Nodes[0].Columns)
	}
	// A schema or state machine is not compared with the architecture design.
	if _, err := delivered(t, deliveredInput(t), DiagramModel{Section: SectionData, Type: DiagramStateMachine, Variant: DiagramVariantDelta}); err == nil {
		t.Error("the type must match the section")
	}
}

func TestValidateDiagram_SameInputSameDigestAndNoDuplicates(t *testing.T) {
	mk := func() DiagramModel {
		return archDelta([]DiagramNode{kgNode("CT-02", "ms-gateway", true, true), kgNode("SVC-05", "ms-bug-management", true, true)},
			[]DiagramEdge{{From: "CT-02", To: "SVC-05", InBase: true, InHead: true}})
	}
	a, err := delivered(t, deliveredInput(t), mk())
	if err != nil {
		t.Fatal(err)
	}
	// The same nodes listed in another order give the same drawing.
	m := mk()
	m.Nodes[0], m.Nodes[1] = m.Nodes[1], m.Nodes[0]
	b, err := delivered(t, deliveredInput(t), m)
	if err != nil {
		t.Fatal(err)
	}
	da := findDiagram(t, a, SectionArchitecture, DiagramVariantDelta).InputDigest
	db := findDiagram(t, b, SectionArchitecture, DiagramVariantDelta).InputDigest
	if da == "" || da != db {
		t.Errorf("digests: %s %s", da, db)
	}
	if _, err := delivered(t, deliveredInput(t), mk(), mk()); err == nil || !strings.Contains(err.Error(), "second diagram") {
		t.Errorf("two diagrams for the same section and variant: %v", err)
	}
}

func TestValidateContext_AgentItems(t *testing.T) {
	d := lockedPilotDesign(t)
	in := ContextValidationInput{Mode: ContextModeDelivered, LockedDesign: &d, Evidence: EvidenceContext{ChangedFiles: map[string]bool{"internal/x.go": true}}}
	raw := ContextResult{Schema: ContextSchema, KG: ContextKG{Available: true}, Items: []AgentReviewItem{
		{Subject: "a", Section: SectionContracts, Change: ChangeModified, Title: "RPC added", Severity: SeverityBreaking, NeedsDecision: true, Evidence: fileEv("internal/x.go")},
		{Subject: "b", Section: SectionArchitecture, Change: ChangeAdded, Title: "No proof", Severity: SeverityDeviation, NeedsDecision: true, ReviewState: KGApproved},
		{Subject: "c", Section: SectionArchitecture, Change: ChangeAdded, Title: "Not approved", Severity: SeverityDeviation, NeedsDecision: true, ReviewState: KGNeedsReview, Evidence: fileEv("internal/x.go")},
		{Subject: "d", Section: SectionFlow, Change: ChangeAdded, Title: "Cites a file nobody changed", Evidence: fileEv("internal/ghost.go")},
	}}
	v, err := ValidateContext(raw, in)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ReviewItem{}
	for _, it := range v.Items {
		by[it.Title] = it
	}
	if it := by["RPC added"]; it.Unverified || !it.NeedsDecision || it.Origin != OriginAgent {
		t.Errorf("verified breaking item: %+v", it)
	}
	if it := by["No proof"]; !it.Unverified || it.NeedsDecision {
		t.Errorf("an item with no evidence is unverified and cannot block approval: %+v", it)
	}
	if it := by["Not approved"]; !it.Advisory || it.NeedsDecision {
		t.Errorf("a deviation on an element nobody approved is advisory and asks for no decision: %+v", it)
	}
	if it := by["Cites a file nobody changed"]; !it.Unverified || len(it.Evidence) != 0 {
		t.Errorf("evidence that cannot be verified is dropped and the item marked: %+v", it)
	}
	if again, _ := ValidateContext(raw, in); again.Items[0].Key != v.Items[0].Key || v.Items[0].Key == "" {
		t.Error("an item key is stable between runs")
	}
	for name, bad := range map[string]AgentReviewItem{
		"unknown section": {Section: "ui", Change: ChangeAdded, Title: "t"},
		"unknown change":  {Section: SectionFlow, Change: "renamed", Title: "t"},
		"empty title":     {Section: SectionFlow, Change: ChangeAdded, Title: "  "},
		"malformed proof": {Section: SectionFlow, Change: ChangeAdded, Title: "t", Evidence: []ReviewEvidence{{Kind: EvidenceKG, Ref: "bad"}}},
	} {
		if _, err := ValidateContext(ContextResult{Schema: ContextSchema, KG: ContextKG{Available: true}, Items: []AgentReviewItem{bad}}, in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestValidateContext_SecretsAreRedactedAndLimitsHold(t *testing.T) {
	redact := func(s string) string { return strings.ReplaceAll(s, "ghp_secrettoken", "[REDACTED]") }
	d := lockedPilotDesign(t)
	in := ContextValidationInput{Mode: ContextModeDelivered, LockedDesign: &d, Redact: redact}
	raw := ContextResult{Schema: ContextSchema, KG: ContextKG{Available: true}, Notes: "leak ghp_secrettoken " + strings.Repeat("x", 2000),
		Items: []AgentReviewItem{{Section: SectionConfig, Change: ChangeAdded, Title: "uses ghp_secrettoken", Detail: "ghp_secrettoken"}}}
	v, err := ValidateContext(raw, in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(v.Items[0].Title+v.Items[0].Detail+v.Notes, "ghp_secrettoken") || len([]rune(v.Notes)) > MaxContextNotesRunes {
		t.Errorf("redaction and clipping: %q %q", v.Items[0].Title, v.Notes)
	}
	raw.Items = make([]AgentReviewItem, MaxContextItems+1)
	if _, err := ValidateContext(raw, in); err == nil {
		t.Error("too many items must be refused")
	}
	if _, err := ValidateContext(ContextResult{Schema: ContextSchema, Warnings: []string{"budget_exhausted", "<script>"}}, ContextValidationInput{Mode: ContextModeDesign}); err != nil {
		t.Fatal(err)
	}
	w, _ := ValidateContext(ContextResult{Schema: ContextSchema, KG: ContextKG{Available: true}, Warnings: []string{"budget_exhausted", "<script>"}}, ContextValidationInput{Mode: ContextModeDesign})
	if !contains(w.Warnings, "budget_exhausted") || contains(w.Warnings, "<script>") {
		t.Errorf("only known agent warnings pass: %v", w.Warnings)
	}
}

// The verdict of the architecture and the flow is a count the screen shows, so its JSON is pinned.
func TestValidateDiagram_VerdictGolden(t *testing.T) {
	in := deliveredInput(t)
	nodes := []DiagramNode{kgNode("CT-02", "ms-gateway", true, true), kgNode("SVC-05", "ms-bug-management", true, true), kgNode("SVC-03", "ms-case-management", true, true), kgNode("SVC-06", "ms-api-test", true, true)}
	edges := []DiagramEdge{
		{From: "CT-02", To: "SVC-05", InBase: true, InHead: true},
		{From: "SVC-05", To: "SVC-03", InHead: true, Evidence: fileEv("internal/x.go")},
		{From: "CT-02", To: "SVC-03", InBase: true, Evidence: fileEv("cmd/main.go")},
		{From: "SVC-05", To: "SVC-06", InHead: true},
	}
	lanes := []DiagramNode{kgNode("CT-02", "ms-gateway", true, true), kgNode("SVC-05", "ms-bug-management", true, true)}
	msgs := []DiagramEdge{{From: "CT-02", To: "SVC-05", Order: 2, BaseOrder: 1, InBase: true, InHead: true}, {From: "SVC-05", To: "CT-02", Order: 3, InHead: true, Evidence: fileEv("internal/x.go")}}
	flow := DiagramModel{Section: SectionFlow, Type: DiagramSequence, Variant: DiagramVariantDelta, Level: LevelContainer, Nodes: lanes, Edges: msgs}
	v, err := delivered(t, in, archDelta(nodes, edges), flow)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.MarshalIndent(map[string]DesignVerdict{"architecture": v.Arch, "flow": v.Flow}, "", "  ")
	want, err := os.ReadFile("testdata/review/verdict.golden.json")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		_ = os.WriteFile("testdata/review/verdict.golden.json", append(got, '\n'), 0o644)
		want = append(got, '\n')
	} else if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(want)) != strings.TrimSpace(string(got)) {
		t.Errorf("verdict changed:\n%s\nwant:\n%s", got, want)
	}
}
