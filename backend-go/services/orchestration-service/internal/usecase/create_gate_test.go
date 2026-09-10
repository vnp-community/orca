package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func TestCreateGate_RequiresTenantContext(t *testing.T) {
	uc := NewCreateGate(newFakeGateRepository(), &synchronousSerializer{}, nil, nil)
	_, err := uc.Execute(context.Background(), CreateGateInput{DispatchContextID: "dc-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateGate_RequiresDispatchContextID(t *testing.T) {
	uc := NewCreateGate(newFakeGateRepository(), &synchronousSerializer{}, nil, nil)
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateGateInput{})
	if err == nil {
		t.Fatal("expected an error for empty dispatch_context_id")
	}
}

func TestCreateGate_CreatesAndKeysSerializerByDispatchContextID(t *testing.T) {
	repo := newFakeGateRepository()
	ser := &synchronousSerializer{}
	uc := NewCreateGate(repo, ser, nil, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, CreateGateInput{DispatchContextID: "dc-1", Question: "proceed?", Options: []string{"yes", "no"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DispatchContextID != "dc-1" || got.Question != "proceed?" {
		t.Errorf("unexpected result: %+v", got)
	}
	if keys := ser.calledKeys(); len(keys) != 1 || keys[0] != "dc-1" {
		t.Errorf("expected serializer keyed by dc-1, got %v", keys)
	}
}

func TestCreateGate_DispatchContextNotFoundPropagates(t *testing.T) {
	repo := newFakeGateRepository()
	repo.err = ErrDispatchContextNotFound
	uc := NewCreateGate(repo, &synchronousSerializer{}, nil, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateGateInput{DispatchContextID: "missing"})
	if !errors.Is(err, ErrDispatchContextNotFound) {
		t.Fatalf("expected wrapped ErrDispatchContextNotFound, got %v", err)
	}
}

// TestCreateGate_DispatchContextHasNoTaskPropagates keeps proving the
// original failure mode (Epic C, docs/execution-plan.md) still correctly
// surfaces as ErrDispatchContextHasNoTask (-> ORCH_DISPATCH_CONTEXT_NO_TASK /
// FailedPrecondition at the gRPC boundary) for a dispatch context that
// genuinely has no owning task — that's the real invariant, not a bug, and
// must keep working once CreateDispatchContextRequest can supply a task id
// for the cases that do have one.
func TestCreateGate_DispatchContextHasNoTaskPropagates(t *testing.T) {
	repo := newFakeGateRepository()
	repo.err = ErrDispatchContextHasNoTask
	uc := NewCreateGate(repo, &synchronousSerializer{}, nil, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateGateInput{DispatchContextID: "dc-adhoc"})
	if !errors.Is(err, ErrDispatchContextHasNoTask) {
		t.Fatalf("expected wrapped ErrDispatchContextHasNoTask, got %v", err)
	}
}

// TestCreateGate_EnqueuesOutboxEventAndOriginTaskID proves BE-SOL-003/
// TASK-FT-003-02: creating a gate enqueues exactly one
// orca.orchestration.decision_gate.opened outbox event, and (per
// TASK-FT-003-04's flagged gap) that event's payload carries the resolved
// origin_task_id when the dispatchContexts/tasks lookups are wired.
func TestCreateGate_EnqueuesOutboxEventAndOriginTaskID(t *testing.T) {
	gateRepo := newFakeGateRepository()
	dcRepo := &fakeDispatchContextRepository{
		byID: map[string]domain.DispatchContext{
			"dc-1": {ID: "dc-1", OrchestrationTaskID: "task-1"},
		},
	}
	taskRepo := newFakeOrchestrationTaskRepository(mustTask(t, "task-1", "run-1", nil))
	origTask := taskRepo.tasks["task-1"]
	origTask.OriginTaskID = "origin-task-1"
	taskRepo.tasks["task-1"] = origTask

	uc := NewCreateGate(gateRepo, &synchronousSerializer{}, dcRepo, taskRepo)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, CreateGateInput{DispatchContextID: "dc-1", Question: "proceed?", Options: []string{"yes", "no"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gateRepo.enqueuedEvents) != 1 {
		t.Fatalf("expected exactly 1 enqueued outbox event, got %d", len(gateRepo.enqueuedEvents))
	}
	ev := gateRepo.enqueuedEvents[0]
	if ev.Subject != "orca.orchestration.decision_gate.opened" {
		t.Errorf("expected subject orca.orchestration.decision_gate.opened, got %q", ev.Subject)
	}
	var payload decisionGateOpenedPayload
	if err := json.Unmarshal(ev.PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.OriginTaskID != "origin-task-1" {
		t.Errorf("expected origin_task_id origin-task-1, got %q", payload.OriginTaskID)
	}
}
