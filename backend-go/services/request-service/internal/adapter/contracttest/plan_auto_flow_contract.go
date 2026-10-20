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

// PlanAutoFlowEnv adds the job store to the analysis-run environment: the chain below runs over real Requests, Solutions, runs, processed events,
// outbox and jobs, with a stand-in for the AI worker and for task-service.
type PlanAutoFlowEnv struct {
	SolutionEnv
	Jobs usecase.PlanAutoJobRepository
}

// RunPlanAutoFlowContract is what both dialects must do for the whole chain of automatic Plan generation (CR-REQ-044): the status_changed event
// becomes one job, the worker starts a run, watches it, commits the Plan and ends the job, whatever is redelivered, raced or restarted.
func RunPlanAutoFlowContract(t *testing.T, env PlanAutoFlowEnv) {
	t.Helper()
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env PlanAutoFlowEnv)
	}{
		{"EventBecomesAPlanThroughTheWorker", flowEventToDone},
		{"TheSameEventTwiceIsOneJobAndOneRun", flowSameEventTwice},
		{"TwoWorkersGiveEachJobOneOwner", flowTwoWorkers},
		{"APersonWhoPressesGenerateIsJoinedNotDuplicated", flowPersonJoins},
		{"APersonWhoCommitsFirstEndsTheJobWithoutASecondPlan", flowPersonCommitsFirst},
		{"ABusyProjectDefersTheStartAndRunsItWhenFree", flowBusyProject},
		{"ARestartedWorkerCarriesOnFromTheStoredJob", flowRestart},
		{"AnInterruptedRunIsRetriedOnceThenGivenUp", flowInterruptedRun},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

const flowOwner = "w-flow"

// flowWriter is task-service for the commit: one Plan per Request, a repeat answers AlreadyExists with the same ids.
type flowWriter struct {
	mu    sync.Mutex
	trees map[string]usecase.PlanTreeResult
	calls int
}

func (w *flowWriter) CreatePlanTree(_ context.Context, in usecase.PlanTreeInput) (usecase.PlanTreeResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls++
	if tree, ok := w.trees[in.Request.ID]; ok && (in.SupersedesPlanID == "" || in.SupersedesPlanID != tree.PlanTaskID) {
		tree.AlreadyExists = true
		return tree, nil
	}
	tree := usecase.PlanTreeResult{PlanTaskID: uuid.NewString()}
	for range in.Proposal.Tasks {
		tree.TaskIDs = append(tree.TaskIDs, uuid.NewString())
	}
	w.trees[in.Request.ID] = tree
	return tree, nil
}

type flowOpener struct {
	mu    sync.Mutex
	opens map[string]int
}

func (o *flowOpener) Execute(_ context.Context, in usecase.OpenApprovalInput) (*domain.Approval, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.opens[in.RequestID]++
	return &domain.Approval{ID: uuid.NewString(), RequestID: in.RequestID}, nil
}

func (o *flowOpener) count(requestID string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.opens[requestID]
}

type flowSpawner struct {
	mu   sync.Mutex
	runs []domain.AnalysisRun
}

func (s *flowSpawner) Spawn(run domain.AnalysisRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = append(s.runs, run)
}

func (s *flowSpawner) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.runs)
}

// flowCanceller records why each pending approval was cancelled.
type flowCanceller struct {
	mu      sync.Mutex
	reasons []string
}

func (c *flowCanceller) CancelPending(_ context.Context, _, why string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reasons = append(c.reasons, why)
	return nil
}

func (c *flowCanceller) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.reasons...)
}

type flowEngine struct{ name domain.EngineName }

func (e flowEngine) Execute(context.Context, string) (domain.EngineName, error) { return e.name, nil }

// flowKit is the service side shared by every tenant of a scenario.
type flowKit struct {
	t        *testing.T
	env      PlanAutoFlowEnv
	spawner  *flowSpawner
	writer   *flowWriter
	opener   *flowOpener
	canceler *flowCanceller
	generate *usecase.GeneratePlan
	commit   *usecase.CommitPlan
	enqueue  *usecase.EnqueuePlanAuto
	policy   domain.PlanAutoPolicy
}

// flowPolicy has delays short enough for a test to wait on, and a window long enough that no job times out.
func flowPolicy() domain.PlanAutoPolicy {
	return domain.PlanAutoPolicy{MaxExtraAttempts: 1, RetryDelay: time.Millisecond, BusyDelay: time.Millisecond, StartWindow: time.Hour, MaxCommitAttempts: 3,
		PollInterval: time.Millisecond, LeaseTTL: time.Minute}
}

// newFlowKit wires the real use cases over the real stores. agentCap > 0 pins the openspec engine, so runs are agent runs capped per project.
func newFlowKit(t *testing.T, env PlanAutoFlowEnv, agentCap int) *flowKit {
	t.Helper()
	k := &flowKit{t: t, env: env, spawner: &flowSpawner{}, writer: &flowWriter{trees: map[string]usecase.PlanTreeResult{}}, opener: &flowOpener{opens: map[string]int{}}, canceler: &flowCanceller{}, policy: flowPolicy()}
	settings := usecase.DefaultAnalysisSettings()
	settings.Heartbeat = time.Hour
	deps := usecase.GeneratePlanDeps{
		Requests: env.Requests, Solutions: env.SolutionStore, Runs: env.AnalysisRuns, Conns: solConns{usecase.AnalysisConnection{ConnectionID: "c1", RepoPath: "/srv/repo"}},
		Auth: planAllowAll{}, Approvals: k.canceler, Transition: realTransitioner(env.IntakeEnv), Tx: env.Tx, Spawner: k.spawner, Settings: settings, LeaseOwner: flowOwner,
	}
	if agentCap > 0 {
		deps.Settings.MaxAgentRunsPerProject = agentCap
		deps.Pin = flowEngine{name: domain.EngineOpenSpec}
	}
	k.generate = usecase.NewGeneratePlan(deps).WithRevisionWorker(env.Jobs, planAllowAll{}) // as wirePlanAuto attaches it
	k.commit = usecase.NewCommitPlan(usecase.CommitPlanDeps{
		Requests: env.Requests, Auth: planAllowAll{}, Writer: k.writer, Policies: domain.NewPolicyRegistry(domain.PolicyDeps{}), Approvals: k.opener,
		Transition: realTransitioner(env.IntakeEnv), Tx: env.Tx, Outbox: env.Outbox, Limits: domain.DefaultPlanLimits,
	})
	k.enqueue = usecase.NewEnqueuePlanAuto(env.Jobs, env.Processed, env.Requests, env.SolutionStore, usecase.PlanAutoSwitch{EnvOn: true, CommitWired: true}, k.policy, env.Tx)
	return k
}

func (k *flowKit) worker(owner string) *usecase.PlanAutoWorker {
	return usecase.NewPlanAutoWorker(usecase.PlanAutoWorkerDeps{
		Jobs: k.env.Jobs, Requests: k.env.Requests, Runs: k.env.AnalysisRuns, Start: k.generate, Commit: k.commit,
		Switch: usecase.PlanAutoSwitch{EnvOn: true, CommitWired: true}, Policy: k.policy, Owner: owner,
	})
}

// flowTenant is one tenant with a `bug` in planning whose diagnosis was approved: what the event of an approved Analysis leaves behind.
type flowTenant struct {
	ctx      context.Context
	tenantID string
	userID   string
	req      domain.Request
	solution domain.Solution
}

func (k *flowKit) seed() flowTenant {
	k.t.Helper()
	tenantID, userID := newTenant(), uuid.NewString()
	ctx := tenant.WithUserID(CtxForTenant(tenantID), userID)
	req := seedRequest(k.t, k.env.IntakeEnv, ctx, domain.RequestStatusPlanning, func(r *domain.Request) {
		r.Type, r.TypeSource, r.Size, r.ReporterID = domain.RequestTypeBug, domain.TypeSourceHuman, domain.RequestSizeS, userID
	})
	now := time.Now().UTC().Truncate(time.Microsecond)
	sol := domain.Solution{ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, Kind: domain.SolutionKindDiagnosis, Status: domain.SolutionStatusApproved,
		OptionsJSON: []byte(`{"a":1}`), Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := k.env.SolutionStore.Insert(ctx, sol); err != nil {
		k.t.Fatalf("seed the approved diagnosis: %v", err)
	}
	return flowTenant{ctx: ctx, tenantID: tenantID, userID: userID, req: req, solution: sol}
}

func (k *flowKit) event(ft flowTenant, id string) usecase.PlanAutoEvent {
	return usecase.PlanAutoEvent{ID: id, TenantID: ft.tenantID, RequestID: ft.req.ID, ProjectID: ft.req.ProjectID, To: string(domain.RequestStatusPlanning), Trigger: string(domain.TriggerAnalysisApproved)}
}

func (k *flowKit) deliver(ft flowTenant, ev usecase.PlanAutoEvent) {
	k.t.Helper()
	if err := k.enqueue.Handle(CtxForTenant(ft.tenantID), ev); err != nil {
		k.t.Fatalf("enqueue %s: %v", ev.ID, err)
	}
}

func (k *flowKit) job(ft flowTenant) domain.PlanAutoJob {
	k.t.Helper()
	j, found, err := k.env.Jobs.LatestByRequest(ft.ctx, ft.req.ID)
	if err != nil || !found {
		k.t.Fatalf("no job for the Request: found=%v err=%v", found, err)
	}
	return j
}

func (k *flowKit) planRuns(ft flowTenant) []domain.AnalysisRun {
	k.t.Helper()
	recent, err := k.env.AnalysisRuns.ListRecent(ft.ctx, ft.req.ID, 20)
	if err != nil {
		k.t.Fatal(err)
	}
	var out []domain.AnalysisRun
	for i := len(recent) - 1; i >= 0; i-- { // oldest first
		if recent[i].Kind == domain.RunKindPlan {
			out = append(out, recent[i])
		}
	}
	return out
}

func (k *flowKit) reload(ft flowTenant) domain.Request {
	k.t.Helper()
	r, err := k.env.Requests.Get(ft.ctx, ft.req.ID)
	if err != nil {
		k.t.Fatal(err)
	}
	return r
}

// pass runs the worker until it has claimed nothing new for a moment or `steps` passes went by; the delays of flowPolicy are milliseconds.
func (k *flowKit) pass(w *usecase.PlanAutoWorker, steps int) {
	k.t.Helper()
	for i := 0; i < steps; i++ {
		if _, err := w.RunOnce(context.Background(), 10); err != nil {
			k.t.Fatalf("worker pass: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// until passes the worker until cond holds or five seconds went by.
func (k *flowKit) until(w *usecase.PlanAutoWorker, what string, cond func() bool) {
	k.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		k.pass(w, 1)
	}
	k.t.Fatalf("timed out waiting for %s", what)
}

// finishRun ends a running run the way the AI worker does: the proposal is the run's output, or it fails with a code.
func (k *flowKit) finishRun(ft flowTenant, runID string, failCode string) {
	k.t.Helper()
	run, err := k.env.AnalysisRuns.Get(ft.ctx, runID)
	if err != nil {
		k.t.Fatal(err)
	}
	if failCode != "" {
		run.Fail(failCode, "scripted", time.Now())
	} else {
		raw, err := usecase.EncodePlanProposal(flowProposal())
		if err != nil {
			k.t.Fatal(err)
		}
		run.Succeed(time.Now())
		run.RawOutput = &raw
	}
	if ok, err := k.env.AnalysisRuns.FinishOwned(ft.ctx, run, flowOwner); err != nil || !ok {
		k.t.Fatalf("finish run %s: ok=%v err=%v", runID, ok, err)
	}
}

// flowKey is the idempotency key of the run a job starts for an approved Solution on its nth attempt.
func flowKey(t *testing.T, solutionID string, attempt int) string {
	t.Helper()
	key, err := domain.PlanAutoKey(domain.SolutionAnchor(solutionID), attempt)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// flowProposal is a three-task plan a bug of size S accepts: the last task carries the regression label.
func flowProposal() domain.PlanProposal {
	return domain.PlanProposal{Title: "Plan", Summary: "s", Tasks: []domain.TaskProposal{
		{Title: "Prepare", Description: "Prepare. Done when the branch builds.", TaskType: "task", EstimatedHours: 2},
		{Title: "Fix", Description: "Fix. Done when the unit tests pass.", TaskType: "task", EstimatedHours: 2, DependsOnIndices: []int{0}},
		{Title: "Verify", Description: "Verify. Done when the symptom is gone.", TaskType: "task", EstimatedHours: 2, DependsOnIndices: []int{1}, Labels: []string{"test:regression"}},
	}}
}

func (k *flowKit) requireDone(ft flowTenant, w *usecase.PlanAutoWorker) {
	k.t.Helper()
	k.until(w, "the job to be done", func() bool { return k.job(ft).State == domain.PlanAutoDone })
	req := k.reload(ft)
	if req.Status != domain.RequestStatusAwaitingPlanApproval || req.PlanTaskID == "" {
		k.t.Fatalf("after the job is done the Request is %s with Plan %q", req.Status, req.PlanTaskID)
	}
}

func flowEventToDone(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 0)
	ft := k.seed()
	k.deliver(ft, k.event(ft, uuid.NewString()))
	job := k.job(ft)
	if job.State != domain.PlanAutoPending || job.Origin != domain.PlanAutoOriginEvent || job.Attempt != 1 || job.MaxAttempts != 2 || job.Anchor != domain.SolutionAnchor(ft.solution.ID) || job.RunID != "" {
		t.Fatalf("job after the event: %+v", job)
	}
	w := k.worker("w1")
	k.until(w, "the run to start", func() bool { return k.job(ft).State == domain.PlanAutoRunning })
	runs := k.planRuns(ft)
	key := ""
	if len(runs) == 1 && runs[0].IdempotencyKey != nil {
		key = *runs[0].IdempotencyKey
	}
	if len(runs) != 1 || key != flowKey(t, ft.solution.ID, 1) || runs[0].ActorID != ft.userID || k.job(ft).RunID != runs[0].ID || k.spawner.count() != 1 {
		t.Fatalf("runs after the start: %+v key=%q spawned=%d job=%+v", runs, key, k.spawner.count(), k.job(ft))
	}
	k.pass(w, 3)
	if k.job(ft).State != domain.PlanAutoRunning {
		t.Fatalf("a run that has not finished must keep the job running: %+v", k.job(ft))
	}
	k.finishRun(ft, runs[0].ID, "")
	k.requireDone(ft, w)
	if k.writer.calls != 1 || k.opener.count(ft.req.ID) != 1 {
		t.Fatalf("the Plan was written %d time(s) and its approval opened %d time(s)", k.writer.calls, k.opener.count(ft.req.ID))
	}
	if got := k.env.CountRows(t, "plan_auto_jobs", ft.tenantID); got != 1 {
		t.Fatalf("%d jobs", got)
	}
}

func flowSameEventTwice(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 0)
	ft := k.seed()
	ev := k.event(ft, uuid.NewString())
	k.deliver(ft, ev)
	k.deliver(ft, ev)                            // the bus redelivers the very same event
	k.deliver(ft, k.event(ft, uuid.NewString())) // a second event for the same approved Analysis (another id, same anchor)
	if got := k.env.CountRows(t, "plan_auto_jobs", ft.tenantID); got != 1 {
		t.Fatalf("%d jobs after the same Analysis was announced three times", got)
	}
	w := k.worker("w1")
	k.until(w, "the run to start", func() bool { return k.job(ft).State == domain.PlanAutoRunning })
	k.pass(w, 3)
	runs := k.planRuns(ft)
	if len(runs) != 1 || k.spawner.count() != 1 {
		t.Fatalf("%d runs (%d spawned) for one Analysis", len(runs), k.spawner.count())
	}
	k.finishRun(ft, runs[0].ID, "")
	k.requireDone(ft, w)
	if k.writer.calls != 1 {
		t.Fatalf("the Plan was written %d times", k.writer.calls)
	}
}

func flowTwoWorkers(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 0)
	var tenants []flowTenant
	for i := 0; i < 6; i++ {
		ft := k.seed()
		k.deliver(ft, k.event(ft, uuid.NewString()))
		tenants = append(tenants, ft)
	}
	w1, w2 := k.worker("w1"), k.worker("w2")
	var wg sync.WaitGroup
	claimed := make([]int, 2)
	for i, w := range []*usecase.PlanAutoWorker{w1, w2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := w.RunOnce(context.Background(), 3)
			if err != nil {
				t.Errorf("worker %d: %v", i+1, err)
			}
			claimed[i] = n
		}()
	}
	wg.Wait()
	// A claim that finds the rows locked by the other worker skips them (MySQL may skip more than it must), so a pass can take fewer than
	// were due, never more: what is left is taken by the next pass.
	if claimed[0]+claimed[1] > len(tenants) {
		t.Fatalf("the two workers claimed %d + %d jobs for %d due jobs: a job was taken twice", claimed[0], claimed[1], len(tenants))
	}
	for pass := 0; pass < 10 && k.spawner.count() < len(tenants); pass++ {
		k.pass(w1, 1)
		k.pass(w2, 1)
	}
	if k.spawner.count() != len(tenants) {
		t.Fatalf("%d runs started for %d jobs", k.spawner.count(), len(tenants))
	}
	for _, ft := range tenants {
		if runs := k.planRuns(ft); len(runs) != 1 {
			t.Fatalf("a Request got %d runs", len(runs))
		}
	}
}

func flowPersonJoins(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 0)
	ft := k.seed()
	k.deliver(ft, k.event(ft, uuid.NewString()))
	click, err := k.generate.Execute(ft.ctx, usecase.GeneratePlanInput{RequestID: ft.req.ID}) // a person presses Generate before the worker looks
	if err != nil || !click.Created {
		t.Fatalf("manual generate: %+v %v", click, err)
	}
	w := k.worker("w1")
	k.until(w, "the job to meet the person's run", func() bool { return k.job(ft).State == domain.PlanAutoRunning })
	if job := k.job(ft); job.RunID != click.RunID {
		t.Fatalf("the job watches run %q, the person's run is %q", job.RunID, click.RunID)
	}
	k.finishRun(ft, click.RunID, "")
	k.requireDone(ft, w)
	if runs := k.planRuns(ft); len(runs) != 1 {
		t.Fatalf("%d runs for a click and a job", len(runs))
	}
	if k.writer.calls != 1 || k.opener.count(ft.req.ID) != 1 {
		t.Fatalf("Plans written %d, approvals opened %d", k.writer.calls, k.opener.count(ft.req.ID))
	}
}

func flowPersonCommitsFirst(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 0)
	w := k.worker("w1")

	// The browser commits the proposal it read while the worker still believes the run goes on: the job steps aside.
	early := k.seed()
	k.deliver(early, k.event(early, uuid.NewString()))
	k.until(w, "the run to start", func() bool { return k.job(early).State == domain.PlanAutoRunning })
	k.finishRun(early, k.planRuns(early)[0].ID, "")
	if _, err := k.commit.Execute(early.ctx, usecase.CommitPlanInput{RequestID: early.req.ID, Proposal: flowProposal()}); err != nil {
		t.Fatalf("manual commit: %v", err)
	}
	k.until(w, "the job to end", func() bool { return !k.job(early).State.IsActive() })
	if job := k.job(early); job.State != domain.PlanAutoSuperseded || job.ReasonCode != "REQUEST_PLAN_REQUEST_MOVED" {
		t.Fatalf("a person who got there first must supersede the job: %+v", job)
	}

	// The same commit after the worker saw the finished run: the Plan the job was writing is there, so the job is done.
	late := k.seed()
	k.deliver(late, k.event(late, uuid.NewString()))
	k.until(w, "the run to start", func() bool { return k.job(late).State == domain.PlanAutoRunning })
	k.finishRun(late, k.planRuns(late)[0].ID, "")
	k.until(w, "the job to start committing", func() bool { return k.job(late).State == domain.PlanAutoCommitting })
	if _, err := k.commit.Execute(late.ctx, usecase.CommitPlanInput{RequestID: late.req.ID, Proposal: flowProposal()}); err != nil {
		t.Fatalf("manual commit: %v", err)
	}
	k.until(w, "the job to end", func() bool { return !k.job(late).State.IsActive() })
	if job := k.job(late); job.State != domain.PlanAutoDone {
		t.Fatalf("a Plan that is already there ends the committing job as done: %+v", job)
	}
	for _, ft := range []flowTenant{early, late} {
		if k.opener.count(ft.req.ID) != 1 {
			t.Fatalf("the approval was opened %d times", k.opener.count(ft.req.ID))
		}
		if req := k.reload(ft); req.Status != domain.RequestStatusAwaitingPlanApproval || req.PlanTaskID == "" {
			t.Fatalf("Request = %s / %q", req.Status, req.PlanTaskID)
		}
	}
}

func flowBusyProject(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 1)
	ft := k.seed()
	// Another Request of the same project already holds the only agent slot.
	other := seedRequest(t, env.IntakeEnv, ft.ctx, domain.RequestStatusPlanning, func(r *domain.Request) {
		r.Type, r.TypeSource, r.Size, r.ReporterID, r.ProjectID = domain.RequestTypeBug, domain.TypeSourceHuman, domain.RequestSizeS, ft.userID, ft.req.ProjectID
	})
	if err := env.AnalysisRuns.EnsureProjectGate(ft.ctx, ft.req.ProjectID); err != nil {
		t.Fatal(err)
	}
	owner := flowOwner
	blocker := domain.AnalysisRun{ID: uuid.NewString(), TenantID: ft.tenantID, RequestID: other.ID, Kind: domain.RunKindPlan, Mode: domain.AnalysisModeAgentProposal, Status: domain.RunStatusRunning,
		Attempt: 1, LeaseOwner: &owner, ProjectID: ft.req.ProjectID, ActorID: ft.userID}
	if err := env.Tx.InTx(ft.ctx, func(ctx context.Context) error {
		_, err := env.AnalysisRuns.StartRun(ctx, blocker, domain.Solution{}, usecase.StartRunOptions{LeaseTTL: time.Minute})
		return err
	}); err != nil {
		t.Fatalf("hold the agent slot: %v", err)
	}
	k.deliver(ft, k.event(ft, uuid.NewString()))
	w := k.worker("w1")
	k.pass(w, 6)
	if job := k.job(ft); job.State != domain.PlanAutoPending || job.RunID != "" || len(k.planRuns(ft)) != 0 {
		t.Fatalf("a busy project must leave the job pending with no run: %+v runs=%d", job, len(k.planRuns(ft)))
	}
	blockerRun, err := env.AnalysisRuns.Get(ft.ctx, blocker.ID)
	if err != nil {
		t.Fatal(err)
	}
	blockerRun.Succeed(time.Now())
	if ok, err := env.AnalysisRuns.FinishOwned(ft.ctx, blockerRun, flowOwner); err != nil || !ok {
		t.Fatalf("free the slot: %v %v", ok, err)
	}
	k.until(w, "the run to start once the slot is free", func() bool { return k.job(ft).State == domain.PlanAutoRunning })
	runs := k.planRuns(ft)
	if len(runs) != 1 || runs[0].Mode != domain.AnalysisModeAgentProposal {
		t.Fatalf("runs = %+v", runs)
	}
	k.finishRun(ft, runs[0].ID, "")
	k.requireDone(ft, w)
}

func flowRestart(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 0)
	ft := k.seed()
	k.deliver(ft, k.event(ft, uuid.NewString()))
	before := k.worker("w-before")
	k.until(before, "the run to start", func() bool { return k.job(ft).State == domain.PlanAutoRunning })
	runs := k.planRuns(ft)
	k.finishRun(ft, runs[0].ID, "")
	// The process died here: a new worker (another owner, nothing in memory) takes over from the stored job.
	after := k.worker("w-after")
	k.requireDone(ft, after)
	if len(k.planRuns(ft)) != 1 || k.spawner.count() != 1 || k.writer.calls != 1 {
		t.Fatalf("a restart repeated work: runs=%d spawned=%d writes=%d", len(k.planRuns(ft)), k.spawner.count(), k.writer.calls)
	}
}

func flowInterruptedRun(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 0)
	ft := k.seed()
	k.deliver(ft, k.event(ft, uuid.NewString()))
	w := k.worker("w1")
	k.until(w, "the first run to start", func() bool { return k.job(ft).State == domain.PlanAutoRunning })
	first := k.planRuns(ft)[0]
	k.finishRun(ft, first.ID, "REQUEST_SOLUTION_RUN_INTERRUPTED") // the sweeper's verdict on a run whose worker died
	k.until(w, "the second run to start", func() bool {
		j := k.job(ft)
		return j.State == domain.PlanAutoRunning && j.Attempt == 2 && j.RunID != "" && j.RunID != first.ID
	})
	runs := k.planRuns(ft)
	if len(runs) != 2 || runs[1].IdempotencyKey == nil || *runs[1].IdempotencyKey != flowKey(t, ft.solution.ID, 2) {
		t.Fatalf("runs after the interruption: %+v", runs)
	}
	k.finishRun(ft, runs[1].ID, "REQUEST_SOLUTION_RUN_INTERRUPTED") // interrupted again: the one extra attempt is spent
	k.until(w, "the job to give up", func() bool { return k.job(ft).State == domain.PlanAutoGaveUp })
	if job := k.job(ft); job.ReasonCode != "REQUEST_SOLUTION_RUN_INTERRUPTED" || len(k.planRuns(ft)) != 2 {
		t.Fatalf("after the second interruption: %+v with %d runs", job, len(k.planRuns(ft)))
	}
	if req := k.reload(ft); req.Status != domain.RequestStatusPlanning || req.PlanTaskID != "" {
		t.Fatalf("a job that gave up must leave the Request in planning: %s / %q", req.Status, req.PlanTaskID)
	}
}
