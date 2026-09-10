package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestListGrants_RequiresTenantContext(t *testing.T) {
	uc := NewListGrants(&fakeGrantRepository{})
	if _, err := uc.Execute(context.Background(), "t1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestListGrants_ReturnsAllGrantsOnTheTask is the task's own named test: a
// task with 3 grants (owner/admin/team) returns all 3, expires_at populated
// correctly for the one with an expiry set.
func TestListGrants_ReturnsAllGrantsOnTheTask(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	repo := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "t1", SubjectID: "user-1", Level: domain.GrantLevelOwner},
		{TaskID: "t1", SubjectID: "user-2", Level: domain.GrantLevelAdmin, ExpiresAt: &expiresAt},
		{TaskID: "t1", SubjectID: "team-a", Level: domain.GrantLevelTeam},
		{TaskID: "t2", SubjectID: "user-3", Level: domain.GrantLevelOwner}, // a different task — must NOT appear
	}}
	uc := NewListGrants(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected exactly 3 grants for t1, got %d: %+v", len(got), got)
	}
	var foundExpiring bool
	for _, g := range got {
		if g.SubjectID == "user-2" {
			foundExpiring = true
			if g.ExpiresAt == nil || !g.ExpiresAt.Equal(expiresAt) {
				t.Errorf("expected ExpiresAt to match, got %+v", g.ExpiresAt)
			}
		}
	}
	if !foundExpiring {
		t.Error("expected the admin grant with an expiry to be present")
	}
}

func TestListGrants_EmptyForATaskWithNoGrants(t *testing.T) {
	uc := NewListGrants(&fakeGrantRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no grants, got %+v", got)
	}
}
