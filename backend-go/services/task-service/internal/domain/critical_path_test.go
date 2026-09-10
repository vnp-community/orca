package domain

import (
	"reflect"
	"testing"
)

func hoursPtr(h float64) *float64 { return &h }

// TestCalculateCriticalPath_FiveNodeTwoBranch_MatchesHandComputedPath builds
// a 5-node DAG with 2 branches:
//
//	A(2h) -> B(3h) -> D(1h)
//	A(2h) -> C(10h) -> D(1h)
//	D(1h) -> E(4h)
//
// Branch through C is the longest: A(2) -> C(10) -> D(1) -> E(4) = 17h,
// versus A(2) -> B(3) -> D(1) -> E(4) = 10h.
func TestCalculateCriticalPath_FiveNodeTwoBranch_MatchesHandComputedPath(t *testing.T) {
	tasks := []Task{
		{ID: "A", EstimatedHours: hoursPtr(2)},
		{ID: "B", EstimatedHours: hoursPtr(3)},
		{ID: "C", EstimatedHours: hoursPtr(10)},
		{ID: "D", EstimatedHours: hoursPtr(1)},
		{ID: "E", EstimatedHours: hoursPtr(4)},
	}
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn},
		{FromTaskID: "A", ToTaskID: "C", Kind: EdgeKindDependsOn},
		{FromTaskID: "B", ToTaskID: "D", Kind: EdgeKindDependsOn},
		{FromTaskID: "C", ToTaskID: "D", Kind: EdgeKindDependsOn},
		{FromTaskID: "D", ToTaskID: "E", Kind: EdgeKindDependsOn},
	}

	got := CalculateCriticalPath(tasks, edges)
	want := []string{"A", "C", "D", "E"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected critical path %v, got %v", want, got)
	}
}

// TestCalculateCriticalPath_Deterministic_AcrossRepeatedCalls is the
// tie-break regression test: repeated calls against identical input must
// return the exact same path.
func TestCalculateCriticalPath_Deterministic_AcrossRepeatedCalls(t *testing.T) {
	tasks := []Task{
		{ID: "A", EstimatedHours: hoursPtr(1)},
		{ID: "B", EstimatedHours: hoursPtr(1)},
		{ID: "C", EstimatedHours: hoursPtr(1)},
	}
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn},
		{FromTaskID: "A", ToTaskID: "C", Kind: EdgeKindDependsOn},
	}

	first := CalculateCriticalPath(tasks, edges)
	for i := 0; i < 10; i++ {
		got := CalculateCriticalPath(tasks, edges)
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("expected deterministic path across repeated calls, got %v then %v", first, got)
		}
	}
}

// TestCalculateCriticalPath_NilEstimatedHours_TreatedAsZero proves a task
// with no estimate contributes 0 to path length but is not excluded from
// the graph.
func TestCalculateCriticalPath_NilEstimatedHours_TreatedAsZero(t *testing.T) {
	tasks := []Task{
		{ID: "A", EstimatedHours: hoursPtr(5)},
		{ID: "B", EstimatedHours: nil},
		{ID: "C", EstimatedHours: hoursPtr(5)},
	}
	edges := []TaskEdge{
		{FromTaskID: "A", ToTaskID: "B", Kind: EdgeKindDependsOn},
		{FromTaskID: "B", ToTaskID: "C", Kind: EdgeKindDependsOn},
	}

	got := CalculateCriticalPath(tasks, edges)
	want := []string{"A", "B", "C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected path %v through the zero-duration node, got %v", want, got)
	}
}

func TestCalculateCriticalPath_EmptyTasks_ReturnsNil(t *testing.T) {
	if got := CalculateCriticalPath(nil, nil); got != nil {
		t.Errorf("expected nil for empty tasks, got %v", got)
	}
}

func TestCalculateCriticalPath_NoEdges_ReturnsSingleHighestEstimateTask(t *testing.T) {
	tasks := []Task{
		{ID: "A", EstimatedHours: hoursPtr(1)},
		{ID: "B", EstimatedHours: hoursPtr(9)},
	}
	got := CalculateCriticalPath(tasks, nil)
	if len(got) != 1 || got[0] != "B" {
		t.Errorf("expected single-node path [B], got %v", got)
	}
}

// TestCalculateCriticalPath_IgnoresParentChildEdges confirms only
// depends_on edges participate — parent_child edges are a different
// relation entirely, and a task with no depends_on edges at all still
// yields a (single-node) path from the highest-estimate task.
func TestCalculateCriticalPath_IgnoresParentChildEdges(t *testing.T) {
	tasks := []Task{
		{ID: "parent", EstimatedHours: hoursPtr(5)},
		{ID: "child", EstimatedHours: hoursPtr(3)},
	}
	edges := []TaskEdge{
		{FromTaskID: "parent", ToTaskID: "child", Kind: EdgeKindParentChild},
	}
	got := CalculateCriticalPath(tasks, edges)
	if len(got) != 1 || got[0] != "parent" {
		t.Errorf("expected single-node path [parent] (parent_child edges ignored), got %v", got)
	}
}
