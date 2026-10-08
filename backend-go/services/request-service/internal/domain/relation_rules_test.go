package domain

import "testing"

func TestRelationRules_FullCartesian(t *testing.T) {
	want := map[relationTriple]bool{}
	for _, tr := range []relationTriple{
		{RelationDerivedFrom, NodeSolution, NodeRequest}, {RelationDerivedFrom, NodePlan, NodeSolution},
		{RelationImplements, NodePlan, NodeOption},
		{RelationContains, NodePlan, NodePhase}, {RelationContains, NodePlan, NodeTask}, {RelationContains, NodePhase, NodeTask},
		{RelationDependsOn, NodeTask, NodeTask}, {RelationDependsOn, NodePhase, NodePhase},
		{RelationVerifies, NodeCheck, NodeAC}, {RelationVerifies, NodeCheck, NodeTask},
		{RelationSupersedes, NodeSolution, NodeSolution}, {RelationSupersedes, NodePlan, NodePlan},
		{RelationSpawnedBy, NodeRequest, NodeRequest}, {RelationEvidencedBy, NodeSolution, NodeEvidence},
	} {
		want[tr] = true
	}
	checked := 0
	for _, rel := range AllRelations() {
		for _, from := range AllNodeKinds() {
			for _, to := range AllNodeKinds() {
				checked++
				if got := AllowedRelation(rel, from, to); got != want[relationTriple{rel, from, to}] {
					t.Errorf("%s %s->%s: got %v", rel, from, to, got)
				}
			}
		}
	}
	if checked != 8*9*9 {
		t.Fatalf("checked %d triples", checked)
	}
}

func TestStoredInRelationsTable_FourHere(t *testing.T) {
	var stored []Relation
	for _, r := range AllRelations() {
		if StoredInRelationsTable(r) {
			stored = append(stored, r)
		}
	}
	// verifies (request_coverage), contains/depends_on (task-service) and spawned_by (request_links) live elsewhere.
	if len(stored) != 4 {
		t.Fatalf("stored = %v", stored)
	}
}

func TestNewArtifactRelation(t *testing.T) {
	if _, err := NewArtifactRelation("i", "t", "r", RelationDerivedFrom, NodeSolution, "s", NodeRequest, "q"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewArtifactRelation("i", "t", "r", RelationDerivedFrom, NodeRequest, "s", NodeSolution, "q"); errCode(err) != "REQUEST_ARTIFACT_RELATION_NOT_ALLOWED" {
		t.Fatalf("reversed ends must be refused: %v", err)
	}
	if _, err := NewArtifactRelation("i", "t", "r", RelationContains, NodePlan, "p", NodePhase, "h"); errCode(err) != "REQUEST_ARTIFACT_RELATION_NOT_ALLOWED" {
		t.Fatalf("contains is not stored here: %v", err)
	}
	if _, err := NewArtifactRelation("i", "t", "r", RelationSupersedes, NodePlan, "", NodePlan, "x"); err == nil {
		t.Fatal("empty id must be refused")
	}
}
