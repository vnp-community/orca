//go:build integration

// Integration tests run against a real MySQL via testcontainers-go — see
// repository_test.go's doc comment for the "integration" build tag
// rationale. Mirrors internal/adapter/postgres/connections_test.go's tests
// 1:1, plus 2 tenant-isolation tests (TestConnectionsRepository_GetStatus_
// DoesNotLeakAcrossTenants, TestConnectionsRepository_GetCredentialID_
// DoesNotLeakAcrossTenants) mirroring TASK-BE-DB-003's pattern for MySQL —
// this service's `connections` table carries a Postgres RLS policy
// (migrations/postgres/0002_connections.up.sql) with no MySQL equivalent,
// so CR-DB-002's acceptance criterion is that tenant isolation holds
// entirely on application-layer scoping here too, not just on the pilot.
package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

func TestConnectionsRepository_MultiSiteUpsert_AddsRowDoesNotOverwrite(t *testing.T) {
	repo := setupRepository(t)

	ctx := context.Background()
	ws1 := domain.Workspace{ID: "https://a.atlassian.net", Name: "Site A"}
	ws2 := domain.Workspace{ID: "https://b.atlassian.net", Name: "Site B"}
	viewer := domain.Viewer{ID: "acc-1", DisplayName: "Ada"}

	if _, err := repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, ws1, viewer, "cred-1"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if _, err := repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, ws2, viewer, "cred-2"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	status, err := repo.GetStatus(ctx, "tenant-1", "user-1", domain.ProviderJira)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if len(status.Workspaces) != 2 {
		t.Fatalf("want 2 connected workspaces, got %d", len(status.Workspaces))
	}
}

func TestConnectionsRepository_SelectWorkspace_MovesIsSelected(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	ws1 := domain.Workspace{ID: "site-a"}
	ws2 := domain.Workspace{ID: "site-b"}
	viewer := domain.Viewer{ID: "acc-1"}
	if _, err := repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, ws1, viewer, "cred-1"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if _, err := repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, ws2, viewer, "cred-2"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	status, err := repo.SelectWorkspace(ctx, "tenant-1", "user-1", domain.ProviderJira, "site-b")
	if err != nil {
		t.Fatalf("select workspace: %v", err)
	}
	if status.SelectedWorkspaceID != "site-b" {
		t.Errorf("want selected site-b, got %q", status.SelectedWorkspaceID)
	}
}

func TestConnectionsRepository_Delete_RemovesOneWorkspaceOnly(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	viewer := domain.Viewer{ID: "acc-1"}
	if _, err := repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, domain.Workspace{ID: "site-a"}, viewer, "cred-1"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if _, err := repo.Upsert(ctx, "tenant-1", "user-1", domain.ProviderJira, domain.Workspace{ID: "site-b"}, viewer, "cred-2"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if err := repo.Delete(ctx, "tenant-1", "user-1", domain.ProviderJira, "site-a"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	status, err := repo.GetStatus(ctx, "tenant-1", "user-1", domain.ProviderJira)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if len(status.Workspaces) != 1 || status.Workspaces[0].ID != "site-b" {
		t.Fatalf("want only site-b remaining, got %+v", status.Workspaces)
	}
}

func TestConnectionsRepository_GetCredentialID_NotFoundReturnsSentinel(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_, err := repo.GetCredentialID(ctx, "tenant-1", "user-1", domain.ProviderJira, "")
	if err == nil {
		t.Fatal("expected an error for a connection that does not exist")
	}
}

// TestConnectionsRepository_GetStatus_DoesNotLeakAcrossTenants is the
// MySQL mirror of TASK-BE-DB-003's tenant-isolation pattern: 2 tenants each
// connect the same provider/workspace shape, GetStatus for tenant A must
// never surface tenant B's rows. connections has no RLS equivalent on
// MySQL (migrations/mysql/0002_connections.up.sql), so the explicit
// `WHERE tenant_id=?` in GetStatus (connections.go) is the ONLY
// enforcement — this proves it holds.
func TestConnectionsRepository_GetStatus_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	viewerA := domain.Viewer{ID: "acc-a", DisplayName: "Tenant A User"}
	viewerB := domain.Viewer{ID: "acc-b", DisplayName: "Tenant B User"}

	if _, err := repo.Upsert(ctx, "tenant-a", "user-1", domain.ProviderJira, domain.Workspace{ID: "site-shared"}, viewerA, "cred-a"); err != nil {
		t.Fatalf("upsert tenant-a: %v", err)
	}
	if _, err := repo.Upsert(ctx, "tenant-b", "user-1", domain.ProviderJira, domain.Workspace{ID: "site-shared"}, viewerB, "cred-b"); err != nil {
		t.Fatalf("upsert tenant-b: %v", err)
	}

	status, err := repo.GetStatus(ctx, "tenant-a", "user-1", domain.ProviderJira)
	if err != nil {
		t.Fatalf("get status tenant-a: %v", err)
	}
	if len(status.Workspaces) != 1 {
		t.Fatalf("tenant-a status leaked rows from tenant-b — application-layer scoping failed, MySQL has no RLS backstop at all (see BE-DB-SOL-001 §4, TASK-BE-DB-003): got %+v", status.Workspaces)
	}
	if status.Viewer.ID != "acc-a" {
		t.Fatalf("tenant-a status returned tenant-b's viewer: got %+v", status.Viewer)
	}
}

// TestConnectionsRepository_GetCredentialID_DoesNotLeakAcrossTenants mirrors
// the above for GetCredentialID, the path credential.Resolver actually
// calls per request.
func TestConnectionsRepository_GetCredentialID_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	viewer := domain.Viewer{ID: "acc-a"}

	if _, err := repo.Upsert(ctx, "tenant-a", "user-1", domain.ProviderJira, domain.Workspace{ID: "site-shared"}, viewer, "cred-a-secret"); err != nil {
		t.Fatalf("upsert tenant-a: %v", err)
	}

	// tenant-b never connected — GetCredentialID must return
	// ErrConnectionNotFound, never tenant-a's credential-id, even though
	// the (provider, workspace) shape matches exactly.
	_, err := repo.GetCredentialID(ctx, "tenant-b", "user-1", domain.ProviderJira, "site-shared")
	if !errors.Is(err, usecase.ErrConnectionNotFound) {
		t.Fatalf("expected ErrConnectionNotFound for tenant-b (no connection), got: %v — possible cross-tenant credential leak", err)
	}
}
