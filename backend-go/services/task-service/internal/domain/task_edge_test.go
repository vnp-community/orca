package domain

import "testing"

func TestNewTaskEdge_ValidatesInvariants(t *testing.T) {
	if _, err := NewTaskEdge("a", "a", EdgeKindDependsOn); err != ErrSelfEdge {
		t.Fatalf("expected ErrSelfEdge, got %v", err)
	}
	if _, err := NewTaskEdge("a", "b", "bogus"); err != ErrInvalidEdgeKind {
		t.Fatalf("expected ErrInvalidEdgeKind, got %v", err)
	}
	edge, err := NewTaskEdge("a", "b", EdgeKindDependsOn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edge.FromTaskID != "a" || edge.ToTaskID != "b" || edge.Kind != EdgeKindDependsOn {
		t.Errorf("unexpected edge: %+v", edge)
	}
}

func dependsOn(from, to string) TaskEdge {
	return TaskEdge{FromTaskID: from, ToTaskID: to, Kind: EdgeKindDependsOn}
}

func parentChild(from, to string) TaskEdge {
	return TaskEdge{FromTaskID: from, ToTaskID: to, Kind: EdgeKindParentChild}
}

func TestDetectCycle_SelfEdgeIsAlwaysCyclic(t *testing.T) {
	if !DetectCycle(nil, dependsOn("a", "a")) {
		t.Error("expected a self-edge to be reported as cyclic")
	}
}

func TestDetectCycle_NoCycle_SimpleChain(t *testing.T) {
	// A depends on B. Proposing B depends on C is a legitimate chain
	// extension, not a cycle.
	existing := []TaskEdge{dependsOn("a", "b")}
	if DetectCycle(existing, dependsOn("b", "c")) {
		t.Error("expected no cycle for a linear extension of the chain")
	}
}

func TestDetectCycle_DirectTwoHopCycle(t *testing.T) {
	// A depends on B. Proposing B depends on A closes an immediate loop.
	existing := []TaskEdge{dependsOn("a", "b")}
	if !DetectCycle(existing, dependsOn("b", "a")) {
		t.Error("expected a cycle for the direct back-edge B -> A")
	}
}

func TestDetectCycle_ThreeHopCycle(t *testing.T) {
	// A depends on B, B depends on C. Proposing C depends on A closes a
	// 3-hop loop: A -> B -> C -> A.
	existing := []TaskEdge{dependsOn("a", "b"), dependsOn("b", "c")}
	if !DetectCycle(existing, dependsOn("c", "a")) {
		t.Error("expected a cycle for the 3-hop loop C -> A -> B -> C")
	}
}

func TestDetectCycle_FourHopCycle(t *testing.T) {
	// A -> B -> C -> D exists. Proposing D -> A closes a 4-hop loop.
	existing := []TaskEdge{dependsOn("a", "b"), dependsOn("b", "c"), dependsOn("c", "d")}
	if !DetectCycle(existing, dependsOn("d", "a")) {
		t.Error("expected a cycle for the 4-hop loop")
	}
}

func TestDetectCycle_NoCycle_JoiningTwoSeparateChains(t *testing.T) {
	// A -> B and C -> D are separate chains. Proposing B -> C merely joins
	// them into one longer chain (A -> B -> C -> D), not a cycle.
	existing := []TaskEdge{dependsOn("a", "b"), dependsOn("c", "d")}
	if DetectCycle(existing, dependsOn("b", "c")) {
		t.Error("expected no cycle when joining two independent chains")
	}
}

func TestDetectCycle_NoCycle_DiamondDependency(t *testing.T) {
	// A depends on both B and C; B and C both depend on D. Proposing A
	// depends on D directly is redundant but not cyclic.
	existing := []TaskEdge{
		dependsOn("a", "b"),
		dependsOn("a", "c"),
		dependsOn("b", "d"),
		dependsOn("c", "d"),
	}
	if DetectCycle(existing, dependsOn("a", "d")) {
		t.Error("expected no cycle for a diamond-shaped dependency graph")
	}
}

func TestDetectCycle_IgnoresEdgesOfADifferentKind(t *testing.T) {
	// A parent_child B, B parent_child A would be a cycle in the hierarchy
	// graph, but the proposed edge is depends_on — the two edge kinds must
	// not be conflated during the graph walk.
	existing := []TaskEdge{parentChild("a", "b")}
	if DetectCycle(existing, dependsOn("b", "a")) {
		t.Error("expected parent_child edges not to influence a depends_on cycle check")
	}
}

func TestDetectCycle_DeepChainWithoutCycle(t *testing.T) {
	// A long acyclic chain (a0 -> a1 -> ... -> a9) followed by proposing an
	// edge that extends it further must not be flagged.
	var existing []TaskEdge
	for i := 0; i < 9; i++ {
		existing = append(existing, dependsOn(nthTask(i), nthTask(i+1)))
	}
	if DetectCycle(existing, dependsOn(nthTask(9), nthTask(10))) {
		t.Error("expected no cycle extending a long acyclic chain")
	}
}

func TestDetectCycle_DeepChainWithCycleAtTheFarEnd(t *testing.T) {
	// Same long chain, but this time close the loop all the way back to
	// the first node — must still be detected even many hops away.
	var existing []TaskEdge
	for i := 0; i < 9; i++ {
		existing = append(existing, dependsOn(nthTask(i), nthTask(i+1)))
	}
	if !DetectCycle(existing, dependsOn(nthTask(9), nthTask(0))) {
		t.Error("expected a cycle to be detected across a long chain")
	}
}

func nthTask(n int) string {
	return string(rune('a' + n))
}
