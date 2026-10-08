package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// fakeTaskSpecRepository mimics the adapters' guarded writes closely enough to test the use cases.
type fakeTaskSpecRepository struct {
	mu        sync.Mutex
	specs     map[string]domain.TaskSpec
	upserts   int
	lookupErr error
}

func newFakeTaskSpecRepository() *fakeTaskSpecRepository {
	return &fakeTaskSpecRepository{specs: map[string]domain.TaskSpec{}}
}

func (f *fakeTaskSpecRepository) Upsert(ctx context.Context, s domain.TaskSpec, expected int64) (domain.TaskSpec, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserts++
	cur, ok := f.specs[s.TaskID]
	switch {
	case !ok && expected != 0:
		return domain.TaskSpec{}, domain.ErrTaskSpecNotFound
	case ok && cur.IsLocked():
		return domain.TaskSpec{}, domain.ErrTaskSpecLocked
	case ok && cur.Version != expected, ok && expected == 0:
		return domain.TaskSpec{}, domain.ErrTaskSpecVersionConflict
	}
	s.Version = expected + 1
	f.specs[s.TaskID] = s
	return s, nil
}

func (f *fakeTaskSpecRepository) GetMany(ctx context.Context, tenantID string, ids []string) ([]domain.TaskSpec, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.TaskSpec
	for _, id := range ids {
		if s, ok := f.specs[id]; ok && s.TenantID == tenantID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeTaskSpecRepository) LockSubtree(ctx context.Context, tenantID string, ids []string, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, id := range ids {
		if s, ok := f.specs[id]; ok && s.TenantID == tenantID && !s.IsLocked() {
			s.LockedAt = &at
			f.specs[id] = s
			n++
		}
	}
	return n, nil
}

func (f *fakeTaskSpecRepository) IsLocked(ctx context.Context, tenantID, id string) (bool, error) {
	if f.lookupErr != nil {
		return false, f.lookupErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.specs[id]
	return ok && s.TenantID == tenantID && s.IsLocked(), nil
}

func (f *fakeTaskSpecRepository) HasSpec(ctx context.Context, tenantID, id string) (bool, error) {
	if f.lookupErr != nil {
		return false, f.lookupErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.specs[id]
	return ok && s.TenantID == tenantID, nil
}

type fakePermission struct {
	deny    bool
	checked []string
}

func (f *fakePermission) Execute(ctx context.Context, in ResolvePermissionInput) (domain.GrantLevel, error) {
	f.checked = append(f.checked, in.Action+":"+in.TaskID)
	if f.deny {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindPermissionDenied, "TASK_NO_GRANT", "denied", nil)
	}
	return domain.GrantLevelOwner, nil
}

func specFixture(t *testing.T) (*fakeTaskRepository, *fakeTaskSpecRepository, context.Context) {
	t.Helper()
	tasks := newFakeTaskRepository()
	for _, id := range []string{"plan", "phase", "t1", "t2"} {
		parent := ""
		switch id {
		case "phase":
			parent = "plan"
		case "t1", "t2":
			parent = "phase"
		}
		task, err := domain.NewTask(id, "tenant-1", "Task "+id, domain.StatusOpen, parent, "proj-1")
		if err != nil {
			t.Fatal(err)
		}
		tasks.tasks[id] = task
	}
	return tasks, newFakeTaskSpecRepository(), withIdentity(context.Background(), "tenant-1", "user-1")
}

func code(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func TestSetTaskSpec_CreateThenUpdate(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	uc := NewSetTaskSpec(tasks, specs, &fakePermission{})
	v1, err := uc.Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"b":1,"a":2}`)})
	if err != nil || v1.Version != 1 || string(v1.Spec) != `{"a":2,"b":1}` {
		t.Fatalf("create: %+v %v", v1, err)
	}
	v2, err := uc.Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"a":3}`), ExpectedVersion: 1})
	if err != nil || v2.Version != 2 {
		t.Fatalf("update: %+v %v", v2, err)
	}
}

func TestSetTaskSpec_IdenticalCreateRetryIsIdempotent(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	uc := NewSetTaskSpec(tasks, specs, &fakePermission{})
	in := SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"a":1}`)}
	if _, err := uc.Execute(ctx, in); err != nil {
		t.Fatal(err)
	}
	again, err := uc.Execute(ctx, in)
	if err != nil || again.Version != 1 || specs.upserts != 1 {
		t.Fatalf("retry must not write again: %+v %v upserts=%d", again, err, specs.upserts)
	}
	in.SpecJSON = []byte(`{"a":2}`)
	if _, err := uc.Execute(ctx, in); code(err) != "TASK_SPEC_VERSION_CONFLICT" {
		t.Fatalf("different spec with expected 0 must conflict, got %v", err)
	}
}

func TestSetTaskSpec_Locked_Rejected(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	set := NewSetTaskSpec(tasks, specs, &fakePermission{})
	_, _ = set.Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"a":1}`)})
	if _, err := NewLockTaskSpecs(tasks, specs, nil).Execute(ctx, "plan"); err != nil {
		t.Fatal(err)
	}
	_, err := set.Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"a":2}`), ExpectedVersion: 1})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_SPEC_LOCKED" || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("got %v", err)
	}
}

func TestSetTaskSpec_VersionConflict(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	set := NewSetTaskSpec(tasks, specs, &fakePermission{})
	_, _ = set.Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"a":1}`)})
	_, err := set.Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"a":2}`), ExpectedVersion: 7})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_SPEC_VERSION_CONFLICT" || ae.Kind != apperrors.KindAborted {
		t.Fatalf("got %v", err)
	}
}

func TestSetTaskSpec_NoPermission(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	_, err := NewSetTaskSpec(tasks, specs, &fakePermission{deny: true}).Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{}`)})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied || specs.upserts != 0 {
		t.Fatalf("got %v, upserts=%d", err, specs.upserts)
	}
}

func TestSetTaskSpec_UnknownTask(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	_, err := NewSetTaskSpec(tasks, specs, &fakePermission{}).Execute(ctx, SetTaskSpecInput{TaskID: "nope", SchemaVersion: 1, SpecJSON: []byte(`{}`)})
	if code(err) != "TASK_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
}

func TestSetTaskSpec_InvalidSpec(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	_, err := NewSetTaskSpec(tasks, specs, &fakePermission{}).Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`[1]`)})
	if code(err) != "TASK_SPEC_INVALID" {
		t.Fatalf("got %v", err)
	}
}

func TestSetTaskSpec_RequiresTenant(t *testing.T) {
	tasks, specs, _ := specFixture(t)
	_, err := NewSetTaskSpec(tasks, specs, nil).Execute(context.Background(), SetTaskSpecInput{TaskID: "t1"})
	if code(err) != "TASK_NO_TENANT" {
		t.Fatalf("got %v", err)
	}
}

func TestGetTaskSpecs_LimitsAndTenant(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	_, _ = NewSetTaskSpec(tasks, specs, nil).Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"a":1}`)})
	get := NewGetTaskSpecs(specs)
	got, err := get.Execute(ctx, []string{"t1", "t2"})
	if err != nil || len(got) != 1 || got[0].TaskID != "t1" {
		t.Fatalf("got %+v %v", got, err)
	}
	other := withIdentity(context.Background(), "tenant-2", "u")
	if got, _ := get.Execute(other, []string{"t1"}); len(got) != 0 {
		t.Fatal("tenant isolation broken")
	}
	ids := make([]string, MaxTaskSpecsPerRead+1)
	if _, err := get.Execute(ctx, ids); code(err) != "TASK_SPEC_TOO_MANY" {
		t.Fatalf("got %v", err)
	}
	if _, err := get.Execute(ctx, nil); code(err) != "TASK_SPEC_TOO_MANY" {
		t.Fatalf("empty list must be rejected, got %v", err)
	}
}

func TestLockTaskSpecs_IdempotentAndCountsOnlyNewlyLocked(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	set := NewSetTaskSpec(tasks, specs, nil)
	for _, id := range []string{"t1", "t2"} {
		if _, err := set.Execute(ctx, SetTaskSpecInput{TaskID: id, SchemaVersion: 1, SpecJSON: []byte(`{"a":1}`)}); err != nil {
			t.Fatal(err)
		}
	}
	lock := NewLockTaskSpecs(tasks, specs, nil)
	if n, err := lock.Execute(ctx, "plan"); err != nil || n != 2 {
		t.Fatalf("first lock: %d %v", n, err)
	}
	if n, err := lock.Execute(ctx, "plan"); err != nil || n != 0 {
		t.Fatalf("second lock must report 0: %d %v", n, err)
	}
	if _, err := lock.Execute(ctx, "missing"); code(err) != "TASK_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateTask_TitleBlockedWhenLocked_StatusAllowed(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	_, _ = NewSetTaskSpec(tasks, specs, nil).Execute(ctx, SetTaskSpecInput{TaskID: "t1", SchemaVersion: 1, SpecJSON: []byte(`{"a":1}`)})
	_, _ = NewLockTaskSpecs(tasks, specs, nil).Execute(ctx, "plan")
	uc := NewUpdateTask(tasks, &fakeEdgeRepository{}).WithTaskSpecLock(specs)

	title := "renamed"
	if _, err := uc.Execute(ctx, UpdateTaskInput{ID: "t1", Title: &title}); code(err) != "TASK_SPEC_LOCKED" {
		t.Fatalf("title change must be blocked, got %v", err)
	}
	same := tasks.tasks["t1"].Title
	if _, err := uc.Execute(ctx, UpdateTaskInput{ID: "t1", Title: &same}); err != nil {
		t.Fatalf("resending the same title must pass: %v", err)
	}
	status := domain.StatusDone
	got, err := uc.Execute(ctx, UpdateTaskInput{ID: "t1", Status: &status})
	if err != nil || got.Status != domain.StatusDone {
		t.Fatalf("status change must stay allowed: %+v %v", got, err)
	}
	labels := []string{"x"}
	if _, err := uc.Execute(ctx, UpdateTaskInput{ID: "t1", Labels: &labels}); err != nil {
		t.Fatalf("labels must stay allowed: %v", err)
	}
	// An unlocked sibling can still be renamed.
	if _, err := uc.Execute(ctx, UpdateTaskInput{ID: "t2", Title: &title}); err != nil {
		t.Fatalf("unlocked task rename failed: %v", err)
	}
}

func TestUpdateTask_NilSpecRepo_NoBehaviourChange(t *testing.T) {
	tasks, _, ctx := specFixture(t)
	title := "renamed"
	if got, err := NewUpdateTask(tasks, &fakeEdgeRepository{}).Execute(ctx, UpdateTaskInput{ID: "t1", Title: &title}); err != nil || got.Title != "renamed" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestUpdateTask_LockLookupErrorFailsClosed(t *testing.T) {
	tasks, specs, ctx := specFixture(t)
	specs.lookupErr = errors.New("db down")
	title := "renamed"
	_, err := NewUpdateTask(tasks, &fakeEdgeRepository{}).WithTaskSpecLock(specs).Execute(ctx, UpdateTaskInput{ID: "t1", Title: &title})
	if code(err) != "TASK_SPEC_LOCK_LOOKUP_FAILED" {
		t.Fatalf("got %v", err)
	}
}

func TestRunInTxWithSpecs_DoesNotAffectRunInTx(t *testing.T) {
	// The old TxRunner shape must keep compiling and working next to the new port.
	tasks, _, ctx := specFixture(t)
	var runner TxRunner = newFakeTxRunner(tasks, &fakeEdgeRepository{})
	called := false
	if err := runner.RunInTx(ctx, func(ctx context.Context, tr TaskRepository, er EdgeRepository) error { called = true; return nil }); err != nil || !called {
		t.Fatal("RunInTx broken")
	}
	var _ SpecTxRunner // new port is separate
}
