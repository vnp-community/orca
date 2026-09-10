package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// fakeOrchestrationServiceClient implements
// orchestrationv1.OrchestrationServiceClient directly — same
// fake-the-generated-client-port convention as fakeInfraFleetServiceClient
// (simple_executor_test.go).
type fakeOrchestrationServiceClient struct {
	orchestrationv1.OrchestrationServiceClient // embed: panics on any unimplemented method, intentional for these tests

	startResp *orchestrationv1.CoordinatorRun
	startErr  error
	got       *orchestrationv1.StartCoordinatorRunRequest
}

func (f *fakeOrchestrationServiceClient) StartCoordinatorRun(_ context.Context, in *orchestrationv1.StartCoordinatorRunRequest, _ ...grpc.CallOption) (*orchestrationv1.CoordinatorRun, error) {
	f.got = in
	if f.startErr != nil {
		return nil, f.startErr
	}
	return f.startResp, nil
}

// complexExecutorEdgeRepository is a minimal usecase.EdgeRepository for
// this file's tests — only ListFrom(parent_child) is exercised by
// ComplexExecutor.buildSpec.
type complexExecutorEdgeRepository struct {
	edges []domain.TaskEdge
}

func (f *complexExecutorEdgeRepository) Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error {
	panic("not implemented")
}
func (f *complexExecutorEdgeRepository) ListByKind(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	panic("not implemented")
}
func (f *complexExecutorEdgeRepository) ListByKindForUpdate(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	panic("not implemented")
}
func (f *complexExecutorEdgeRepository) ListFrom(ctx context.Context, tenantID, fromTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	var out []domain.TaskEdge
	for _, e := range f.edges {
		if e.Kind == kind && e.FromTaskID == fromTaskID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *complexExecutorEdgeRepository) ListTo(ctx context.Context, tenantID, toTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	panic("not implemented")
}

func TestComplexExecutor_NoChildren_BuildsSingleRootNode(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{
		"task-1": {ID: "task-1", TenantID: "tenant-1", Title: "Root Task"},
	}}
	edges := &complexExecutorEdgeRepository{}
	client := &fakeOrchestrationServiceClient{startResp: &orchestrationv1.CoordinatorRun{Id: "run-1"}}
	c := NewComplexExecutor(tasks, edges, client)

	ref, err := c.Execute(ctxWithTenant(t), "tenant-1", "task-1", "req-1", "wt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "run-1" {
		t.Errorf("expected the coordinator run id to be returned, got %q", ref)
	}
	if client.got.GetOriginTaskId() != "task-1" {
		t.Errorf("expected origin_task_id=task-1, got %q", client.got.GetOriginTaskId())
	}
	if client.got.GetWorktreeId() != "wt-1" {
		t.Errorf("expected the dispatched worktree_id to pass through, got %q", client.got.GetWorktreeId())
	}
	if tasks.tasks["task-1"].ActiveExecutionID != "run-1" {
		t.Errorf("expected UpdateActiveExecutionID to persist the new run's id, got %q", tasks.tasks["task-1"].ActiveExecutionID)
	}
	var nodes []specNode
	if err := json.Unmarshal([]byte(client.got.GetSpecJson()), &nodes); err != nil {
		t.Fatalf("spec_json did not unmarshal: %v", err)
	}
	if len(nodes) != 1 || nodes[0].TempID != "task-1" || len(nodes[0].Deps) != 0 {
		t.Fatalf("expected a single, dep-free root node, got %+v", nodes)
	}
}

func TestComplexExecutor_WithChildren_RootDependsOnEveryChild(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{
		"task-1": {ID: "task-1", TenantID: "tenant-1", Title: "Root Task"},
		"sub-1":  {ID: "sub-1", TenantID: "tenant-1", Title: "Subtask 1"},
		"sub-2":  {ID: "sub-2", TenantID: "tenant-1", Title: "Subtask 2"},
	}}
	edges := &complexExecutorEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "sub-1", Kind: domain.EdgeKindParentChild},
		{FromTaskID: "task-1", ToTaskID: "sub-2", Kind: domain.EdgeKindParentChild},
	}}
	client := &fakeOrchestrationServiceClient{startResp: &orchestrationv1.CoordinatorRun{Id: "run-2"}}
	c := NewComplexExecutor(tasks, edges, client)

	if _, err := c.Execute(ctxWithTenant(t), "tenant-1", "task-1", "req-1", "wt-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var nodes []specNode
	if err := json.Unmarshal([]byte(client.got.GetSpecJson()), &nodes); err != nil {
		t.Fatalf("spec_json did not unmarshal: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes (root + 2 children), got %d: %+v", len(nodes), nodes)
	}
	root := nodes[0]
	if root.TempID != "task-1" {
		t.Fatalf("expected the root node first (index 0), got %+v", root)
	}
	if len(root.Deps) != 2 {
		t.Errorf("expected the root to depend on both children, got deps=%v", root.Deps)
	}
}

func TestComplexExecutor_StartCoordinatorRunFailurePropagates(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{
		"task-1": {ID: "task-1", TenantID: "tenant-1", Title: "Root Task"},
	}}
	client := &fakeOrchestrationServiceClient{startErr: errors.New("orchestration-service unavailable")}
	c := NewComplexExecutor(tasks, &complexExecutorEdgeRepository{}, client)

	if _, err := c.Execute(ctxWithTenant(t), "tenant-1", "task-1", "req-1", "wt-1"); err == nil {
		t.Fatal("expected the orchestration-service failure to propagate")
	}
}
