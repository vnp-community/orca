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

// TestAIApply_DependsOnIndex_CreatesDependsOnEdgesAmongSiblings is
// TASK-TG-002-02's core new-behavior test: a proposal's DependsOnIndex
// resolves to a real depends_on edge between the two CREATED sibling
// subtasks (not the parent), added in a second pass after every subtask in
// the batch has a real ID.
func TestAIApply_DependsOnIndex_CreatesDependsOnEdgesAmongSiblings(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1", Status: domain.StatusOpen}
	edges := &fakeEdgeRepository{}
	uc := NewAIApply(newFakeTxRunner(tasks, edges))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	// proposal 1 ("Implement handler") depends on proposal 0 ("Design API").
	proposals := []domain.SubtaskProposal{
		{Title: "Design API"},
		{Title: "Implement handler", DependsOnIndex: []int{0}},
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
	if dependsOnEdges[0].FromTaskID != created[0].ID || dependsOnEdges[0].ToTaskID != created[1].ID {
		t.Errorf("expected depends_on edge %s -> %s, got %+v", created[0].ID, created[1].ID, dependsOnEdges[0])
	}

	// Auto-block (TASK-TG-001-04): "Design API" is still open (not done),
	// so "Implement handler" must be auto-blocked by the same AddEdge call.
	blocked := tasks.tasks[created[1].ID]
	if blocked.Status != domain.StatusBlocked {
		t.Errorf("expected the dependent subtask to be auto-blocked, got status %q", blocked.Status)
	}
}

// TestAIApply_DependsOnIndex_OutOfRange_RollsBackWholeBatch proves an
// invalid DependsOnIndex (out of range for this batch) fails the whole
// RunInTx closure — zero tasks/edges from the batch survive.
func TestAIApply_DependsOnIndex_OutOfRange_RollsBackWholeBatch(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	edges := &fakeEdgeRepository{}
	uc := NewAIApply(newFakeTxRunner(tasks, edges))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	proposals := []domain.SubtaskProposal{
		{Title: "Design API", DependsOnIndex: []int{5}}, // out of range: batch has only 1 proposal
	}
	created, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: proposals})
	if err == nil {
		t.Fatal("expected an error for an out-of-range depends_on_index")
	}
	if created != nil {
		t.Errorf("expected no created subtasks returned to the caller, got %+v", created)
	}
	if len(tasks.tasks) != 1 { // only "parent" should remain
		t.Errorf("expected only the pre-existing parent task to remain, got %d: %+v", len(tasks.tasks), tasks.tasks)
	}
	if len(edges.edges) != 0 {
		t.Errorf("expected no edges to remain after rollback, got %+v", edges.edges)
	}
}

// TestAIApply_DependsOnIndex_SameBatchCycle_RollsBackWholeBatch proves a
// DependsOnIndex cycle within the same proposal batch is rejected by the
// reused AddEdge.Execute's cycle check exactly like a manual AddEdge call
// would reject it, and the whole transaction rolls back — zero tasks
// created, matching the existing MidLoopFailure fixture's assertion shape.
func TestAIApply_DependsOnIndex_SameBatchCycle_RollsBackWholeBatch(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	edges := &fakeEdgeRepository{}
	uc := NewAIApply(newFakeTxRunner(tasks, edges))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	// 0 depends on 1, and 1 depends on 0 — a 2-cycle within the same batch.
	proposals := []domain.SubtaskProposal{
		{Title: "A", DependsOnIndex: []int{1}},
		{Title: "B", DependsOnIndex: []int{0}},
	}
	created, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: proposals})
	if err == nil {
		t.Fatal("expected an error for a same-batch depends_on cycle")
	}
	if created != nil {
		t.Errorf("expected no created subtasks returned to the caller, got %+v", created)
	}
	if len(tasks.tasks) != 1 {
		t.Errorf("expected only the pre-existing parent task to remain (whole batch rolled back), got %d: %+v", len(tasks.tasks), tasks.tasks)
	}
	if len(edges.edges) != 0 {
		t.Errorf("expected no edges to remain after rollback, got %+v", edges.edges)
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
