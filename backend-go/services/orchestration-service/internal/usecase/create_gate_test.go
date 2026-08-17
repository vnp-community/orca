package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestCreateGate_RequiresTenantContext(t *testing.T) {
	uc := NewCreateGate(newFakeGateRepository(), &synchronousSerializer{})
	_, err := uc.Execute(context.Background(), CreateGateInput{DispatchContextID: "dc-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateGate_RequiresDispatchContextID(t *testing.T) {
	uc := NewCreateGate(newFakeGateRepository(), &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateGateInput{})
	if err == nil {
		t.Fatal("expected an error for empty dispatch_context_id")
	}
}

func TestCreateGate_CreatesAndKeysSerializerByDispatchContextID(t *testing.T) {
	repo := newFakeGateRepository()
	ser := &synchronousSerializer{}
	uc := NewCreateGate(repo, ser)

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
	uc := NewCreateGate(repo, &synchronousSerializer{})

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
	uc := NewCreateGate(repo, &synchronousSerializer{})

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateGateInput{DispatchContextID: "dc-adhoc"})
	if !errors.Is(err, ErrDispatchContextHasNoTask) {
		t.Fatalf("expected wrapped ErrDispatchContextHasNoTask, got %v", err)
	}
}
