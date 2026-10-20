package usecase

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestPlanAutoWorker_PendingJobStartsARunWithTheAttemptKeyAndFollowsIt(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(nil)
	if n := k.pass(0); n != 1 {
		t.Fatalf("claimed %d jobs", n)
	}
	job := k.job()
	runs := k.planRuns()
	if len(runs) != 1 || job.State != domain.PlanAutoRunning || job.RunID != runs[0].ID {
		t.Fatalf("job = %+v runs = %d", job, len(runs))
	}
	if runs[0].IdempotencyKey == nil || *runs[0].IdempotencyKey != "plan-auto-sol-1" || runs[0].ActorID != k.req.ReporterID {
		t.Fatalf("run = %s key=%v", describe(runs[0]), runs[0].IdempotencyKey)
	}
	if !job.NextCheckAt.Equal(k.clock.Add(k.policy.PollInterval)) || job.LeaseOwner != "" {
		t.Fatalf("schedule/lease = %v / %q", job.NextCheckAt, job.LeaseOwner)
	}
	if len(k.spawner.runs) != 1 {
		t.Fatal("the run must be handed to the runner")
	}
}

func TestPlanAutoWorker_AJobIsNotTouchedBeforeItIsDue(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(func(j *domain.PlanAutoJob) { j.NextCheckAt = k.clock.Add(time.Minute) })
	if n := k.pass(10 * time.Second); n != 0 || k.start.calls != 0 {
		t.Fatalf("claimed=%d starts=%d", n, k.start.calls)
	}
}

func TestPlanAutoWorker_BusyProjectDefersTheJobThenGivesUpAfterTheWindow(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.start.err = domain.ErrAnalysisBusy()
	k.seedJob(nil)
	k.pass(0)
	job := k.job()
	if job.State != domain.PlanAutoPending || job.ReasonCode != "REQUEST_ANALYSIS_BUSY" || !job.NextCheckAt.Equal(k.clock.Add(k.policy.BusyDelay)) {
		t.Fatalf("job = %+v", job)
	}
	k.pass(k.policy.BusyDelay)
	if got := k.job(); got.State != domain.PlanAutoPending || k.start.calls != 2 {
		t.Fatalf("still within the window: %+v calls=%d", got, k.start.calls)
	}
	k.pass(k.policy.StartWindow)
	job = k.job()
	if job.State != domain.PlanAutoGaveUp || job.ReasonCode != "REQUEST_ANALYSIS_BUSY" {
		t.Fatalf("after the window: %+v", job)
	}
}

func TestPlanAutoWorker_PermanentStartRefusalsAreSkippedWithTheirCode(t *testing.T) {
	cases := map[string]error{
		"REQUEST_PLAN_AI_UNAVAILABLE":        domain.ErrPlanAIUnavailable(),
		"REQUEST_ANALYSIS_NO_REPO_PATH":      domain.ErrAnalysisNoRepoPath(),
		"REQUEST_ENGINE_UNAVAILABLE":         domain.ErrEngineUnavailable,
		"REQUEST_STATE_STALE":                domain.ErrPlanWrongState(domain.RequestStatusAnalyzing),
		"REQUEST_PLAN_NOT_APPLICABLE":        domain.ErrPlanNotApplicable(domain.RequestTypeSpike),
		"REQUEST_PLAN_SOLUTION_NOT_APPROVED": domain.ErrPlanSolutionNotApproved("r"),
	}
	for code, err := range cases {
		k := newWorkerKit(t, domain.RequestTypeTask)
		k.start.err = err
		k.seedJob(nil)
		k.pass(0)
		job := k.job()
		if job.State != domain.PlanAutoSkipped || job.ReasonCode != code {
			t.Errorf("%s: job = %s/%s", code, job.State, job.ReasonCode)
		}
		if k.start.calls != 1 {
			t.Errorf("%s: a permanent refusal must not be retried (%d calls)", code, k.start.calls)
		}
		if k.pass(time.Hour); k.start.calls != 1 {
			t.Errorf("%s: a skipped job was started again", code)
		}
	}
}

func TestPlanAutoWorker_InfrastructureErrorsRetryInsideTheWindowThenGiveUp(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.start.err = errBoom
	k.seedJob(nil)
	k.pass(0)
	job := k.job()
	if job.State != domain.PlanAutoPending || !job.NextCheckAt.Equal(k.clock.Add(k.policy.RetryDelay)) {
		t.Fatalf("job = %+v", job)
	}
	k.pass(k.policy.RetryDelay)
	if k.start.calls != 2 || k.job().State != domain.PlanAutoPending {
		t.Fatalf("calls=%d job=%+v", k.start.calls, k.job())
	}
	k.pass(k.policy.StartWindow)
	if got := k.job(); got.State != domain.PlanAutoGaveUp || got.ReasonCode != "REQUEST_SOLUTION_INTERNAL_ERROR" {
		t.Fatalf("job = %+v", got)
	}
	if len(k.planRuns()) != 0 {
		t.Fatal("no run may exist when the start never worked")
	}
}

// A person pressed Generate while the job was waiting: the job follows that run instead of starting a second one.
func TestPlanAutoWorker_AdoptsARunThePersonStartedMeanwhile(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.seedJob(nil)
	manual, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: k.req.ID})
	if err != nil || !manual.Created {
		t.Fatalf("manual start: %+v %v", manual, err)
	}
	k.pass(0)
	job := k.job()
	if job.State != domain.PlanAutoRunning || job.RunID != manual.RunID {
		t.Fatalf("job = %+v, want it to follow %s", job, manual.RunID)
	}
	if len(k.planRuns()) != 1 || len(k.spawner.runs) != 1 {
		t.Fatalf("runs=%d workers=%d", len(k.planRuns()), len(k.spawner.runs))
	}
}

func TestPlanAutoWorker_ARunThePersonFinishedAlreadySupersedesThePendingJob(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.clock = k.clock.Add(-time.Minute) // the job is older than the person's run
	k.seedJob(nil)
	manual, _ := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: k.req.ID})
	k.succeedRun(t, manual.RunID)
	k.clock = k.clock.Add(time.Minute)
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoSuperseded || k.start.calls != 0 {
		t.Fatalf("job = %+v starts=%d", job, k.start.calls)
	}
}

func TestPlanAutoWorker_TheProjectSwitchIsReadAgainBeforeARunStarts(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.rows.rows[k.req.ProjectID] = domain.ProjectEngineSettings{ProjectID: k.req.ProjectID, PlanAutoGenerate: false}
	k.seedJob(nil)
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoSkipped || job.ReasonCode != domain.PlanAutoCodeDisabled || k.start.calls != 0 {
		t.Fatalf("job = %+v starts=%d", job, k.start.calls)
	}

	// A person's Retry is not stopped by the project switch.
	k2 := newWorkerKit(t, domain.RequestTypeTask)
	k2.rows.rows[k2.req.ProjectID] = domain.ProjectEngineSettings{ProjectID: k2.req.ProjectID, PlanAutoGenerate: false}
	k2.seedJob(func(j *domain.PlanAutoJob) { j.Origin, j.MaxAttempts = domain.PlanAutoOriginRetry, 1 })
	k2.pass(0)
	if job := k2.job(); job.State != domain.PlanAutoRunning {
		t.Fatalf("retry job = %+v", job)
	}
}

func TestPlanAutoWorker_ASettingsReadFailureKeepsTheJobWaiting(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.worker.sw.Settings = failingEngineRows{err: errBoom}
	k.seedJob(nil)
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoPending || k.start.calls != 0 {
		t.Fatalf("job = %+v starts=%d", job, k.start.calls)
	}
}
