package contracttest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// PlanRevisionFailureEnv adds the job store of the plan-auto worker to the analysis-run environment.
type PlanRevisionFailureEnv struct {
	SolutionEnv
	Jobs usecase.PlanAutoJobRepository
}

// RunPlanRevisionFailureContract is what both dialects must do when people ask for changes to a Plan (CR-REQ-045): one
// run and one commit job per ask, a clear refusal for everybody else, and a retry that carries the failed ask.
func RunPlanRevisionFailureContract(t *testing.T, env PlanRevisionFailureEnv) {
	t.Helper()
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env PlanRevisionFailureEnv)
	}{
		{"ConcurrentRevisersStartOneRunAndOneJob", planConcurrentRevisers},
		{"AFailedRevisionIsRetriedWithItsAsk", planFailedRevisionRetry},
		{"OriginAcceptsRevisionAndRefusesUnknownValues", planJobOrigins},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

// planNoSpawn leaves the run running, as a worker that is still busy would.
type planNoSpawn struct{}

func (planNoSpawn) Spawn(domain.AnalysisRun) {}

type planAllowAll struct{}

func (planAllowAll) AuthorizePlan(context.Context, domain.Request) error       { return nil }
func (planAllowAll) AuthorizePlanRevise(context.Context, domain.Request) error { return nil }

type planRevisionKit struct {
	ctx      context.Context
	tenantID string
	userID   string
	req      domain.Request
	generate *usecase.GeneratePlan
}

func newPlanRevisionKit(t *testing.T, env PlanRevisionFailureEnv) *planRevisionKit {
	t.Helper()
	tenantID, userID := newTenant(), uuid.NewString()
	k := &planRevisionKit{tenantID: tenantID, userID: userID, ctx: tenant.WithUserID(CtxForTenant(tenantID), userID)}
	// A task needs no approved Analysis, so the Request can wait at the Plan gate with a Plan from the start.
	k.req = seedRequest(t, env.IntakeEnv, k.ctx, domain.RequestStatusAwaitingPlanApproval, func(r *domain.Request) {
		r.Type, r.TypeSource, r.Size, r.ReporterID, r.PlanTaskID = domain.RequestTypeTask, domain.TypeSourceHuman, domain.RequestSizeM, userID, uuid.NewString()
	})
	settings := usecase.DefaultAnalysisSettings()
	settings.Heartbeat = time.Hour
	k.generate = usecase.NewGeneratePlan(usecase.GeneratePlanDeps{
		Requests: env.Requests, Solutions: env.SolutionStore, Runs: env.AnalysisRuns, Conns: solConns{usecase.AnalysisConnection{ConnectionID: "c1"}},
		Auth: planAllowAll{}, Approvals: noopCanceller{}, Transition: realTransitioner(env.IntakeEnv), Tx: env.Tx, Spawner: planNoSpawn{},
		Settings: settings, LeaseOwner: "worker-1",
	}).WithRevisionWorker(env.Jobs, planAllowAll{})
	return k
}

func planConcurrentRevisers(t *testing.T, env PlanRevisionFailureEnv) {
	k := newPlanRevisionKit(t, env)
	const n = 6
	results := make([]usecase.GeneratePlanResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = retryDeadlock(func() error {
				res, err := k.generate.Execute(k.ctx, usecase.GeneratePlanInput{RequestID: k.req.ID, Feedback: "ASK-RACE", IdempotencyKey: "race-" + string(rune('a'+i))})
				results[i] = res
				return err
			})
		}()
	}
	wg.Wait()
	winner, lost := "", 0
	for i := range errs {
		switch {
		case errs[i] == nil && results[i].Created && winner == "":
			winner = results[i].RunID
		case hasErrorCode(errs[i], "REQUEST_PLAN_REVISION_IN_PROGRESS"):
			lost++
		default:
			t.Fatalf("racer %d: %+v %v", i, results[i], errs[i])
		}
	}
	if winner == "" || lost != n-1 {
		t.Fatalf("winner=%q lost=%d of %d", winner, lost, n)
	}
	active, found, err := env.Jobs.ActiveByRequest(k.ctx, k.req.ID)
	if err != nil || !found || active.Origin != domain.PlanAutoOriginRevision || active.RunID != winner || active.State != domain.PlanAutoRunning ||
		active.Anchor != domain.RevisionAnchor(winner) || active.RequestedBy != k.userID || active.MaxAttempts != 1 {
		t.Fatalf("the winner's job: %+v found=%v err=%v", active, found, err)
	}
	if n := env.CountRows(t, "analysis_runs", k.tenantID); n != 1 {
		t.Fatalf("%d run(s) stored, want exactly the winner's", n)
	}
	if n := env.CountRows(t, "plan_auto_jobs", k.tenantID); n != 1 {
		t.Fatalf("%d job(s) stored, want one", n)
	}

	// The winner's click repeated with its own key is the same run and no new job.
	var key string
	for i := range results {
		if results[i].RunID == winner && results[i].Created {
			key = "race-" + string(rune('a'+i))
		}
	}
	again, err := k.generate.Execute(k.ctx, usecase.GeneratePlanInput{RequestID: k.req.ID, Feedback: "ASK-RACE", IdempotencyKey: key})
	if err != nil || again.Created || again.RunID != winner {
		t.Fatalf("repeated click: %+v %v", again, err)
	}
	if got, err := env.Requests.Get(k.ctx, k.req.ID); err != nil || got.Status != domain.RequestStatusPlanning {
		t.Fatalf("request = %+v %v, want planning", got.Status, err)
	}
}

func planFailedRevisionRetry(t *testing.T, env PlanRevisionFailureEnv) {
	k := newPlanRevisionKit(t, env)
	const ask = "ASK-FAILED keep the schema and shrink the rollout"
	first, err := k.generate.Execute(k.ctx, usecase.GeneratePlanInput{RequestID: k.req.ID, Feedback: ask})
	if err != nil || !first.Created {
		t.Fatalf("first: %+v %v", first, err)
	}
	run, err := env.AnalysisRuns.Get(k.ctx, first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	owner := "worker-1"
	run.LeaseOwner = &owner
	run.Fail(domain.RunErrSpecTimeout, "timed out", time.Now().UTC())
	if ok, err := env.AnalysisRuns.FinishOwned(k.ctx, run, owner); err != nil || !ok {
		t.Fatalf("finish the run: ok=%v err=%v", ok, err)
	}

	// Until the worker has seen the run end, its job still counts as active: the retry is told so and changes nothing.
	_, err = k.generate.Execute(k.ctx, usecase.GeneratePlanInput{RequestID: k.req.ID})
	if !hasErrorCode(err, "REQUEST_PLAN_REVISION_IN_PROGRESS") {
		t.Fatalf("retry while the job is active: %v", err)
	}
	if n := env.CountRows(t, "analysis_runs", k.tenantID); n != 1 {
		t.Fatalf("a refused retry stored a run: %d", n)
	}

	claimed, err := env.Jobs.ClaimDue(k.ctx, "contract-worker", time.Minute, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range claimed {
		if j.RequestID != k.req.ID {
			continue
		}
		j.State, j.ReasonCode = domain.PlanAutoGaveUp, domain.RunErrSpecTimeout
		if ok, err := env.Jobs.Update(k.ctx, j, "contract-worker"); err != nil || !ok {
			t.Fatalf("give the job up: ok=%v err=%v", ok, err)
		}
	}

	retry, err := k.generate.Execute(k.ctx, usecase.GeneratePlanInput{RequestID: k.req.ID})
	if err != nil || !retry.Created || retry.RunID == first.RunID {
		t.Fatalf("retry: %+v %v", retry, err)
	}
	got, err := env.AnalysisRuns.Get(k.ctx, retry.RunID)
	if err != nil || got.Feedback != ask {
		t.Fatalf("the retry must carry the failed ask: %+v %v", got, err)
	}
	active, found, err := env.Jobs.ActiveByRequest(k.ctx, k.req.ID)
	if err != nil || !found || active.RunID != retry.RunID || active.Origin != domain.PlanAutoOriginRevision {
		t.Fatalf("the retry needs its own job: %+v found=%v err=%v", active, found, err)
	}
}

func planJobOrigins(t *testing.T, env PlanRevisionFailureEnv) {
	k := newPlanRevisionKit(t, env)
	job := func(origin domain.PlanAutoOrigin, state domain.PlanAutoState) domain.PlanAutoJob {
		return domain.PlanAutoJob{TenantID: k.tenantID, RequestID: k.req.ID, Anchor: "origin-" + uuid.NewString(), Origin: origin, ProjectID: k.req.ProjectID,
			State: state, Attempt: 1, MaxAttempts: 1}
	}
	for _, origin := range []domain.PlanAutoOrigin{domain.PlanAutoOriginEvent, domain.PlanAutoOriginRetry, domain.PlanAutoOriginRevision} {
		if ok, err := env.Jobs.Insert(k.ctx, job(origin, domain.PlanAutoDone)); err != nil || !ok {
			t.Fatalf("origin %q: inserted=%v err=%v", origin, ok, err)
		}
	}
	if ok, err := env.Jobs.Insert(k.ctx, job("manual", domain.PlanAutoDone)); err == nil && ok {
		t.Fatal("an origin nobody defined must be refused by the database")
	}
}
