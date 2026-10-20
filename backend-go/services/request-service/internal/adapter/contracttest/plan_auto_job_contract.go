package contracttest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RunPlanAutoJobContract is what both dialects must do for the stored automatic Plan generation jobs: one active job per
// request, claims that never overlap, lease-guarded updates and tenant isolation. The database must be empty of other
// plan-auto jobs because ClaimDue spans tenants.
func RunPlanAutoJobContract(t *testing.T, env Env, repo usecase.PlanAutoJobRepository) {
	t.Helper()
	tenantA, tenantB := newTenant(), newTenant()
	ctxA, ctxB := CtxForTenant(tenantA), CtxForTenant(tenantB)
	base := time.Now().UTC().Truncate(time.Microsecond)
	past, future := base.Add(-time.Hour), base.Add(time.Hour)

	// newJob makes a request of its own, because only one job per request may be active.
	newJob := func(t *testing.T, ctx context.Context, anchor string, state domain.PlanAutoState, nextCheck time.Time) domain.PlanAutoJob {
		t.Helper()
		tenantID, _ := tenant.TenantID(ctx)
		req := createRequest(t, env, ctx, nil)
		return domain.PlanAutoJob{
			TenantID: tenantID, RequestID: req.ID, Anchor: anchor, Origin: domain.PlanAutoOriginEvent, ProjectID: req.ProjectID, State: state,
			Attempt: 1, MaxAttempts: 2, NextCheckAt: nextCheck, CreatedAt: base, UpdatedAt: base,
		}
	}
	mustInsert := func(t *testing.T, ctx context.Context, j domain.PlanAutoJob) {
		t.Helper()
		if ok, err := repo.Insert(ctx, j); err != nil || !ok {
			t.Fatalf("insert %s/%s: inserted=%v err=%v", j.RequestID, j.Anchor, ok, err)
		}
	}

	t.Run("a request without a job reads as not found", func(t *testing.T) {
		for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
			if _, found, err := repo.LatestByRequest(ctxA, id); err != nil || found {
				t.Fatalf("latest %q: found=%v err=%v", id, found, err)
			}
			if _, found, err := repo.ActiveByRequest(ctxA, id); err != nil || found {
				t.Fatalf("active %q: found=%v err=%v", id, found, err)
			}
		}
	})

	t.Run("insert then read back every field", func(t *testing.T) {
		want := newJob(t, ctxA, "sol-"+uuid.NewString(), domain.PlanAutoPending, past)
		want.Origin, want.Attempt, want.MaxAttempts, want.CommitAttempts = domain.PlanAutoOriginRetry, 2, 3, 1
		want.RunID, want.ReasonCode, want.RequestedBy = uuid.NewString(), "REQUEST_AI_BUSY", uuid.NewString()
		mustInsert(t, ctxA, want)
		for name, read := range map[string]func(context.Context, string) (domain.PlanAutoJob, bool, error){"latest": repo.LatestByRequest, "active": repo.ActiveByRequest} {
			got, found, err := read(ctxA, want.RequestID)
			if err != nil || !found {
				t.Fatalf("%s: found=%v err=%v", name, found, err)
			}
			if got.TenantID != want.TenantID || got.RequestID != want.RequestID || got.Anchor != want.Anchor || got.Origin != want.Origin ||
				got.ProjectID != want.ProjectID || got.State != want.State || got.Attempt != 2 || got.MaxAttempts != 3 || got.RunID != want.RunID ||
				got.ReasonCode != want.ReasonCode || got.CommitAttempts != 1 || got.RequestedBy != want.RequestedBy || got.LeaseOwner != "" || got.LeaseExpiresAt != nil {
				t.Fatalf("%s: %+v, want %+v", name, got, want)
			}
			if !got.NextCheckAt.Equal(want.NextCheckAt) || !got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
				t.Fatalf("%s: times %v %v %v, want %v %v %v", name, got.NextCheckAt, got.CreatedAt, got.UpdatedAt, want.NextCheckAt, want.CreatedAt, want.UpdatedAt)
			}
		}
	})

	t.Run("a time left empty is the database's now", func(t *testing.T) {
		j := newJob(t, ctxA, "sol-"+uuid.NewString(), domain.PlanAutoPending, time.Time{})
		j.CreatedAt, j.UpdatedAt = time.Time{}, time.Time{}
		mustInsert(t, ctxA, j)
		got, found, err := repo.ActiveByRequest(ctxA, j.RequestID)
		if err != nil || !found || got.NextCheckAt.IsZero() || got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() || got.NextCheckAt.Before(base.Add(-time.Hour)) {
			t.Fatalf("defaults: %+v found=%v %v", got, found, err)
		}
	})

	t.Run("the same request and anchor cannot be inserted twice", func(t *testing.T) {
		j := newJob(t, ctxA, "sol-dup", domain.PlanAutoPending, past)
		mustInsert(t, ctxA, j)
		other := j
		other.State, other.Attempt = domain.PlanAutoRunning, 2
		if ok, err := repo.Insert(ctxA, other); err != nil || ok {
			t.Fatalf("duplicate key: inserted=%v err=%v", ok, err)
		}
		if got, _, _ := repo.LatestByRequest(ctxA, j.RequestID); got.State != domain.PlanAutoPending || got.Attempt != 1 {
			t.Fatalf("the first job must stay as it was: %+v", got)
		}
	})

	t.Run("a request has one active job whatever its state", func(t *testing.T) {
		j := newJob(t, ctxA, "sol-one", domain.PlanAutoPending, past)
		mustInsert(t, ctxA, j)
		for i, state := range []domain.PlanAutoState{domain.PlanAutoPending, domain.PlanAutoRunning, domain.PlanAutoCommitting} {
			second := j
			second.Anchor, second.State = fmt.Sprintf("sol-two-%d", i), state
			if ok, err := repo.Insert(ctxA, second); err != nil || ok {
				t.Fatalf("second active job (%s): inserted=%v err=%v", state, ok, err)
			}
		}
		if got, found, err := repo.ActiveByRequest(ctxA, j.RequestID); err != nil || !found || got.Anchor != "sol-one" {
			t.Fatalf("the first job stays the active one: %+v found=%v %v", got, found, err)
		}
	})

	t.Run("a finished job never blocks a new one, and the latest is the one created last", func(t *testing.T) {
		first := newJob(t, ctxA, "a1", domain.PlanAutoDone, past)
		mustInsert(t, ctxA, first)
		for i, state := range []domain.PlanAutoState{domain.PlanAutoGaveUp, domain.PlanAutoSkipped, domain.PlanAutoSuperseded} {
			j := first
			j.Anchor, j.State, j.CreatedAt, j.UpdatedAt = fmt.Sprintf("a%d", i+2), state, base.Add(time.Duration(i+1)*time.Second), base.Add(time.Duration(i+1)*time.Second)
			mustInsert(t, ctxA, j)
		}
		if _, found, err := repo.ActiveByRequest(ctxA, first.RequestID); err != nil || found {
			t.Fatalf("only finished jobs: active found=%v err=%v", found, err)
		}
		active := first
		active.Anchor, active.State, active.CreatedAt, active.UpdatedAt = "a5", domain.PlanAutoPending, base.Add(10*time.Second), base.Add(10*time.Second)
		mustInsert(t, ctxA, active)
		if got, found, _ := repo.ActiveByRequest(ctxA, first.RequestID); !found || got.Anchor != "a5" {
			t.Fatalf("active: %+v found=%v", got, found)
		}
		if got, _, _ := repo.LatestByRequest(ctxA, first.RequestID); got.Anchor != "a5" {
			t.Fatalf("latest: %+v", got)
		}
	})

	t.Run("ClaimDue leaves out jobs not due, finished, or still leased, and hands a lapsed lease to the next owner", func(t *testing.T) {
		due := newJob(t, ctxA, "claim-due", domain.PlanAutoPending, past)
		notYet := newJob(t, ctxA, "claim-later", domain.PlanAutoRunning, future)
		finished := newJob(t, ctxA, "claim-done", domain.PlanAutoDone, past)
		for _, j := range []domain.PlanAutoJob{due, notYet, finished} {
			mustInsert(t, ctxA, j)
		}
		got, err := repo.ClaimDue(context.Background(), "worker-1", time.Hour, 100)
		if err != nil {
			t.Fatal(err)
		}
		claimed := map[string]domain.PlanAutoJob{}
		for _, j := range got {
			claimed[j.RequestID] = j
		}
		if _, ok := claimed[notYet.RequestID]; ok {
			t.Error("a job that is not due yet was claimed")
		}
		if _, ok := claimed[finished.RequestID]; ok {
			t.Error("a finished job was claimed")
		}
		mine, ok := claimed[due.RequestID]
		if !ok || mine.LeaseOwner != "worker-1" || mine.LeaseExpiresAt == nil || !mine.LeaseExpiresAt.After(time.Now().Add(30*time.Minute)) || mine.Anchor != "claim-due" || mine.State != domain.PlanAutoPending {
			t.Fatalf("the due job must come back leased to worker-1: %+v (claimed %d)", mine, len(got))
		}
		again, err := repo.ClaimDue(context.Background(), "worker-2", time.Hour, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range again {
			if j.RequestID == due.RequestID {
				t.Fatal("a job under a live lease was claimed again")
			}
		}

		short := newJob(t, ctxA, "claim-short", domain.PlanAutoRunning, past)
		mustInsert(t, ctxA, short)
		if _, err := repo.ClaimDue(context.Background(), "worker-1", 50*time.Millisecond, 100); err != nil {
			t.Fatal(err)
		}
		time.Sleep(400 * time.Millisecond)
		later, err := repo.ClaimDue(context.Background(), "worker-2", time.Hour, 100)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, j := range later {
			found = found || (j.RequestID == short.RequestID && j.LeaseOwner == "worker-2")
		}
		if !found {
			t.Fatalf("a job whose lease lapsed must be claimable by another worker: %+v", later)
		}
	})

	t.Run("ClaimDue honours the batch size and serves every tenant", func(t *testing.T) {
		a := newJob(t, ctxA, "tenant-a", domain.PlanAutoPending, past.Add(-time.Hour))
		b := newJob(t, ctxB, "tenant-b", domain.PlanAutoPending, past.Add(-time.Hour))
		mustInsert(t, ctxA, a)
		mustInsert(t, ctxB, b)
		got, err := repo.ClaimDue(context.Background(), "worker-batch", time.Hour, 2)
		if err != nil || len(got) != 2 {
			t.Fatalf("batch of 2: got %d err=%v", len(got), err)
		}
		more, err := repo.ClaimDue(context.Background(), "worker-batch", time.Hour, 100)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]string{}
		for _, j := range append(got, more...) {
			seen[j.RequestID] = j.TenantID
		}
		if seen[a.RequestID] != tenantA || seen[b.RequestID] != tenantB {
			t.Fatalf("both tenants' due jobs must be claimed with their own tenant id: %v", seen)
		}
	})

	t.Run("workers claiming at the same time never receive the same job", func(t *testing.T) {
		const jobs = 12
		want := map[string]bool{}
		for i := 0; i < jobs; i++ {
			j := newJob(t, ctxA, fmt.Sprintf("race-%d", i), domain.PlanAutoPending, past.Add(-2*time.Hour))
			mustInsert(t, ctxA, j)
			want[j.RequestID] = true
		}
		var mu sync.Mutex
		count := map[string]int{}
		record := func(js []domain.PlanAutoJob) {
			mu.Lock()
			defer mu.Unlock()
			for _, j := range js {
				count[j.RequestID]++
			}
		}
		var wg sync.WaitGroup
		for w := 0; w < 4; w++ {
			wg.Add(1)
			go func(owner string) {
				defer wg.Done()
				for {
					js, err := repo.ClaimDue(context.Background(), owner, time.Hour, 3)
					if err != nil {
						t.Errorf("claim: %v", err)
						return
					}
					if len(js) == 0 {
						return
					}
					record(js)
				}
			}(fmt.Sprintf("racer-%d", w))
		}
		wg.Wait()
		// SKIP LOCKED may leave a row for a later pass while another worker holds its lock.
		for {
			js, err := repo.ClaimDue(context.Background(), "racer-sweep", time.Hour, 100)
			if err != nil {
				t.Fatal(err)
			}
			if len(js) == 0 {
				break
			}
			record(js)
		}
		for id := range want {
			if count[id] != 1 {
				t.Errorf("job of request %s was claimed %d times, want exactly once", id, count[id])
			}
		}
	})

	t.Run("Update needs the lease, writes the progress and releases it", func(t *testing.T) {
		j := newJob(t, ctxA, "upd", domain.PlanAutoPending, past.Add(-3*time.Hour))
		mustInsert(t, ctxA, j)
		var claimed domain.PlanAutoJob
		for {
			js, err := repo.ClaimDue(context.Background(), "owner-1", time.Hour, 100)
			if err != nil || len(js) == 0 {
				t.Fatalf("claim: %v %d", err, len(js))
			}
			for _, c := range js {
				if c.RequestID == j.RequestID {
					claimed = c
				}
			}
			if claimed.RequestID != "" {
				break
			}
		}

		next := claimed
		next.State, next.Attempt, next.RunID, next.ReasonCode, next.CommitAttempts = domain.PlanAutoRunning, 2, uuid.NewString(), "REQUEST_AI_COMPLETE_TIMEOUT", 3
		next.NextCheckAt = future
		next.MaxAttempts, next.Origin = 9, domain.PlanAutoOriginRetry // not part of the progress: must not be written
		if ok, err := repo.Update(ctxA, next, "someone-else"); err != nil || ok {
			t.Fatalf("update by a worker without the lease: ok=%v err=%v", ok, err)
		}
		if got, _, _ := repo.ActiveByRequest(ctxA, j.RequestID); got.State != domain.PlanAutoPending || got.Attempt != 1 || got.LeaseOwner != "owner-1" {
			t.Fatalf("a refused update must change nothing: %+v", got)
		}
		if ok, err := repo.Update(ctxB, next, "owner-1"); err != nil || ok {
			t.Fatalf("update from another tenant: ok=%v err=%v", ok, err)
		}

		if ok, err := repo.Update(ctxA, next, "owner-1"); err != nil || !ok {
			t.Fatalf("update by the lease owner: ok=%v err=%v", ok, err)
		}
		got, found, err := repo.ActiveByRequest(ctxA, j.RequestID)
		if err != nil || !found {
			t.Fatalf("read back: found=%v err=%v", found, err)
		}
		if got.State != domain.PlanAutoRunning || got.Attempt != 2 || got.RunID != next.RunID || got.ReasonCode != next.ReasonCode || got.CommitAttempts != 3 ||
			!got.NextCheckAt.Equal(future) || got.LeaseOwner != "" || got.LeaseExpiresAt != nil || got.MaxAttempts != 2 || got.Origin != domain.PlanAutoOriginEvent {
			t.Fatalf("progress not stored as given: %+v", got)
		}
		if got.UpdatedAt.Equal(base) {
			t.Fatalf("updated_at must move: %v", got.UpdatedAt)
		}
		if ok, err := repo.Update(ctxA, next, "owner-1"); err != nil || ok {
			t.Fatalf("the lease is released after one update: ok=%v err=%v", ok, err)
		}

	})

	t.Run("finishing a job frees the request for a new one", func(t *testing.T) {
		j := newJob(t, ctxA, "finish", domain.PlanAutoPending, past.Add(-4*time.Hour))
		mustInsert(t, ctxA, j)
		for {
			js, err := repo.ClaimDue(context.Background(), "owner-fin", time.Hour, 100)
			if err != nil || len(js) == 0 {
				t.Fatalf("claim: %v %d", err, len(js))
			}
			var mine *domain.PlanAutoJob
			for i := range js {
				if js[i].RequestID == j.RequestID {
					mine = &js[i]
				}
			}
			if mine == nil {
				continue
			}
			mine.State, mine.RunID, mine.ReasonCode = domain.PlanAutoGaveUp, "", "REQUEST_PLAN_COMMIT_FAILED"
			if ok, err := repo.Update(ctxA, *mine, "owner-fin"); err != nil || !ok {
				t.Fatalf("finish: ok=%v err=%v", ok, err)
			}
			break
		}
		if got, _, _ := repo.LatestByRequest(ctxA, j.RequestID); got.State != domain.PlanAutoGaveUp || got.RunID != "" || got.ReasonCode != "REQUEST_PLAN_COMMIT_FAILED" {
			t.Fatalf("finished job: %+v", got)
		}
		retry := j
		retry.Anchor, retry.Origin, retry.State, retry.NextCheckAt, retry.CreatedAt = "retry-"+uuid.NewString(), domain.PlanAutoOriginRetry, domain.PlanAutoPending, base, base.Add(time.Minute)
		mustInsert(t, ctxA, retry)
		if got, _, _ := repo.LatestByRequest(ctxA, j.RequestID); got.Anchor != retry.Anchor {
			t.Fatalf("latest after the retry: %+v", got)
		}
	})

	t.Run("a tenant sees and writes only its own jobs", func(t *testing.T) {
		j := newJob(t, ctxA, "iso", domain.PlanAutoPending, past)
		mustInsert(t, ctxA, j)
		if _, found, err := repo.LatestByRequest(ctxB, j.RequestID); err != nil || found {
			t.Fatalf("latest from another tenant: found=%v err=%v", found, err)
		}
		if _, found, err := repo.ActiveByRequest(ctxB, j.RequestID); err != nil || found {
			t.Fatalf("active from another tenant: found=%v err=%v", found, err)
		}
		foreign := j
		foreign.Anchor = "iso-2"
		if ok, err := repo.Insert(ctxB, foreign); err == nil && ok {
			t.Fatal("a job stamped with another tenant's id must not be inserted")
		}
		if _, err := repo.Insert(context.Background(), j); err == nil {
			t.Fatal("an insert without a tenant must fail")
		}
	})
}
