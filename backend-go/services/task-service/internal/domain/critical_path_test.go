package domain

import (
	"reflect"
	"testing"
)

// TestCalculateCriticalPath_DiamondDAG: A depends on B and C; B and C each
// depend on D. D=1h, B=2h, C=5h, A=1h. The D->C->A leg (1+5+1=7) beats the
// D->B->A leg (1+2+1=4) — critical path picks the longer one.
func TestCalculateCriticalPath_DiamondDAG(t *testing.T) {
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn},
		{FromTaskID: "A", ToTaskID: "C", Kind: EdgeKindDependsOn},
		{FromTaskID: "B", ToTaskID: "D", Kind: EdgeKindDependsOn},
		{FromTaskID: "C", ToTaskID: "D", Kind: EdgeKindDependsOn},
	}
	hours := map[string]float64{"A": 1, "B": 2, "C": 5, "D": 1}

	path, total := CalculateCriticalPath(edges, hours)
	if !reflect.DeepEqual(path, []string{"D", "C", "A"}) {
		t.Errorf("expected path [D C A], got %v", path)
	}
	if total != 7 {
		t.Errorf("expected total 7, got %v", total)
	}
}

// TestCalculateCriticalPath_ParallelChains_LongestWins: two independent
// chains (no edges between them) — the longer one is the critical path.
func TestCalculateCriticalPath_ParallelChains_LongestWins(t *testing.T) {
	edges := []TaskEdge{
		// chain 1: A -> B -> C (3 nodes)
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn},
		{FromTaskID: "B", ToTaskID: "C", Kind: EdgeKindDependsOn},
		// chain 2: X -> Y (2 nodes)
		{FromTaskID: "X", ToTaskID: "Y", Kind: EdgeKindDependsOn},
	}
	hours := map[string]float64{"A": 1, "B": 1, "C": 1, "X": 10, "Y": 10}

	path, total := CalculateCriticalPath(edges, hours)
	if !reflect.DeepEqual(path, []string{"Y", "X"}) {
		t.Errorf("expected path [Y X] (the heavier chain), got %v", path)
	}
	if total != 20 {
		t.Errorf("expected total 20, got %v", total)
	}
}

// TestCalculateCriticalPath_NoEdges_ReturnsEmpty: with no edges, no nodes
// are ever discovered (nodes come only from edge endpoints) — must not
// panic or divide by zero, just return an empty result.
func TestCalculateCriticalPath_NoEdges_ReturnsEmpty(t *testing.T) {
	path, total := CalculateCriticalPath(nil, map[string]float64{"solo": 5})
	if path != nil {
		t.Errorf("expected a nil path, got %v", path)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %v", total)
	}
}

// TestCalculateCriticalPath_AllZeroHours_StillWalksFullChain locks in the
// tie-break fix: a linear chain with every hours entry defaulting to 0
// (spec: "if AI/user leaves estimate blank") must still return the FULL
// chain (path length == node count), not collapse to a single node just
// because every candidate ties at 0.
func TestCalculateCriticalPath_AllZeroHours_StillWalksFullChain(t *testing.T) {
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn},
		{FromTaskID: "B", ToTaskID: "C", Kind: EdgeKindDependsOn},
		{FromTaskID: "C", ToTaskID: "D", Kind: EdgeKindDependsOn},
	}
	path, total := CalculateCriticalPath(edges, map[string]float64{})
	if len(path) != 4 {
		t.Fatalf("expected path length == node count (4), got %d: %v", len(path), path)
	}
	if !reflect.DeepEqual(path, []string{"D", "C", "B", "A"}) {
		t.Errorf("expected path [D C B A], got %v", path)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %v", total)
	}
}

// TestCalculateCriticalPath_IgnoresParentChildEdges confirms only
// depends_on edges participate — parent_child edges are a different
// relation entirely.
func TestCalculateCriticalPath_IgnoresParentChildEdges(t *testing.T) {
	edges := []TaskEdge{
		{FromTaskID: "parent", ToTaskID: "child", Kind: EdgeKindParentChild},
	}
	path, total := CalculateCriticalPath(edges, map[string]float64{"parent": 5, "child": 3})
	if path != nil || total != 0 {
		t.Errorf("expected no path for a parent_child-only edge set, got %v/%v", path, total)
	}
}
