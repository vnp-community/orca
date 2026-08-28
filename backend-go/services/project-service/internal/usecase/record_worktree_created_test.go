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

// TestRecordWorktreeCreated_WritesOutboxEventInSameTransaction (SOL-WT-01):
// asserts the event built by Execute (via WorktreeRepository.
// CreateWorktreeWithEvent) has the expected subject and a non-empty id, and
// that its payload round-trips to the created worktree's id/project id —
// see worktreeLifecycleEventPayload's doc comment for why repo_id/path/
// branch aren't part of this payload (it mirrors
// projectv1.WorktreeLifecycleEvent's wire fields, not the full Worktree row).
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
	if len(repo.enqueuedEvents) != 1 {
		t.Fatalf("expected exactly one enqueued event, got %d", len(repo.enqueuedEvents))
	}
	event := repo.enqueuedEvents[0]

	if event.Subject != "orca.project.worktree.created" {
		t.Fatalf("expected subject orca.project.worktree.created, got %q", event.Subject)
	}
	if event.ID == "" {
		t.Error("expected a non-empty event id")
	}

	var payload worktreeLifecycleEventPayload
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.WorktreeID != got.ID || payload.ProjectID != "p1" {
		t.Errorf("expected payload to round-trip the created worktree's id/project id, got %+v", payload)
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
