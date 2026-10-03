package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func decodeLifecycle(t *testing.T, raw []byte) worktreeLifecycleEventPayload {
	t.Helper()
	var p worktreeLifecycleEventPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return p
}

// issue-status-sync can only act with the credential of whoever did it, so the
// events must say who that was.
func TestRecordWorktreeCreated_PayloadCarriesActorUser(t *testing.T) {
	repo := newFakeWorktreeRepository()
	ctx := tenant.WithUserID(withTenant(context.Background(), "tenant-1"), "user-7")

	if _, err := NewRecordWorktreeCreated(repo).Execute(ctx, RecordWorktreeCreatedInput{
		ProjectID: "p1", RepoID: "r1", Path: "/w1", Branch: "b", LinkedIssueProvider: "jira", LinkedIssueRef: "ENG-1",
	}); err != nil {
		t.Fatal(err)
	}
	if got := decodeLifecycle(t, repo.enqueuedEvents[0].PayloadJSON).ActorUserID; got != "user-7" {
		t.Errorf("want actor user-7, got %q", got)
	}
}

func TestRecordWorktreeRemoved_PayloadCarriesActorUser(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/w1", Branch: "b", Active: true,
		LinkedIssueProvider: "jira", LinkedIssueRef: "ENG-1"}
	ctx := tenant.WithUserID(withTenant(context.Background(), "tenant-1"), "user-7")

	if err := NewRecordWorktreeRemoved(repo).Execute(ctx, RecordWorktreeRemovedInput{WorktreeID: "w1"}); err != nil {
		t.Fatal(err)
	}
	if got := decodeLifecycle(t, repo.enqueuedEvents[0].PayloadJSON).ActorUserID; got != "user-7" {
		t.Errorf("want actor user-7, got %q", got)
	}
}

// Backwards compatible: no user on the request means no actor field at all, the
// same payload shape existing consumers already read.
func TestLifecyclePayload_OmitsActorWhenUnknown(t *testing.T) {
	repo := newFakeWorktreeRepository()
	if _, err := NewRecordWorktreeCreated(repo).Execute(withTenant(context.Background(), "tenant-1"), RecordWorktreeCreatedInput{
		ProjectID: "p1", RepoID: "r1", Path: "/w1", Branch: "b",
	}); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(repo.enqueuedEvents[0].PayloadJSON, &m)
	if _, present := m["actor_user_id"]; present {
		t.Errorf("actor_user_id must be omitted when unknown, payload=%v", m)
	}
}
