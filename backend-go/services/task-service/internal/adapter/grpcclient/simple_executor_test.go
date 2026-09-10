package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
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
	// relayBlock, if set, makes Relay wait for it to close before returning
	// — lets throttle-behavior tests control exactly how long Execute's
	// concurrent streaming goroutine (TASK-AG-FLOWTASK-003) runs before its
	// owning Relay call completes.
	relayBlock <-chan struct{}
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
	if f.relayBlock != nil {
		<-f.relayBlock
	}
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
func (f *fakeTaskRepository) SetActiveExecutionLink(ctx context.Context, tenantID, id, linkID string) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	panic("not implemented")
}
func (f *fakeTaskRepository) List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error) {
	panic("not implemented")
}
func (f *fakeTaskRepository) Update(ctx context.Context, tenantID string, task domain.Task, events []domain.OutboxEvent) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) FindByNumber(ctx context.Context, tenantID, projectID string, taskNumber int64) (domain.Task, error) {
	panic("not implemented")
}
func (f *fakeTaskRepository) Delete(ctx context.Context, tenantID, id string) error {
	panic("not implemented")
}
func (f *fakeTaskRepository) UpdateWorktreeID(ctx context.Context, tenantID, id, worktreeID string) error {
	panic("not implemented")
}

// UpdateActiveExecutionID is real (not panicking) — ComplexExecutor.Execute
// (complex_executor_test.go) calls this unconditionally after a successful
// StartCoordinatorRun dispatch.
func (f *fakeTaskRepository) UpdateActiveExecutionID(ctx context.Context, tenantID, id, activeExecutionID string) error {
	if t, ok := f.tasks[id]; ok {
		t.ActiveExecutionID = activeExecutionID
		f.tasks[id] = t
	}
	return nil
}

// UpdateLastExecutionOutput is real (not panicking) — SimpleExecutor.Execute
// (TASK-TG-04-07) calls this unconditionally on every successful run, so
// every existing test in this file needs it to succeed, not crash. Mutates
// the fake's map so a later Get (e.g. a subsequent task's completed-deps
// lookup) sees the persisted output.
func (f *fakeTaskRepository) UpdateLastExecutionOutput(ctx context.Context, tenantID, id, output string) error {
	if t, ok := f.tasks[id]; ok {
		t.LastExecutionOutput = output
		f.tasks[id] = t
	}
	return nil
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

// fakeSimpleExecutorProfileResolver is an in-memory usecase.ProfileResolver.
type fakeSimpleExecutorProfileResolver struct {
	settings map[string]any
	err      error
	calls    []string // userIDs, in call order
}

func (f *fakeSimpleExecutorProfileResolver) GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error) {
	f.calls = append(f.calls, userID)
	if f.err != nil {
		return nil, f.err
	}
	return f.settings, nil
}

// fakeSimpleExecutorProjectContextResolver is an in-memory
// usecase.ProjectContextResolver.
type fakeSimpleExecutorProjectContextResolver struct {
	ctx   usecase.ProjectContext
	err   error
	calls []string // projectIDs, in call order
}

func (f *fakeSimpleExecutorProjectContextResolver) GetProjectContext(ctx context.Context, projectID string) (usecase.ProjectContext, error) {
	f.calls = append(f.calls, projectID)
	if f.err != nil {
		return usecase.ProjectContext{}, f.err
	}
	return f.ctx, nil
}

// fakeOutboxWriter records every InsertOutboxEvent call — TASK-AG-FLOWTASK-003's
// publishThrottledOutput/publishAgentOutputPartial tests read events back
// from this instead of a real Postgres table.
type fakeOutboxWriter struct {
	mu     sync.Mutex
	events []fakeOutboxEvent
}

type fakeOutboxEvent struct {
	TenantID, Subject string
	Payload           []byte
}

func (f *fakeOutboxWriter) InsertOutboxEvent(ctx context.Context, id, tenantID, subject string, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, fakeOutboxEvent{TenantID: tenantID, Subject: subject, Payload: append([]byte(nil), payload...)})
	return nil
}

func (f *fakeOutboxWriter) snapshot() []fakeOutboxEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeOutboxEvent(nil), f.events...)
}

// fakeAgentExecOutputStreamer backs SimpleExecutor's tests without a real
// infra-fleet-service StreamExecOutput call — chunks (if set) is returned
// as-is; a nil chunks field returns an already-closed empty channel,
// matching usecase.AgentExecOutputStreamer's "a streaming failure never
// blocks Execute" contract for the tests that don't care about streaming.
type fakeAgentExecOutputStreamer struct {
	chunks chan usecase.AgentExecOutputChunk
}

func (f *fakeAgentExecOutputStreamer) StreamExecOutput(ctx context.Context, connectionID, stepID string) <-chan usecase.AgentExecOutputChunk {
	if f.chunks == nil {
		ch := make(chan usecase.AgentExecOutputChunk)
		close(ch)
		return ch
	}
	return f.chunks
}

// newTestSimpleExecutor builds a SimpleExecutor with fresh no-op profile/
// project-context/outbox/streamer fakes — used by every test that doesn't
// care about the profile-aware env injection path or TASK-AG-FLOWTASK-003's
// mid-run output streaming.
func newTestSimpleExecutor(tasks usecase.TaskRepository, edges usecase.EdgeRepository, resolver usecase.ProjectExecutionResolver, relay infrafleetv1.InfraFleetServiceClient) *SimpleExecutor {
	return NewSimpleExecutor(tasks, edges, resolver, relay, &fakeSimpleExecutorProfileResolver{}, &fakeSimpleExecutorProjectContextResolver{}, &fakeOutboxWriter{}, &fakeAgentExecOutputStreamer{})
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
	exec := newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	ref, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", "")
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
	exec := newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, &fakeInfraFleetServiceClient{})

	_, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", "")
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
	exec := newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, &fakeInfraFleetServiceClient{})

	_, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", "")
	if err == nil {
		t.Fatal("expected a real error when connected but no worktreePath resolved")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
}

func TestSimpleExecutor_TaskNotFound(t *testing.T) {
	exec := newTestSimpleExecutor(&fakeTaskRepository{tasks: map[string]domain.Task{}}, &fakeEdgeRepository{}, &fakeProjectExecutionResolver{}, &fakeInfraFleetServiceClient{})
	if _, err := exec.Execute(context.Background(), "tenant-1", "does-not-exist", "req-1", ""); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

func TestSimpleExecutor_RelayErrorPropagates(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{relayErr: errors.New("boom")}
	exec := newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", ""); err == nil {
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
	exec := newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", ""); err == nil {
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
	exec := newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", ""); err == nil {
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

// TestSimpleExecutor_MethodStaysAgentExecPrompt_NoRegression confirms this
// executor was already correct (unlike workflow-service's AgentExecutor)
// and profile-aware env injection doesn't change the method string.
func TestSimpleExecutor_MethodStaysAgentExecPrompt_NoRegression(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","stderr":"","exitCode":0,"timedOut":false}`},
	}
	profiles := &fakeSimpleExecutorProfileResolver{settings: map[string]any{"agent": map[string]any{"preferredModel": "claude-opus-4-5"}}}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay, profiles, &fakeSimpleExecutorProjectContextResolver{}, &fakeOutboxWriter{}, &fakeAgentExecOutputStreamer{})

	ctx := tenant.WithUserID(context.Background(), "user-1")
	if _, err := exec.Execute(ctx, "tenant-1", "t1", "req-1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if relay.gotRelay.GetMethod() != "agent.execPrompt" {
		t.Errorf("expected method=agent.execPrompt, got %q", relay.gotRelay.GetMethod())
	}
}

// TestSimpleExecutor_ResolvableUserID_PopulatesEnvAndModel asserts the
// profile-aware path: a resolvable actor id populates env/model.
func TestSimpleExecutor_ResolvableUserID_PopulatesEnvAndModel(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","stderr":"","exitCode":0,"timedOut":false}`},
	}
	profiles := &fakeSimpleExecutorProfileResolver{settings: map[string]any{"agent": map[string]any{"preferredModel": "claude-opus-4-5"}}}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay, profiles, &fakeSimpleExecutorProjectContextResolver{}, &fakeOutboxWriter{}, &fakeAgentExecOutputStreamer{})

	ctx := tenant.WithUserID(context.Background(), "user-1")
	if _, err := exec.Execute(ctx, "tenant-1", "t1", "req-1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles.calls) != 1 || profiles.calls[0] != "user-1" {
		t.Errorf("expected ProfileResolver called with user-1, got %v", profiles.calls)
	}
	var sentParams agentExecPromptParams
	if err := json.Unmarshal([]byte(relay.gotRelay.GetParamsJson()), &sentParams); err != nil {
		t.Fatalf("params_json didn't decode: %v", err)
	}
	if sentParams.Model != "claude-opus-4-5" {
		t.Errorf("expected model claude-opus-4-5, got %q", sentParams.Model)
	}
	if len(sentParams.Env) == 0 {
		t.Error("expected env to be populated")
	}
}

// TestSimpleExecutor_ProfileResolverError_DegradesToLegacyPassthrough
// asserts a ProfileResolver failure never blocks task execution, but still
// carries TASK-TG-04-06's base ORCA_TASK_ID/ORCA_PROJECT_ID env pair.
func TestSimpleExecutor_ProfileResolverError_DegradesToLegacyPassthrough(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","stderr":"","exitCode":0,"timedOut":false}`},
	}
	profiles := &fakeSimpleExecutorProfileResolver{err: errors.New("tenant-service unreachable")}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay, profiles, &fakeSimpleExecutorProjectContextResolver{}, &fakeOutboxWriter{}, &fakeAgentExecOutputStreamer{})

	ctx := tenant.WithUserID(context.Background(), "user-1")
	if _, err := exec.Execute(ctx, "tenant-1", "t1", "req-1", ""); err != nil {
		t.Fatalf("expected profile-resolve failure to degrade to legacy passthrough, got error: %v", err)
	}
	var sentParams agentExecPromptParams
	if err := json.Unmarshal([]byte(relay.gotRelay.GetParamsJson()), &sentParams); err != nil {
		t.Fatalf("params_json didn't decode: %v", err)
	}
	if sentParams.Model != "" {
		t.Errorf("expected no model on profile-resolve failure, got %+v", sentParams)
	}
	if sentParams.Env["ORCA_TASK_ID"] != "t1" || sentParams.Env["ORCA_PROJECT_ID"] != "p1" {
		t.Errorf("expected TASK-TG-04-06's base env to survive a profile-resolve failure, got %+v", sentParams.Env)
	}
}

// TestSimpleExecutor_ProjectContextResolverError_SpawnStillProceeds asserts
// the same best-effort posture for the project-context lookup.
func TestSimpleExecutor_ProjectContextResolverError_SpawnStillProceeds(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"","stderr":"","exitCode":0,"timedOut":false}`},
	}
	profiles := &fakeSimpleExecutorProfileResolver{settings: map[string]any{}}
	projects := &fakeSimpleExecutorProjectContextResolver{err: errors.New("project-service unreachable")}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay, profiles, projects, &fakeOutboxWriter{}, &fakeAgentExecOutputStreamer{})

	ctx := tenant.WithUserID(context.Background(), "user-1")
	if _, err := exec.Execute(ctx, "tenant-1", "t1", "req-1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var sentParams agentExecPromptParams
	if err := json.Unmarshal([]byte(relay.gotRelay.GetParamsJson()), &sentParams); err != nil {
		t.Fatalf("params_json didn't decode: %v", err)
	}
	if sentParams.InitFile != "" {
		t.Errorf("expected empty InitFile on project-context failure, got %q", sentParams.InitFile)
	}
}

// TestSimpleExecutor_Execute_EnvAlwaysContainsTaskAndProjectID locks in
// TASK-TG-04-06's other fix: agent.execPrompt's already-supported (but
// previously never populated) env map always at least carries the base
// task/project id pair, even with no actor in context (no profile lookup).
func TestSimpleExecutor_Execute_EnvAlwaysContainsTaskAndProjectID(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`},
	}
	exec := newTestSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", ""); err != nil {
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
	exec := newTestSimpleExecutor(tasks, edges, resolver, relay)

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", ""); err != nil {
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

// executeResult carries Execute's two return values through a channel —
// TestSimpleExecutor_Execute_PublishesThrottledPartialOutputToOutbox below
// runs Execute in a goroutine (so the test can feed streamer chunks and let
// a throttle tick fire before the blocked Relay call is allowed to return).
type executeResult struct {
	ref string
	err error
}

// TestSimpleExecutor_Execute_PublishesThrottledPartialOutputToOutbox is
// TASK-AG-FLOWTASK-003's core acceptance test: SimpleExecutor.Execute
// consumes AgentExecOutputStreamer concurrently with its own Relay call,
// and publishes a throttled agent_output_partial outbox event carrying the
// cumulative buffer — matching CR-FLOW-TASK-003's TaskActivityFrame shape
// via origin_task_id (the field channels_task_activity.go's
// translateToTaskActivity filters every subject's payload on).
func TestSimpleExecutor_Execute_PublishesThrottledPartialOutputToOutbox(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1", Title: "Do the thing"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relayBlock := make(chan struct{})
	relay := &fakeInfraFleetServiceClient{
		relayResp:  &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`},
		relayBlock: relayBlock,
	}
	chunks := make(chan usecase.AgentExecOutputChunk, 4)
	outbox := &fakeOutboxWriter{}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay, &fakeSimpleExecutorProfileResolver{}, &fakeSimpleExecutorProjectContextResolver{}, outbox, &fakeAgentExecOutputStreamer{chunks: chunks})
	exec.throttleInterval = 20 * time.Millisecond // real production default (2s) would make this test far too slow

	resultCh := make(chan executeResult, 1)
	go func() {
		ref, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", "")
		resultCh <- executeResult{ref: ref, err: err}
	}()

	chunks <- usecase.AgentExecOutputChunk{Stream: "stdout", Data: "hello "}
	chunks <- usecase.AgentExecOutputChunk{Stream: "stdout", Data: "world"}

	// Give the throttle ticker time to fire at least once before letting
	// the blocked Relay call (and therefore Execute) return.
	time.Sleep(80 * time.Millisecond)
	close(relayBlock)

	res := <-resultCh
	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}

	events := outbox.snapshot()
	if len(events) == 0 {
		t.Fatal("expected at least one throttled agent_output_partial event")
	}
	for _, ev := range events {
		if ev.Subject != agentOutputPartialSubject {
			t.Errorf("expected subject %q, got %q", agentOutputPartialSubject, ev.Subject)
		}
		if ev.TenantID != "tenant-1" {
			t.Errorf("expected tenantID=tenant-1, got %q", ev.TenantID)
		}
	}
	var payload agentOutputPartialPayload
	if err := json.Unmarshal(events[len(events)-1].Payload, &payload); err != nil {
		t.Fatalf("payload didn't decode: %v", err)
	}
	if payload.OriginTaskID != "t1" {
		t.Errorf("expected origin_task_id=t1, got %q", payload.OriginTaskID)
	}
	if payload.Stdout != "hello world" {
		t.Errorf("expected the cumulative stdout buffer, got %q", payload.Stdout)
	}
}

// TestSimpleExecutor_Execute_BatchesManyChunksIntoAtMostOneEvent locks in
// the "never one outbox event per raw chunk" acceptance criterion —
// CR-FLOW-TASK-003 explicitly scoped itself to discrete events, not a byte
// stream (SOL-AG-FLOWTASK-001 §2.3).
func TestSimpleExecutor_Execute_BatchesManyChunksIntoAtMostOneEvent(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`},
	}
	const chunkCount = 50
	chunks := make(chan usecase.AgentExecOutputChunk, chunkCount)
	for range chunkCount {
		chunks <- usecase.AgentExecOutputChunk{Stream: "stdout", Data: "x"}
	}
	close(chunks) // simulates the run's stream ending — StreamExecOutput's real channel closes the same way once ctx is cancelled
	outbox := &fakeOutboxWriter{}
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay, &fakeSimpleExecutorProfileResolver{}, &fakeSimpleExecutorProjectContextResolver{}, outbox, &fakeAgentExecOutputStreamer{chunks: chunks})
	exec.throttleInterval = time.Hour // no tick fires during this fast test — only the closed-channel final flush publishes

	if _, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := outbox.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected exactly one batched event for %d chunks (never one-event-per-chunk), got %d", chunkCount, len(events))
	}
	var payload agentOutputPartialPayload
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("payload didn't decode: %v", err)
	}
	if len(payload.Stdout) != chunkCount {
		t.Errorf("expected all %d chunks batched into one buffer, got len=%d", chunkCount, len(payload.Stdout))
	}
}

// TestSimpleExecutor_Execute_StreamingFailureDoesNotFailExecute proves a
// streaming subscribe failure (e.g. StreamExecOutput's connectionId
// resolve failing on infra-fleet-service's side) never blocks or fails
// Execute — its own unary Relay call remains the sole source of truth for
// success/failure, per usecase.AgentExecOutputStreamer's doc comment.
func TestSimpleExecutor_Execute_StreamingFailureDoesNotFailExecute(t *testing.T) {
	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{connectionID: "conn-1", worktreePath: "/srv/worktrees/p1", connected: true}
	relay := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`},
	}
	outbox := &fakeOutboxWriter{}
	// fakeAgentExecOutputStreamer{} with a nil chunks field returns an
	// already-closed empty channel — simulates a streaming subscribe that
	// never delivered anything.
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relay, &fakeSimpleExecutorProfileResolver{}, &fakeSimpleExecutorProjectContextResolver{}, outbox, &fakeAgentExecOutputStreamer{})

	ref, err := exec.Execute(context.Background(), "tenant-1", "t1", "req-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "task-exec:t1:req-1" {
		t.Errorf("expected the normal executionRef, got %q", ref)
	}
	if len(outbox.snapshot()) != 0 {
		t.Error("expected no outbox events when the stream never delivered any chunks")
	}
}
