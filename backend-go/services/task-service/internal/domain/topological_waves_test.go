package domain

import "testing"

func waveContains(wave []string, id string) bool {
	for _, w := range wave {
		if w == id {
			return true
		}
	}
	return false
}

func indexOfWave(waves [][]string, id string) int {
	for i, wave := range waves {
		if waveContains(wave, id) {
			return i
		}
	}
	return -1
}

// TestTopologicalWaves_DiamondDependency: A depends on B and C; B and C
// each depend on D. Correct grouping: wave0=[D], wave1=[B,C] (independent
// siblings — NOT serialized into separate waves), wave2=[A].
func TestTopologicalWaves_DiamondDependency(t *testing.T) {
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn},
		{FromTaskID: "A", ToTaskID: "C", Kind: EdgeKindDependsOn},
		{FromTaskID: "B", ToTaskID: "D", Kind: EdgeKindDependsOn},
		{FromTaskID: "C", ToTaskID: "D", Kind: EdgeKindDependsOn},
	}
	waves := TopologicalWaves(edges, []string{"A", "B", "C", "D"})
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves, got %d: %+v", len(waves), waves)
	}
	if len(waves[0]) != 1 || waves[0][0] != "D" {
		t.Errorf("expected wave0=[D], got %+v", waves[0])
	}
	if len(waves[1]) != 2 || !waveContains(waves[1], "B") || !waveContains(waves[1], "C") {
		t.Errorf("expected wave1=[B,C] (independent siblings in ONE wave), got %+v", waves[1])
	}
	if len(waves[2]) != 1 || waves[2][0] != "A" {
		t.Errorf("expected wave2=[A], got %+v", waves[2])
	}
}

// TestTopologicalWaves_NoDependencyAmongBatch_IsWaveZero: a task with no
// dependency among the batch set (even if it has real dependency edges
// outside the batch) is wave 0.
func TestTopologicalWaves_NoDependencyAmongBatch_IsWaveZero(t *testing.T) {
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "outside-the-batch", Kind: EdgeKindDependsOn},
	}
	waves := TopologicalWaves(edges, []string{"A", "B"})
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave (both tasks independent within the batch), got %d: %+v", len(waves), waves)
	}
	if !waveContains(waves[0], "A") || !waveContains(waves[0], "B") {
		t.Errorf("expected wave0=[A,B], got %+v", waves[0])
	}
}

func TestTopologicalWaves_ParentChildEdgesIgnored(t *testing.T) {
	edges := []TaskEdge{
		{FromTaskID: "parent", ToTaskID: "child", Kind: EdgeKindParentChild},
	}
	waves := TopologicalWaves(edges, []string{"parent", "child"})
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave (parent_child edges don't create dependency ordering), got %d: %+v", len(waves), waves)
	}
}

func TestTopologicalWaves_EdgesOutsideBatchScope_Ignored(t *testing.T) {
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn}, // B not in the batch
	}
	waves := TopologicalWaves(edges, []string{"A"})
	if len(waves) != 1 || !waveContains(waves[0], "A") {
		t.Fatalf("expected a single wave containing A (B is out of scope), got %+v", waves)
	}
}

func TestTopologicalWaves_LinearChain(t *testing.T) {
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn},
		{FromTaskID: "B", ToTaskID: "C", Kind: EdgeKindDependsOn},
	}
	waves := TopologicalWaves(edges, []string{"A", "B", "C"})
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves for a linear chain, got %d: %+v", len(waves), waves)
	}
	if indexOfWave(waves, "C") >= indexOfWave(waves, "B") || indexOfWave(waves, "B") >= indexOfWave(waves, "A") {
		t.Errorf("expected wave order C, B, A, got %+v", waves)
	}
}
