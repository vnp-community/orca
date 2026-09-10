//go:build integration

// See repository_test.go's file header for the shared testcontainers setup.
package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TestRepository_Revoke_And_ListByTask covers TASK-TG-003-04's public
// grant-management surface against a real database: ListByTask returns
// every grant on one task, and Revoke removes exactly the matching
// (task_id, subject_id, level) row, idempotently.
func TestRepository_Revoke_And_ListByTask(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}

	user1, user2 := uuid.NewString(), uuid.NewString()
	if err := repo.Grant(ctx, tenantID, domain.Grant{TaskID: task.ID, SubjectID: user1, Level: domain.GrantLevelOwner, ApplyTree: true}); err != nil {
		t.Fatalf("granting owner: %v", err)
	}
	if err := repo.Grant(ctx, tenantID, domain.Grant{TaskID: task.ID, SubjectID: user2, Level: domain.GrantLevelAdmin}); err != nil {
		t.Fatalf("granting admin: %v", err)
	}

	grants, err := repo.ListByTask(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("ListByTask: %v", err)
	}
	if len(grants) != 2 {
		t.Fatalf("expected 2 grants, got %d: %+v", len(grants), grants)
	}

	if err := repo.Revoke(ctx, tenantID, task.ID, user1, domain.GrantLevelOwner); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	grants, err = repo.ListByTask(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("ListByTask after revoke: %v", err)
	}
	if len(grants) != 1 || grants[0].SubjectID != user2 {
		t.Errorf("expected only user2's grant to remain, got %+v", grants)
	}

	// Idempotent: revoking again must not error.
	if err := repo.Revoke(ctx, tenantID, task.ID, user1, domain.GrantLevelOwner); err != nil {
		t.Errorf("expected the second revoke to no-op without error, got %v", err)
	}
}
