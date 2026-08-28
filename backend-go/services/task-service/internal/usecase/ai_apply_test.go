package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestAIApply_CreatesSubtaskPerProposalAndLinksParentChild(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1", Title: "Parent"}
	edges := &fakeEdgeRepository{}
	uc := NewAIApply(newFakeTxRunner(tasks, edges))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	proposals := []domain.SubtaskProposal{
		{Title: "Design API"},
		{Title: "Implement handler"},
	}
	created, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: proposals})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 created subtasks, got %d", len(created))
	}
	for i, c := range created {
		if c.Title != proposals[i].Title {
			t.Errorf("expected subtask %d title %q, got %q", i, proposals[i].Title, c.Title)
		}
		if c.ParentID != "parent" {
			t.Errorf("expected subtask %d ParentID=parent, got %q", i, c.ParentID)
		}
	}
	if len(edges.edges) != 2 {
		t.Fatalf("expected 2 parent_child edges, got %d", len(edges.edges))
	}
	for _, e := range edges.edges {
		if e.Kind != domain.EdgeKindParentChild || e.FromTaskID != "parent" {
			t.Errorf("unexpected edge: %+v", e)
		}
	}
}

func TestAIApply_EmptyProposals_CreatesNothing(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	uc := NewAIApply(newFakeTxRunner(tasks, &fakeEdgeRepository{}))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	created, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("expected no created subtasks, got %+v", created)
	}
}

func TestAIApply_CreateFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	tasks.createErr = errors.New("boom")
	uc := NewAIApply(newFakeTxRunner(tasks, &fakeEdgeRepository{}))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: []domain.SubtaskProposal{{Title: "x"}}}); err == nil {
		t.Fatal("expected an error when subtask creation fails")
	}
}

// TestAIApply_MidLoopFailure_RollsBackEntireSubtree closes TASK-224 Gap 2:
// a failure partway through the proposal loop must be both a real,
// detectable error AND leave NO partial subtree behind — proposal 1's
// subtask+edge, already committed inside the same transaction as proposal
// 2's failing AddEdge call, must roll back together. This replaces the
// previous (pre-fix) test of the same scenario,
// TestAIApply_MidLoopFailure_SurfacesErrorButLeavesPartialSubtree, which
// asserted the OPPOSITE of the last assertion below (that proposal 1's
// subtask WAS still present) — see ai_apply.go's doc comment for exactly
// what changed and why.
func TestAIApply_MidLoopFailure_RollsBackEntireSubtree(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	edges := &fakeEdgeRepository{addErr: errors.New("boom"), addErrAfterCalls: 1}
	uc := NewAIApply(newFakeTxRunner(tasks, edges))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	proposals := []domain.SubtaskProposal{
		{Title: "Design API"},
		{Title: "Implement handler"},
	}
	created, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: proposals})

	if err == nil {
		t.Fatal("expected a real error when a later proposal's AddEdge fails — not a silently-succeeded partial subtree")
	}
	if created != nil {
		t.Errorf("expected no created subtasks returned to the caller on failure, got %+v", created)
	}

	// The fix, made concrete: proposal 1's subtask must NOT remain in the
	// repository after the rollback, even though its own CreateTask+AddEdge
	// calls succeeded before proposal 2's AddEdge failed.
	for _, tk := range tasks.tasks {
		if tk.Title == "Design API" {
			t.Errorf("expected proposal 1's subtask to be rolled back, but found it still committed: %+v", tk)
		}
	}
	if len(tasks.tasks) != 1 { // only "parent" should remain
		t.Errorf("expected only the pre-existing parent task to remain, got %d tasks: %+v", len(tasks.tasks), tasks.tasks)
	}
	if len(edges.edges) != 0 {
		t.Errorf("expected no edges to remain after rollback, got %+v", edges.edges)
	}
}

// TestAIApply_DependsOnIndices_CreatesDependsOnEdgesAfterAllSiblingsExist
// is TASK-TG-02-05's core regression test: proposal[1] depends on
// proposal[0] (by index) — the resulting depends_on edge must reference the
// REAL created Task.IDs, not the proposal indices themselves, and can only
// be built once every sibling proposal has been created.
func TestAIApply_DependsOnIndices_CreatesDependsOnEdgesAfterAllSiblingsExist(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	edges := &fakeEdgeRepository{}
	uc := NewAIApply(newFakeTxRunner(tasks, edges))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	proposals := []domain.SubtaskProposal{
		{Title: "Design API"}, // index 0
		{Title: "Implement handler", DependsOnIndices: []int{0}}, // index 1 depends on index 0
	}
	created, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: proposals})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 created subtasks, got %d", len(created))
	}

	var dependsOnEdges []domain.TaskEdge
	for _, e := range edges.edges {
		if e.Kind == domain.EdgeKindDependsOn {
			dependsOnEdges = append(dependsOnEdges, e)
		}
	}
	if len(dependsOnEdges) != 1 {
		t.Fatalf("expected exactly 1 depends_on edge, got %d: %+v", len(dependsOnEdges), dependsOnEdges)
	}
	if dependsOnEdges[0].FromTaskID != created[1].ID || dependsOnEdges[0].ToTaskID != created[0].ID {
		t.Errorf("expected depends_on edge %s -> %s, got %+v", created[1].ID, created[0].ID, dependsOnEdges[0])
	}
}

// TestAIApply_OutOfRangeDependsOnIndex_SkippedNotFailed: a hallucinated
// out-of-range DependsOnIndices entry is silently skipped, not a hard
// failure of the whole apply.
func TestAIApply_OutOfRangeDependsOnIndex_SkippedNotFailed(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	edges := &fakeEdgeRepository{}
	uc := NewAIApply(newFakeTxRunner(tasks, edges))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	proposals := []domain.SubtaskProposal{
		{Title: "Only one", DependsOnIndices: []int{5}}, // out of range — no index 5 exists
	}
	created, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: proposals})
	if err != nil {
		t.Fatalf("expected no error for an out-of-range depends_on index, got %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 created subtask, got %d", len(created))
	}
	for _, e := range edges.edges {
		if e.Kind == domain.EdgeKindDependsOn {
			t.Errorf("expected no depends_on edge for an out-of-range index, got %+v", e)
		}
	}
}

// TestAIApply_RawAIResponse_PersistedToAIPlanJSON confirms the raw AI
// response is echoed through to Task.AIPlanJSON via UpdateAIPlanJSON.
func TestAIApply_RawAIResponse_PersistedToAIPlanJSON(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	uc := NewAIApply(newFakeTxRunner(tasks, &fakeEdgeRepository{}))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	raw := `{"subtasks":[{"title":"x"}]}`
	if _, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: []domain.SubtaskProposal{{Title: "x"}}, RawAIResponse: raw}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tasks.tasks["parent"].AIPlanJSON; got != raw {
		t.Errorf("expected parent.AIPlanJSON=%q, got %q", raw, got)
	}
}

// TestAIApply_DependencyPassFailure_RollsBackWholeTransaction extends the
// rollback guarantee to the SECOND pass (dependency edges): a failure
// there must still roll back the already-created subtasks + parent_child
// edges from the first pass, not just the dependency edges themselves.
func TestAIApply_DependencyPassFailure_RollsBackWholeTransaction(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	// Two parent_child Add calls (one per proposal) succeed; the third Add
	// call (the depends_on edge, in the second pass) fails.
	edges := &fakeEdgeRepository{addErr: errors.New("boom"), addErrAfterCalls: 2}
	uc := NewAIApply(newFakeTxRunner(tasks, edges))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	proposals := []domain.SubtaskProposal{
		{Title: "Design API"},
		{Title: "Implement handler", DependsOnIndices: []int{0}},
	}
	created, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: proposals})
	if err == nil {
		t.Fatal("expected an error when the dependency-edge pass fails")
	}
	if created != nil {
		t.Errorf("expected no created subtasks returned on failure, got %+v", created)
	}
	if len(tasks.tasks) != 1 {
		t.Errorf("expected only the pre-existing parent task to remain, got %d: %+v", len(tasks.tasks), tasks.tasks)
	}
	if len(edges.edges) != 0 {
		t.Errorf("expected no edges to remain after rollback, got %+v", edges.edges)
	}
}
