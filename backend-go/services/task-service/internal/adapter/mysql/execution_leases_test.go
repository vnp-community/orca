//go:build integration

package mysql

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

func newLeasedLink(t *testing.T, repo *Repository, tenantID string, ttl time.Duration, previous string) (taskID, linkID string) {
	t.Helper()
	ctx := context.Background()
	task, err := domain.NewTask(uuid.NewString(), tenantID, "t", domain.StatusOpen, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	link, err := repo.CreateExecutionLink(ctx, tenantID, task.ID, domain.EngineDirectAgent, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.StartLease(ctx, tenantID, link.ID, "owner-a", previous, ttl); err != nil {
		t.Fatalf("StartLease: %v", err)
	}
	return task.ID, link.ID
}

func TestExecutionLeases_ClaimExpired_OnlyExpiredAndReturnsPreviousStatus(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	expiredTask, expiredLink := newLeasedLink(t, repo, tenantID, time.Millisecond, "review")
	_, liveLink := newLeasedLink(t, repo, tenantID, time.Hour, "open")
	time.Sleep(30 * time.Millisecond)

	got, err := repo.ClaimExpired(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimExpired: %v", err)
	}
	if len(got) != 1 || got[0].LinkID != expiredLink || got[0].TaskID != expiredTask || got[0].PreviousStatus != "review" || got[0].TenantID != tenantID {
		t.Fatalf("want only the expired link, got %+v", got)
	}
	if again, _ := repo.ClaimExpired(ctx, 10); len(again) != 0 {
		t.Errorf("an already-claimed link must not be returned again, got %+v", again)
	}
	link, err := repo.GetExecutionLink(ctx, tenantID, liveLink)
	if err != nil || link.StatusMirror != "in_progress" {
		t.Errorf("a live lease must stay in progress: %+v, %v", link, err)
	}
}

func TestExecutionLeases_Renew_KeepsLeaseAliveAndRejectsWrongOwnerOrSwept(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	_, linkID := newLeasedLink(t, repo, tenantID, 200*time.Millisecond, "open")

	if held, err := repo.RenewLease(ctx, tenantID, linkID, "owner-b", time.Hour); err != nil || held {
		t.Fatalf("a different owner must not renew: held=%v err=%v", held, err)
	}
	if held, err := repo.RenewLease(ctx, tenantID, linkID, "owner-a", time.Hour); err != nil || !held {
		t.Fatalf("owner must renew: held=%v err=%v", held, err)
	}
	time.Sleep(250 * time.Millisecond) // past the ORIGINAL ttl
	if got, _ := repo.ClaimExpired(ctx, 10); len(got) != 0 {
		t.Fatalf("a renewed lease must not be claimed, got %+v", got)
	}

	_, sweptLink := newLeasedLink(t, repo, tenantID, time.Millisecond, "open")
	time.Sleep(20 * time.Millisecond)
	if got, _ := repo.ClaimExpired(ctx, 10); len(got) != 1 {
		t.Fatalf("want 1 swept, got %+v", got)
	}
	if held, _ := repo.RenewLease(ctx, tenantID, sweptLink, "owner-a", time.Hour); held {
		t.Error("a swept run must not be able to resurrect its lease")
	}
}

func TestExecutionLeases_ClaimExpired_ConcurrentSweepersNeverDoubleClaim(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	const total = 24
	want := map[string]bool{}
	for i := 0; i < total; i++ {
		_, id := newLeasedLink(t, repo, tenantID, time.Millisecond, "open")
		want[id] = true
	}
	time.Sleep(30 * time.Millisecond)

	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for w := 0; w < 6; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				got, err := repo.ClaimExpired(context.Background(), 4)
				if err != nil {
					t.Errorf("ClaimExpired: %v", err)
					return
				}
				if len(got) == 0 {
					return
				}
				mu.Lock()
				for _, r := range got {
					seen[r.LinkID]++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for id, n := range seen {
		if n != 1 {
			t.Errorf("link %s claimed %d times", id, n)
		}
		if !want[id] {
			t.Errorf("unexpected link %s", id)
		}
	}
	if len(seen) != total {
		t.Errorf("want all %d links claimed exactly once, got %d", total, len(seen))
	}
}

func TestExecutionLeases_UnleasedLinkIsNeverSwept(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	task, _ := domain.NewTask(uuid.NewString(), tenantID, "legacy", domain.StatusInProgress, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateExecutionLink(ctx, tenantID, task.ID, domain.EngineDirectAgent, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.ClaimExpired(ctx, 10); len(got) != 0 {
		t.Errorf("rows from before the lease migration must be left alone, got %+v", got)
	}
}

func TestExecutionLeases_ClaimForExecution_ExactlyOneConcurrentWinner(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	task, _ := domain.NewTask(uuid.NewString(), tenantID, "t", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	var wins int32
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.ClaimForExecution(ctx, tenantID, task.ID, domain.StatusOpen)
			if err != nil {
				t.Errorf("ClaimForExecution: %v", err)
				return
			}
			if ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("want exactly one winner among 12 concurrent claims, got %d", wins)
	}
	got, _ := repo.Get(ctx, tenantID, task.ID)
	if got.Status != domain.StatusInProgress {
		t.Errorf("want in_progress, got %s", got.Status)
	}
	if ok, _ := repo.ClaimForExecution(ctx, tenantID, task.ID, domain.StatusOpen); ok {
		t.Error("a claim from a stale status must fail")
	}
}

// The whole recovery path against a real database: a run starts, its process
// "dies" (no heartbeat), the lease expires, a sweep puts the task back.
func TestExecutionLeases_RecoveryRevertsAbandonedRun_EndToEnd(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	task, _ := domain.NewTask(uuid.NewString(), tenantID, "t", domain.StatusReview, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	link, err := repo.CreateExecutionLink(ctx, tenantID, task.ID, domain.EngineDirectAgent, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetActiveExecutionLink(ctx, tenantID, task.ID, link.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.ClaimForExecution(ctx, tenantID, task.ID, domain.StatusReview); err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	if err := repo.StartLease(ctx, tenantID, link.ID, "dead-process", string(domain.StatusReview), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)

	n, err := usecase.NewRecoverInterruptedExecutions(repo, repo).Execute(ctx)
	if err != nil || n != 1 {
		t.Fatalf("recovery = %d, %v; want 1, nil", n, err)
	}
	got, _ := repo.Get(ctx, tenantID, task.ID)
	if got.Status != domain.StatusReview {
		t.Errorf("task must return to the status before the dispatch (review), got %s", got.Status)
	}
	l, _ := repo.GetExecutionLink(ctx, tenantID, link.ID)
	if l.StatusMirror != "failed" || l.CompletedAt == nil {
		t.Errorf("link must be failed and completed, got %+v", l)
	}
	if again, _ := usecase.NewRecoverInterruptedExecutions(repo, repo).Execute(ctx); again != 0 {
		t.Errorf("a second sweep must be a no-op, reverted %d", again)
	}
}
