package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestCreateTask_RequiresTenantContext(t *testing.T) {
	uc := NewCreateTask(newFakeTaskRepository(), &fakeGrantRepository{})
	_, err := uc.Execute(context.Background(), CreateTaskInput{ID: "t1", Title: "Title"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateTask_CreatesARootTask(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo, &fakeGrantRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, CreateTaskInput{ID: "t1", Title: "Title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" || got.Status != domain.StatusOpen {
		t.Errorf("unexpected task: %+v", got)
	}
}

// TestCreateTask_SetsOwnerIDToCreatingUser is TASK-TG-03-01's bootstrap
// fix: without OwnerID set at creation, a brand-new task's creator would be
// locked out of Grant's new manage-access check (no grant rows exist yet).
func TestCreateTask_SetsOwnerIDToCreatingUser(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo, &fakeGrantRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, CreateTaskInput{ID: "t1", Title: "Title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OwnerID != "user-1" {
		t.Errorf("expected OwnerID=user-1, got %q", got.OwnerID)
	}
}

func TestCreateTask_RejectsAMissingParent(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo, &fakeGrantRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateTaskInput{ID: "t2", Title: "Title", ParentID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for a nonexistent parent")
	}
}

func TestCreateTask_AllowsAnExistingParent(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo, &fakeGrantRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, CreateTaskInput{ID: "parent", Title: "Parent"}); err != nil {
		t.Fatalf("unexpected error creating parent: %v", err)
	}
	child, err := uc.Execute(ctx, CreateTaskInput{ID: "child", Title: "Child", ParentID: "parent"})
	if err != nil {
		t.Fatalf("unexpected error creating child: %v", err)
	}
	if child.ParentID != "parent" {
		t.Errorf("expected ParentID=parent, got %q", child.ParentID)
	}
}

// TestCreateTask_CreatorID_InsertsOwnerGrant is TASK-TG-003-02's core
// regression test: a caller-supplied CreatorID mints a real
// GrantLevelOwner Grant row — not just Task.OwnerID (which this task's
// Context section explicitly says is informational-only and never read by
// ResolveGrant).
func TestCreateTask_CreatorID_InsertsOwnerGrant(t *testing.T) {
	repo := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	uc := NewCreateTask(repo, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	created, err := uc.Execute(ctx, CreateTaskInput{ID: "t1", Title: "Title", CreatorID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(grants.grants) != 1 {
		t.Fatalf("expected exactly 1 grant inserted, got %d: %+v", len(grants.grants), grants.grants)
	}
	g := grants.grants[0]
	if g.TaskID != created.ID || g.SubjectID != "user-1" || g.Level != domain.GrantLevelOwner || !g.ApplyTree {
		t.Errorf("unexpected grant: %+v", g)
	}
}

// TestCreateTask_NoCreatorID_NoGrantInserted proves the grant step is
// skipped entirely when CreatorID is empty — e.g. AIApply's call site,
// which passes a nil GrantRepository specifically because it never sets
// CreatorID either.
func TestCreateTask_NoCreatorID_NoGrantInserted(t *testing.T) {
	repo := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	uc := NewCreateTask(repo, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, CreateTaskInput{ID: "t1", Title: "Title"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(grants.grants) != 0 {
		t.Errorf("expected no grant inserted without a CreatorID, got %+v", grants.grants)
	}
}

// TestCreateTask_NilGrantRepository_SkipsGrantStep proves AIApply's
// NewCreateTask(tasks, nil) call site is safe even if a future change
// accidentally sets CreatorID on that path — Execute must never
// nil-pointer-dereference uc.grants.
func TestCreateTask_NilGrantRepository_SkipsGrantStep(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, CreateTaskInput{ID: "t1", Title: "Title", CreatorID: "user-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCreateTask_CreatorID_ThenResolvePermission_ResolvesOwner is the
// task's own named integration-style test: a task created with CreatorID
// set must immediately resolve GrantLevelOwner for that same caller via
// ResolvePermission end-to-end, not just a special-cased assertion on the
// inserted Grant row's shape.
func TestCreateTask_CreatorID_ThenResolvePermission_ResolvesOwner(t *testing.T) {
	tasks := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	createTask := NewCreateTask(tasks, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "creator-1")

	created, err := createTask.Execute(ctx, CreateTaskInput{ID: "t1", Title: "Title", CreatorID: "creator-1"})
	if err != nil {
		t.Fatalf("unexpected error creating task: %v", err)
	}

	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true}, nil)
	level, err := resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: created.ID, UserID: "creator-1", Action: "admin"})
	if err != nil {
		t.Fatalf("unexpected error resolving permission: %v", err)
	}
	if level != domain.GrantLevelOwner {
		t.Errorf("expected the creator to resolve GrantLevelOwner, got %v", level)
	}
}

// TestCreateTask_GrantInsertFailure_StillReturnsCreatedTask is the
// best-effort regression test: a failed owner-grant insert must not fail
// task creation outright.
func TestCreateTask_GrantInsertFailure_StillReturnsCreatedTask(t *testing.T) {
	repo := newFakeTaskRepository()
	grants := &fakeGrantRepository{grantErr: errors.New("db unavailable")}
	uc := NewCreateTask(repo, grants)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, CreateTaskInput{ID: "t1", Title: "Title", CreatorID: "user-1"})
	if err != nil {
		t.Fatalf("expected task creation to succeed despite the grant-insert failure, got %v", err)
	}
	if got.ID != "t1" {
		t.Errorf("expected the created task to still be returned, got %+v", got)
	}
}
