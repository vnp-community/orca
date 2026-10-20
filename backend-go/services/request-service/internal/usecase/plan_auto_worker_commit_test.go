package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func committingKit(t *testing.T) (*workerKit, domain.AnalysisRun) {
	t.Helper()
	k, run := startedKit(t)
	k.succeedRun(t, run.ID)
	k.pass(k.policy.PollInterval)
	if k.job().State != domain.PlanAutoCommitting {
		t.Fatalf("setup: %+v", k.job())
	}
	return k, k.w.runs[run.ID]
}

func TestPlanAutoWorker_CommitsTheProposalOfTheRunAndFinishes(t *testing.T) {
	k, _ := committingKit(t)
	k.pass(0)
	job := k.job()
	if job.State != domain.PlanAutoDone || job.ReasonCode != "" {
		t.Fatalf("job = %+v", job)
	}
	got := k.w.requests[k.req.ID]
	if got.Status != domain.RequestStatusAwaitingPlanApproval || got.PlanTaskID == "" {
		t.Fatalf("request = %s plan=%q", got.Status, got.PlanTaskID)
	}
	if len(k.opener.opened) != 1 || k.opener.opened[0].RequestedBy != k.req.ReporterID {
		t.Fatalf("approval = %+v", k.opener.opened)
	}
}

func TestPlanAutoWorker_ThePersonCommittingFirstEndsTheJobAsDone(t *testing.T) {
	k, _ := committingKit(t)
	r := k.w.requests[k.req.ID]
	r.Status, r.PlanTaskID = domain.RequestStatusAwaitingPlanApproval, "22222222-2222-4222-8222-222222222222"
	k.w.requests[k.req.ID] = r
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoDone || k.commit.calls != 0 {
		t.Fatalf("job = %+v commits=%d", job, k.commit.calls)
	}
}

func TestPlanAutoWorker_AStaleAnswerFromTheCommitIsDoneWhenThePlanIsThere(t *testing.T) {
	k, _ := committingKit(t)
	// The other side finishes between the step's read of the Request and its commit.
	k.commit.inner = staleThenCommitted{k: k}
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoDone {
		t.Fatalf("job = %+v", job)
	}
}

// staleThenCommitted answers like CommitPlan does when the other side wins the race, after recording the Plan.
type staleThenCommitted struct{ k *workerKit }

func (s staleThenCommitted) ExecuteForSystem(_ context.Context, in CommitPlanInput) (CommitPlanResult, error) {
	r := s.k.w.requests[in.RequestID]
	r.Status, r.PlanTaskID = domain.RequestStatusAwaitingPlanApproval, "33333333-3333-4333-8333-333333333333"
	s.k.w.requests[in.RequestID] = r
	return CommitPlanResult{}, domain.ErrPlanWrongState(domain.RequestStatusAwaitingPlanApproval)
}

func TestPlanAutoWorker_AStaleAnswerWhenTheRequestMovedOnSupersedesTheJob(t *testing.T) {
	k, _ := committingKit(t)
	k.commit.inner = movedThenStale{k: k}
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoSuperseded {
		t.Fatalf("job = %+v", job)
	}
}

type movedThenStale struct{ k *workerKit }

func (m movedThenStale) ExecuteForSystem(_ context.Context, in CommitPlanInput) (CommitPlanResult, error) {
	r := m.k.w.requests[in.RequestID]
	r.Status = domain.RequestStatusCancelled
	m.k.w.requests[in.RequestID] = r
	return CommitPlanResult{}, domain.ErrPlanWrongState(domain.RequestStatusCancelled)
}

func TestPlanAutoWorker_AProposalThePolicyRefusesGivesUpWithItsCodeAndNeverRerunsTheAI(t *testing.T) {
	k, _ := committingKit(t)
	k.commit.err = domain.ErrPlanRegressionTestMissing()
	k.pass(0)
	job := k.job()
	if job.State != domain.PlanAutoGaveUp || job.ReasonCode != "REQUEST_PLAN_REGRESSION_TEST_MISSING" {
		t.Fatalf("job = %+v", job)
	}
	k.pass(time.Hour)
	if len(k.planRuns()) != 1 || k.start.calls != 1 {
		t.Fatalf("runs=%d starts=%d", len(k.planRuns()), k.start.calls)
	}
}

func TestPlanAutoWorker_TaskServiceDownRetriesTheCommitFiveTimesOnTheSameRun(t *testing.T) {
	k, run := committingKit(t)
	k.commit.err = domain.ErrPlanTaskServiceUnavailable(errBoom)
	for i := 1; i <= k.policy.MaxCommitAttempts-1; i++ {
		k.pass(k.policy.RetryDelay)
		job := k.job()
		if job.State != domain.PlanAutoCommitting || job.CommitAttempts != i || job.RunID != run.ID {
			t.Fatalf("after failure %d: %+v", i, job)
		}
	}
	k.pass(k.policy.RetryDelay)
	job := k.job()
	if job.State != domain.PlanAutoGaveUp || job.ReasonCode != domain.PlanAutoCodeCommitFailed || job.CommitAttempts != k.policy.MaxCommitAttempts {
		t.Fatalf("job = %+v", job)
	}
	if k.commit.calls != k.policy.MaxCommitAttempts || k.start.calls != 1 || len(k.planRuns()) != 1 {
		t.Fatalf("commits=%d starts=%d runs=%d", k.commit.calls, k.start.calls, len(k.planRuns()))
	}
}

func TestPlanAutoWorker_ACommitThatRecoversFinishesTheJob(t *testing.T) {
	k, _ := committingKit(t)
	k.commit.err = domain.ErrPlanTaskServiceUnavailable(errBoom)
	k.pass(0)
	k.commit.err = nil
	k.pass(k.policy.RetryDelay)
	if job := k.job(); job.State != domain.PlanAutoDone {
		t.Fatalf("job = %+v", job)
	}
}

func TestPlanAutoWorker_AnUnreadableStoredProposalGivesUpWithoutRerunning(t *testing.T) {
	k, run := committingKit(t)
	bad := "not a proposal"
	r := k.w.runs[run.ID]
	r.RawOutput = &bad
	k.w.runs[run.ID] = r
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoGaveUp || job.ReasonCode != domain.PlanAutoCodeCommitFailed || k.commit.calls != 0 {
		t.Fatalf("job = %+v commits=%d", job, k.commit.calls)
	}
}

// Both engines end in the same stored form: the worker commits what an agent_proposal run produced.
func TestPlanAutoWorker_CommitsTheRawOutputOfAnOpenSpecRun(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	pk, run, proposalWorker := proposalKit(t, eng, nil)
	if err := proposalWorker.Execute(pk.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	k := &workerKit{planKit: pk, policy: domain.DefaultPlanAutoPolicy, clock: time.Now().UTC(), rows: &memEngineSettings{rows: map[string]domain.ProjectEngineSettings{}}, req: pk.w.requests[run.RequestID]}
	k.jobs = newMemPlanAutoJobs(func() time.Time { return k.clock })
	k.start = &countingStarter{inner: pk.generate}
	k.commit = &countingCommitter{inner: pk.commit}
	k.build()
	k.seedJob(func(j *domain.PlanAutoJob) { j.State, j.RunID = domain.PlanAutoCommitting, run.ID })
	k.pass(0)
	if job := k.job(); job.State != domain.PlanAutoDone {
		t.Fatalf("job = %+v", job)
	}
	if got := pk.w.requests[run.RequestID]; got.Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("request = %s", got.Status)
	}
}
