//go:build integration

// See repository_test.go's file header — same testcontainers-gated
// integration suite, split into its own file since TASK-TG-001-04's
// coverage is specifically about usecase.AddEdge's atomicity guarantee
// against a REAL Postgres transaction, not just this package's own
// RunInTx/CRUD primitives.
package postgres

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// TestAddEdge_ConcurrentCycleRace_ExactlyOneSucceeds proves TASK-TG-001-04's
// atomicity fix against a real Postgres transaction (not a mock): two
// goroutines each try to add an edge that, together, would close a cycle
// (a->b already exists; b->c and c->a race each other). Wrapping each
// AddEdge.Execute call in repo.RunInTx serializes the cycle
// check-then-write per BE-SOL-001's requirement — at most one of the two
// racing edges may be persisted (whichever transaction's cycle check runs
// against the post-commit state of the other loses), never both.
func TestAddEdge_ConcurrentCycleRace_ExactlyOneSucceeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	a, _ := domain.NewTask(uuid.NewString(), tenantID, "a", domain.StatusDone, "", "")
	b, _ := domain.NewTask(uuid.NewString(), tenantID, "b", domain.StatusDone, "", "")
	c, _ := domain.NewTask(uuid.NewString(), tenantID, "c", domain.StatusDone, "", "")
	for _, task := range []domain.Task{a, b, c} {
		if _, err := repo.Create(ctx, task); err != nil {
			t.Fatalf("creating task %s: %v", task.ID, err)
		}
	}

	tenantCtx := tenant.WithTenantID(ctx, tenantID)

	// Seed a -> b so that b -> c and c -> a would each close a 3-cycle.
	if err := runAddEdge(tenantCtx, repo, a.ID, b.ID); err != nil {
		t.Fatalf("seeding a->b: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = runAddEdge(tenantCtx, repo, b.ID, c.ID)
	}()
	go func() {
		defer wg.Done()
		errs[1] = runAddEdge(tenantCtx, repo, c.ID, a.ID)
	}()
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one of the two racing edges to succeed, got %d successes (errs=%v)", successes, errs)
	}
}

// TestAddEdge_AutoBlock_PersistsAgainstRealDB proves the auto-block write
// (TaskRepository.UpdateStatus inside the same transaction as the edge
// insert) actually persists against a real database, not just a fake.
func TestAddEdge_AutoBlock_PersistsAgainstRealDB(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	from, _ := domain.NewTask(uuid.NewString(), tenantID, "from", domain.StatusOpen, "", "")
	to, _ := domain.NewTask(uuid.NewString(), tenantID, "to", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, from); err != nil {
		t.Fatalf("creating from-task: %v", err)
	}
	if _, err := repo.Create(ctx, to); err != nil {
		t.Fatalf("creating to-task: %v", err)
	}

	tenantCtx := tenant.WithTenantID(ctx, tenantID)
	if err := runAddEdge(tenantCtx, repo, from.ID, to.ID); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	got, err := repo.Get(ctx, tenantID, to.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.StatusBlocked {
		t.Errorf("expected dependent task to be auto-blocked, got status %q", got.Status)
	}
}

func runAddEdge(ctx context.Context, repo *Repository, fromID, toID string) error {
	return repo.RunInTx(ctx, func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error {
		_, err := usecase.NewAddEdge(tasks, edges).Execute(ctx, usecase.AddEdgeInput{
			FromTaskID: fromID, ToTaskID: toID, Kind: domain.EdgeKindDependsOn,
		})
		return err
	})
}
