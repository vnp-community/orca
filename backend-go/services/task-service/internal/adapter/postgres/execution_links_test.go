//go:build integration

// Integration tests run against a real Postgres via testcontainers-go — see
// repository_test.go's setupRepository, reused as-is here.
package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// seedTenantAndTask creates a minimal task ("tenant-1"-shaped, freshly
// UUID'd) for execution_links FK tests to point at.
func seedTenantAndTask(t *testing.T, repo *Repository) (tenantID, taskID string) {
	t.Helper()
	ctx := context.Background()
	tenantID = uuid.NewString()
	task, err := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}
	return tenantID, task.ID
}

func TestExecutionLinks_CreateThenComplete_RoundTrips(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, taskID := seedTenantAndTask(t, repo)

	link, err := repo.CreateExecutionLink(ctx, tenantID, taskID, domain.EngineDirectAgent, "")
	if err != nil {
		t.Fatalf("CreateExecutionLink: %v", err)
	}
	if link.ID == "" {
		t.Fatal("expected a non-empty link id")
	}
	if link.Engine != domain.EngineDirectAgent {
		t.Errorf("expected engine %q, got %q", domain.EngineDirectAgent, link.Engine)
	}
	if link.StatusMirror != "in_progress" {
		t.Errorf("expected default status_mirror in_progress, got %q", link.StatusMirror)
	}

	if err := repo.SetExternalRef(ctx, tenantID, link.ID, "ref-123"); err != nil {
		t.Fatalf("SetExternalRef: %v", err)
	}
	if err := repo.Complete(ctx, tenantID, link.ID, "completed"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
}

func TestExecutionLinks_SetExternalRef_UnknownID_Fails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, _ := seedTenantAndTask(t, repo)

	if err := repo.SetExternalRef(ctx, tenantID, uuid.NewString(), "ref-123"); err == nil {
		t.Fatal("expected an error for an unknown execution link id")
	}
}

func TestExecutionLinks_Complete_UnknownID_Fails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, _ := seedTenantAndTask(t, repo)

	if err := repo.Complete(ctx, tenantID, uuid.NewString(), "completed"); err == nil {
		t.Fatal("expected an error for an unknown execution link id")
	}
}
