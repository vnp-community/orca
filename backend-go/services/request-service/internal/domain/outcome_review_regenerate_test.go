package domain

import (
	"strings"
	"testing"
)

func TestRegenRequest_Validate(t *testing.T) {
	ok := []RegenRequest{
		{Scope: RegenArtifact, Sections: []ReviewSection{SectionFlow}, Reason: RegenLooksWrong},
		{Scope: RegenContext, Reason: RegenKGChanged, Note: "the SAD changed"},
		{Scope: RegenAll, Reason: RegenOther, Sections: []ReviewSection{SectionArchitecture, SectionData}},
	}
	for _, r := range ok {
		if err := r.Validate(); err != nil {
			t.Errorf("%+v: %v", r, err)
		}
	}
	bad := map[string]RegenRequest{
		"no scope":              {Reason: RegenOther},
		"another scope":         {Scope: "everything", Reason: RegenOther},
		"no reason":             {Scope: RegenContext},
		"another reason":        {Scope: RegenContext, Reason: "because"},
		"a long note":           {Scope: RegenContext, Reason: RegenOther, Note: strings.Repeat("n", 201)},
		"artifact with no part": {Scope: RegenArtifact, Reason: RegenLooksWrong},
		"a section not drawn":   {Scope: RegenArtifact, Reason: RegenLooksWrong, Sections: []ReviewSection{SectionConfig}},
	}
	for name, r := range bad {
		if err := r.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if !RegenAll.RunsContext() || !RegenAll.Draws() || RegenContext.Draws() || RegenArtifact.RunsContext() {
		t.Error("what each scope does")
	}
}

func TestRegenBudget(t *testing.T) {
	if err := (RegenBudget{Limit: 5, UsedToday: 4, ActiveInTenant: 1, MaxConcurrent: 2}).Check(); err != nil {
		t.Errorf("inside the limits: %v", err)
	}
	if err := (RegenBudget{Limit: 5, UsedToday: 5, MaxConcurrent: 2}).Check(); err == nil || !strings.Contains(err.Error(), "limit is 5") {
		t.Errorf("the day's limit: %v", err)
	}
	if err := (RegenBudget{Limit: 5, ActiveInTenant: 2, MaxConcurrent: 2}).Check(); err == nil || !strings.Contains(err.Error(), "already going") {
		t.Errorf("two at once: %v", err)
	}
}

func TestDiffModels_ComputedAndOrderIndependent(t *testing.T) {
	a := DiagramModel{
		Nodes: []DiagramNode{{ID: "gw", Label: "ms-gateway", State: StateSame}, {ID: "bug", Label: "bugs", State: StateSame}, {ID: "old", Label: "old", State: StateSame}},
		Edges: []DiagramEdge{{From: "gw", To: "bug", Protocol: "http", State: StateSame}, {From: "bug", To: "old", Protocol: "grpc", State: StateSame}},
	}
	b := DiagramModel{
		Nodes: []DiagramNode{{ID: "bug", Label: "bugs", State: StateAdded}, {ID: "gw", Label: "ms-gateway", State: StateSame}, {ID: "new", Label: "new", State: StateAdded}},
		Edges: []DiagramEdge{{From: "bug", To: "new", Protocol: "kafka", State: StateAdded}, {From: "gw", To: "bug", Protocol: "http", State: StateDeviation}},
	}
	d := DiffModels(a, b)
	want := ModelDiff{
		AddedNodes: []string{"new"}, RemovedNodes: []string{"old"}, ChangedNodes: []string{"bug"},
		AddedEdges: []string{"bug -> new [kafka] #0"}, RemovedEdges: []string{"bug -> old [grpc] #0"}, ChangedEdges: []string{"gw -> bug [http] #0"},
	}
	if strings.Join(d.AddedNodes, ",") != "new" || strings.Join(d.RemovedNodes, ",") != "old" || strings.Join(d.ChangedNodes, ",") != "bug" ||
		strings.Join(d.AddedEdges, ",") != want.AddedEdges[0] || strings.Join(d.RemovedEdges, ",") != want.RemovedEdges[0] || strings.Join(d.ChangedEdges, ",") != want.ChangedEdges[0] {
		t.Errorf("diff = %+v", d)
	}
	if d.Empty() || !DiffModels(a, a).Empty() {
		t.Error("a model has no difference with itself")
	}
	b.Nodes[0], b.Nodes[2] = b.Nodes[2], b.Nodes[0]
	if again := DiffModels(a, b); strings.Join(again.AddedNodes, ",") != "new" || len(again.ChangedNodes) != 1 {
		t.Errorf("order must not matter: %+v", again)
	}
}
