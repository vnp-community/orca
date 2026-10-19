//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// An id that names no row and an id that is not even a uuid (SQLSTATE 22P02) must both come back as the
// domain sentinel, so the usecase answers 404 instead of a 500.
func TestRepository_UnknownOrMalformedTaskID_ReturnsErrTaskNotFound(t *testing.T) {
	links, repo := setupShareLinkStore(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		task := domain.Task{ID: id, TenantID: tenantID, Title: "ghost", Status: domain.StatusOpen}
		calls := map[string]func() error{
			"Get": func() error { _, err := repo.Get(ctx, tenantID, id); return err },
			"FindByNumber": func() error {
				_, err := repo.FindByNumber(ctx, tenantID, id, 1)
				return err
			},
			"GetAncestors": func() error { _, err := repo.GetAncestors(ctx, tenantID, id, 0); return err },
			"GetSubtree":   func() error { _, _, err := repo.GetSubtree(ctx, tenantID, id, 0); return err },
			"GetSubtreeWithChildPercents": func() error {
				_, err := repo.GetSubtreeWithChildPercents(ctx, tenantID, id)
				return err
			},
			"UpdateStatus": func() error { return repo.UpdateStatus(ctx, tenantID, id, domain.StatusDone) },
			"SetActiveExecutionLink": func() error {
				return repo.SetActiveExecutionLink(ctx, tenantID, id, uuid.NewString())
			},
			"CompleteExecution": func() error {
				return repo.CompleteExecution(ctx, tenantID, id, string(domain.StatusReview), 1, nil)
			},
			"Update": func() error { return repo.Update(ctx, tenantID, task, nil) },
			"Delete": func() error { return repo.Delete(ctx, tenantID, id) },
		}
		for name, call := range calls {
			if name == "GetSubtreeWithChildPercents" && id != "not-a-uuid" {
				continue // an unknown but well-formed root yields an empty progress set, not an error
			}
			if err := call(); !errors.Is(err, domain.ErrTaskNotFound) {
				t.Errorf("%s(%q): want ErrTaskNotFound, got %v", name, id, err)
			}
		}
	}

	if _, err := repo.GetByShareToken(ctx, "no-such-token"); !errors.Is(err, domain.ErrTaskNotFound) {
		t.Errorf("GetByShareToken: want ErrTaskNotFound, got %v", err)
	}

	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		if err := links.Revoke(ctx, tenantID, id); !errors.Is(err, domain.ErrShareLinkNotFound) {
			t.Errorf("Revoke(%q): want ErrShareLinkNotFound, got %v", id, err)
		}
		if _, err := links.TaskIDFor(ctx, tenantID, id); !errors.Is(err, domain.ErrShareLinkNotFound) {
			t.Errorf("TaskIDFor(%q): want ErrShareLinkNotFound, got %v", id, err)
		}
	}
	if _, err := links.ResolveActive(ctx, tenantID, "no-such-hash"); !errors.Is(err, domain.ErrShareLinkNotFound) {
		t.Errorf("ResolveActive: want ErrShareLinkNotFound, got %v", err)
	}

	// Edges and comments reference tasks by FK; an unknown endpoint must not surface as an Internal failure.
	known := domain.Task{ID: uuid.NewString(), TenantID: tenantID, Title: "real", Status: domain.StatusOpen}
	if _, err := repo.Create(ctx, known); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	for _, ghost := range []string{uuid.NewString(), "not-a-uuid"} {
		edge := domain.TaskEdge{FromTaskID: known.ID, ToTaskID: ghost, Kind: domain.EdgeKindDependsOn}
		if err := repo.Add(ctx, tenantID, edge); !errors.Is(err, domain.ErrTaskNotFound) {
			t.Errorf("Add edge to %q: want ErrTaskNotFound, got %v", ghost, err)
		}
		comment := domain.TaskComment{TaskID: ghost, AuthorID: uuid.NewString(), Content: "hi"}
		if _, err := repo.AddComment(ctx, tenantID, comment); !errors.Is(err, domain.ErrTaskNotFound) {
			t.Errorf("AddComment on %q: want ErrTaskNotFound, got %v", ghost, err)
		}
	}
}
