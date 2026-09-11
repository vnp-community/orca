//go:build integration

package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestSourceProjectRepository_LinkRelinkUnlink_RoundTrip(t *testing.T) {
	db := setupDB(t)
	projRepo := New(db)
	sourceRepo := NewSourceProjectRepository(db)
	ctx := context.Background()

	tenantID := uuid.NewString()
	container := newTestProject(uuid.NewString(), tenantID, "container")
	if _, err := projRepo.Create(ctx, container); err != nil {
		t.Fatalf("create container: %v", err)
	}
	source := newTestProject(uuid.NewString(), tenantID, "source")
	if _, err := projRepo.Create(ctx, source); err != nil {
		t.Fatalf("create source: %v", err)
	}

	firstLinker := uuid.NewString()
	sp, err := domain.NewSourceProject(uuid.NewString(), container.ID, source.ID, firstLinker)
	if err != nil {
		t.Fatalf("NewSourceProject: %v", err)
	}
	linked, err := sourceRepo.Link(ctx, sp)
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if linked.LinkedBy != firstLinker {
		t.Errorf("unexpected linked_by: %+v", linked)
	}

	// Re-link (already-linked pair) bumps linked_by/linked_at rather than
	// erroring — ON DUPLICATE KEY UPDATE translation of the Postgres
	// source's ON CONFLICT DO UPDATE.
	secondLinker := uuid.NewString()
	sp2, err := domain.NewSourceProject(uuid.NewString(), container.ID, source.ID, secondLinker)
	if err != nil {
		t.Fatalf("NewSourceProject (relink): %v", err)
	}
	relinked, err := sourceRepo.Link(ctx, sp2)
	if err != nil {
		t.Fatalf("Link (relink): %v", err)
	}
	if relinked.LinkedBy != secondLinker {
		t.Errorf("expected linked_by updated by relink, got %+v", relinked)
	}
	if relinked.ID != linked.ID {
		t.Errorf("expected the SAME join row reused on relink (id unchanged), got first=%q second=%q", linked.ID, relinked.ID)
	}

	list, err := sourceRepo.List(ctx, container.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 link (relink upserts, not duplicates), got %d", len(list))
	}

	got, err := sourceRepo.Get(ctx, container.ID, source.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SourceProjectID != source.ID {
		t.Errorf("unexpected source project: %+v", got)
	}

	if err := sourceRepo.Unlink(ctx, container.ID, source.ID); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if _, err := sourceRepo.Get(ctx, container.ID, source.ID); err == nil {
		t.Error("expected an error looking up an unlinked source project")
	}

	// Unlink is idempotent — a second call on an already-absent row is a
	// no-op, not an error.
	if err := sourceRepo.Unlink(ctx, container.ID, source.ID); err != nil {
		t.Errorf("expected idempotent Unlink to succeed on an absent row, got %v", err)
	}
}
