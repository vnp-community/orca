package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func requestTask(id string) domain.Task {
	return domain.Task{ID: id, TenantID: "tenant-1", ProjectID: "proj-1", Type: domain.TypeTask, ParentID: "phase-1", RequestID: "req-1", WorktreeID: "wt-1"}
}

func decodeRunPayload(t *testing.T, ev domain.OutboxEvent) taskStatusChangedPayload {
	t.Helper()
	var p taskStatusChangedPayload
	if err := json.Unmarshal(ev.PayloadJSON, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNewRunStatusEvent_NoRequestID_NoEvent(t *testing.T) {
	task := requestTask("t")
	task.RequestID = ""
	if _, ok := newRunStatusEvent(task, domain.StatusOpen, domain.StatusInProgress, CauseExecuteClaim, "", "", "", time.Now()); ok {
		t.Fatal("plain task must not emit")
	}
}

func TestNewRunStatusEvent_TruncatesErrorMessageUTF8(t *testing.T) {
	msg := strings.Repeat("é", 700) // 1400 bytes, 2 bytes per rune
	ev, ok := newRunStatusEvent(requestTask("t"), domain.StatusInProgress, domain.StatusOpen, CauseExecutionFailed, "l", domain.EngineDirectAgent, msg, time.Now())
	if !ok {
		t.Fatal("expected event")
	}
	got := decodeRunPayload(t, ev).ErrorMessage
	if len(got) > 1024 || len(got) < 1022 || strings.Contains(got, "�") {
		t.Fatalf("bad truncation: %d bytes", len(got))
	}
}

func TestNewRunStatusEvent_IncludesLinkAndEngine(t *testing.T) {
	ev, _ := newRunStatusEvent(requestTask("t"), domain.StatusInProgress, domain.StatusReview, CauseExecutionCompleted, "link-9", domain.EngineWorkflow, "", time.Now())
	p := decodeRunPayload(t, ev)
	if ev.Subject != "orca.task.task.statuschanged" || p.ExecutionLinkID != "link-9" || p.Engine != "workflow" ||
		p.Cause != "execution_completed" || p.RequestID != "req-1" || p.ParentID != "phase-1" || p.TaskType != domain.TypeTask {
		t.Fatalf("bad payload: %s", ev.PayloadJSON)
	}
}

// The consumer contract fixture must stay parseable with the exact keys request-service reads.
func TestNewRunStatusEvent_MatchesConsumerFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/statuschanged_execution_failed.json")
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	ev, _ := newRunStatusEvent(requestTask("task-1"), domain.StatusInProgress, domain.StatusOpen, CauseExecutionFailed, "link-1", domain.EngineDirectAgent, "agent crashed", time.Now())
	if err := json.Unmarshal(ev.PayloadJSON, &got); err != nil {
		t.Fatal(err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %s: fixture %v, produced %v", k, v, got[k])
		}
	}
	if len(got) != len(want) {
		t.Errorf("key sets differ: fixture %d keys, produced %d", len(want), len(got))
	}
}

func seedRequestTask(t *testing.T, tasks *fakeTaskRepository, grants *fakeGrantRepository, withRequest bool) {
	t.Helper()
	seedExecutableTask(t, tasks, grants, "task-1", "proj-1")
	if withRequest {
		task := tasks.tasks["task-1"]
		task.RequestID = "req-1"
		task.ParentID = "phase-1"
		tasks.tasks["task-1"] = task
	}
}

func TestExecuteTask_RequestTask_EmitsClaimEvent(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedRequestTask(t, tasks, grants, true)
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, &fakeSimpleExecutor{ref: "r"}, &fakeExecutor{}, grants)
	claimer := &fakeClaimer{claimed: true}
	uc.WithExecutionClaim(claimer)
	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if len(claimer.gotEvents) != 1 || len(claimer.gotEvents[0]) != 1 {
		t.Fatalf("want one claim event, got %v", claimer.gotEvents)
	}
	p := decodeRunPayload(t, claimer.gotEvents[0][0])
	if p.Cause != "execute_claim" || p.PreviousStatus != "open" || p.NewStatus != "in_progress" || p.Engine != "direct_agent" || p.WorktreeID != "wt-1" {
		t.Fatalf("bad claim payload: %+v", p)
	}
}

func TestExecuteTask_PlainTask_NoOutboxRows(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedRequestTask(t, tasks, grants, false)
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, &fakeSimpleExecutor{ref: "r"}, &fakeExecutor{}, grants)
	claimer := &fakeClaimer{claimed: true}
	uc.WithExecutionClaim(claimer).WithRunEvents(newLeases(tasks), nil)
	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if len(claimer.gotEvents[0]) != 0 || len(tasks.execEvents) != 0 {
		t.Fatalf("plain task wrote events: claim=%v exec=%v", claimer.gotEvents[0], tasks.execEvents)
	}
}

func TestExecuteTask_RequestTask_DirectAgentSuccess_EmitsCompleted(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedRequestTask(t, tasks, grants, true)
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, &fakeSimpleExecutor{ref: "r"}, &fakeExecutor{}, grants)
	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if len(tasks.execEvents) != 1 {
		t.Fatalf("want one completion event, got %d", len(tasks.execEvents))
	}
	p := decodeRunPayload(t, tasks.execEvents[0])
	if p.Cause != "execution_completed" || p.NewStatus != "review" || p.ExecutionLinkID == "" {
		t.Fatalf("bad payload: %+v", p)
	}
}

func TestExecuteTask_RequestTask_DispatchFailure_EmitsExecutionFailedWithError(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedRequestTask(t, tasks, grants, true)
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{{FromTaskID: "task-1", ToTaskID: "sub", Kind: domain.EdgeKindParentChild}}}
	uc, _, _, _, _ := newExecutableExecuteTask(tasks, edges, &fakeSimpleExecutor{}, &fakeExecutor{err: errors.New("orchestration down")}, grants)
	leases := newLeases(tasks)
	uc.WithRunEvents(leases, nil)
	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "task-1", RequestID: "r"}); err == nil {
		t.Fatal("expected dispatch error")
	}
	if len(tasks.execEvents) != 1 {
		t.Fatalf("want one failure event, got %d", len(tasks.execEvents))
	}
	p := decodeRunPayload(t, tasks.execEvents[0])
	if p.Cause != "execution_failed" || p.NewStatus != "open" || !strings.Contains(p.ErrorMessage, "orchestration down") || p.Engine != "orchestration" {
		t.Fatalf("bad payload: %+v", p)
	}
	if tasks.tasks["task-1"].Status != domain.StatusOpen {
		t.Errorf("status not reverted: %s", tasks.tasks["task-1"].Status)
	}
}

func reportEnv(t *testing.T, withRequest bool) (*fakeTaskRepository, *fakeExecutionLinkRepository, *ReportTaskExecutionResult) {
	t.Helper()
	tasks := newFakeTaskRepository()
	task, _ := domain.NewTask("task-1", "tenant-1", "Task", domain.StatusInProgress, "", "proj-1")
	if withRequest {
		task.RequestID = "req-1"
	}
	tasks.tasks["task-1"] = task
	links := &fakeExecutionLinkRepository{}
	seedTaskWithActiveLink(tasks, links, "task-1", domain.EngineOrchestration, "run-1")
	return tasks, links, NewReportTaskExecutionResult(tasks, links).WithExecutionRelease(newLeases(tasks))
}

func TestReportExecutionResult_Success_EmitsCompleted(t *testing.T) {
	tasks, _, uc := reportEnv(t, true)
	if err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "run-1", Engine: "orchestration", Success: true}); err != nil {
		t.Fatal(err)
	}
	if len(tasks.execEvents) != 1 || decodeRunPayload(t, tasks.execEvents[0]).Cause != "execution_completed" {
		t.Fatalf("events: %v", tasks.execEvents)
	}
}

func TestReportExecutionResult_Failure_EmitsFailedWithMessage(t *testing.T) {
	tasks, _, uc := reportEnv(t, true)
	if err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "run-1", Engine: "orchestration", ErrorMessage: "boom"}); err != nil {
		t.Fatal(err)
	}
	if len(tasks.execEvents) != 1 {
		t.Fatalf("events: %v", tasks.execEvents)
	}
	p := decodeRunPayload(t, tasks.execEvents[0])
	if p.Cause != "execution_failed" || p.ErrorMessage != "boom" || p.ExecutionLinkID != "link-1" {
		t.Fatalf("bad payload: %+v", p)
	}
}

func TestReportExecutionResult_StaleCallback_NoEvent(t *testing.T) {
	tasks, _, uc := reportEnv(t, true)
	if err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), ReportTaskExecutionResultInput{TaskID: "task-1", ExecutionRef: "old-run", Engine: "orchestration", Success: true}); err != nil {
		t.Fatal(err)
	}
	if len(tasks.execEvents) != 0 {
		t.Fatalf("stale callback emitted %v", tasks.execEvents)
	}
}

func TestRecovery_Release_EmitsRecoveryCause(t *testing.T) {
	tasks := newFakeTaskRepository()
	inProgressTask(t, tasks, "task-1", "link-1")
	task := tasks.tasks["task-1"]
	task.RequestID = "req-1"
	tasks.tasks["task-1"] = task
	leases := newLeases(tasks)
	leases.expired = []domain.ExpiredRun{{TenantID: "tenant-1", LinkID: "link-1", TaskID: "task-1", PreviousStatus: "open"}}
	uc := NewRecoverInterruptedExecutions(leases).WithRunEvents(tasks, nil)
	if _, err := uc.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(tasks.execEvents) != 1 {
		t.Fatalf("events: %v", tasks.execEvents)
	}
	p := decodeRunPayload(t, tasks.execEvents[0])
	if p.Cause != "recovery" || p.NewStatus != "open" || p.ExecutionLinkID != "link-1" {
		t.Fatalf("bad payload: %+v", p)
	}
}
