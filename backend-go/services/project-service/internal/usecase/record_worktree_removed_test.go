package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRecordWorktreeRemoved_DeletesWorktree(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main", Active: true}
	uc := NewRecordWorktreeRemoved(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, RecordWorktreeRemovedInput{WorktreeID: "w1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.worktrees["w1"]; ok {
		t.Error("expected worktree to be hard-deleted")
	}
}

// TestRecordWorktreeRemoved_AlwaysPublishesHadOpenPrFalse asserts
// worktree.deleted's had_open_pr is always published false — resolving the
// real value needs a live scm-integration-service call the consumer makes
// at processing time, never this transaction (record_worktree_removed.go's
// doc comment).
func TestRecordWorktreeRemoved_AlwaysPublishesHadOpenPrFalse(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{
		ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main", Active: true,
		LinkedIssueProvider: "github", LinkedIssueRef: "owner/repo#42",
	}
	uc := NewRecordWorktreeRemoved(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, RecordWorktreeRemovedInput{WorktreeID: "w1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.enqueuedEvents) != 1 {
		t.Fatalf("expected exactly one enqueued event, got %d", len(repo.enqueuedEvents))
	}
	event := repo.enqueuedEvents[0]
	if event.Subject != subjectWorktreeDeleted {
		t.Errorf("expected subject %q, got %q", subjectWorktreeDeleted, event.Subject)
	}
	var payload worktreeLifecycleEventPayload
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload.HadOpenPr {
		t.Error("expected had_open_pr to always be published false")
	}
	if payload.LinkedIssueRef != "owner/repo#42" {
		t.Errorf("expected linked_issue_ref to carry the removed worktree's link, got %q", payload.LinkedIssueRef)
	}
}

func TestRecordWorktreeRemoved_NotFound(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeRemoved(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, RecordWorktreeRemovedInput{WorktreeID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND")
}

func TestRecordWorktreeRemoved_RequiresWorktreeID(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeRemoved(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, RecordWorktreeRemovedInput{WorktreeID: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED")
}
