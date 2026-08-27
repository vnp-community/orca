//go:build integration

// Integration tests run against a real Postgres via testcontainers-go — see
// repository_test.go's doc comment for the "integration" build tag
// rationale; run with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
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
