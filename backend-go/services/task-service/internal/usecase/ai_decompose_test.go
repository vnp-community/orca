package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type fakeAIProviderContextResolver struct {
	ctx string
	err error
}

func (f *fakeAIProviderContextResolver) ResolveContext(ctx context.Context, tenantID, userID string) (string, error) {
	return f.ctx, f.err
}

type fakeProjectExecutionResolver struct {
	connectionID string
	worktreePath string
	connected    bool
	err          error
}

func (f *fakeProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, string, bool, error) {
	return f.connectionID, f.worktreePath, f.connected, f.err
}

type fakeAICompleter struct {
	content string
	err     error
	gotConn string
}

func (f *fakeAICompleter) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	f.gotConn = connectionID
	return f.content, f.err
}

func TestAIDecompose_RequiresTenantContext(t *testing.T) {
	uc := NewAIDecompose(newFakeTaskRepository(), &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{})
	if _, err := uc.Execute(context.Background(), AIDecomposeInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestAIDecompose_NotConnected_ReturnsFailedPrecondition is TASK-224's
// core regression guard: a not-connected project must never silently
// return an empty proposal list.
func TestAIDecompose_NotConnected_ReturnsFailedPrecondition(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connected: false}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected error when project has no connected dev server")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition — a not-connected project must never silently return an empty proposal list, got %v", err)
	}
}

func TestAIDecompose_Connected_ReturnsParsedProposals(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	completer := &fakeAICompleter{content: "1. Design API\n2. Implement handler"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{ctx: "anthropic"}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one parsed proposal")
	}
	if completer.gotConn != "conn-1" {
		t.Errorf("expected resolved connectionID to be passed through, got %q", completer.gotConn)
	}
}

func TestAIDecompose_TaskNotFound(t *testing.T) {
	uc := NewAIDecompose(newFakeTaskRepository(), &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "does-not-exist"}); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

func TestAIDecompose_ProviderResolveFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{err: errors.New("boom")}, &fakeProjectExecutionResolver{connected: true}, &fakeAICompleter{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when AI provider context resolution fails")
	}
}

func TestAIDecompose_RelayFailurePropagates(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	uc := NewAIDecompose(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, &fakeAICompleter{err: errors.New("boom")})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, AIDecomposeInput{TaskID: "t1"}); err == nil {
		t.Fatal("expected an error when the AI relay call fails")
	}
}
