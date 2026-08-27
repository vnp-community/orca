package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// fakeOrchestrationServiceClient implements
// orchestrationv1.OrchestrationServiceClient directly (embeds the nil
// interface, panics on any unimplemented method) — same convention as
// fakeGitGatewayCreateWorktreeClient, kept separate since this file only
// needs StartCoordinatorRun.
type fakeOrchestrationServiceClient struct {
	orchestrationv1.OrchestrationServiceClient

	startCoordinatorRunResp *orchestrationv1.StartCoordinatorRunResponse
	startCoordinatorRunErr  error
	gotStartCoordinatorRun  *orchestrationv1.StartCoordinatorRunRequest
}

func (f *fakeOrchestrationServiceClient) StartCoordinatorRun(ctx context.Context, in *orchestrationv1.StartCoordinatorRunRequest, _ ...grpc.CallOption) (*orchestrationv1.StartCoordinatorRunResponse, error) {
	f.gotStartCoordinatorRun = in
	if f.startCoordinatorRunErr != nil {
		return nil, f.startCoordinatorRunErr
	}
	return f.startCoordinatorRunResp, nil
}

// fakeSubtreeTaskRepository implements usecase.TaskRepository directly
// (embeds the nil interface, panics on any unimplemented method) — this
// file needs GetSubtree (buildOrchestrationSpec's one dependency) and
// UpdateActiveExecutionID (Execute persists the new run's id right after
// StartCoordinatorRun succeeds, TASK-TG-04-05).
type fakeSubtreeTaskRepository struct {
	usecase.TaskRepository

	subtree              []domain.Task
	subtreeEdges         []domain.TaskEdge
	subtreeErr           error
	gotActiveExecutionID string
}

func (f *fakeSubtreeTaskRepository) UpdateActiveExecutionID(ctx context.Context, tenantID, id, activeExecutionID string) error {
	f.gotActiveExecutionID = activeExecutionID
	return nil
}

func (f *fakeSubtreeTaskRepository) GetSubtree(ctx context.Context, tenantID, rootID string, maxDepth int) ([]domain.Task, []domain.TaskEdge, error) {
	if f.subtreeErr != nil {
		return nil, nil, f.subtreeErr
	}
	return f.subtree, f.subtreeEdges, nil
}

func TestComplexExecutor_Execute_BuildsSpecAndStartsCoordinatorRun(t *testing.T) {
	tasks := &fakeSubtreeTaskRepository{
		subtree: []domain.Task{
			{ID: "root", Title: "Root", PromptTemplate: "do the root thing"},
			{ID: "child-1", Title: "Child 1", Description: "child 1 description"},
		},
		subtreeEdges: []domain.TaskEdge{
			{FromTaskID: "child-1", ToTaskID: "root", Kind: domain.EdgeKindDependsOn},
		},
	}
	orch := &fakeOrchestrationServiceClient{
		startCoordinatorRunResp: &orchestrationv1.StartCoordinatorRunResponse{Id: "run-1"},
	}
	c := NewComplexExecutor(orch, tasks, nil)

	ref, err := c.Execute(ctxWithTenant(t), "tenant-1", "root", "req-1", "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "run-1" {
		t.Errorf("expected the coordinator run's id, got %q", ref)
	}
	if tasks.gotActiveExecutionID != "run-1" {
		t.Errorf("expected UpdateActiveExecutionID to be called with the new run's id, got %q", tasks.gotActiveExecutionID)
	}

	got := orch.gotStartCoordinatorRun
	if got.GetTenantId() != "tenant-1" || got.GetOriginTaskId() != "root" || got.GetWorktreeId() != "wt-1" {
		t.Errorf("unexpected request fields: %+v", got)
	}
	if len(got.GetTasks()) != 2 {
		t.Fatalf("expected one spec node per subtree task, got %d", len(got.GetTasks()))
	}

	byTempID := map[string]*orchestrationv1.OrchestrationTaskSpec{}
	for _, spec := range got.GetTasks() {
		byTempID[spec.GetTempId()] = spec
	}
	rootSpec, ok := byTempID["root"]
	if !ok {
		t.Fatal("expected a spec node with temp_id=root")
	}
	if rootSpec.GetPrompt() != "do the root thing" {
		t.Errorf("expected root's prompt to come from PromptTemplate, got %q", rootSpec.GetPrompt())
	}
	childSpec, ok := byTempID["child-1"]
	if !ok {
		t.Fatal("expected a spec node with temp_id=child-1")
	}
	if childSpec.GetPrompt() != "child 1 description" {
		t.Errorf("expected child-1's prompt to fall back to Description, got %q", childSpec.GetPrompt())
	}
	// deps are temp-id based, translated from the depends_on edge
	// (child-1 -> root), not raw task-service ids re-used as orchestration
	// primary keys — this repo's edges ARE task-service temp_ids already,
	// so the assertion is that Deps threads them through unchanged.
	if len(childSpec.GetDeps()) != 1 || childSpec.GetDeps()[0] != "root" {
		t.Errorf("expected child-1.deps=[root], got %v", childSpec.GetDeps())
	}
	if len(rootSpec.GetDeps()) != 0 {
		t.Errorf("expected root to have no deps, got %v", rootSpec.GetDeps())
	}
}

func TestComplexExecutor_Execute_SubtreeFetchError_NeverCallsStartCoordinatorRun(t *testing.T) {
	tasks := &fakeSubtreeTaskRepository{subtreeErr: errors.New("db unavailable")}
	orch := &fakeOrchestrationServiceClient{}
	c := NewComplexExecutor(orch, tasks, nil)

	if _, err := c.Execute(ctxWithTenant(t), "tenant-1", "root", "req-1", "wt-1"); err == nil {
		t.Fatal("expected an error when the subtree fetch fails")
	}
	if orch.gotStartCoordinatorRun != nil {
		t.Error("expected StartCoordinatorRun to never be called when the subtree fetch fails")
	}
}

func TestComplexExecutor_Execute_StartCoordinatorRunError_Propagates(t *testing.T) {
	tasks := &fakeSubtreeTaskRepository{subtree: []domain.Task{{ID: "root", Title: "Root"}}}
	orch := &fakeOrchestrationServiceClient{startCoordinatorRunErr: errors.New("orchestration-service unavailable")}
	c := NewComplexExecutor(orch, tasks, nil)

	if _, err := c.Execute(ctxWithTenant(t), "tenant-1", "root", "req-1", "wt-1"); err == nil {
		t.Fatal("expected the StartCoordinatorRun error to propagate")
	}
}
