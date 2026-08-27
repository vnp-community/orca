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

// TestRecordWorktreeCreated_EnqueuesLifecycleEvent asserts the SOL-PI-03
// outbox payload has the right subject and linked_issue_ref.
func TestRecordWorktreeCreated_EnqueuesLifecycleEvent(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RecordWorktreeCreatedInput{
		ProjectID: "p1", RepoID: "r1", Path: "/srv/worktrees/w1", Branch: "feature/x",
		LinkedIssueProvider: "github", LinkedIssueRef: "owner/repo#42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.enqueuedEvents) != 1 {
		t.Fatalf("expected exactly one enqueued event, got %d", len(repo.enqueuedEvents))
	}
	event := repo.enqueuedEvents[0]
	if event.Subject != subjectWorktreeCreated {
		t.Errorf("expected subject %q, got %q", subjectWorktreeCreated, event.Subject)
	}
	var payload worktreeLifecycleEventPayload
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.WorktreeID != got.ID {
		t.Errorf("expected worktree_id=%q, got %q", got.ID, payload.WorktreeID)
	}
	if payload.LinkedIssueRef != "owner/repo#42" {
		t.Errorf("expected linked_issue_ref=owner/repo#42, got %q", payload.LinkedIssueRef)
	}
}

// TestRecordWorktreeCreated_EnqueuesEmptyLinkWhenNotProvided asserts the
// request-didn't-carry-one case: linked_issue_ref stays empty.
func TestRecordWorktreeCreated_EnqueuesEmptyLinkWhenNotProvided(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, RecordWorktreeCreatedInput{
		ProjectID: "p1", RepoID: "r1", Path: "/srv/worktrees/w1", Branch: "feature/x",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload worktreeLifecycleEventPayload
	if err := json.Unmarshal(repo.enqueuedEvents[0].PayloadJSON, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.LinkedIssueProvider != "" || payload.LinkedIssueRef != "" {
		t.Errorf("expected empty linked-issue fields, got provider=%q ref=%q", payload.LinkedIssueProvider, payload.LinkedIssueRef)
	}
}

func TestRecordWorktreeCreated_RejectsEmptyPath(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RecordWorktreeCreatedInput{ProjectID: "p1", RepoID: "r1", Path: "", Branch: "main"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_WORKTREE_INVALID")
}

func TestRecordWorktreeCreated_RequiresTenantContext(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	_, err := uc.Execute(context.Background(), RecordWorktreeCreatedInput{ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
