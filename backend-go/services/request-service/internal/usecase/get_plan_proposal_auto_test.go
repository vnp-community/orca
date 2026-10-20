package usecase

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func autoGetKit(t *testing.T) (*workerKit, *GetPlanProposal) {
	t.Helper()
	k := newWorkerKit(t, domain.RequestTypeTask)
	get := NewGetPlanProposal(k.w, fakeRuns{k.w}).WithAutoPlan(k.jobs, PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: k.rows})
	return k, get
}

func TestGetPlanProposal_AutoPlanCarriesTheLatestJobWhileThePlanIsWaited(t *testing.T) {
	k, get := autoGetKit(t)
	run := k.startedRun(k.req.ID)
	k.failRun(run.ID, "REQUEST_PLAN_AI_INVALID_JSON")
	k.seedJob(func(j *domain.PlanAutoJob) {
		j.State, j.ReasonCode, j.RunID = domain.PlanAutoGaveUp, "REQUEST_PLAN_AI_INVALID_JSON", run.ID
	})

	v, err := get.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.Status != "failed" || v.ErrorCode != "REQUEST_PLAN_AI_INVALID_JSON" {
		t.Fatalf("run part: %+v %v", v, err)
	}
	if a := v.AutoPlan; a == nil || !a.Enabled || a.State != "gave_up" || a.ReasonCode != "REQUEST_PLAN_AI_INVALID_JSON" || a.Origin != "event" || a.MaxAttempts != 2 {
		t.Fatalf("auto plan = %+v", v.AutoPlan)
	}
}

func TestGetPlanProposal_OnlyAJobReturnsAnEmptyViewInsteadOfRunNotFound(t *testing.T) {
	k, get := autoGetKit(t)
	k.seedJob(func(j *domain.PlanAutoJob) {
		j.ReasonCode, j.Attempt = "REQUEST_ANALYSIS_BUSY", 1
		j.NextCheckAt = k.clock.Add(time.Minute)
	})
	v, err := get.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.RunID != "" || v.Status != "" || v.Proposal != nil {
		t.Fatalf("view = %+v %v", v, err)
	}
	if a := v.AutoPlan; a == nil || a.State != "pending" || a.ReasonCode != "REQUEST_ANALYSIS_BUSY" || !a.NextAttemptAt.Equal(k.clock.Add(time.Minute)) {
		t.Fatalf("auto plan = %+v", v.AutoPlan)
	}
	// An explicit unknown run id is still an error.
	_, err = get.Execute(k.w.ctx(), k.req.ID, "00000000-0000-4000-8000-000000000000", 0)
	requireCode(t, err, "REQUEST_PLAN_RUN_NOT_FOUND")
}

func TestGetPlanProposal_NoRunAndNoJobStillCarriesWhetherAutoPlanIsEnabled(t *testing.T) {
	k, get := autoGetKit(t)
	v, err := get.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.RunID != "" || v.AutoPlan == nil || !v.AutoPlan.Enabled || v.AutoPlan.State != "" {
		t.Fatalf("view = %+v %v", v, err)
	}
	k.rows.rows[k.req.ProjectID] = domain.ProjectEngineSettings{ProjectID: k.req.ProjectID, PlanAutoGenerate: false}
	v, err = get.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.AutoPlan == nil || v.AutoPlan.Enabled {
		t.Fatalf("project switch off: %+v %v", v.AutoPlan, err)
	}
	// Without the reader (a server without the feature) the old answer stays.
	_, err = NewGetPlanProposal(k.w, fakeRuns{k.w}).Execute(k.w.ctx(), k.req.ID, "", 0)
	requireCode(t, err, "REQUEST_PLAN_RUN_NOT_FOUND")
}

func TestGetPlanProposal_TheDoneJobStaysReadableAfterTheCommitMovedTheRequest(t *testing.T) {
	k, get := autoGetKit(t)
	run := k.startedRun(k.req.ID)
	k.seedJob(func(j *domain.PlanAutoJob) { j.State = domain.PlanAutoDone; j.RunID = run.ID })
	r := k.w.requests[k.req.ID]
	r.Status, r.PlanTaskID = domain.RequestStatusAwaitingPlanApproval, "11111111-1111-4111-8111-111111111111"
	k.w.requests[k.req.ID] = r
	v, err := get.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.AutoPlan == nil || v.AutoPlan.State != "done" || !v.AutoPlan.Enabled {
		t.Fatalf("view = %+v %v", v.AutoPlan, err)
	}
}

func TestGetPlanProposal_AFinishedJobNextToAnExistingPlanIsHistory(t *testing.T) {
	k, get := autoGetKit(t)
	run := k.startedRun(k.req.ID)
	k.seedJob(func(j *domain.PlanAutoJob) { j.State = domain.PlanAutoDone; j.RunID = run.ID })
	r := k.w.requests[k.req.ID]
	r.PlanTaskID = "11111111-1111-4111-8111-111111111111" // a replan: planning again with the old Plan recorded
	k.w.requests[k.req.ID] = r
	v, err := get.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.AutoPlan == nil || v.AutoPlan.State != "" {
		t.Fatalf("view = %+v %v", v.AutoPlan, err)
	}
}

func TestGetPlanProposal_EnabledFollowsBothSwitches(t *testing.T) {
	k, get := autoGetKit(t)
	k.startedRun(k.req.ID)
	k.rows.rows[k.req.ProjectID] = domain.ProjectEngineSettings{ProjectID: k.req.ProjectID, PlanAutoGenerate: false}
	v, err := get.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.AutoPlan == nil || v.AutoPlan.Enabled {
		t.Fatalf("project switch off: %+v %v", v.AutoPlan, err)
	}
	off := NewGetPlanProposal(k.w, fakeRuns{k.w}).WithAutoPlan(k.jobs, PlanAutoSwitch{})
	v, err = off.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.AutoPlan == nil || v.AutoPlan.Enabled {
		t.Fatalf("deployment off: %+v %v", v.AutoPlan, err)
	}
	failing := NewGetPlanProposal(k.w, fakeRuns{k.w}).WithAutoPlan(k.jobs, PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: failingEngineRows{err: errBoom}})
	v, err = failing.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.AutoPlan == nil || v.AutoPlan.Enabled {
		t.Fatalf("unreadable settings must read as not enabled: %+v %v", v.AutoPlan, err)
	}
}

func TestGetPlanProposal_WithoutTheAutoPlanReaderTheViewIsUnchanged(t *testing.T) {
	k := newWorkerKit(t, domain.RequestTypeTask)
	k.startedRun(k.req.ID)
	v, err := NewGetPlanProposal(k.w, fakeRuns{k.w}).Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil || v.AutoPlan != nil || v.Status != "running" {
		t.Fatalf("view = %+v %v", v, err)
	}
}

// CR-REQ-045: while a change request is being written and committed by the worker, the Plan tab learns it from the job.
func TestGetPlanProposal_AutoPlanReportsARevisionJobWhileThePlanIsRevised(t *testing.T) {
	k, get := autoGetKit(t)
	r := k.w.requests[k.req.ID]
	r.PlanTaskID = "11111111-1111-4111-8111-111111111111"
	k.w.requests[k.req.ID] = r
	run := k.startedRun(k.req.ID)
	k.seedJob(func(j *domain.PlanAutoJob) {
		j.Anchor, j.Origin, j.State, j.RunID, j.MaxAttempts = domain.RevisionAnchor(run.ID), domain.PlanAutoOriginRevision, domain.PlanAutoRunning, run.ID, 1
	})
	v, err := get.Execute(k.w.ctx(), k.req.ID, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if a := v.AutoPlan; a == nil || a.Origin != "revision" || a.State != "running" || a.Attempt != 1 || a.MaxAttempts != 1 {
		t.Fatalf("auto plan = %+v", v.AutoPlan)
	}
}
