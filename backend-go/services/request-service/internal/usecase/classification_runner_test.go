package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type runnerFixture struct {
	*proposeFixture
	runner *ClassificationRunner
}

func newRunnerFixture(out func(int) (domain.ClassificationProposal, error), ttl time.Duration) *runnerFixture {
	p := newProposeFixture(out)
	r := NewClassificationRunner(p.s, runView{p.s}, p.uc, p.s, "instance-a", ttl)
	return &runnerFixture{proposeFixture: p, runner: r}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (f *runnerFixture) runStatus(id string) domain.ClassificationRunStatus {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	return f.s.runs[id].Status
}

func TestRunner_EnqueueReturnsRunIDAndCompletesInBackground(t *testing.T) {
	release := make(chan struct{})
	f := newRunnerFixture(func(int) (domain.ClassificationProposal, error) {
		<-release
		return okProposal(domain.RequestTypeBug)(1)
	}, time.Minute)
	defer f.runner.Close()
	r := f.classifying(t)

	res, err := f.runner.Enqueue(tctx(), EnqueueClassificationInput{RequestID: r.ID, Manual: true})
	if err != nil || !res.Started || res.Run.ID == "" {
		t.Fatalf("%+v %v", res, err)
	}
	if f.runStatus(res.Run.ID) != domain.ClassificationRunRunning {
		t.Fatal("enqueue must return before the AI finished")
	}
	// A second manual click while the run is live gets the same run, not a second AI call.
	again, err := f.runner.Enqueue(tctx(), EnqueueClassificationInput{RequestID: r.ID, Manual: true})
	if err != nil || again.Started || again.Run.ID != res.Run.ID {
		t.Fatalf("%+v %v", again, err)
	}
	close(release)
	waitFor(t, "run to succeed", func() bool { return f.runStatus(res.Run.ID) == domain.ClassificationRunSucceeded })
	f.s.mu.Lock()
	got := f.s.requests[r.ID]
	f.s.mu.Unlock()
	if got.Status != domain.RequestStatusAwaitingTypeConfirmation || got.Type != domain.RequestTypeBug || f.cl.calls != 1 {
		t.Fatalf("%+v calls=%d", got, f.cl.calls)
	}
}

func TestRunner_RedeliveredEventStartsOneRun(t *testing.T) {
	f := newRunnerFixture(okProposal(domain.RequestTypeTask), time.Minute)
	r := f.classifying(t)
	ev := uuid.NewString()
	for i := 0; i < 3; i++ {
		if err := f.runner.Run(tctx(), r.ID, ev, "start_classification"); err != nil {
			t.Fatal(err)
		}
	}
	f.runner.Close()
	if f.cl.calls != 1 || len(f.s.runs) != 1 || f.s.requests[r.ID].ClassificationAttempts != 1 {
		t.Fatalf("calls=%d runs=%d attempts=%d", f.cl.calls, len(f.s.runs), f.s.requests[r.ID].ClassificationAttempts)
	}
}

func TestRunner_ConsumerPathSkipsPermanentConditions(t *testing.T) {
	f := newRunnerFixture(okProposal(domain.RequestTypeTask), time.Minute)
	defer f.runner.Close()
	if err := f.runner.Run(tctx(), uuid.NewString(), uuid.NewString(), ""); err != nil {
		t.Fatalf("missing request must be acked: %v", err)
	}
	done := f.s.seed(t, func(r *domain.Request) { r.Status = domain.RequestStatusCompleted })
	if err := f.runner.Run(tctx(), done.ID, uuid.NewString(), ""); err != nil {
		t.Fatalf("non-classifiable request must be acked: %v", err)
	}
	if len(f.s.runs) != 0 {
		t.Fatal("no run should exist")
	}
}

func TestRunner_ManualEnqueueValidates(t *testing.T) {
	f := newRunnerFixture(okProposal(domain.RequestTypeTask), time.Minute)
	defer f.runner.Close()
	bad := f.s.seed(t, func(r *domain.Request) { r.Status = domain.RequestStatusExecuting })
	_, err := f.runner.Enqueue(tctx(), EnqueueClassificationInput{RequestID: bad.ID, Manual: true})
	mustCode(t, err, "REQUEST_NOT_CLASSIFIABLE")
	capped := f.s.seed(t, func(r *domain.Request) {
		r.Status, r.ClassificationAttempts = domain.RequestStatusAwaitingTypeConfirmation, domain.MaxClassificationAttempts
	})
	_, err = f.runner.Enqueue(tctx(), EnqueueClassificationInput{RequestID: capped.ID, Manual: true})
	mustCode(t, err, "REQUEST_CLASSIFICATION_LIMIT")
}

func TestRunner_RecoversRunOfCrashedInstance(t *testing.T) {
	f := newRunnerFixture(okProposal(domain.RequestTypeBug), time.Minute)
	defer f.runner.Close()
	r := f.classifying(t)
	// A run left behind by an instance that died: lease already expired, nobody executing it.
	stale := domain.ClassificationRun{ID: uuid.NewString(), TenantID: testTenant, RequestID: r.ID, Trigger: "start_classification", SourceEventID: uuid.NewString(),
		ActorID: testReporter, Status: domain.ClassificationRunRunning, Claims: 1, LeaseOwner: "dead-instance", LeaseExpiresAt: time.Now().Add(-time.Minute)}
	f.s.runs[stale.ID] = stale

	n, err := f.runner.RecoverOnce(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("claimed %d err=%v", n, err)
	}
	waitFor(t, "recovered run to succeed", func() bool { return f.runStatus(stale.ID) == domain.ClassificationRunSucceeded })
	if f.s.requests[r.ID].Status != domain.RequestStatusAwaitingTypeConfirmation {
		t.Fatal("recovered run must write the result")
	}
	if n, _ := f.runner.RecoverOnce(context.Background()); n != 0 {
		t.Fatal("finished run must not be re-claimed")
	}
}

func TestRunner_RunOutOfClaimsIsFailedNotRetried(t *testing.T) {
	f := newRunnerFixture(okProposal(domain.RequestTypeBug), time.Minute)
	defer f.runner.Close()
	r := f.classifying(t)
	stale := domain.ClassificationRun{ID: uuid.NewString(), TenantID: testTenant, RequestID: r.ID, ActorID: testReporter,
		Status: domain.ClassificationRunRunning, Claims: domain.MaxClassificationRunClaims, LeaseExpiresAt: time.Now().Add(-time.Minute)}
	f.s.runs[stale.ID] = stale
	if n, _ := f.runner.RecoverOnce(context.Background()); n != 0 {
		t.Fatal("exhausted run must not restart")
	}
	if f.runStatus(stale.ID) != domain.ClassificationRunFailed || f.cl.calls != 0 {
		t.Fatalf("status=%s calls=%d", f.runStatus(stale.ID), f.cl.calls)
	}
}

func TestRunner_HeartbeatKeepsLeaseAlive(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	f := newRunnerFixture(func(int) (domain.ClassificationProposal, error) {
		once.Do(func() { time.Sleep(150 * time.Millisecond) })
		<-release
		return okProposal(domain.RequestTypeBug)(1)
	}, 90*time.Millisecond)
	r := f.classifying(t)
	res, err := f.runner.Enqueue(tctx(), EnqueueClassificationInput{RequestID: r.ID, Manual: true})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // longer than the lease: only renewals keep it valid
	f.s.mu.Lock()
	expires := f.s.runs[res.Run.ID].LeaseExpiresAt
	f.s.mu.Unlock()
	if !expires.After(time.Now()) {
		t.Fatal("lease expired while the run was still working")
	}
	close(release)
	waitFor(t, "run to finish", func() bool { return f.runStatus(res.Run.ID) == domain.ClassificationRunSucceeded })
	f.runner.Close()
}
