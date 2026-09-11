//go:build integration

package mysql

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestWorktreeRepository_CreateWithEvent_RoundTripsAndEnqueuesOutbox(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepoAdapter := NewRepoRepository(db)
	worktreeRepo := NewWorktreeRepository(db)
	outboxRepo := NewOutboxRepository(db)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	r, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	repo, err := repoRepoAdapter.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	wt, err := domain.NewWorktree(uuid.NewString(), p.ID, repo.ID, "/srv/worktree", "main", "", "", domain.WorktreeLineageCapture{})
	if err != nil {
		t.Fatalf("NewWorktree: %v", err)
	}
	event := domain.OutboxEvent{ID: uuid.NewString(), TenantID: p.TenantID, Subject: "orca.project.worktree.created", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{"worktree_id":"x"}`)}

	created, err := worktreeRepo.CreateWorktreeWithEvent(ctx, wt, event)
	if err != nil {
		t.Fatalf("CreateWorktreeWithEvent: %v", err)
	}
	if created.Path != "/srv/worktree" || created.Status != domain.WorktreeStatusActive {
		t.Errorf("unexpected worktree: %+v", created)
	}
	if string(created.Metadata) != "{}" {
		t.Errorf("expected metadata to default to {} via column DEFAULT (JSON_OBJECT()), got %q", created.Metadata)
	}

	records, err := outboxRepo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(records) != 1 || records[0].ID != event.ID {
		t.Fatalf("expected the worktree.created event enqueued in the same transaction, got %+v", records)
	}

	if err := outboxRepo.MarkPublished(ctx, []string{event.ID}); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	afterMark, err := outboxRepo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished after mark: %v", err)
	}
	if len(afterMark) != 0 {
		t.Errorf("expected no unpublished events after MarkPublished, got %+v", afterMark)
	}
}

// TestWorktreeRepository_UpdateWorktreeMeta_MergePatch exercises MySQL's
// JSON_MERGE_PATCH translation of Postgres jsonb `||` — see
// worktree_repository.go's UpdateWorktreeMeta doc comment for the RFC 7396
// null-deletes-key semantics this relies on.
func TestWorktreeRepository_UpdateWorktreeMeta_MergePatch(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepoAdapter := NewRepoRepository(db)
	worktreeRepo := NewWorktreeRepository(db)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	r, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	repo, err := repoRepoAdapter.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	wt, err := domain.NewWorktree(uuid.NewString(), p.ID, repo.ID, "/srv/wt", "main", "", "", domain.WorktreeLineageCapture{})
	if err != nil {
		t.Fatalf("NewWorktree: %v", err)
	}
	event := domain.OutboxEvent{ID: uuid.NewString(), TenantID: p.TenantID, Subject: "s", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)}
	created, err := worktreeRepo.CreateWorktreeWithEvent(ctx, wt, event)
	if err != nil {
		t.Fatalf("CreateWorktreeWithEvent: %v", err)
	}

	after1, err := worktreeRepo.UpdateWorktreeMeta(ctx, created.ID, json.RawMessage(`{"displayName":"My WT","isPinned":true}`))
	if err != nil {
		t.Fatalf("UpdateWorktreeMeta (1): %v", err)
	}
	var meta1 map[string]any
	if err := json.Unmarshal(after1.Metadata, &meta1); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta1["displayName"] != "My WT" || meta1["isPinned"] != true {
		t.Fatalf("unexpected metadata after first patch: %+v", meta1)
	}

	// Second patch: add a new key, leave displayName alone, explicitly null
	// out isPinned — JSON_MERGE_PATCH removes the key entirely on null.
	after2, err := worktreeRepo.UpdateWorktreeMeta(ctx, created.ID, json.RawMessage(`{"comment":"hi","isPinned":null}`))
	if err != nil {
		t.Fatalf("UpdateWorktreeMeta (2): %v", err)
	}
	var meta2 map[string]any
	if err := json.Unmarshal(after2.Metadata, &meta2); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta2["displayName"] != "My WT" {
		t.Errorf("expected displayName to survive the second merge, got %+v", meta2)
	}
	if meta2["comment"] != "hi" {
		t.Errorf("expected comment added, got %+v", meta2)
	}
	if _, stillPresent := meta2["isPinned"]; stillPresent {
		t.Errorf("expected isPinned removed by the explicit-null merge patch, got %+v", meta2)
	}
}

func TestWorktreeRepository_ListWorktrees_StatusFilterAndOlderThan(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepoAdapter := NewRepoRepository(db)
	worktreeRepo := NewWorktreeRepository(db)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	r, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	repo, err := repoRepoAdapter.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	for i, branch := range []string{"active-wt", "stopped-wt"} {
		wt, err := domain.NewWorktree(uuid.NewString(), p.ID, repo.ID, "/srv/"+branch, branch, "", "", domain.WorktreeLineageCapture{})
		if err != nil {
			t.Fatalf("NewWorktree %d: %v", i, err)
		}
		event := domain.OutboxEvent{ID: uuid.NewString(), TenantID: p.TenantID, Subject: "s", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)}
		created, err := worktreeRepo.CreateWorktreeWithEvent(ctx, wt, event)
		if err != nil {
			t.Fatalf("CreateWorktreeWithEvent %d: %v", i, err)
		}
		if branch == "stopped-wt" {
			if _, err := worktreeRepo.db.ExecContext(ctx, `UPDATE worktrees SET status = ? WHERE id = ?`, string(domain.WorktreeStatusStopped), created.ID); err != nil {
				t.Fatalf("seed status: %v", err)
			}
		}
	}

	onlyStopped, err := worktreeRepo.ListWorktrees(ctx, p.ID, []string{string(domain.WorktreeStatusStopped)}, nil)
	if err != nil {
		t.Fatalf("ListWorktrees (status filter): %v", err)
	}
	if len(onlyStopped) != 1 || onlyStopped[0].Branch != "stopped-wt" {
		t.Fatalf("expected only the stopped worktree, got %+v", onlyStopped)
	}

	all, err := worktreeRepo.ListWorktrees(ctx, p.ID, nil, nil)
	if err != nil {
		t.Fatalf("ListWorktrees (no filter): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected both worktrees with no filter, got %d", len(all))
	}
}

func TestWorktreeRepository_SetWorktreeLineage_RoundTrips(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepoAdapter := NewRepoRepository(db)
	worktreeRepo := NewWorktreeRepository(db)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	r, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	repo, err := repoRepoAdapter.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}

	parent, err := domain.NewWorktree(uuid.NewString(), p.ID, repo.ID, "/srv/parent", "main", "", "", domain.WorktreeLineageCapture{})
	if err != nil {
		t.Fatalf("NewWorktree parent: %v", err)
	}
	parentCreated, err := worktreeRepo.CreateWorktreeWithEvent(ctx, parent, domain.OutboxEvent{ID: uuid.NewString(), TenantID: p.TenantID, Subject: "s", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	child, err := domain.NewWorktree(uuid.NewString(), p.ID, repo.ID, "/srv/child", "feature", "", "", domain.WorktreeLineageCapture{})
	if err != nil {
		t.Fatalf("NewWorktree child: %v", err)
	}
	childCreated, err := worktreeRepo.CreateWorktreeWithEvent(ctx, child, domain.OutboxEvent{ID: uuid.NewString(), TenantID: p.TenantID, Subject: "s", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	updated, err := worktreeRepo.SetWorktreeLineage(ctx, childCreated.ID, &parentCreated.ID)
	if err != nil {
		t.Fatalf("SetWorktreeLineage: %v", err)
	}
	if updated.ParentWorktreeID == nil || *updated.ParentWorktreeID != parentCreated.ID {
		t.Fatalf("expected parent worktree id set, got %+v", updated.ParentWorktreeID)
	}
	if updated.CaptureConfidence == nil || *updated.CaptureConfidence != "explicit" {
		t.Errorf("expected capture_confidence 'explicit', got %+v", updated.CaptureConfidence)
	}

	lineage, err := worktreeRepo.ListLineage(ctx)
	if err != nil {
		t.Fatalf("ListLineage: %v", err)
	}
	found := false
	for _, wt := range lineage {
		if wt.ID == childCreated.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected child worktree in ListLineage, got %+v", lineage)
	}

	// Clearing lineage (nil parent) must also round-trip.
	cleared, err := worktreeRepo.SetWorktreeLineage(ctx, childCreated.ID, nil)
	if err != nil {
		t.Fatalf("SetWorktreeLineage (clear): %v", err)
	}
	if cleared.ParentWorktreeID != nil {
		t.Errorf("expected parent worktree id cleared, got %+v", cleared.ParentWorktreeID)
	}
}

func TestWorktreeRepository_RemoveWorktreeWithEvent_DeletesAndEnqueues(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	repoRepoAdapter := NewRepoRepository(db)
	worktreeRepo := NewWorktreeRepository(db)
	outboxRepo := NewOutboxRepository(db)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	r, err := domain.NewRepo(uuid.NewString(), p.ID, "https://example.com/repo.git", "repo", "")
	if err != nil {
		t.Fatalf("NewRepo: %v", err)
	}
	repo, err := repoRepoAdapter.AddRepo(ctx, r)
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	wt, err := domain.NewWorktree(uuid.NewString(), p.ID, repo.ID, "/srv/wt", "main", "", "", domain.WorktreeLineageCapture{})
	if err != nil {
		t.Fatalf("NewWorktree: %v", err)
	}
	created, err := worktreeRepo.CreateWorktreeWithEvent(ctx, wt, domain.OutboxEvent{ID: uuid.NewString(), TenantID: p.TenantID, Subject: "s", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	deletedEventID := uuid.NewString()
	err = worktreeRepo.RemoveWorktreeWithEvent(ctx, created.ID, func(removed domain.Worktree) domain.OutboxEvent {
		if removed.ID != created.ID {
			t.Errorf("buildEvent should see the just-deleted row, got %+v", removed)
		}
		return domain.OutboxEvent{ID: deletedEventID, TenantID: p.TenantID, Subject: "s.deleted", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)}
	})
	if err != nil {
		t.Fatalf("RemoveWorktreeWithEvent: %v", err)
	}

	if _, err := worktreeRepo.GetWorktree(ctx, created.ID); err != domain.ErrWorktreeNotFound {
		t.Errorf("expected worktree removed, got %v", err)
	}

	records, err := outboxRepo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	found := false
	for _, rec := range records {
		if rec.ID == deletedEventID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected worktree.deleted event enqueued, got %+v", records)
	}
}
