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
	createTask := NewCreateTask(tasks)
	addEdge := NewAddEdge(edges)
	uc := NewAIApply(createTask, addEdge)
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
	uc := NewAIApply(NewCreateTask(tasks), NewAddEdge(&fakeEdgeRepository{}))
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
	uc := NewAIApply(NewCreateTask(tasks), NewAddEdge(&fakeEdgeRepository{}))
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIApplyInput{TaskID: "parent", Proposals: []domain.SubtaskProposal{{Title: "x"}}}); err == nil {
		t.Fatal("expected an error when subtask creation fails")
	}
}

// TestAIApply_MidLoopFailure_SurfacesErrorButLeavesPartialSubtree locks in
// ai_apply.go's documented non-transactional gap: a failure partway through
// the proposal loop must be a real, detectable error (never silently
// swallowed into an apparently-successful partial result) — but, absent a
// WithTx/UnitOfWork primitive anywhere in this repo (checked, none exists),
// the first proposal's task+edge are NOT rolled back. This test exists to
// keep that gap honest and concrete rather than just prose in a doc comment.
func TestAIApply_MidLoopFailure_SurfacesErrorButLeavesPartialSubtree(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["parent"] = domain.Task{ID: "parent", TenantID: "tenant-1"}
	edges := &fakeEdgeRepository{addErr: errors.New("boom"), addErrAfterCalls: 1}
	uc := NewAIApply(NewCreateTask(tasks), NewAddEdge(edges))
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

	// The gap itself, made concrete: proposal 1's subtask IS still present
	// in the repository despite the error above, because this loop has no
	// transaction to roll it back — exactly what ai_apply.go's doc comment
	// flags.
	foundFirstProposal := false
	for _, tk := range tasks.tasks {
		if tk.Title == "Design API" {
			foundFirstProposal = true
		}
	}
	if !foundFirstProposal {
		t.Error("expected proposal 1's subtask to remain committed — if this starts failing, the non-transactional gap in ai_apply.go's doc comment may no longer be accurate")
	}
}
