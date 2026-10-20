package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// serialRuns serialises the fake store: the heartbeat and the worker's commit run on different goroutines
// and the in-memory fake is not safe for that on its own.
type serialRuns struct {
	AnalysisRunStore
	mu     sync.Mutex
	renews int
}

func (s *serialRuns) RenewLease(ctx context.Context, id, owner string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renews++
	return s.AnalysisRunStore.RenewLease(ctx, id, owner, ttl)
}

func (s *serialRuns) FinishOwned(ctx context.Context, run domain.AnalysisRun, owner string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.AnalysisRunStore.FinishOwned(ctx, run, owner)
}

func (s *serialRuns) renewCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.renews
}

// slowWorker stands in for an authoring run that outlasts many lease cycles.
type slowWorker struct {
	runs   AnalysisRunStore
	owner  string
	runFor time.Duration
	ctxErr error
}

func (s *slowWorker) Execute(ctx context.Context, run domain.AnalysisRun) error {
	select {
	case <-time.After(s.runFor):
	case <-ctx.Done():
		s.ctxErr = ctx.Err()
		return ctx.Err()
	}
	done := run
	done.Succeed(time.Now().UTC())
	ok, err := s.runs.FinishOwned(ctx, done, s.owner)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLeaseLost
	}
	return nil
}

func startedOpenSpecRun(t *testing.T) (*planKit, domain.AnalysisRun) {
	t.Helper()
	k := newPlanKit(fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/r"}})
	k.generate = NewGeneratePlan(GeneratePlanDeps{
		Requests: k.w, Solutions: fakeSolutions{k.w}, Runs: fakeRuns{k.w}, Conns: fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/r"}},
		Auth: NewPlanActorAuthorizer(allowAll{}, nil, false), Approvals: fakeCanceller{k.w}, Transition: k.w.transitioner(), Tx: k.w, Spawner: k.spawner,
		LeaseOwner: testOwner, Pin: &stubPinner{engine: domain.EngineOpenSpec},
	})
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	return k, k.startedRun(req.ID)
}

// A proposal run lasts minutes; a tiny heartbeat and lease stand in for 30 s and 90 s so it spans many more cycles than one lease holds.
func TestAnalysisRunner_AnAgentProposalRunSpanningManyLeaseCyclesFinishesOnItsOwn(t *testing.T) {
	k, run := startedOpenSpecRun(t)
	store := &serialRuns{AnalysisRunStore: fakeRuns{k.w}}
	settings := AnalysisSettings{Heartbeat: 5 * time.Millisecond, LeaseTTL: 20 * time.Millisecond}
	worker := &slowWorker{runs: store, owner: testOwner, runFor: 150 * time.Millisecond}
	runner := NewAnalysisRunner(store, fakeSolutions{k.w}, k.w, nil, nil, settings, testOwner).WithProposalWorker(worker)

	runner.execute(run)

	if worker.ctxErr != nil {
		t.Fatalf("the worker was cancelled while it still held the lease: %v", worker.ctxErr)
	}
	if got := store.renewCount(); got < 5 {
		t.Fatalf("lease renewed %d times; a run of many lease cycles needs a steady heartbeat", got)
	}
	if got := k.w.runs[run.ID].Status; got != domain.RunStatusSucceeded {
		t.Fatalf("run status = %s, want succeeded", got)
	}
}

func TestAnalysisRunner_AProposalRunWhoseLeaseIsTakenOverStopsAndIsNotFailedByThisInstance(t *testing.T) {
	k, run := startedOpenSpecRun(t)
	other := "another-instance"
	taken := k.w.runs[run.ID]
	taken.LeaseOwner = &other // the sweeper of another instance reclaimed the run
	k.w.runs[run.ID] = taken
	store := &serialRuns{AnalysisRunStore: fakeRuns{k.w}}
	settings := AnalysisSettings{Heartbeat: 5 * time.Millisecond, LeaseTTL: 20 * time.Millisecond}
	worker := &slowWorker{runs: store, owner: testOwner, runFor: 5 * time.Second}
	runner := NewAnalysisRunner(store, fakeSolutions{k.w}, k.w, nil, nil, settings, testOwner).WithProposalWorker(worker)

	done := make(chan struct{})
	go func() { runner.execute(run); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the worker kept running after another instance took the lease")
	}
	if worker.ctxErr == nil {
		t.Fatal("the worker was not cancelled")
	}
	// The new owner decides the outcome; this instance must not overwrite it.
	if got := k.w.runs[run.ID].Status; got != domain.RunStatusRunning {
		t.Fatalf("run status = %s, want it left running for the new owner", got)
	}
}
