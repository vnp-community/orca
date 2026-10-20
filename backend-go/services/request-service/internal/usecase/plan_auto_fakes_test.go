package usecase

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// memPlanAutoJobs mirrors the contract of the real repositories: one job per (request, anchor), one active job per
// request, lease-guarded updates and a claim that never hands the same job twice.
type memPlanAutoJobs struct {
	mu      sync.Mutex
	now     func() time.Time
	jobs    map[string]domain.PlanAutoJob
	order   []string
	inserts int
	// failClaim, failInsert and failUpdate make the matching call fail once.
	failInsert error
	failUpdate error
	updates    []domain.PlanAutoJob
}

func newMemPlanAutoJobs(now func() time.Time) *memPlanAutoJobs {
	return &memPlanAutoJobs{now: now, jobs: map[string]domain.PlanAutoJob{}}
}

func planAutoKeyOf(j domain.PlanAutoJob) string { return j.RequestID + "|" + j.Anchor }

func (m *memPlanAutoJobs) Insert(_ context.Context, job domain.PlanAutoJob) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failInsert != nil {
		err := m.failInsert
		m.failInsert = nil
		return false, err
	}
	if _, dup := m.jobs[planAutoKeyOf(job)]; dup {
		return false, nil
	}
	if job.State.IsActive() {
		for _, j := range m.jobs {
			if j.RequestID == job.RequestID && j.State.IsActive() {
				return false, nil
			}
		}
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = m.now()
	}
	job.UpdatedAt = job.CreatedAt
	m.jobs[planAutoKeyOf(job)] = job
	m.order = append(m.order, planAutoKeyOf(job))
	m.inserts++
	return true, nil
}

func (m *memPlanAutoJobs) LatestByRequest(_ context.Context, requestID string) (domain.PlanAutoJob, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best domain.PlanAutoJob
	found := false
	for _, k := range m.order {
		j := m.jobs[k]
		if j.RequestID == requestID && (!found || !j.CreatedAt.Before(best.CreatedAt)) {
			best, found = j, true
		}
	}
	return best, found, nil
}

func (m *memPlanAutoJobs) ActiveByRequest(_ context.Context, requestID string) (domain.PlanAutoJob, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.order {
		if j := m.jobs[k]; j.RequestID == requestID && j.State.IsActive() {
			return j, true, nil
		}
	}
	return domain.PlanAutoJob{}, false, nil
}

func (m *memPlanAutoJobs) ClaimDue(_ context.Context, owner string, ttl time.Duration, batch int) ([]domain.PlanAutoJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	var due []domain.PlanAutoJob
	for _, k := range m.order {
		j := m.jobs[k]
		leaseFree := j.LeaseOwner == "" || (j.LeaseExpiresAt != nil && j.LeaseExpiresAt.Before(now))
		if j.State.IsActive() && !j.NextCheckAt.After(now) && leaseFree {
			due = append(due, j)
		}
	}
	sort.SliceStable(due, func(a, b int) bool { return due[a].NextCheckAt.Before(due[b].NextCheckAt) })
	if len(due) > batch {
		due = due[:batch]
	}
	for i := range due {
		exp := now.Add(ttl)
		due[i].LeaseOwner, due[i].LeaseExpiresAt = owner, &exp
		m.jobs[planAutoKeyOf(due[i])] = due[i]
	}
	return due, nil
}

func (m *memPlanAutoJobs) Update(_ context.Context, job domain.PlanAutoJob, owner string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failUpdate != nil {
		err := m.failUpdate
		m.failUpdate = nil
		return false, err
	}
	cur, ok := m.jobs[planAutoKeyOf(job)]
	if !ok || cur.LeaseOwner != owner || owner == "" {
		return false, nil
	}
	cur.State, cur.Attempt, cur.RunID, cur.ReasonCode, cur.CommitAttempts, cur.NextCheckAt = job.State, job.Attempt, job.RunID, job.ReasonCode, job.CommitAttempts, job.NextCheckAt
	cur.LeaseOwner, cur.LeaseExpiresAt, cur.UpdatedAt = "", nil, m.now()
	m.jobs[planAutoKeyOf(cur)] = cur
	m.updates = append(m.updates, cur)
	return true, nil
}

// seed stores a job as given (state, attempt and schedule included), for tests that start mid-way.
func (m *memPlanAutoJobs) seed(job domain.PlanAutoJob) domain.PlanAutoJob {
	if job.CreatedAt.IsZero() {
		job.CreatedAt = m.now()
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = job.CreatedAt
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[planAutoKeyOf(job)] = job
	m.order = append(m.order, planAutoKeyOf(job))
	return job
}

func (m *memPlanAutoJobs) get(requestID, anchor string) domain.PlanAutoJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[requestID+"|"+anchor]
}

func (m *memPlanAutoJobs) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.jobs)
}

// memProcessedEvents is a processed_events table that forgets marks of a rolled-back transaction.
type memProcessedEvents struct {
	mu   sync.Mutex
	seen map[string]bool
}

func newMemProcessedEvents() *memProcessedEvents { return &memProcessedEvents{seen: map[string]bool{}} }

func (m *memProcessedEvents) MarkProcessed(_ context.Context, id, _ string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.seen[id] {
		return true, nil
	}
	m.seen[id] = true
	return false, nil
}

func (m *memProcessedEvents) Prune(context.Context, time.Time) (int64, error) { return 0, nil }

func (m *memProcessedEvents) snapshot() map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make(map[string]bool, len(m.seen))
	for k, v := range m.seen {
		cp[k] = v
	}
	return cp
}

func (m *memProcessedEvents) restore(s map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen = s
}

// planAutoTx rolls the processed marks and the jobs back with the request world, as one database transaction would.
type planAutoTx struct {
	w    *solWorld
	proc *memProcessedEvents
	jobs *memPlanAutoJobs
}

func (t planAutoTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	if t.w.InTransaction(ctx) {
		return fn(ctx)
	}
	return t.w.InTx(ctx, func(ctx context.Context) error {
		procBefore := t.proc.snapshot()
		t.jobs.mu.Lock()
		jobsBefore := make(map[string]domain.PlanAutoJob, len(t.jobs.jobs))
		for k, v := range t.jobs.jobs {
			jobsBefore[k] = v
		}
		orderBefore := append([]string(nil), t.jobs.order...)
		t.jobs.mu.Unlock()
		err := fn(ctx)
		if err != nil {
			t.proc.restore(procBefore)
			t.jobs.mu.Lock()
			t.jobs.jobs, t.jobs.order = jobsBefore, orderBefore
			t.jobs.mu.Unlock()
		}
		return err
	})
}

// failingEngineRows makes the per-project settings read fail.
type failingEngineRows struct{ err error }

func (f failingEngineRows) Get(context.Context, string) (domain.ProjectEngineSettings, bool, error) {
	return domain.ProjectEngineSettings{}, false, f.err
}

func (f failingEngineRows) Upsert(_ context.Context, s domain.ProjectEngineSettings, _ int64) (domain.ProjectEngineSettings, error) {
	return s, f.err
}

func planAutoTenantCtx() context.Context { return tenant.WithTenantID(context.Background(), "t1") }
