package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestPlanAutoWorker_ARequestThatLeftPlanningSupersedesTheJobInEveryState(t *testing.T) {
	for _, state := range []domain.PlanAutoState{domain.PlanAutoPending, domain.PlanAutoRunning, domain.PlanAutoCommitting} {
		k := newWorkerKit(t, domain.RequestTypeTask)
		k.seedJob(func(j *domain.PlanAutoJob) { j.State, j.RunID = state, "run-x" })
		r := k.w.requests[k.req.ID]
		r.Status = domain.RequestStatusCancelled
		k.w.requests[k.req.ID] = r
		k.pass(0)
		if job := k.job(); job.State != domain.PlanAutoSuperseded || job.ReasonCode != planAutoReasonMoved || k.start.calls != 0 || k.commit.calls != 0 {
			t.Errorf("%s: job = %+v starts=%d commits=%d", state, job, k.start.calls, k.commit.calls)
		}
	}
}

func TestPlanAutoWorker_ARequestThatAlreadyHasAPlanSupersedesEventAndRetryJobs(t *testing.T) {
	for _, origin := range []domain.PlanAutoOrigin{domain.PlanAutoOriginEvent, domain.PlanAutoOriginRetry} {
		k := newWorkerKit(t, domain.RequestTypeTask)
		k.seedJob(func(j *domain.PlanAutoJob) { j.Origin = origin })
		r := k.w.requests[k.req.ID]
		r.PlanTaskID = "11111111-1111-4111-8111-111111111111"
		k.w.requests[k.req.ID] = r
		k.pass(0)
		if job := k.job(); job.State != domain.PlanAutoSuperseded || job.ReasonCode != planAutoReasonPlanExist || k.start.calls != 0 {
			t.Errorf("%s: job = %+v starts=%d", origin, job, k.start.calls)
		}
	}
}

// CR-REQ-045 queues a job that revises an existing Plan: finding one is its normal case, not a reason to step aside.
func TestPlanAutoWorker_AJobOfAnotherOriginMayWorkOnARequestThatHasAPlan(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(func(j *domain.PlanAutoJob) {
		j.Origin, j.State, j.RunID, j.MaxAttempts = domain.PlanAutoOrigin("revision"), domain.PlanAutoCommitting, "", 1
	})
	r := k.w.requests[k.req.ID]
	r.PlanTaskID = "11111111-1111-4111-8111-111111111111"
	k.w.requests[k.req.ID] = r
	k.pass(0)
	// It was not superseded for having a Plan: it went on and failed for its own reason (no run to commit).
	if job := k.job(); job.State == domain.PlanAutoSuperseded || job.ReasonCode == planAutoReasonPlanExist {
		t.Fatalf("job = %+v", job)
	}
	if job := k.job(); job.State != domain.PlanAutoGaveUp || job.ReasonCode != domain.PlanAutoCodeCommitFailed {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_ADeletedRequestSupersedesTheJob(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(nil)
	delete(k.w.requests, k.req.ID)
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoSuperseded {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_ALostLeaseWritesNothing(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	stored := k.seedJob(func(j *domain.PlanAutoJob) { j.LeaseOwner = "somebody-else" })
	// The step was handed a copy it no longer owns: the run it starts must not be recorded on the job.
	k.worker.process(planAutoTenantCtx(), stored)
	job := k.job()
	if job.State != domain.PlanAutoPending || job.RunID != "" || job.LeaseOwner != "somebody-else" {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_AStoreFailureLeavesTheJobForALaterPass(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(nil)
	k.jobs.failUpdate = errBoom
	k.pass(0)
	// The run exists and the job still says pending; the same key finds the run on the next pass.
	if len(k.planRuns()) != 1 || k.job().State != domain.PlanAutoPending {
		t.Fatalf("runs=%d job=%+v", len(k.planRuns()), k.job())
	}
	k.pass(k.policy.LeaseTTL + time.Second)
	job := k.job()
	if job.State != domain.PlanAutoRunning || len(k.planRuns()) != 1 || job.RunID != k.planRuns()[0].ID {
		t.Fatalf("after the lease expired: job=%+v runs=%d", job, len(k.planRuns()))
	}
}

func TestPlanAutoWorker_TwoWorkersNeverHandleTheSameJob(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(nil)
	other := NewPlanAutoWorker(PlanAutoWorkerDeps{
		Jobs: k.jobs, Requests: k.w, Runs: fakeRuns{k.w}, Start: k.start, Commit: k.commit,
		Switch: PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: k.rows}, Policy: k.policy, Owner: "worker-2",
	})
	other.now = k.worker.now
	var wg sync.WaitGroup
	claimed := make([]int, 2)
	for i, w := range []*PlanAutoWorker{k.worker, other} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed[i], _ = w.RunOnce(planAutoTenantCtx(), 10)
		}()
	}
	wg.Wait()
	if claimed[0]+claimed[1] != 1 || k.start.calls != 1 || len(k.planRuns()) != 1 {
		t.Fatalf("claimed=%v starts=%d runs=%d", claimed, k.start.calls, len(k.planRuns()))
	}
}

func TestPlanAutoWorker_RunLoopStopsWhenTheContextEnds(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(nil)
	ctx, cancel := context.WithCancel(planAutoTenantCtx())
	done := make(chan struct{})
	go func() { k.worker.RunLoop(ctx, 5*time.Millisecond, 10); close(done) }()
	deadline := time.After(2 * time.Second)
	for k.job().State == domain.PlanAutoPending {
		select {
		case <-deadline:
			t.Fatal("the loop never picked the job up")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the loop kept running after cancel")
	}
}
