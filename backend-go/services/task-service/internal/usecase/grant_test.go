package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestGrant_RequiresTenantContext(t *testing.T) {
	uc := NewGrant(&fakeGrantRepository{})
	err := uc.Execute(context.Background(), GrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGrant_PersistsAValidGrant(t *testing.T) {
	repo := &fakeGrantRepository{}
	uc := NewGrant(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelAdmin, ApplyTree: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.grants) != 1 || repo.grants[0].Level != domain.GrantLevelAdmin {
		t.Errorf("unexpected grants: %+v", repo.grants)
	}
}

// TestGrant_PersistsExpiresAt is TASK-TG-003-03's write-path regression
// test: a grant created with ExpiresAt set must persist it unchanged, not
// silently drop it (a field with no write path is dead — see this task's
// own Changes-to-make note).
func TestGrant_PersistsExpiresAt(t *testing.T) {
	repo := &fakeGrantRepository{}
	uc := NewGrant(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	expiresAt := time.Now().Add(24 * time.Hour)

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevelUser, ExpiresAt: &expiresAt})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.grants) != 1 || repo.grants[0].ExpiresAt == nil || !repo.grants[0].ExpiresAt.Equal(expiresAt) {
		t.Errorf("expected ExpiresAt to persist unchanged, got %+v", repo.grants)
	}
}

func TestGrant_RejectsAnUnrecognizedLevel(t *testing.T) {
	uc := NewGrant(&fakeGrantRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "u1", Level: domain.GrantLevel(99)})
	if err == nil {
		t.Fatal("expected an error for an unrecognized grant level")
	}
}

func TestGrant_RejectsAnEmptySubject(t *testing.T) {
	uc := NewGrant(&fakeGrantRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	err := uc.Execute(ctx, GrantInput{TaskID: "t1", SubjectID: "", Level: domain.GrantLevelUser})
	if err == nil {
		t.Fatal("expected an error for an empty subject id")
	}
}
