package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// countingStarter wraps the real GeneratePlan so tests can count calls and inject failures.
type countingStarter struct {
	inner PlanAutoStarter
	calls int
	keys  []string
	err   error
}

func (c *countingStarter) ExecuteForSystem(ctx context.Context, in GeneratePlanInput) (GeneratePlanResult, error) {
	c.calls++
	c.keys = append(c.keys, in.IdempotencyKey)
	if c.err != nil {
		return GeneratePlanResult{}, c.err
	}
	return c.inner.ExecuteForSystem(ctx, in)
}

type countingCommitter struct {
	inner PlanAutoCommitter
	calls int
	err   error
}

func (c *countingCommitter) ExecuteForSystem(ctx context.Context, in CommitPlanInput) (CommitPlanResult, error) {
	c.calls++
	if c.err != nil {
		return CommitPlanResult{}, c.err
	}
	return c.inner.ExecuteForSystem(ctx, in)
}

type workerKit struct {
	*planKit
	jobs   *memPlanAutoJobs
	start  *countingStarter
	commit *countingCommitter
	rows   *memEngineSettings
	worker *PlanAutoWorker
	policy domain.PlanAutoPolicy
	clock  time.Time
	req    domain.Request
}

const workerOwner = "worker-1"

func newWorkerKit(t *testing.T, typ domain.RequestType) *workerKit {
	t.Helper()
	k := &workerKit{planKit: newPlanKit(nil), policy: domain.DefaultPlanAutoPolicy, clock: time.Now().UTC(), rows: &memEngineSettings{rows: map[string]domain.ProjectEngineSettings{}}}
	k.jobs = newMemPlanAutoJobs(func() time.Time { return k.clock })
	k.start = &countingStarter{inner: k.generate}
	k.commit = &countingCommitter{inner: k.planKit.commit}
	req := k.w.seedRequest(typ, domain.RequestStatusPlanning)
	r := k.w.requests[req.ID]
	r.Size = domain.RequestSizeS
	k.w.requests[req.ID] = r
	k.req = r
	k.build()
	return k
}

func (k *workerKit) build() {
	k.worker = NewPlanAutoWorker(PlanAutoWorkerDeps{
		Jobs: k.jobs, Requests: k.w, Runs: fakeRuns{k.w}, Start: k.start, Commit: k.commit,
		Switch: PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: k.rows}, Policy: k.policy, Owner: workerOwner,
	})
	k.worker.now = func() time.Time { return k.clock }
}

// seedJob stores a pending event job due now, like the consumer would have.
func (k *workerKit) seedJob(mod func(j *domain.PlanAutoJob)) domain.PlanAutoJob {
	j := domain.PlanAutoJob{
		TenantID: "t1", RequestID: k.req.ID, Anchor: "sol-1", Origin: domain.PlanAutoOriginEvent, ProjectID: k.req.ProjectID,
		State: domain.PlanAutoPending, Attempt: 1, MaxAttempts: k.policy.MaxAttempts(), NextCheckAt: k.clock, CreatedAt: k.clock,
	}
	if mod != nil {
		mod(&j)
	}
	return k.jobs.seed(j)
}

func (k *workerKit) job() domain.PlanAutoJob { return k.jobs.get(k.req.ID, "sol-1") }

// pass advances the clock by d and runs one pass of the worker.
func (k *workerKit) pass(d time.Duration) int {
	k.clock = k.clock.Add(d)
	n, err := k.worker.RunOnce(planAutoTenantCtx(), 10)
	if err != nil {
		panic(err)
	}
	return n
}

func (k *workerKit) planRuns() []domain.AnalysisRun {
	var out []domain.AnalysisRun
	for _, r := range k.w.runs {
		if r.Kind == domain.RunKindPlan {
			out = append(out, r)
		}
	}
	return out
}

func (k *workerKit) failRun(runID, code string) {
	r := k.w.runs[runID]
	now := k.clock
	r.Status, r.ErrorCode, r.FinishedAt = domain.RunStatusFailed, &code, &now
	k.w.runs[runID] = r
}

func (k *workerKit) succeedRun(t *testing.T, runID string) {
	t.Helper()
	raw, err := EncodePlanProposal(proposalOf(t, false))
	if err != nil {
		t.Fatal(err)
	}
	r := k.w.runs[runID]
	now := k.clock
	r.Status, r.RawOutput, r.FinishedAt = domain.RunStatusSucceeded, &raw, &now
	k.w.runs[runID] = r
}
