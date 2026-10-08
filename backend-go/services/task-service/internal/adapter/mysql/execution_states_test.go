//go:build integration

package mysql

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestExecutionStates_Integration(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	taskA, _ := domain.NewTask(uuid.NewString(), tenantID, "A", domain.StatusOpen, "", "")
	taskB, _ := domain.NewTask(uuid.NewString(), tenantID, "B", domain.StatusOpen, "", "")
	taskC, _ := domain.NewTask(uuid.NewString(), tenantID, "C", domain.StatusOpen, "", "")
	taskD, _ := domain.NewTask(uuid.NewString(), tenantID, "D", domain.StatusOpen, "", "")
	taskE, _ := domain.NewTask(uuid.NewString(), tenantID, "E", domain.StatusOpen, "", "")
	taskF, _ := domain.NewTask(uuid.NewString(), tenantID, "F", domain.StatusDone, "", "")

	for _, task := range []domain.Task{taskA, taskB, taskC, taskD, taskE, taskF} {
		if _, err := repo.Create(ctx, task); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	now := time.Now().Truncate(time.Second).UTC()
	yesterday := now.Add(-24 * time.Hour)

	// Task A: two links, one old failed, one new completed
	linkA1, _ := repo.CreateExecutionLink(ctx, tenantID, taskA.ID, "workflow", "refA1")
	repo.db.ExecContext(ctx, "UPDATE execution_links SET started_at = ? WHERE id = ?", yesterday, linkA1.ID)
	repo.Complete(ctx, tenantID, linkA1.ID, "failed")

	linkA2, _ := repo.CreateExecutionLink(ctx, tenantID, taskA.ID, "workflow", "refA2")
	repo.db.ExecContext(ctx, "UPDATE execution_links SET started_at = ? WHERE id = ?", now, linkA2.ID)
	repo.Complete(ctx, tenantID, linkA2.ID, "completed")

	// Task B: three links failed, all with same started_at to test id DESC tie-break
	for i := 0; i < 3; i++ {
		link, _ := repo.CreateExecutionLink(ctx, tenantID, taskB.ID, "workflow", fmt.Sprintf("refB%d", i))
		repo.db.ExecContext(ctx, "UPDATE execution_links SET started_at = ? WHERE id = ?", now, link.ID)
		repo.Complete(ctx, tenantID, link.ID, "failed")
	}

	// Task D depends on E (open) and F (done)
	repo.Add(ctx, tenantID, domain.TaskEdge{FromTaskID: taskD.ID, ToTaskID: taskE.ID, Kind: domain.EdgeKindDependsOn})
	repo.Add(ctx, tenantID, domain.TaskEdge{FromTaskID: taskD.ID, ToTaskID: taskF.ID, Kind: domain.EdgeKindDependsOn})

	t.Run("LastLink_PicksNewest", func(t *testing.T) {
		states, err := repo.ListExecutionStates(ctx, tenantID, []string{taskA.ID})
		if err != nil {
			t.Fatalf("ListExecutionStates: %v", err)
		}
		if len(states) != 1 {
			t.Fatalf("expected 1 state, got %d", len(states))
		}
		if states[0].LastLinkStatus != "completed" {
			t.Errorf("expected LastLinkStatus completed, got %q", states[0].LastLinkStatus)
		}
		if states[0].FailedAttempts != 1 {
			t.Errorf("expected 1 failed attempt, got %d", states[0].FailedAttempts)
		}
	})

	t.Run("FailedAttempts_Counts", func(t *testing.T) {
		states, err := repo.ListExecutionStates(ctx, tenantID, []string{taskB.ID})
		if err != nil {
			t.Fatalf("ListExecutionStates: %v", err)
		}
		if len(states) != 1 {
			t.Fatalf("expected 1 state, got %d", len(states))
		}
		if states[0].LastLinkStatus != "failed" {
			t.Errorf("expected LastLinkStatus failed, got %q", states[0].LastLinkStatus)
		}
		if states[0].FailedAttempts != 3 {
			t.Errorf("expected 3 failed attempts, got %d", states[0].FailedAttempts)
		}
	})

	t.Run("NoLink_Empty", func(t *testing.T) {
		states, err := repo.ListExecutionStates(ctx, tenantID, []string{taskC.ID})
		if err != nil {
			t.Fatalf("ListExecutionStates: %v", err)
		}
		if len(states) != 1 {
			t.Fatalf("expected 1 state, got %d", len(states))
		}
		if states[0].LastLinkStatus != "" || states[0].FailedAttempts != 0 {
			t.Errorf("expected empty state, got %+v", states[0])
		}
	})

	t.Run("BlockedBy_ExcludesDone", func(t *testing.T) {
		states, err := repo.ListExecutionStates(ctx, tenantID, []string{taskD.ID})
		if err != nil {
			t.Fatalf("ListExecutionStates: %v", err)
		}
		if len(states) != 1 {
			t.Fatalf("expected 1 state, got %d", len(states))
		}
		if len(states[0].BlockedByTaskIDs) != 1 || states[0].BlockedByTaskIDs[0] != taskE.ID {
			t.Errorf("expected blocked by E only, got %v", states[0].BlockedByTaskIDs)
		}
	})

	t.Run("TenantIsolation", func(t *testing.T) {
		otherTenant := uuid.NewString()
		states, err := repo.ListExecutionStates(ctx, otherTenant, []string{taskA.ID})
		if err != nil {
			t.Fatalf("ListExecutionStates: %v", err)
		}
		if len(states) != 1 {
			t.Fatalf("expected 1 state, got %d", len(states))
		}
		if states[0].LastLinkStatus != "" {
			t.Errorf("expected empty state due to tenant isolation, got %q", states[0].LastLinkStatus)
		}
	})

	t.Run("ManyIDs_500", func(t *testing.T) {
		var ids []string
		for i := 0; i < 500; i++ {
			ids = append(ids, uuid.NewString())
		}
		states, err := repo.ListExecutionStates(ctx, tenantID, ids)
		if err != nil {
			t.Fatalf("ListExecutionStates: %v", err)
		}
		if len(states) != 500 {
			t.Fatalf("expected 500 states, got %d", len(states))
		}
	})
}
