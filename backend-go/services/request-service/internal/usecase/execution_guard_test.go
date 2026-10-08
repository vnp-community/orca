package usecase

import (
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestExecutionGuard_ActiveTask_True(t *testing.T) {
	f := newExTasks()
	f.add(domain.TaskView{Type: "task", Status: domain.TaskStatusInProgress, RequestID: "r1"})
	f.add(domain.TaskView{Type: "task", Status: domain.TaskStatusOpen, RequestID: "r1"})
	active, err := (&TaskExecutionGuard{Tasks: f}).HasActiveExecution(lcCtx(), "r1")
	if err != nil || !active {
		t.Fatalf("got %v %v", active, err)
	}
}

func TestExecutionGuard_NoActive_False(t *testing.T) {
	f := newExTasks()
	f.add(domain.TaskView{Type: "task", Status: domain.TaskStatusReview, RequestID: "r1"})
	f.add(domain.TaskView{Type: "task", Status: domain.TaskStatusInProgress, RequestID: "r2"})
	f.add(domain.TaskView{Type: domain.TaskTypePhase, Status: domain.TaskStatusInProgress, RequestID: "r1"})
	active, err := (&TaskExecutionGuard{Tasks: f}).HasActiveExecution(lcCtx(), "r1")
	if err != nil || active {
		t.Fatalf("only a working task of this request counts: %v %v", active, err)
	}
}

func TestExecutionGuard_TaskServiceDown_ReturnsError(t *testing.T) {
	f := newExTasks()
	f.listErr = domain.ErrTaskServiceDown
	active, err := (&TaskExecutionGuard{Tasks: f}).HasActiveExecution(lcCtx(), "r1")
	if err == nil || active || !errorHasCode(err, "REQUEST_EXECUTION_TASK_SERVICE_UNAVAILABLE") || !errors.Is(err, domain.ErrTaskServiceDown) {
		t.Fatalf("fail closed: an unknown state is an error, never 'no active task': %v %v", active, err)
	}
}

func TestExecutionGuard_BlocksReturnWhileATaskRuns(t *testing.T) {
	r := newExRig(t)
	req, _, _ := runningTask(t, r, domain.RequestTypeTask)
	guard := &TaskExecutionGuard{Tasks: r.tasks}
	returner := NewReturnRequestToBacklog(r.store, newLcTransition(r.store), lcHistory{r.store}, &lcCanceller{}, guard, r.store, r.store)
	_, err := returner.Execute(lcCtx(), ReturnInput{RequestID: req.ID, Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryOther, Reason: "stop", ActorID: "u"})
	if !errorHasCode(err, "REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION") {
		t.Fatalf("got %v", err)
	}
}
