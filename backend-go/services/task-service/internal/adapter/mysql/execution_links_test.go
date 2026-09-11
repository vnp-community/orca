//go:build integration

// Integration tests run against a real MySQL via testcontainers-go — see
// repository_test.go's setupRepository, reused as-is here. Mirrors
// internal/adapter/postgres/execution_links_test.go 1:1.
package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

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
	if link.StartedAt.IsZero() {
		t.Error("expected a non-zero StartedAt read back from the DB")
	}

	if err := repo.SetExternalRef(ctx, tenantID, link.ID, "ref-123"); err != nil {
		t.Fatalf("SetExternalRef: %v", err)
	}
	if err := repo.Complete(ctx, tenantID, link.ID, "completed"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := repo.GetExecutionLink(ctx, tenantID, link.ID)
	if err != nil {
		t.Fatalf("GetExecutionLink: %v", err)
	}
	if got.ExternalRefID != "ref-123" || got.StatusMirror != "completed" || got.CompletedAt == nil {
		t.Errorf("unexpected execution link after complete: %+v", got)
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

// TestExecutionLinks_UpdateStatusMirror_UnknownRef_IsNoop mirrors
// usecase.ExecutionLinkRepository.UpdateStatusMirror's documented no-op
// contract for a stale/duplicate/unrelated externalRefID.
func TestExecutionLinks_UpdateStatusMirror_UnknownRef_IsNoop(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, _ := seedTenantAndTask(t, repo)

	if err := repo.UpdateStatusMirror(ctx, tenantID, "no-such-ref", "completed"); err != nil {
		t.Fatalf("expected a no-op, not an error, for an unknown external_ref_id: %v", err)
	}
}

func TestExecutionLinks_UpdateStatusMirror_MatchingRef_Updates(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, taskID := seedTenantAndTask(t, repo)

	link, err := repo.CreateExecutionLink(ctx, tenantID, taskID, domain.EngineOrchestration, "coord-run-1")
	if err != nil {
		t.Fatalf("CreateExecutionLink: %v", err)
	}

	if err := repo.UpdateStatusMirror(ctx, tenantID, "coord-run-1", "failed"); err != nil {
		t.Fatalf("UpdateStatusMirror: %v", err)
	}

	got, err := repo.GetExecutionLink(ctx, tenantID, link.ID)
	if err != nil {
		t.Fatalf("GetExecutionLink: %v", err)
	}
	if got.StatusMirror != "failed" {
		t.Errorf("expected status_mirror=failed, got %q", got.StatusMirror)
	}
}
