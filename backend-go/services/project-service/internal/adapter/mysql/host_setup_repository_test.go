//go:build integration

package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestHostSetupRepository_CreateGetUpdateComplete_RoundTrip(t *testing.T) {
	db := setupDB(t)
	hostSetupRepo := NewHostSetupRepository(db)
	projRepo := New(db)
	ctx := context.Background()

	tenantID, devServerID, createdBy := uuid.NewString(), uuid.NewString(), uuid.NewString()
	setup, err := domain.NewHostSetup(uuid.NewString(), tenantID, devServerID, "/srv/host-project", "Host Project", createdBy)
	if err != nil {
		t.Fatalf("NewHostSetup: %v", err)
	}
	created, err := hostSetupRepo.Create(ctx, setup)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Status != domain.HostSetupPending {
		t.Errorf("expected new host setup to start Pending, got %q", created.Status)
	}

	got, err := hostSetupRepo.Get(ctx, tenantID, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FolderPath != "/srv/host-project" {
		t.Errorf("unexpected host setup: %+v", got)
	}

	updated, err := hostSetupRepo.Update(ctx, tenantID, created.ID, domain.HostSetupPatch{DisplayName: "Renamed"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DisplayName != "Renamed" || updated.FolderPath != "/srv/host-project" {
		t.Errorf("expected only display_name patched, got %+v", updated)
	}

	if err := hostSetupRepo.SetStatus(ctx, tenantID, created.ID, domain.HostSetupValidated); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	afterStatus, err := hostSetupRepo.Get(ctx, tenantID, created.ID)
	if err != nil {
		t.Fatalf("Get after SetStatus: %v", err)
	}
	if afterStatus.Status != domain.HostSetupValidated {
		t.Errorf("expected Validated status, got %q", afterStatus.Status)
	}

	p := newTestProject(uuid.NewString(), tenantID, "finalized-project")
	if _, err := projRepo.Create(ctx, p); err != nil {
		t.Fatalf("create finalizing project: %v", err)
	}
	completed, err := hostSetupRepo.Complete(ctx, tenantID, created.ID, p.ID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if completed.Status != domain.HostSetupCompleted || completed.ProjectID != p.ID {
		t.Errorf("expected completed host setup linked to project, got %+v", completed)
	}

	list, err := hostSetupRepo.List(ctx, tenantID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 host setup, got %d", len(list))
	}

	if err := hostSetupRepo.Delete(ctx, tenantID, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := hostSetupRepo.Get(ctx, tenantID, created.ID); err != domain.ErrHostSetupNotFound {
		t.Errorf("expected ErrHostSetupNotFound after delete, got %v", err)
	}
}

// TestHostSetupRepository_Get_DoesNotLeakAcrossTenants is the
// TASK-BE-DB-003 pattern for `project_host_setups`, which has a direct
// tenant_id column and an RLS policy on Postgres (migrations/postgres/0007)
// with no MySQL equivalent.
func TestHostSetupRepository_Get_DoesNotLeakAcrossTenants(t *testing.T) {
	db := setupDB(t)
	hostSetupRepo := NewHostSetupRepository(db)
	ctx := context.Background()

	setup, err := domain.NewHostSetup(uuid.NewString(), uuid.NewString(), uuid.NewString(), "/srv/p", "P", uuid.NewString())
	if err != nil {
		t.Fatalf("NewHostSetup: %v", err)
	}
	created, err := hostSetupRepo.Create(ctx, setup)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := hostSetupRepo.Get(ctx, uuid.NewString(), created.ID); err != domain.ErrHostSetupNotFound {
		t.Errorf("expected ErrHostSetupNotFound for a mismatched tenant, got %v", err)
	}
}
