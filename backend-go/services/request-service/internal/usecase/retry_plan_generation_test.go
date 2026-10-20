package usecase

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type retryKit struct {
	*workerKit
	retry *RetryPlanGeneration
}

func newRetryKit(t *testing.T) *retryKit {
	t.Helper()
	k := &retryKit{workerKit: newWorkerKit(t, domain.RequestTypeTask)}
	k.retry = NewRetryPlanGeneration(k.w, fakeRuns{k.w}, k.jobs, k.generate.auth, PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: k.rows})
	k.retry.now = func() time.Time { return k.clock }
	return k
}

func (k *retryKit) activeJob(t *testing.T) domain.PlanAutoJob {
	t.Helper()
	job, found, _ := k.jobs.ActiveByRequest(planAutoTenantCtx(), k.req.ID)
	if !found {
		t.Fatal("no active job")
	}
	return job
}

func TestRetryPlanGeneration_QueuesAPendingJobWithAFreshAnchorAndASingleAttempt(t *testing.T) {
	k := newRetryKit(t)
	view, err := k.retry.Execute(k.w.ctx(), k.req.ID)
	if err != nil || view.State != "pending" || view.Origin != "retry" || view.MaxAttempts != 1 || !view.Enabled {
		t.Fatalf("view = %+v %v", view, err)
	}
	job := k.activeJob(t)
	if len(job.Anchor) < len("retry-") || job.Anchor[:6] != "retry-" || job.RequestedBy == "" || job.RunID != "" || job.ProjectID != k.req.ProjectID {
		t.Fatalf("job = %+v", job)
	}
	if k.start.calls != 0 {
		t.Fatal("Retry queues the work, the worker does it")
	}
}

func TestRetryPlanGeneration_AnActiveJobIsReturnedNotDuplicated(t *testing.T) {
	k := newRetryKit(t)
	first, err := k.retry.Execute(k.w.ctx(), k.req.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := k.retry.Execute(k.w.ctx(), k.req.ID)
	if err != nil || second != first || k.jobs.count() != 1 {
		t.Fatalf("second = %+v %v jobs=%d", second, err, k.jobs.count())
	}
}

func TestRetryPlanGeneration_ASucceededUncommittedRunContinuesFromTheCommit(t *testing.T) {
	k := newRetryKit(t)
	run := k.startedRun(k.req.ID)
	k.succeedRun(t, run.ID)
	k.clock = time.Now().UTC().Add(time.Minute)
	view, err := k.retry.Execute(k.w.ctx(), k.req.ID)
	if err != nil || view.State != "committing" {
		t.Fatalf("view = %+v %v", view, err)
	}
	if job := k.activeJob(t); job.RunID != run.ID {
		t.Fatalf("job = %+v, want it on run %s", job, run.ID)
	}
	k.pass(0)
	if got := k.w.requests[k.req.ID]; got.Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("request = %s", got.Status)
	}
	if k.start.calls != 0 || len(k.planRuns()) != 1 {
		t.Fatalf("the AI was paid for again: starts=%d runs=%d", k.start.calls, len(k.planRuns()))
	}
}

func TestRetryPlanGeneration_AFailedRunGetsAFreshPendingJob(t *testing.T) {
	k := newRetryKit(t)
	run := k.startedRun(k.req.ID)
	k.failRun(run.ID, "REQUEST_PLAN_AI_INVALID_JSON")
	k.clock = time.Now().UTC().Add(time.Minute) // the person presses Retry after the run failed
	view, err := k.retry.Execute(k.w.ctx(), k.req.ID)
	if err != nil || view.State != "pending" {
		t.Fatalf("view = %+v %v", view, err)
	}
	k.pass(0)
	runs := k.planRuns()
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want the failed one and one new", len(runs))
	}
}

func TestRetryPlanGeneration_Refusals(t *testing.T) {
	k := newRetryKit(t)
	r := k.w.requests[k.req.ID]
	r.Status = domain.RequestStatusAwaitingPlanApproval
	k.w.requests[k.req.ID] = r
	_, err := k.retry.Execute(k.w.ctx(), k.req.ID)
	requireCode(t, err, "REQUEST_STATE_STALE")

	k = newRetryKit(t)
	k.retry.sw = PlanAutoSwitch{EnvOn: false, CommitWired: true}
	_, err = k.retry.Execute(k.w.ctx(), k.req.ID)
	requireCode(t, err, "REQUEST_PLAN_AUTO_DISABLED")
	k.retry.sw = PlanAutoSwitch{EnvOn: true, CommitWired: false}
	_, err = k.retry.Execute(k.w.ctx(), k.req.ID)
	requireCode(t, err, "REQUEST_PLAN_AUTO_DISABLED")

	k = newRetryKit(t)
	k.retry.auth = refusingPlanAuth{k.retry.auth}
	_, err = k.retry.Execute(k.w.ctx(), k.req.ID)
	requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
	if k.jobs.count() != 0 {
		t.Fatal("a refused call left a job")
	}
	_, err = k.retry.Execute(planAutoTenantCtx(), k.req.ID)
	requireCode(t, err, "REQUEST_REPORTER_REQUIRED")
}

func TestRetryPlanGeneration_ProjectSwitchOffDoesNotStopAPersonsRetry(t *testing.T) {
	k := newRetryKit(t)
	k.rows.rows[k.req.ProjectID] = domain.ProjectEngineSettings{ProjectID: k.req.ProjectID, PlanAutoGenerate: false}
	view, err := k.retry.Execute(k.w.ctx(), k.req.ID)
	if err != nil || view.State != "pending" || view.Enabled {
		t.Fatalf("view = %+v %v", view, err)
	}
}

func TestRetryPlanGeneration_PressedTwiceInARowIsOneJobAndOneRun(t *testing.T) {
	k := newRetryKit(t)
	for i := 0; i < 2; i++ {
		if _, err := k.retry.Execute(k.w.ctx(), k.req.ID); err != nil {
			t.Fatal(err)
		}
	}
	k.pass(0)
	if k.jobs.count() != 1 || len(k.planRuns()) != 1 {
		t.Fatalf("jobs=%d runs=%d", k.jobs.count(), len(k.planRuns()))
	}
}
