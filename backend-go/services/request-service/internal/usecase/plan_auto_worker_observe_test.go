package usecase

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// started returns a kit whose job already follows a running run.
func startedKit(t *testing.T) (*workerKit, domain.AnalysisRun) {
	t.Helper()
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(nil)
	k.pass(0)
	runs := k.planRuns()
	if len(runs) != 1 || k.job().State != domain.PlanAutoRunning {
		t.Fatalf("setup: %+v", k.job())
	}
	return k, runs[0]
}

func TestPlanAutoWorker_ARunningRunIsPolledAgain(t *testing.T) {
	k, run := startedKit(t)
	k.pass(k.policy.PollInterval)
	job := k.job()
	if job.State != domain.PlanAutoRunning || job.RunID != run.ID || !job.NextCheckAt.Equal(k.clock.Add(k.policy.PollInterval)) {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_ASucceededRunMovesToCommitting(t *testing.T) {
	k, run := startedKit(t)
	k.succeedRun(t, run.ID)
	k.pass(k.policy.PollInterval)
	if job := k.job(); job.State != domain.PlanAutoCommitting || job.RunID != run.ID {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_ATransientFailureBuysExactlyOneMoreRunWithANewKey(t *testing.T) {
	k, first := startedKit(t)
	k.failRun(first.ID, "REQUEST_AI_COMPLETE_TIMEOUT")
	k.pass(k.policy.PollInterval)
	job := k.job()
	if job.State != domain.PlanAutoPending || job.Attempt != 2 || job.RunID != "" || job.ReasonCode != "REQUEST_AI_COMPLETE_TIMEOUT" || !job.NextCheckAt.Equal(k.clock.Add(k.policy.RetryDelay)) {
		t.Fatalf("job = %+v", job)
	}
	if k.pass(k.policy.RetryDelay - time.Second); k.start.calls != 1 {
		t.Fatal("the retry must wait for its delay")
	}
	k.pass(time.Second)
	job = k.job()
	if job.State != domain.PlanAutoRunning || job.Attempt != 2 || job.ReasonCode != "" {
		t.Fatalf("second attempt = %+v", job)
	}
	if got := k.start.keys; len(got) != 2 || got[0] != "plan-auto-sol-1" || got[1] != "plan-auto-sol-1-2" {
		t.Fatalf("keys = %v", got)
	}
	second := k.w.runs[job.RunID]
	if second.ID == first.ID {
		t.Fatal("the retry reused the failed run")
	}

	k.failRun(second.ID, "REQUEST_AI_COMPLETE_TIMEOUT")
	k.pass(k.policy.PollInterval)
	if job = k.job(); job.State != domain.PlanAutoGaveUp || job.ReasonCode != "REQUEST_AI_COMPLETE_TIMEOUT" {
		t.Fatalf("out of attempts: %+v", job)
	}
	if len(k.planRuns()) != 2 {
		t.Fatalf("runs = %d, want exactly two", len(k.planRuns()))
	}
}

func TestPlanAutoWorker_EveryOtherFailureGivesUpAtOnceWithoutASecondRun(t *testing.T) {
	codes := []string{
		"REQUEST_PLAN_AI_INVALID_JSON", "REQUEST_OPENSPEC_INVALID_OUTPUT", "REQUEST_SPEC_RUN_OUT_OF_SCOPE", "REQUEST_AI_NOT_CONFIGURED",
		"REQUEST_AI_QUOTA_EXHAUSTED", "REQUEST_AI_AGENT_UNAVAILABLE", "REQUEST_AI_SESSION_EXPIRED", "REQUEST_ANALYSIS_NO_CONNECTION",
		"REQUEST_BASE_REF_NOT_FOUND", "REQUEST_PLAN_REQUEST_MOVED", "REQUEST_SOLUTION_INTERNAL_ERROR", "SOMETHING_NEW",
	}
	for _, code := range codes {
		k, run := startedKit(t)
		k.failRun(run.ID, code)
		k.pass(k.policy.PollInterval)
		job := k.job()
		if job.State != domain.PlanAutoGaveUp || job.ReasonCode != code {
			t.Errorf("%s: job = %s/%s", code, job.State, job.ReasonCode)
		}
		k.pass(time.Hour)
		if len(k.planRuns()) != 1 || k.start.calls != 1 {
			t.Errorf("%s: runs=%d starts=%d, a non-transient failure must not cost a second run", code, len(k.planRuns()), k.start.calls)
		}
	}
}

func TestPlanAutoWorker_AFailedRunWithoutACodeIsAnInternalError(t *testing.T) {
	k, run := startedKit(t)
	r := k.w.runs[run.ID]
	r.Status = domain.RunStatusFailed
	k.w.runs[run.ID] = r
	k.pass(k.policy.PollInterval)
	if job := k.job(); job.State != domain.PlanAutoGaveUp || job.ReasonCode != domain.RunErrInternal {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_AJobWithNoExtraAttemptsNeverRetries(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(func(j *domain.PlanAutoJob) { j.MaxAttempts = 1 })
	k.pass(0)
	k.failRun(k.job().RunID, "REQUEST_AI_COMPLETE_TIMEOUT")
	k.pass(k.policy.PollInterval)
	if job := k.job(); job.State != domain.PlanAutoGaveUp {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_ARunThatDisappearedSupersedesTheJob(t *testing.T) {
	k, run := startedKit(t)
	delete(k.w.runs, run.ID)
	k.pass(k.policy.PollInterval)
	if job := k.job(); job.State != domain.PlanAutoSuperseded {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_APersonRegeneratingWhileTheJobWaitsSupersedesIt(t *testing.T) {
	k, run := startedKit(t)
	// The job's run failed and a person pressed Generate: their run is the newer one.
	k.failRun(run.ID, "REQUEST_PLAN_AI_INVALID_JSON")
	r := k.w.runs[run.ID]
	r.Status, r.ErrorCode = domain.RunStatusRunning, nil
	k.w.runs[run.ID] = r
	manual := domain.AnalysisRun{ID: "manual-run", TenantID: "t1", RequestID: k.req.ID, Kind: domain.RunKindPlan, Status: domain.RunStatusRunning, StartedAt: r.StartedAt.Add(time.Second)}
	k.w.runs[manual.ID] = manual
	k.pass(k.policy.PollInterval)
	if job := k.job(); job.State != domain.PlanAutoSuperseded || job.ReasonCode != planAutoReasonNewerRun {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_AStartThatReturnsAFailedRunForTheKeyIsTreatedAsThatFailure(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(func(j *domain.PlanAutoJob) { j.MaxAttempts = 1 })
	// The job started the run but crashed before saving: the run failed meanwhile and the key still finds it.
	res, err := k.generate.ExecuteForSystem(planAutoTenantCtx(), GeneratePlanInput{RequestID: k.req.ID, IdempotencyKey: "plan-auto-sol-1"})
	if err != nil {
		t.Fatal(err)
	}
	k.failRun(res.RunID, "REQUEST_AI_COMPLETE_TIMEOUT")
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoGaveUp || job.ReasonCode != "REQUEST_AI_COMPLETE_TIMEOUT" || len(k.planRuns()) != 1 {
		t.Fatalf("job = %+v runs=%d", job, len(k.planRuns()))
	}
}
