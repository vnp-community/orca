package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func mustReadyTask(t *testing.T, id, tenantID, coordinatorRunID string, spec []byte) domain.OrchestrationTask {
	t.Helper()
	task, err := domain.NewOrchestrationTask(id, tenantID, coordinatorRunID, "", "", "title-"+id, spec, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task.Status = domain.TaskStatusReady
	return task
}

func newTestTickDispatch(taskRepo *fakeOrchestrationTaskRepository, runRepo *fakeCoordinatorRunRepository, dcRepo *fakeDispatchContextRepository, worker *fakeWorkerDispatcher, reporter *fakeTaskServiceReporter) *TickDispatch {
	ser := &synchronousSerializer{}
	dispatch := NewCreateDispatchContext(dcRepo, ser, nil)
	fail := NewFailDispatch(dcRepo)
	return NewTickDispatch(taskRepo, runRepo, dispatch, worker, fail, reporter)
}

func TestTickDispatch_ClaimsAndDispatchesReadyTask(t *testing.T) {
	task := mustReadyTask(t, "t1", "tenant-1", "run-1", []byte(`{"connectionId":"c1"}`))
	taskRepo := newFakeOrchestrationTaskRepository(task)
	runRepo := newFakeCoordinatorRunRepository()
	taskRepo.runs = runRepo
	dcRepo := &fakeDispatchContextRepository{}
	worker := &fakeWorkerDispatcher{}
	reporter := &fakeTaskServiceReporter{}

	uc := newTestTickDispatch(taskRepo, runRepo, dcRepo, worker, reporter)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(dcRepo.created) != 1 {
		t.Fatalf("expected exactly 1 dispatch context created, got %d", len(dcRepo.created))
	}
	if got := worker.dispatchedIDs(); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("expected exactly 1 dispatch for t1, got %v", got)
	}
}

// TestTickDispatch_RacingClaimResultsInExactlyOneDispatch simulates two
// tick cycles observing the same ready task — the first claims and
// dispatches it, the second (like a concurrent/racing tick instance) finds
// nothing left to claim, per ClaimReady's CAS contract.
func TestTickDispatch_RacingClaimResultsInExactlyOneDispatch(t *testing.T) {
	task := mustReadyTask(t, "t1", "tenant-1", "run-1", []byte(`{"connectionId":"c1"}`))
	taskRepo := newFakeOrchestrationTaskRepository(task)
	runRepo := newFakeCoordinatorRunRepository()
	taskRepo.runs = runRepo
	dcRepo := &fakeDispatchContextRepository{}
	worker := &fakeWorkerDispatcher{}
	reporter := &fakeTaskServiceReporter{}

	uc := newTestTickDispatch(taskRepo, runRepo, dcRepo, worker, reporter)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("first Execute: unexpected error: %v", err)
	}
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("second Execute (racing tick): unexpected error: %v", err)
	}

	if got := worker.dispatchedIDs(); len(got) != 1 {
		t.Fatalf("expected exactly 1 dispatch across both tick cycles, got %v", got)
	}
}

func TestTickDispatch_WorkerDispatchErrorRoutesToFailDispatch(t *testing.T) {
	task := mustReadyTask(t, "t1", "tenant-1", "run-1", []byte(`{"connectionId":"c1"}`))
	taskRepo := newFakeOrchestrationTaskRepository(task)
	runRepo := newFakeCoordinatorRunRepository()
	taskRepo.runs = runRepo
	dcRepo := &fakeDispatchContextRepository{}
	worker := &fakeWorkerDispatcher{err: errors.New("dispatch boom")}
	reporter := &fakeTaskServiceReporter{}

	uc := newTestTickDispatch(taskRepo, runRepo, dcRepo, worker, reporter)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(dcRepo.created) != 1 {
		t.Fatalf("expected the dispatch context to still be created, got %d", len(dcRepo.created))
	}
	dc := dcRepo.created[0]
	updated, ok := dcRepo.byID[dc.ID]
	if !ok {
		t.Fatalf("expected FailDispatch to have recorded a failure for dispatch context %q", dc.ID)
	}
	if updated.FailureCount != 1 {
		t.Errorf("expected failure_count=1, got %d", updated.FailureCount)
	}
}

func TestTickDispatch_ListReadyUnclaimedError_SurfacesAndSkipsRetry(t *testing.T) {
	taskRepo := newFakeOrchestrationTaskRepository()
	taskRepo.err = errors.New("list boom")
	runRepo := newFakeCoordinatorRunRepository(mustRunNoT("run-1", "tenant-1", domain.RunStatusCompleted))
	// runRepo has an unreported terminal run — if retryUnreported were
	// (incorrectly) attempted despite the list error, this would call
	// ReportResult; assert it does NOT.
	dcRepo := &fakeDispatchContextRepository{}
	worker := &fakeWorkerDispatcher{}
	reporter := &fakeTaskServiceReporter{}

	uc := newTestTickDispatch(taskRepo, runRepo, dcRepo, worker, reporter)
	err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error from Execute when ListReadyUnclaimed fails")
	}
	if len(dcRepo.created) != 0 {
		t.Errorf("expected no dispatch contexts created, got %d", len(dcRepo.created))
	}
	if reporter.callCount() != 0 {
		t.Errorf("expected retryUnreported NOT attempted when dispatchReady fails, got %d ReportResult calls", reporter.callCount())
	}
}

func TestTickDispatch_RetryUnreported_CompletedRunReportsSuccess(t *testing.T) {
	taskRepo := newFakeOrchestrationTaskRepository()
	run := mustRunNoT("run-1", "tenant-1", domain.RunStatusCompleted)
	run.OriginTaskID = "origin-1"
	runRepo := newFakeCoordinatorRunRepository(run)
	taskRepo.runs = runRepo
	reporter := &fakeTaskServiceReporter{}

	uc := newTestTickDispatch(taskRepo, runRepo, &fakeDispatchContextRepository{}, &fakeWorkerDispatcher{}, reporter)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reporter.callCount() != 1 {
		t.Fatalf("expected ReportResult called once, got %d", reporter.callCount())
	}
	if !reporter.calls[0].success || reporter.calls[0].taskID != "origin-1" {
		t.Errorf("unexpected call: %+v", reporter.calls[0])
	}
	if len(runRepo.markReportedCalls) != 1 {
		t.Errorf("expected MarkReported called once, got %d", len(runRepo.markReportedCalls))
	}
}

func TestTickDispatch_RetryUnreported_FailedRunReportsFailureWithMessage(t *testing.T) {
	taskRepo := newFakeOrchestrationTaskRepository()
	run := mustRunNoT("run-1", "tenant-1", domain.RunStatusFailed)
	run.OriginTaskID = "origin-1"
	run.ErrorMessage = "agent crashed"
	runRepo := newFakeCoordinatorRunRepository(run)
	taskRepo.runs = runRepo
	reporter := &fakeTaskServiceReporter{}

	uc := newTestTickDispatch(taskRepo, runRepo, &fakeDispatchContextRepository{}, &fakeWorkerDispatcher{}, reporter)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reporter.callCount() != 1 {
		t.Fatalf("expected ReportResult called once, got %d", reporter.callCount())
	}
	if reporter.calls[0].success || reporter.calls[0].errMsg != "agent crashed" {
		t.Errorf("unexpected call: %+v", reporter.calls[0])
	}
}

func TestTickDispatch_RetryUnreported_ReportFailureSkipsMarkReportedButDoesNotFailExecute(t *testing.T) {
	taskRepo := newFakeOrchestrationTaskRepository()
	run := mustRunNoT("run-1", "tenant-1", domain.RunStatusCompleted)
	runRepo := newFakeCoordinatorRunRepository(run)
	taskRepo.runs = runRepo
	reporter := &fakeTaskServiceReporter{err: errors.New("report boom")}

	uc := newTestTickDispatch(taskRepo, runRepo, &fakeDispatchContextRepository{}, &fakeWorkerDispatcher{}, reporter)
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("expected Execute to succeed overall (best-effort retry), got: %v", err)
	}
	if len(runRepo.markReportedCalls) != 0 {
		t.Errorf("expected MarkReported NOT called when ReportResult fails, got %v", runRepo.markReportedCalls)
	}
}

// mustRunNoT is mustRun without requiring a *testing.T — tick_dispatch_test
// needs to build runs outside of a t.Fatalf-capable helper call in a couple
// of spots where domain.NewCoordinatorRun cannot fail for these fixed inputs.
func mustRunNoT(id, tenantID string, status domain.RunStatus) domain.CoordinatorRun {
	run, err := domain.NewCoordinatorRun(id, tenantID, "origin-1", "coord-"+id, nil, 0)
	if err != nil {
		panic(err)
	}
	run.Status = status
	return run
}
