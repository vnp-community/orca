package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func mustTask(t *testing.T, id, coordinatorRunID string, deps []string) domain.OrchestrationTask {
	t.Helper()
	task, err := domain.NewOrchestrationTask(id, "tenant-1", coordinatorRunID, "", "", "title-"+id, nil, deps)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	return task
}

// newTestUpdateTaskStatusAndPromote wires repo.runs to a fresh
// fakeCoordinatorRunRepository so the in-memory UpdateStatusAndPromote can
// detect run finalization exactly like the real Postgres tail
// (TASK-TASKV1-005-07), and returns the fakes so tests can assert on the
// reporter/runs side effects.
func newTestUpdateTaskStatusAndPromote(repo *fakeOrchestrationTaskRepository, ser HandleSerializer) (*UpdateTaskStatusAndPromote, *fakeTaskServiceReporter, *fakeCoordinatorRunRepository) {
	reporter := &fakeTaskServiceReporter{}
	runs := newFakeCoordinatorRunRepository()
	repo.runs = runs
	return NewUpdateTaskStatusAndPromote(repo, ser, reporter, runs), reporter, runs
}

func TestUpdateTaskStatusAndPromote_RequiresTenantContext(t *testing.T) {
	uc, _, _ := newTestUpdateTaskStatusAndPromote(newFakeOrchestrationTaskRepository(mustTask(t, "t1", "run-1", nil)), &synchronousSerializer{})
	_, err := uc.Execute(context.Background(), UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "completed"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestUpdateTaskStatusAndPromote_RejectsInvalidStatus(t *testing.T) {
	uc, _, _ := newTestUpdateTaskStatusAndPromote(newFakeOrchestrationTaskRepository(mustTask(t, "t1", "run-1", nil)), &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an invalid status")
	}
}

// TestUpdateTaskStatusAndPromote_PromotesReadySiblings is the core
// business-logic test for the atomic promote saga (§8): completing a task
// must promote a sibling whose only dependency was that task, in the same
// call.
func TestUpdateTaskStatusAndPromote_PromotesReadySiblings(t *testing.T) {
	root := mustTask(t, "t1", "run-1", nil)
	dependent := mustTask(t, "t2", "run-1", []string{"t1"})
	stillBlocked := mustTask(t, "t3", "run-1", []string{"t1", "t4"})

	repo := newFakeOrchestrationTaskRepository(root, dependent, stillBlocked)
	ser := &synchronousSerializer{}
	uc, reporter, _ := newTestUpdateTaskStatusAndPromote(repo, ser)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "completed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Task.Status != domain.TaskStatusCompleted {
		t.Errorf("expected t1 status completed, got %s", out.Task.Status)
	}
	if len(out.PromotedTaskIDs) != 1 || out.PromotedTaskIDs[0] != "t2" {
		t.Fatalf("expected only t2 promoted, got %v", out.PromotedTaskIDs)
	}
	if keys := ser.calledKeys(); len(keys) != 1 || keys[0] != "t1" {
		t.Errorf("expected serializer keyed by orchestration_task_id t1, got %v", keys)
	}

	t2, _ := repo.Get(ctx, "tenant-1", "t2")
	if t2.Status != domain.TaskStatusReady {
		t.Errorf("expected t2 promoted to ready, got %s", t2.Status)
	}
	t3, _ := repo.Get(ctx, "tenant-1", "t3")
	if t3.Status != domain.TaskStatusPending {
		t.Errorf("expected t3 to remain pending (dep t4 not completed), got %s", t3.Status)
	}
	// Siblings remain non-terminal (t2 ready, t3 pending) — the run must
	// not be finalized yet.
	if reporter.callCount() != 0 {
		t.Errorf("expected no ReportResult call while siblings remain non-terminal, got %d", reporter.callCount())
	}
}

func TestUpdateTaskStatusAndPromote_TaskNotFound(t *testing.T) {
	uc, _, _ := newTestUpdateTaskStatusAndPromote(newFakeOrchestrationTaskRepository(), &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "missing", NewStatus: "completed"})
	if err == nil {
		t.Fatal("expected an error for a missing task")
	}
}

// TestUpdateTaskStatusAndPromote_FinalizesRunWhenLastTaskCompletes is the
// core proof of TASK-TASKV1-005-07: completing the LAST non-terminal task
// in a run finalizes it and reports success exactly once.
func TestUpdateTaskStatusAndPromote_FinalizesRunWhenLastTaskCompletes(t *testing.T) {
	only := mustTask(t, "t1", "run-1", nil)
	repo := newFakeOrchestrationTaskRepository(only)
	uc, reporter, runs := newTestUpdateTaskStatusAndPromote(repo, &synchronousSerializer{})
	runs.runs["run-1"] = mustRun(t, "run-1", "tenant-1", domain.RunStatusRunning)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "completed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reporter.callCount() != 1 {
		t.Fatalf("expected ReportResult called exactly once, got %d", reporter.callCount())
	}
	if !reporter.calls[0].success {
		t.Errorf("expected success=true, got false")
	}
	if len(runs.markReportedCalls) != 1 || runs.markReportedCalls[0] != "run-1" {
		t.Errorf("expected MarkReported called once for run-1, got %v", runs.markReportedCalls)
	}
}

// TestUpdateTaskStatusAndPromote_FailedSiblingFailsWholeRun proves the
// fail-closed rule: one task ending failed and all others completed
// finalizes the run as failed, not completed.
func TestUpdateTaskStatusAndPromote_FailedSiblingFailsWholeRun(t *testing.T) {
	a := mustTask(t, "t1", "run-1", nil)
	b := mustTask(t, "t2", "run-1", nil)
	b.Status = domain.TaskStatusCompleted

	repo := newFakeOrchestrationTaskRepository(a, b)
	uc, reporter, runs := newTestUpdateTaskStatusAndPromote(repo, &synchronousSerializer{})
	runs.runs["run-1"] = mustRun(t, "run-1", "tenant-1", domain.RunStatusRunning)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "failed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reporter.callCount() != 1 {
		t.Fatalf("expected ReportResult called exactly once, got %d", reporter.callCount())
	}
	if reporter.calls[0].success {
		t.Errorf("expected success=false when any sibling failed, got true")
	}
}

// TestUpdateTaskStatusAndPromote_ReportResultErrorIsSwallowed proves a
// reporter failure doesn't fail the RPC and doesn't call MarkReported —
// left for the tick loop's retry pass instead.
func TestUpdateTaskStatusAndPromote_ReportResultErrorIsSwallowed(t *testing.T) {
	only := mustTask(t, "t1", "run-1", nil)
	repo := newFakeOrchestrationTaskRepository(only)
	uc, reporter, runs := newTestUpdateTaskStatusAndPromote(repo, &synchronousSerializer{})
	runs.runs["run-1"] = mustRun(t, "run-1", "tenant-1", domain.RunStatusRunning)
	reporter.err = errors.New("boom")

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "completed"})
	if err != nil {
		t.Fatalf("expected Execute to still succeed for the task-status write, got: %v", err)
	}
	if out.Task.Status != domain.TaskStatusCompleted {
		t.Errorf("expected task status write to have committed, got %s", out.Task.Status)
	}
	if len(runs.markReportedCalls) != 0 {
		t.Errorf("expected MarkReported NOT called when ReportResult fails, got %v", runs.markReportedCalls)
	}
}

// TestUpdateTaskStatusAndPromote_EnqueuesOutboxEvent proves BE-SOL-003/
// TASK-FT-003-01: a status transition enqueues exactly one
// orca.orchestration.task.statuschanged outbox row in the same fake
// transaction as the status write, carrying the task's OriginTaskID.
func TestUpdateTaskStatusAndPromote_EnqueuesOutboxEvent(t *testing.T) {
	task, err := domain.NewOrchestrationTask("t1", "tenant-1", "run-1", "", "origin-1", "title-t1", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	repo := newFakeOrchestrationTaskRepository(task)
	uc, _, _ := newTestUpdateTaskStatusAndPromote(repo, &synchronousSerializer{})

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "dispatched"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.enqueuedEvents) != 1 {
		t.Fatalf("expected exactly 1 enqueued outbox event, got %d", len(repo.enqueuedEvents))
	}
	ev := repo.enqueuedEvents[0]
	if ev.Subject != "orca.orchestration.task.statuschanged" {
		t.Errorf("expected subject orca.orchestration.task.statuschanged, got %q", ev.Subject)
	}
	var payload taskStatusChangedPayload
	if err := json.Unmarshal(ev.PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.OriginTaskID != "origin-1" {
		t.Errorf("expected origin_task_id origin-1, got %q", payload.OriginTaskID)
	}
	if payload.NewStatus != "dispatched" {
		t.Errorf("expected new_status dispatched, got %q", payload.NewStatus)
	}
}
