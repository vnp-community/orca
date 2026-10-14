package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type fakeContractExecutor struct {
	got  ContractExecuteInput
	out  ContractExecuteOutput
	err  error
	runs int
}

func (f *fakeContractExecutor) ExecuteWithContract(ctx context.Context, in ContractExecuteInput) (ContractExecuteOutput, error) {
	f.runs++
	f.got = in
	return f.out, f.err
}

type contractFixture struct {
	uc       *ExecuteTask
	tasks    *fakeTaskRepository
	edges    *fakeEdgeRepository
	simple   *fakeSimpleExecutor
	contract *fakeContractExecutor
	complex  *fakeExecutor
	ctx      context.Context
}

func newContractFixture(t *testing.T) *contractFixture {
	t.Helper()
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	task := tasks.tasks["task-1"]
	task.RequestID = "11111111-1111-1111-1111-111111111111"
	tasks.tasks["task-1"] = task
	f := &contractFixture{tasks: tasks, edges: &fakeEdgeRepository{}, simple: &fakeSimpleExecutor{ref: "plain-ref"},
		contract: &fakeContractExecutor{out: ContractExecuteOutput{ExecutionRef: "contract-ref", Record: domain.ExecutionRecord{ID: "rec-1"}}},
		complex:  &fakeExecutor{ref: "orch"}, ctx: withIdentity(context.Background(), "tenant-1", "user-1")}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, f.edges, f.simple, f.complex, grants)
	f.uc = uc.WithContract(f.contract).WithRunEvents(newLeases(tasks), nil)
	return f
}

func dependsOn(f *contractFixture) {
	f.edges.edges = append(f.edges.edges, domain.TaskEdge{FromTaskID: "task-1", ToTaskID: "dep", Kind: domain.EdgeKindDependsOn})
}

// A Request's task is ordered by request-service, so depends_on must not hand it to the coordinator: that run never
// launched the agent and left the task in_progress for good.
func TestSelectEngine_RequestTaskWithDependsOn_IsDirectAgent(t *testing.T) {
	f := newContractFixture(t)
	dependsOn(f)
	engine, err := f.uc.selectEngine(f.ctx, "tenant-1", f.tasks.tasks["task-1"])
	if err != nil || engine != domain.EngineDirectAgent {
		t.Fatalf("got %s %v", engine, err)
	}
}

func TestSelectEngine_TaskWithoutRequestWithDependsOn_StaysOrchestration(t *testing.T) {
	f := newContractFixture(t)
	task := f.tasks.tasks["task-1"]
	task.RequestID = ""
	dependsOn(f)
	if engine, _ := f.uc.selectEngine(f.ctx, "tenant-1", task); engine != domain.EngineOrchestration {
		t.Fatalf("got %s", engine)
	}
}

func TestSelectEngine_ContainerWithChildren_StaysOrchestration(t *testing.T) {
	f := newContractFixture(t)
	f.edges.edges = append(f.edges.edges, domain.TaskEdge{FromTaskID: "task-1", ToTaskID: "child", Kind: domain.EdgeKindParentChild})
	if engine, _ := f.uc.selectEngine(f.ctx, "tenant-1", f.tasks.tasks["task-1"]); engine != domain.EngineOrchestration {
		t.Fatalf("got %s", engine)
	}
}

func TestSelectEngine_WorkflowTemplateWins(t *testing.T) {
	f := newContractFixture(t)
	task := f.tasks.tasks["task-1"]
	task.WorkflowTemplateID = "wf"
	if engine, _ := f.uc.selectEngine(f.ctx, "tenant-1", task); engine != domain.EngineWorkflow {
		t.Fatalf("got %s", engine)
	}
}

func TestExecuteTask_RequestTaskWithDependsOn_RunsTheAgentNotTheCoordinator(t *testing.T) {
	f := newContractFixture(t)
	dependsOn(f)
	if _, err := f.uc.Execute(f.ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req:r:task-1:1"}); err != nil {
		t.Fatal(err)
	}
	if !f.simple.called || f.complex.called {
		t.Fatalf("agent=%v coordinator=%v", f.simple.called, f.complex.called)
	}
	if got := f.tasks.tasks["task-1"].Status; got != domain.StatusReview {
		t.Fatalf("the run completes inline, got %s", got)
	}
}

func TestExecuteTask_PromptOverrideAllowedForRequestTaskWithDeps(t *testing.T) {
	f := newContractFixture(t)
	dependsOn(f)
	_, err := f.uc.Execute(f.ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req:r:task-1:3", Prompt: "do it", ResultNonce: "abcdef0123456789"})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if f.contract.runs != 1 || f.contract.got.Prompt != "do it" || f.contract.got.ResultNonce != "abcdef0123456789" || f.contract.got.Attempt != 3 {
		t.Fatalf("contract input: %+v", f.contract.got)
	}
	if f.simple.called {
		t.Fatal("plain executor must not run on the contract path")
	}
}

func TestExecuteTask_ContractPathCarriesTheKnowledgeAndVerifyFlagsAndAPlainRunIgnoresThem(t *testing.T) {
	f := newContractFixture(t)
	in := ExecuteTaskInput{TaskID: "task-1", RequestID: "req:r:task-1:1", ResultNonce: "abcdef0123456789", UseKnowledgeHub: true, RequirePassingCheck: true}
	if _, err := f.uc.Execute(f.ctx, in); err != nil {
		t.Fatal(err)
	}
	if got := f.contract.got; !got.UseKnowledgeHub || !got.RequirePassingCheck || got.Prompt != "" {
		t.Fatalf("contract input: %+v", got)
	}
	plain := newContractFixture(t)
	if _, err := plain.uc.Execute(plain.ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1", UseKnowledgeHub: true, RequirePassingCheck: true}); err != nil {
		t.Fatal(err)
	}
	if plain.contract.runs != 0 || !plain.simple.called {
		t.Fatalf("flags without a nonce must not select the contract path: contract=%d simple=%v", plain.contract.runs, plain.simple.called)
	}
}

func TestExecuteTask_NoNonce_UsesPlainExecutor(t *testing.T) {
	f := newContractFixture(t)
	if _, err := f.uc.Execute(f.ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatal(err)
	}
	if !f.simple.called || f.contract.runs != 0 {
		t.Fatalf("simple=%v contract=%d", f.simple.called, f.contract.runs)
	}
}

func runEventsOf(t *testing.T, f *contractFixture) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ev := range f.tasks.execEvents {
		if ev.Subject != statusChangedSubject {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(ev.PayloadJSON, &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func onlyEvent(t *testing.T, f *contractFixture) map[string]any {
	t.Helper()
	evs := runEventsOf(t, f)
	if len(evs) != 1 {
		t.Fatalf("a run must emit exactly one statuschanged event, got %d: %v", len(evs), evs)
	}
	return evs[0]
}

func TestExecuteTask_ContractFailure_RevertsAndEmitsFailureClass(t *testing.T) {
	f := newContractFixture(t)
	f.contract.err = &ExecutionFailure{Class: domain.FailureAgentDefect, Code: "RESULT_BLOCK_MISSING", RecordID: "rec-9"}
	if _, err := f.uc.Execute(f.ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req:r:task-1:1", Prompt: "p", ResultNonce: "abcdef0123456789"}); err != nil {
		t.Fatal(err)
	}
	if got := f.tasks.tasks["task-1"].Status; got != domain.StatusOpen {
		t.Fatalf("task must revert, got %s", got)
	}
	ev := onlyEvent(t, f)
	if ev["failure_class"] != "agent_defect" || ev["execution_record_id"] != "rec-9" || ev["cause"] != "execution_failed" || ev["new_status"] != "open" {
		t.Fatalf("event: %v", ev)
	}
}

func TestExecuteTask_ContractSuccess_GoesToReview(t *testing.T) {
	f := newContractFixture(t)
	if _, err := f.uc.Execute(f.ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req:r:task-1:1", Prompt: "p", ResultNonce: "abcdef0123456789"}); err != nil {
		t.Fatal(err)
	}
	if len(f.tasks.completeExecutionCalls) != 1 || f.tasks.completeExecutionCalls[0].status != string(domain.StatusReview) {
		t.Fatalf("calls: %+v", f.tasks.completeExecutionCalls)
	}
	ev := onlyEvent(t, f)
	if ev["execution_record_id"] != "rec-1" || ev["cause"] != "execution_completed" {
		t.Fatalf("event: %v", ev)
	}
	if _, has := ev["failure_class"]; has {
		t.Fatal("failure_class must be omitted on success")
	}
}

func TestStatusChangedPayload_OmitsContractFieldsWhenEmpty(t *testing.T) {
	b, _ := json.Marshal(taskStatusChangedPayload{TaskID: "t"})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if _, ok := m["failure_class"]; ok {
		t.Fatalf("%v", m)
	}
	if _, ok := m["execution_record_id"]; ok {
		t.Fatalf("%v", m)
	}
}

func TestExecuteTask_PlainPath_EmitsExactlyOneStatusChanged(t *testing.T) {
	f := newContractFixture(t)
	if _, err := f.uc.Execute(f.ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatal(err)
	}
	if ev := onlyEvent(t, f); ev["cause"] != "execution_completed" {
		t.Fatalf("event: %v", ev)
	}
}

func TestParseAttempt(t *testing.T) {
	for in, want := range map[string]int{"req:r:t:4": 4, "req-1": 1, "req:r:t:x": 1, "req:r:t:0": 1, "": 1} {
		if got := parseAttempt(in); got != want {
			t.Errorf("%q: got %d want %d", in, got, want)
		}
	}
}

type fakeExecutionRecordRepository struct {
	recs    []domain.ExecutionRecord
	gotIDs  []string
	latest  bool
	listErr error
}

func (f *fakeExecutionRecordRepository) InsertExecutionRecord(ctx context.Context, r domain.ExecutionRecord) (domain.ExecutionRecord, error) {
	f.recs = append(f.recs, r)
	return r, nil
}

func (f *fakeExecutionRecordRepository) ListExecutionRecords(ctx context.Context, tenantID string, ids []string, latestOnly bool, limit int) ([]domain.ExecutionRecord, error) {
	f.gotIDs, f.latest = ids, latestOnly
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.ExecutionRecord
	for _, r := range f.recs {
		if r.TenantID == tenantID {
			out = append(out, r)
		}
	}
	return out, nil
}

func TestListExecutionRecords_BadRequest(t *testing.T) {
	uc := NewListExecutionRecords(&fakeExecutionRecordRepository{}, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "u")
	for _, ids := range [][]string{nil, {}, {""}, make([]string, MaxExecutionRecordLimit+1)} {
		if _, err := uc.Execute(ctx, ListExecutionRecordsInput{TaskIDs: ids}); code(err) != "TASK_EXECUTION_RECORD_BAD_REQUEST" {
			t.Errorf("%d ids: got %v", len(ids), err)
		}
	}
}

func TestListExecutionRecords_TenantIsolated(t *testing.T) {
	repo := &fakeExecutionRecordRepository{recs: []domain.ExecutionRecord{{ID: "a", TenantID: "tenant-1"}, {ID: "b", TenantID: "tenant-2"}}}
	got, err := NewListExecutionRecords(repo, nil).Execute(withIdentity(context.Background(), "tenant-1", "u"), ListExecutionRecordsInput{TaskIDs: []string{"t"}})
	if err != nil || len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestListExecutionRecords_LatestOnlyAndDedup(t *testing.T) {
	repo := &fakeExecutionRecordRepository{}
	_, err := NewListExecutionRecords(repo, nil).Execute(withIdentity(context.Background(), "tenant-1", "u"), ListExecutionRecordsInput{TaskIDs: []string{"t", "t", "u"}, LatestOnly: true})
	if err != nil || !repo.latest || len(repo.gotIDs) != 2 {
		t.Fatalf("latest=%v ids=%v err=%v", repo.latest, repo.gotIDs, err)
	}
}

func TestListExecutionRecords_PermissionDenied(t *testing.T) {
	repo := &fakeExecutionRecordRepository{}
	_, err := NewListExecutionRecords(repo, &fakePermission{deny: true}).Execute(withIdentity(context.Background(), "tenant-1", "u"), ListExecutionRecordsInput{TaskIDs: []string{"t"}})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied || repo.gotIDs != nil {
		t.Fatalf("got %v", err)
	}
}
