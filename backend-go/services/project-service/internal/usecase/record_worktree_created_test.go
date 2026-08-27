package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestRecordWorktreeCreated_PersistsAndStartsActive(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RecordWorktreeCreatedInput{
		ProjectID: "p1", RepoID: "r1", Path: "/srv/worktrees/w1", Branch: "feature/x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Active {
		t.Error("expected a freshly recorded worktree to be active")
	}
	if got.Branch != "feature/x" {
		t.Errorf("expected Branch=feature/x, got %q", got.Branch)
	}
}

func TestRecordWorktreeCreated_RejectsEmptyPath(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RecordWorktreeCreatedInput{ProjectID: "p1", RepoID: "r1", Path: "", Branch: "main"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_WORKTREE_INVALID")
}

// TestRecordWorktreeCreated_WritesOutboxEventInSameTransaction (SOL-WT-01):
// asserts the event built by Execute has the expected subject and that its
// payload round-trips to the created worktree's id/project id/repo id/
// path/branch.
func TestRecordWorktreeCreated_WritesOutboxEventInSameTransaction(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RecordWorktreeCreatedInput{
		ProjectID: "p1", RepoID: "r1", Path: "/srv/worktrees/w1", Branch: "feature/x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.lastOutboxEvent.Subject != "orca.project.worktree.created" {
		t.Fatalf("expected subject orca.project.worktree.created, got %q", repo.lastOutboxEvent.Subject)
	}
	if repo.lastOutboxEvent.ID == "" {
		t.Error("expected a non-empty event id")
	}

	var payload worktreeCreatedPayload
	if err := json.Unmarshal(repo.lastOutboxEvent.PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.WorktreeID != got.ID || payload.ProjectID != "p1" || payload.RepoID != "r1" ||
		payload.Path != "/srv/worktrees/w1" || payload.Branch != "feature/x" {
		t.Errorf("expected payload to round-trip the created worktree's fields, got %+v", payload)
	}
}

func TestRecordWorktreeCreated_RequiresTenantContext(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	_, err := uc.Execute(context.Background(), RecordWorktreeCreatedInput{ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
