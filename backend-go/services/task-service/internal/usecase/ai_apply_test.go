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
