package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// fakeInfraFleetServiceClient implements infrafleetv1.InfraFleetServiceClient
// directly (no bufconn/wire-level gRPC) — this package's tests fake the
// port (the generated client interface), not the transport, mirroring
// git-gateway-service/internal/adapter/grpcclient's fake of the same name.
type fakeInfraFleetServiceClient struct {
	infrafleetv1.InfraFleetServiceClient // embed: panics on any unimplemented method, intentional for these tests

	resolveConnectionResp *infrafleetv1.ResolveConnectionResponse
	resolveConnectionErr  error
	gotResolveConnection  *infrafleetv1.ResolveConnectionRequest

	relayResp *infrafleetv1.RelayResponse
	relayErr  error
	gotRelay  *infrafleetv1.RelayRequest
}

func (f *fakeInfraFleetServiceClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	f.gotResolveConnection = in
	if f.resolveConnectionErr != nil {
		return nil, f.resolveConnectionErr
	}
	return f.resolveConnectionResp, nil
}

func (f *fakeInfraFleetServiceClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.gotRelay = in
	if f.relayErr != nil {
		return nil, f.relayErr
	}
	return f.relayResp, nil
}

// fakeTaskRepository backs SimpleExecutor's tests without a database —
// only Get is exercised here, so the rest of usecase.TaskRepository panics
// via the embed if a test ever calls something unexpected.
type fakeTaskRepository struct {
	tasks map[string]domain.Task
}

func (f *fakeTaskRepository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	panic("not implemented")
}
func (f *fakeTaskRepository) Get(ctx context.Context, tenantID, id string) (domain.Task, error) {
	t, ok := f.tasks[id]
	if !ok {
		return domain.Task{}, errors.New("not found")
	}
	return t, nil
}

// GetAncestors returns a not-found error rather than panicking — TASK-TG-04-06's
// buildExecutePrompt context preamble calls this unconditionally
// (best-effort: an error just means no parent context, never a failed
// dispatch), so every existing test in this file (none of which cares
// about parent context) needs this to degrade gracefully, not crash.
func (f *fakeTaskRepository) GetAncestors(ctx context.Context, tenantID, id string, maxDepth int) ([]domain.Task, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTaskRepository) UpdateStatus(ctx context.Context, tenantID, id, status string) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	panic("not implemented")
}
func (f *fakeTaskRepository) List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error) {
	panic("not implemented")
}
func (f *fakeTaskRepository) Update(ctx context.Context, tenantID string, task domain.Task) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) Delete(ctx context.Context, tenantID, id string) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) UpdateWorktreeID(ctx context.Context, tenantID, id, worktreeID string) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) UpdateActiveExecutionID(ctx context.Context, tenantID, id, activeExecutionID string) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) UpdatePromptTemplate(ctx context.Context, tenantID, id, promptTemplate string) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) UpdateAIPlanJSON(ctx context.Context, tenantID, id, aiPlanJSON string) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) GetSubtree(ctx context.Context, tenantID, rootID string, maxDepth int) ([]domain.Task, []domain.TaskEdge, error) {
	panic("not implemented")
}
func (f *fakeTaskRepository) GetSubtreeWithChildPercents(ctx context.Context, tenantID, rootID string) ([]usecase.SubtreeProgressNode, error) {
	panic("not implemented")
}
func (f *fakeTaskRepository) BatchUpdateProgress(ctx context.Context, tenantID string, updates map[string]int) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) CompleteExecution(ctx context.Context, tenantID, id, status string, actualHours float64) error {
	panic("not implemented")
}

// fakeEdgeRepository backs SimpleExecutor's TASK-TG-04-06 completed-deps
// lookup (ListFrom(..., EdgeKindDependsOn)) without a database — a nil
// edges slice by default (no deps) unless a test sets one.
type fakeEdgeRepository struct {
	edges []domain.TaskEdge
}

func (f *fakeEdgeRepository) Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error {
	panic("not implemented")
}
func (f *fakeEdgeRepository) ListByKind(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	panic("not implemented")
}
func (f *fakeEdgeRepository) ListByKindForUpdate(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	panic("not implemented")
}
func (f *fakeEdgeRepository) ListFrom(ctx context.Context, tenantID, fromTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	var out []domain.TaskEdge
	for _, e := range f.edges {
		if e.FromTaskID == fromTaskID && e.Kind == kind {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeEdgeRepository) ListTo(ctx context.Context, tenantID, toTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	panic("not implemented")
}

// fakeProjectExecutionResolver backs SimpleExecutor's tests without a real
// infra-fleet-service call.
type fakeProjectExecutionResolver struct {
	connectionID string
	worktreePath string
	worktreeID   string
	connected    bool
	err          error
}

func (f *fakeProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, string, string, bool, error) {
	return f.connectionID, f.worktreePath, f.worktreeID, f.connected, f.err
}

// TestSimpleExecutor_Execute_RelaysAgentExecPrompt locks in TASK-224 Gap 1's
// fix: SimpleExecutor must call "agent.execPrompt" (prompt/worktreePath),
// not "agent.exec" (binary/args/cwd) — see simple_executor.go's doc comment
// for the full source citation behind this.
func TestSimpleExecutor_Execute_RelaysAgentExecPrompt(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`},
	}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	ref, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "task-exec:t1:req-1" {
		t.Errorf("expected a synthesized executionRef, got %q", ref)
	}
	if relay.gotRelay.GetMethod() != "agent.execPrompt" {
		t.Errorf("expected method=agent.execPrompt, got %q", relay.gotRelay.GetMethod())
	}
	if relay.gotRelay.GetConnectionId() != "conn-1" {
		t.Errorf("expected resolved connectionId to be used, got %q", relay.gotRelay.GetConnectionId())
	}
	var sentParams agentExecPromptParams
	if err := json.Unmarshal([]byte(relay.gotRelay.GetParamsJson()), &sentParams); err != nil {
		t.Fatalf("params_json didn't decode: %v", err)
	}
	if sentParams.WorktreePath != "/srv/worktrees/p1" {
		t.Errorf("expected resolved worktreePath to be forwarded, got %q", sentParams.WorktreePath)
	}
	if sentParams.Prompt == "" {
		t.Error("expected a non-empty prompt naming the task")
	}
}

// TestSimpleExecutor_NotConnected_ReturnsTypedError locks in that the stub
// behavior (a synthesized placeholder ref, no error) is actually gone.
func TestSimpleExecutor_NotConnected_ReturnsTypedError(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connected: false}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, &fakeInfraFleetServiceClient{})

	_, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1")
	if err == nil {
		t.Fatal("expected a real error for a not-connected project, not a synthesized placeholder ref")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
}

// TestSimpleExecutor_ConnectedButNoWorktreePath_ReturnsTypedError covers
// agent.execPrompt's required worktreePath field having nothing to resolve
// it from — a distinct failure mode from "not connected at all".
func TestSimpleExecutor_ConnectedButNoWorktreePath_ReturnsTypedError(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true, worktreePath: ""}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, &fakeInfraFleetServiceClient{})

	_, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1")
	if err == nil {
		t.Fatal("expected a real error when connected but no worktreePath resolved")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
}

func TestSimpleExecutor_TaskNotFound(t *testing.T) {
	exec := NewSimpleExecutor(&fakeTaskRepository{tasks: map[string]domain.Task{}}, &fakeEdgeRepository{}, &fakeProjectExecutionResolver{}, &fakeInfraFleetServiceClient{})
	if _, err := exec.Execute(context.Background(), "tenant-1", "does-not-exist", "req-1"); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

func TestSimpleExecutor_RelayErrorPropagates(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{relayErr: errors.New("boom")}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1"); err == nil {
		t.Fatal("expected an error when the relay call fails")
	}
}

// TestSimpleExecutor_NonZeroExitCode_ReturnsError proves a failed
// agent.execPrompt run (non-zero exit) surfaces as a real error rather than
// a successful executionRef — there is no separate completion callback to
// catch this later (see simple_executor.go's doc comment's honest-limits
// note).
func TestSimpleExecutor_NonZeroExitCode_ReturnsError(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","stderr":"boom","exitCode":1,"timedOut":false}`},
	}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1"); err == nil {
		t.Fatal("expected an error for a non-zero agent.execPrompt exit code")
	}
}

// TestSimpleExecutor_TimedOut_ReturnsError mirrors the non-zero-exit case
// for agent.execPrompt's other failure signal.
func TestSimpleExecutor_TimedOut_ReturnsError(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","stderr":"","exitCode":null,"timedOut":true}`},
	}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1"); err == nil {
		t.Fatal("expected an error for a timed-out agent.execPrompt run")
	}
}

// TASK-TG-04-06: buildExecutePrompt golden-output tests.

func TestBuildExecutePrompt_TitleOnly_NoOptionalLines(t *testing.T) {
	got := buildExecutePrompt(domain.Task{Title: "Write tests"}, nil, nil)
	want := "Complete the following task.\n\nTask: Write tests\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildExecutePrompt_PromptTemplate_ReplacesGenericOpener(t *testing.T) {
	got := buildExecutePrompt(domain.Task{Title: "Write tests", PromptTemplate: "Custom instructions here."}, nil, nil)
	if strings.Contains(got, "Complete the following task.") {
		t.Errorf("expected the generic opener to be replaced by PromptTemplate, got %q", got)
	}
	if !strings.HasPrefix(got, "Custom instructions here.\n\n") {
		t.Errorf("expected the prompt to start with the verbatim PromptTemplate, got %q", got)
	}
}

func TestBuildExecutePrompt_DescriptionAndAIContext_AppearWhenSet(t *testing.T) {
	got := buildExecutePrompt(domain.Task{Title: "Write tests", Description: "cover the edge cases", AIContext: "prior attempt failed on nil input"}, nil, nil)
	if !strings.Contains(got, "Description: cover the edge cases\n") {
		t.Errorf("expected a Description line, got %q", got)
	}
	if !strings.Contains(got, "Context: prior attempt failed on nil input\n") {
		t.Errorf("expected a Context line, got %q", got)
	}
}

func TestBuildExecutePrompt_EmptyDescriptionAndAIContext_OmittedCleanly(t *testing.T) {
	got := buildExecutePrompt(domain.Task{Title: "Write tests"}, nil, nil)
	if strings.Contains(got, "Description:") {
		t.Errorf("expected no Description line when empty, got %q", got)
	}
	if strings.Contains(got, "Context:") {
		t.Errorf("expected no Context line when empty, got %q", got)
	}
}

func TestBuildExecutePrompt_Parent_AppearsWhenSet(t *testing.T) {
	parent := domain.Task{Title: "Parent epic", Description: "the umbrella feature"}
	got := buildExecutePrompt(domain.Task{Title: "Subtask"}, &parent, nil)
	if !strings.Contains(got, "Parent task: Parent epic\nthe umbrella feature\n") {
		t.Errorf("expected parent context, got %q", got)
	}
}

func TestBuildExecutePrompt_NilParent_OmittedCleanly(t *testing.T) {
	got := buildExecutePrompt(domain.Task{Title: "Root task"}, nil, nil)
	if strings.Contains(got, "Parent task:") {
		t.Errorf("expected no Parent task line for a nil parent, got %q", got)
	}
}

func TestBuildExecutePrompt_CompletedDeps_AppearWhenPresent(t *testing.T) {
	deps := []domain.Task{
		{Title: "Dep A", Description: "first dependency"},
		{Title: "Dep B", Description: "second dependency"},
	}
	got := buildExecutePrompt(domain.Task{Title: "Task"}, nil, deps)
	if !strings.Contains(got, "Completed dependencies:\n- Dep A: first dependency\n- Dep B: second dependency\n") {
		t.Errorf("expected both completed deps listed, got %q", got)
	}
}

func TestBuildExecutePrompt_NoCompletedDeps_OmittedCleanly(t *testing.T) {
	got := buildExecutePrompt(domain.Task{Title: "Task"}, nil, nil)
	if strings.Contains(got, "Completed dependencies:") {
		t.Errorf("expected no Completed dependencies section when empty, got %q", got)
	}
}

// TestSimpleExecutor_Execute_EnvAlwaysContainsTaskAndProjectID locks in
// TASK-TG-04-06's other fix: agent.execPrompt's already-supported (but
// previously never populated) env map.
func TestSimpleExecutor_Execute_EnvAlwaysContainsTaskAndProjectID(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`},
	}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var sentParams agentExecPromptParams
	if err := json.Unmarshal([]byte(relay.gotRelay.GetParamsJson()), &sentParams); err != nil {
		t.Fatalf("params_json didn't decode: %v", err)
	}
	if sentParams.Env["ORCA_TASK_ID"] != "t1" {
		t.Errorf("expected env.ORCA_TASK_ID=t1, got %q", sentParams.Env["ORCA_TASK_ID"])
	}
	if sentParams.Env["ORCA_PROJECT_ID"] != "p1" {
		t.Errorf("expected env.ORCA_PROJECT_ID=p1, got %q", sentParams.Env["ORCA_PROJECT_ID"])
	}
}

// TestSimpleExecutor_Execute_CompletedDepsThreadIntoPrompt is an
// integration-level check that Execute actually resolves completed
// dependencies (via edges.ListFrom + tasks.Get) and threads them into the
// prompt, not just that buildExecutePrompt itself formats them correctly.
func TestSimpleExecutor_Execute_CompletedDepsThreadIntoPrompt(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{
		"t1":  {ID: "t1", ProjectID: "p1", Title: "Do the thing"},
		"dep": {ID: "dep", ProjectID: "p1", Title: "Setup DB", Description: "created the schema", Status: domain.StatusDone},
	}}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "t1", ToTaskID: "dep", Kind: domain.EdgeKindDependsOn},
	}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`},
	}
	exec := NewSimpleExecutor(tasks, edges, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var sentParams agentExecPromptParams
	if err := json.Unmarshal([]byte(relay.gotRelay.GetParamsJson()), &sentParams); err != nil {
		t.Fatalf("params_json didn't decode: %v", err)
	}
	if !strings.Contains(sentParams.Prompt, "Completed dependencies:\n- Setup DB: created the schema\n") {
		t.Errorf("expected the completed dependency to appear in the prompt, got %q", sentParams.Prompt)
	}
}
