package domain

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const sadDoc = "docs/SAD.md"
const sadSum = "0e40ce490996bf86782ced541ab97ecb09d605714378daeda8fdb5a711a267f4"

func readSAD(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/sad/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func srcEv(section string) []ReviewEvidence {
	return []ReviewEvidence{{Kind: EvidenceSource, Ref: sadDoc, Doc: sadDoc, Section: section}}
}

// pilotDesign is the design of vnp-qa-platform as an agent would report it from the knowledge graph and the SAD: ids follow the
// graph (SVC-0x, CT-0x), data stores and outside systems exist only in the SAD text.
func pilotDesign(t *testing.T) ContextDesign {
	t.Helper()
	kg := func(id, label, kind, tech string, state KGReviewState, aliases ...string) DesignContainer {
		return DesignContainer{ID: id, KGID: strp(id), Source: SourceKG, Label: label, Kind: kind, Tech: tech, ReviewState: state, Aliases: aliases}
	}
	sad := func(id, label, kind string, aliases ...string) DesignContainer {
		return DesignContainer{ID: id, Source: SourceSADText, Label: label, Kind: kind, Aliases: aliases, Evidence: srcEv("5.1")}
	}
	return ContextDesign{
		Available: true,
		DocSha256: map[string]string{sadDoc: sadSum},
		ElementsRead: []ContextElement{
			{ID: "CT-01", Class: "Constraint", ReviewState: KGNeedsReview}, {ID: "CT-02", Class: "Constraint", ReviewState: KGNeedsReview},
			{ID: "SVC-01", Class: "Component", ReviewState: KGNeedsReview}, {ID: "SVC-02", Class: "Component", ReviewState: KGNeedsReview},
			{ID: "SVC-03", Class: "Component", ReviewState: KGNeedsReview}, {ID: "SVC-04", Class: "Component", ReviewState: KGNeedsReview},
			{ID: "SVC-05", Class: "Component", ReviewState: KGApproved}, {ID: "SVC-06", Class: "Component", ReviewState: KGNeedsReview},
			{ID: "SVC-07", Class: "Component", ReviewState: KGNeedsReview}, {ID: "SVC-08", Class: "Component", ReviewState: KGNeedsReview},
			{ID: "SVC-BUG-09", Class: "Component", ReviewState: KGNeedsReview}, {ID: "SVC-JIR-13", Class: "Component", ReviewState: KGNeedsReview},
		},
		Containers: []DesignContainer{
			kg("CT-01", "SPA Vue 3", "app", "Vue 3", KGNeedsReview, "SPA"),
			kg("CT-02", "ms-gateway", "service", "Go", KGNeedsReview),
			kg("SVC-01", "ms-system-setting", "service", "Go, gRPC", KGNeedsReview),
			kg("SVC-02", "ms-project-management", "service", "Go", KGNeedsReview),
			kg("SVC-03", "ms-case-management", "service", "Go", KGNeedsReview),
			kg("SVC-04", "ms-test-plan", "service", "Go", KGNeedsReview),
			kg("SVC-05", "ms-bug-management", "service", "Go", KGApproved),
			kg("SVC-06", "ms-api-test", "service", "Go", KGNeedsReview),
			kg("SVC-07", "ms-dashboard", "service", "Go", KGNeedsReview),
			kg("SVC-08", "ms-integration", "service", "Go", KGNeedsReview),
			sad("sad:DS-01", "MySQL 8", "datastore"), sad("sad:DS-02", "Redis", "datastore"), sad("sad:DS-03", "Kafka", "queue"),
			sad("sad:DS-04", "MinIO", "datastore"), sad("sad:EXT-01", "Jira, Google", "external", "EXT", "Jira"),
			sad("sad:EXT-02", "CI/CD", "external"), sad("sad:EXT-03", "AI Agent", "external"),
		},
		SourceBlocks: []SourceBlock{
			{Doc: sadDoc, Section: "5.5", Kind: "mermaid", Text: readSAD(t, "sad-5-5-container-flowchart.mmd")},
			{Doc: sadDoc, Section: "8.1", Kind: "mermaid", Text: readSAD(t, "sad-8-1-sequence.mmd")},
			{Doc: sadDoc, Section: "8.4", Kind: "mermaid", Text: readSAD(t, "sad-8-4-bug-jira-flowchart.mmd")},
		},
	}
}

func pilotResult(t *testing.T) ContextResult {
	return ContextResult{Schema: ContextSchema, KG: ContextKG{Available: true, Snapshot: "snap-000004", Read: []string{"FR-12"}}, Design: pilotDesign(t)}
}

func TestParseContextResult(t *testing.T) {
	good := `{"schema":"outcome-review-context/1","kg":{"available":true,"snapshot":"s"},"items":[{"section":"requirements","change":"added","title":"t","state":"deviation"}]}`
	r, err := ParseContextResult([]byte(good))
	if err != nil || len(r.Items) != 1 {
		t.Fatalf("good block: %+v %v", r, err)
	}
	for name, block := range map[string]string{
		"empty":        ``,
		"not json":     `hello`,
		"wrong schema": `{"schema":"other/2"}`,
		"two values":   good + good,
		"too big":      `{"schema":"outcome-review-context/1","notes":"` + strings.Repeat("a", MaxContextResultBytes) + `"}`,
	} {
		if _, err := ParseContextResult([]byte(block)); err == nil {
			t.Errorf("%s: expected a violation", name)
		} else if _, ok := AsContextViolations(err); !ok {
			t.Errorf("%s: error is %T, want ContextViolations", name, err)
		}
	}
}

func TestEvidence_KGIDShapes(t *testing.T) {
	for _, id := range []string{"SVC-BUG-09", "CT-02", "ENT-02:attribute:email", "FR-12", "ADR-09", "NFR-3.1", "BRL-01"} {
		if !ValidKGID(id) {
			t.Errorf("%q must be accepted", id)
		}
	}
	for _, id := range []string{"", "svc-bug", "X-1", "SVC", "SVC-", "SVC 09", "SVC-09;rm", strings.Repeat("A", 90)} {
		if ValidKGID(id) {
			t.Errorf("%q must be refused", id)
		}
	}
}

func TestEvidence_CheckTable(t *testing.T) {
	ctx := EvidenceContext{
		KGRead: map[string]bool{"FR-12": true}, Docs: map[string]string{sadDoc: sadSum},
		ChangedFiles:    map[string]bool{"internal/a.go": true, "migrations/0105_x.up.sql": true},
		BaseHasFile:     func(p string) bool { return p == "cmd/main.go" },
		ContractChanged: true, CheckIDs: map[string]bool{"unit": true}, TaskIDs: map[string]bool{"t-1": true},
	}
	good := []ReviewEvidence{
		{Kind: EvidenceKG, Ref: "FR-12"}, {Kind: EvidenceSource, Doc: sadDoc, Section: "5.5"}, {Kind: EvidenceFile, Ref: "internal/a.go"},
		{Kind: EvidenceFile, Ref: "cmd/main.go"}, {Kind: EvidenceRPC, Ref: "orca.request.v1.RequestService/GetRequest"},
		{Kind: EvidenceChannel, Ref: "request.review.generate"}, {Kind: EvidenceMigration, Ref: "migrations/0105_x.up.sql"},
		{Kind: EvidenceCheck, Ref: "unit"}, {Kind: EvidenceTask, Ref: "t-1"},
	}
	for _, e := range good {
		if err := ctx.CheckEvidence(e); err != nil {
			t.Errorf("%+v: %v", e, err)
		}
	}
	bad := map[string]ReviewEvidence{
		"kg not read":         {Kind: EvidenceKG, Ref: "FR-99"},
		"kg bad shape":        {Kind: EvidenceKG, Ref: "fr12"},
		"source unlisted":     {Kind: EvidenceSource, Doc: "docs/OTHER.md", Section: "1"},
		"source no section":   {Kind: EvidenceSource, Doc: sadDoc},
		"file elsewhere":      {Kind: EvidenceFile, Ref: "internal/zzz.go"},
		"file escapes":        {Kind: EvidenceFile, Ref: "../../etc/passwd"},
		"file absolute":       {Kind: EvidenceFile, Ref: "/etc/passwd"},
		"migration unchanged": {Kind: EvidenceMigration, Ref: "migrations/0999_y.up.sql"},
		"migration not one":   {Kind: EvidenceMigration, Ref: "internal/a.go"},
		"check unknown":       {Kind: EvidenceCheck, Ref: "e2e"},
		"task unknown":        {Kind: EvidenceTask, Ref: "t-9"},
		"unknown kind":        {Kind: "magic", Ref: "x"},
		"empty ref":           {Kind: EvidenceFile},
	}
	for name, e := range bad {
		if err := ctx.CheckEvidence(e); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	noContract := ctx
	noContract.ContractChanged = false
	if noContract.CheckEvidence(ReviewEvidence{Kind: EvidenceRPC, Ref: "a.B"}) == nil || noContract.CheckEvidence(ReviewEvidence{Kind: EvidenceChannel, Ref: "a.b"}) == nil {
		t.Error("rpc and channel evidence needs a changed contract file")
	}
	// A malformed piece is a shape error; a well formed piece that cannot be verified is not.
	if _, ok := ctx.CheckEvidence(ReviewEvidence{Kind: EvidenceKG, Ref: "fr12"}).(EvidenceShapeError); !ok {
		t.Error("a malformed id is an EvidenceShapeError")
	}
	if _, ok := ctx.CheckEvidence(ReviewEvidence{Kind: EvidenceKG, Ref: "FR-99"}).(EvidenceShapeError); ok {
		t.Error("an id that is well formed but unread is not a shape error")
	}
}

func TestNormalizeDesign_PilotSADMergesParserEdges(t *testing.T) {
	d, viol, warn := NormalizeDesign(ContextKG{Available: true, Snapshot: "snap-000004"}, pilotDesign(t), nil)
	if len(viol) != 0 {
		t.Fatalf("violations: %v", viol)
	}
	if !d.Available || d.Snapshot != "snap-000004" || len(d.Containers) != 17 {
		t.Fatalf("design: available=%v snapshot=%s containers=%d", d.Available, d.Snapshot, len(d.Containers))
	}
	has := func(from, to string) bool { return d.HasEdge(from, to) }
	for _, e := range [][2]string{{"CT-01", "CT-02"}, {"CT-02", "SVC-05"}, {"SVC-08", "SVC-05"}, {"SVC-05", "sad:DS-01"}, {"SVC-03", "sad:DS-03"}, {"sad:DS-03", "SVC-07"}, {"SVC-04", "SVC-06"}, {"SVC-08", "sad:EXT-01"}, {"SVC-02", "sad:DS-04"}} {
		if !has(e[0], e[1]) {
			t.Errorf("missing designed edge %s -> %s", e[0], e[1])
		}
	}
	// Section 8.4 describes the inside of ms-integration: its worker calling bug-management is an edge of the container.
	if !has("SVC-08", "sad:EXT-01") || !has("SVC-08", "SVC-01") {
		t.Errorf("the calls of the parts of ms-integration are lifted to the container")
	}
	if has("SVC-08", "SVC-08") {
		t.Error("a container never has an edge to itself")
	}
	// 8.1: U (a person) and R (a part of bug-management) are not containers, so their messages are not flow messages.
	var flow DesignFlow
	for _, f := range d.Flows {
		if f.Name == "Flow 8.1" {
			flow = f
		}
	}
	if len(flow.Messages) == 0 {
		t.Fatalf("flow 8.1 missing: %+v", d.Flows)
	}
	first := flow.Messages[0]
	if first.From != "CT-02" || first.To != "SVC-05" || first.Order != 1 {
		t.Errorf("first container message: %+v", first)
	}
	if !contains(warn, "design_flow_participant_unmapped:8.1") {
		t.Errorf("warnings: %v", warn)
	}
	if got := d.ReviewedRatio(); got < 0.08 || got > 0.09 {
		t.Errorf("reviewed ratio = %v, want 1/12", got)
	}
}

func TestNormalizeDesign_Violations(t *testing.T) {
	kg := ContextKG{Available: true, Snapshot: "s"}
	cases := map[string]func(d *ContextDesign){
		"kg container with an unread id": func(d *ContextDesign) {
			d.Containers = append(d.Containers, DesignContainer{ID: "SVC-99", KGID: strp("SVC-99"), Source: SourceKG, Label: "ms-ghost", Kind: "service"})
		},
		"sad-text container with no source evidence": func(d *ContextDesign) {
			d.Containers = append(d.Containers, DesignContainer{ID: "sad:DS-09", Source: SourceSADText, Label: "Cache", Kind: "datastore"})
		},
		"sad-text evidence in an unlisted document": func(d *ContextDesign) {
			d.Containers = append(d.Containers, DesignContainer{ID: "sad:DS-09", Source: SourceSADText, Label: "Cache", Kind: "datastore",
				Evidence: []ReviewEvidence{{Kind: EvidenceSource, Doc: "docs/OTHER.md", Section: "1"}}})
		},
		"component named like a class": func(d *ContextDesign) {
			d.Containers = append(d.Containers, DesignContainer{ID: "sad:x", Source: SourceSADText, Label: "BugRepository", Kind: "service", Evidence: srcEv("5.2")})
		},
		"component named like a function": func(d *ContextDesign) {
			d.Containers = append(d.Containers, DesignContainer{ID: "sad:x", Source: SourceSADText, Label: "handleBug()", Kind: "service", Evidence: srcEv("5.2")})
		},
		"bad kind": func(d *ContextDesign) { d.Containers[0].Kind = "module" },
		"code source in a design": func(d *ContextDesign) {
			d.Containers = append(d.Containers, DesignContainer{ID: "code:x", Source: SourceCode, Label: "extra", Kind: "service"})
		},
		"edge to an unknown container":    func(d *ContextDesign) { d.Edges = []DesignEdge{{From: "CT-02", To: "SVC-77"}} },
		"edge from a container to itself": func(d *ContextDesign) { d.Edges = []DesignEdge{{From: "CT-02", To: "CT-02"}} },
		"self call in a flow": func(d *ContextDesign) {
			d.Flows = []DesignFlow{{Name: "f", Messages: []DesignMessage{{From: "SVC-05", To: "SVC-05", Order: 1}}}}
		},
		"document sum is not a sha256": func(d *ContextDesign) { d.DocSha256[sadDoc] = "abc" },
		"element id with a bad shape":  func(d *ContextDesign) { d.ElementsRead = append(d.ElementsRead, ContextElement{ID: "bad id"}) },
	}
	for name, mutate := range cases {
		d := pilotDesign(t)
		mutate(&d)
		if _, viol, _ := NormalizeDesign(kg, d, nil); len(viol) == 0 {
			t.Errorf("%s: expected a violation", name)
		}
	}
}

func TestNormalizeDesign_ComponentsFoldIntoTheirContainer(t *testing.T) {
	d := pilotDesign(t)
	for i := range d.Containers {
		if d.Containers[i].ID == "CT-02" {
			continue
		}
	}
	// SVC-BUG-09 is a class Component of the graph but part of ms-bug-management (AM-5): it is reported with a parent.
	d.Containers = append(d.Containers, DesignContainer{ID: "SVC-BUG-09", KGID: strp("SVC-BUG-09"), Source: SourceKG, Label: "bug core", Kind: "service", Parent: "SVC-05"})
	got, viol, _ := NormalizeDesign(ContextKG{Available: true, Snapshot: "s"}, d, nil)
	if len(viol) != 0 {
		t.Fatal(viol)
	}
	if got.Components["SVC-BUG-09"] != "SVC-05" {
		t.Errorf("components: %v", got.Components)
	}
	if _, ok := got.ContainerByID("SVC-BUG-09"); ok {
		t.Error("a component must not be a container of the design")
	}
}

func TestNormalizeDesign_ParserWinsOverAgentEdges(t *testing.T) {
	d := pilotDesign(t)
	// The agent claims 5.5 shows the SPA calling bug-management directly, and misses that it shows the gateway calling it.
	d.Edges = []DesignEdge{
		{From: "CT-01", To: "SVC-05", Evidence: srcEv("5.5")},
		{From: "CT-02", To: "SVC-05", Evidence: srcEv("5.5")},
		{From: "SVC-05", To: "SVC-03", Protocol: "grpc"}, // no block speaks for this one: it is kept
	}
	got, viol, warn := NormalizeDesign(ContextKG{Available: true, Snapshot: "s"}, d, nil)
	if len(viol) != 0 {
		t.Fatal(viol)
	}
	if got.HasEdge("CT-01", "SVC-05") {
		t.Error("an edge the block does not draw must be dropped")
	}
	if !got.HasEdge("CT-02", "SVC-05") || !got.HasEdge("SVC-05", "SVC-03") {
		t.Error("parsed edges and edges no block covers must stay")
	}
	if !contains(warn, "design_edge_mismatch:CT-01->SVC-05") {
		t.Errorf("an agent edge dropped by the parser is reported: %v", warn)
	}
	var plus bool
	for _, w := range warn {
		plus = plus || strings.HasPrefix(w, "design_edge_mismatch:+")
	}
	if !plus {
		t.Errorf("an edge the parser added that the agent missed is reported: %v", warn)
	}
}

func TestNormalizeDesign_KGUnavailableHasNoDesign(t *testing.T) {
	d, viol, warn := NormalizeDesign(ContextKG{Available: false}, pilotDesign(t), nil)
	if len(viol) != 0 || d.Available || len(d.Containers) != 0 || len(d.Edges) != 0 || !contains(warn, "design_ignored:kg_unavailable") {
		t.Errorf("design=%+v viol=%v warn=%v", d, viol, warn)
	}
	if a, f := d.Checkable(); a || f {
		t.Error("an absent design checks nothing")
	}
}

func TestNormalizeDesign_Deterministic(t *testing.T) {
	a, _, _ := NormalizeDesign(ContextKG{Available: true, Snapshot: "s"}, pilotDesign(t), nil)
	b, _, _ := NormalizeDesign(ContextKG{Available: true, Snapshot: "s"}, pilotDesign(t), nil)
	da, _ := DesignInputDigest(a)
	db, _ := DesignInputDigest(b)
	if da == "" || da != db {
		t.Errorf("digests differ: %s %s", da, db)
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Error("the same input must give the same bytes")
	}
}

func TestDesignDigestAndContextDigest(t *testing.T) {
	a, _ := DesignDigest("snap-1", map[string]string{"a": "1", "b": "2"}, "p1")
	b, _ := DesignDigest("snap-1", map[string]string{"b": "2", "a": "1"}, "p1")
	c, _ := DesignDigest("snap-2", map[string]string{"a": "1", "b": "2"}, "p1")
	d, _ := DesignDigest("snap-1", map[string]string{"a": "1", "b": "9"}, "p1")
	e, _ := DesignDigest("snap-1", map[string]string{"a": "1", "b": "2"}, "p2")
	if a != b || a == c || a == d || a == e {
		t.Errorf("design lock must follow snapshot, documents and prompt, not map order: %s %s %s %s %s", a, b, c, d, e)
	}
	x, _ := ContextDigest(ContextModeDesign, "in", "dd", "p")
	y, _ := ContextDigest(ContextModeDelivered, "in", "dd", "p")
	if x == y {
		t.Error("mode is part of the context digest")
	}
}
