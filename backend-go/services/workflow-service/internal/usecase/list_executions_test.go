package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestListExecutions_FiltersByTenantAndProject(t *testing.T) {
	repo := newFakeExecutionRepository()
	ctx1 := withTenantContext(context.Background(), "tenant-1")
	ctx2 := withTenantContext(context.Background(), "tenant-2")

	exec1, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "proj-a", "")
	exec2, _ := domain.NewWorkflowExecution("exec-2", "tenant-1", "tmpl-1", "trace-1", "proj-b", "")
	exec3, _ := domain.NewWorkflowExecution("exec-3", "tenant-2", "tmpl-1", "trace-1", "proj-a", "")
	_ = repo.CreateExecution(ctx1, exec1)
	_ = repo.CreateExecution(ctx1, exec2)
	_ = repo.CreateExecution(ctx2, exec3)

	uc := NewListExecutions(repo)
	out, err := uc.Execute(ctx1, ListExecutionsInput{ProjectID: "proj-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Executions) != 1 || out.Executions[0].ID != "exec-1" {
		t.Errorf("expected exactly tenant-1/proj-a's execution, got %+v", out.Executions)
	}
}

func TestListExecutions_NewestFirst(t *testing.T) {
	repo := newFakeExecutionRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	for _, id := range []string{"exec-1", "exec-2", "exec-3"} {
		exec, _ := domain.NewWorkflowExecution(id, "tenant-1", "tmpl-1", "trace-1", "proj-a", "")
		if err := repo.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("creating %s: %v", id, err)
		}
	}

	uc := NewListExecutions(repo)
	out, err := uc.Execute(ctx, ListExecutionsInput{ProjectID: "proj-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Executions) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(out.Executions))
	}
	wantOrder := []string{"exec-3", "exec-2", "exec-1"}
	for i, want := range wantOrder {
		if out.Executions[i].ID != want {
			t.Errorf("position %d: expected %s, got %s", i, want, out.Executions[i].ID)
		}
	}
}

func TestListExecutions_PaginationCursorAdvances(t *testing.T) {
	repo := newFakeExecutionRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	for _, id := range []string{"exec-1", "exec-2", "exec-3"} {
		exec, _ := domain.NewWorkflowExecution(id, "tenant-1", "tmpl-1", "trace-1", "proj-a", "")
		if err := repo.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("creating %s: %v", id, err)
		}
	}

	uc := NewListExecutions(repo)
	firstPage, err := uc.Execute(ctx, ListExecutionsInput{ProjectID: "proj-a", Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(firstPage.Executions) != 2 || firstPage.NextCursor == "" {
		t.Fatalf("expected a full first page with a next cursor, got %+v", firstPage)
	}

	secondPage, err := uc.Execute(ctx, ListExecutionsInput{ProjectID: "proj-a", Limit: 2, Cursor: firstPage.NextCursor})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secondPage.Executions) != 1 || secondPage.NextCursor != "" {
		t.Errorf("expected exactly one remaining execution and no further page, got %+v", secondPage)
	}
}

// TestListExecutions_EmptyProjectReturnsNoExecutions covers the usecase
// layer only — the "[] not null on the wire" convention is enforced one
// layer up, at the gRPC/wscompat response boundary (both always
// `make([]T, 0, ...)` regardless of what the usecase returns), matching
// ListTemplates' own layering.
func TestListExecutions_EmptyProjectReturnsNoExecutions(t *testing.T) {
	repo := newFakeExecutionRepository()
	ctx := withTenantContext(context.Background(), "tenant-1")

	uc := NewListExecutions(repo)
	out, err := uc.Execute(ctx, ListExecutionsInput{ProjectID: "does-not-exist"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Executions) != 0 {
		t.Errorf("expected zero executions, got %+v", out.Executions)
	}
}

func TestListExecutions_NoTenantInContextErrors(t *testing.T) {
	repo := newFakeExecutionRepository()
	uc := NewListExecutions(repo)

	_, err := uc.Execute(context.Background(), ListExecutionsInput{ProjectID: "proj-a"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
