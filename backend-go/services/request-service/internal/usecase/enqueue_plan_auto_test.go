package usecase

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type enqueueKit struct {
	w      *solWorld
	jobs   *memPlanAutoJobs
	proc   *memProcessedEvents
	rows   *memEngineSettings
	enq    *EnqueuePlanAuto
	policy domain.PlanAutoPolicy
	clock  time.Time
}

func newEnqueueKit() *enqueueKit {
	k := &enqueueKit{w: newSolWorld(), proc: newMemProcessedEvents(), rows: &memEngineSettings{rows: map[string]domain.ProjectEngineSettings{}}, policy: domain.DefaultPlanAutoPolicy, clock: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)}
	k.jobs = newMemPlanAutoJobs(func() time.Time { return k.clock })
	k.build(PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: k.rows})
	return k
}

func (k *enqueueKit) build(sw PlanAutoSwitch) {
	k.enq = NewEnqueuePlanAuto(k.jobs, k.proc, k.w, fakeSolutions{k.w}, sw, k.policy, planAutoTx{w: k.w, proc: k.proc, jobs: k.jobs})
	k.enq.now = func() time.Time { return k.clock }
}

// planning seeds a Request waiting for its Plan, with an approved Solution when its type has an Analysis.
func (k *enqueueKit) planning(typ domain.RequestType) (domain.Request, string) {
	req := k.w.seedRequest(typ, domain.RequestStatusPlanning)
	flow, _ := domain.FlowFor(typ)
	solutionID := ""
	if flow.AnalysisKind != domain.AnalysisNone {
		kind := domain.SolutionKindSolution
		if flow.AnalysisKind == domain.AnalysisDiagnosis {
			kind = domain.SolutionKindDiagnosis
		}
		solutionID = k.w.seedApprovedSolution(req.ID, kind).ID
	}
	return k.w.requests[req.ID], solutionID
}

func approvedEvent(id string, req domain.Request) PlanAutoEvent {
	return PlanAutoEvent{ID: id, TenantID: "t1", RequestID: req.ID, ProjectID: req.ProjectID, To: "planning", Trigger: string(domain.TriggerAnalysisApproved)}
}

func TestEnqueuePlanAuto_AnApprovedAnalysisCreatesOneJobAnchoredOnTheSolution(t *testing.T) {
	k := newEnqueueKit()
	req, solutionID := k.planning(domain.RequestTypeBug)
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil {
		t.Fatal(err)
	}
	job := k.jobs.get(req.ID, solutionID)
	if job.Anchor != solutionID || job.State != domain.PlanAutoPending || job.Origin != domain.PlanAutoOriginEvent || job.Attempt != 1 || job.MaxAttempts != 2 {
		t.Fatalf("job = %+v", job)
	}
	if job.ProjectID != req.ProjectID || job.TenantID != "t1" || !job.NextCheckAt.Equal(k.clock) {
		t.Fatalf("job = %+v", job)
	}
}

func TestEnqueuePlanAuto_RedeliveringTheSameEventKeepsOneJob(t *testing.T) {
	k := newEnqueueKit()
	req, _ := k.planning(domain.RequestTypeBug)
	for i := 0; i < 3; i++ {
		if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil {
			t.Fatal(err)
		}
	}
	if k.jobs.count() != 1 || k.jobs.inserts != 1 {
		t.Fatalf("jobs = %d inserts = %d", k.jobs.count(), k.jobs.inserts)
	}
}

func TestEnqueuePlanAuto_AnotherEventForARequestWithAnActiveJobIsNotAnError(t *testing.T) {
	k := newEnqueueKit()
	req, _ := k.planning(domain.RequestTypeBug)
	k.jobs.seed(domain.PlanAutoJob{TenantID: "t1", RequestID: req.ID, Anchor: "retry-1", Origin: domain.PlanAutoOriginRetry, State: domain.PlanAutoRunning, Attempt: 1, MaxAttempts: 1})
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-2", req)); err != nil {
		t.Fatal(err)
	}
	if k.jobs.count() != 1 {
		t.Fatalf("a second active job was created: %d", k.jobs.count())
	}
}

func TestEnqueuePlanAuto_BothSwitchesMustBeOn(t *testing.T) {
	cases := []struct {
		name string
		sw   func(k *enqueueKit) PlanAutoSwitch
	}{
		{"deployment switch off", func(k *enqueueKit) PlanAutoSwitch {
			return PlanAutoSwitch{EnvOn: false, CommitWired: true, Settings: k.rows}
		}},
		{"commit not wired", func(k *enqueueKit) PlanAutoSwitch {
			return PlanAutoSwitch{EnvOn: true, CommitWired: false, Settings: k.rows}
		}},
		{"project switch off", func(k *enqueueKit) PlanAutoSwitch {
			return PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: k.rows}
		}},
	}
	for _, c := range cases {
		k := newEnqueueKit()
		req, _ := k.planning(domain.RequestTypeBug)
		if c.name == "project switch off" {
			k.rows.rows[req.ProjectID] = domain.ProjectEngineSettings{ProjectID: req.ProjectID, PlanAutoGenerate: false}
		}
		k.build(c.sw(k))
		if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if k.jobs.count() != 0 {
			t.Errorf("%s: a job was created", c.name)
		}
	}
}

func TestEnqueuePlanAuto_ASettingsReadFailureIsRedeliveredAndCreatesNothing(t *testing.T) {
	k := newEnqueueKit()
	req, _ := k.planning(domain.RequestTypeBug)
	k.build(PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: failingEngineRows{err: errBoom}})
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err == nil {
		t.Fatal("the failure must be returned so the bus redelivers")
	}
	if k.jobs.count() != 0 {
		t.Fatal("no job may exist after a failed read")
	}
	// The mark rolled back with the transaction, so the redelivery is handled, not skipped as already processed.
	k.build(PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: k.rows})
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil || k.jobs.count() != 1 {
		t.Fatalf("redelivery: err=%v jobs=%d", err, k.jobs.count())
	}
}

func TestEnqueuePlanAuto_ADatabaseFailureOfTheInsertIsReturned(t *testing.T) {
	k := newEnqueueKit()
	req, _ := k.planning(domain.RequestTypeBug)
	k.jobs.failInsert = errBoom
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err == nil {
		t.Fatal("want the insert failure")
	}
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil || k.jobs.count() != 1 {
		t.Fatalf("redelivery: err=%v jobs=%d", err, k.jobs.count())
	}
}

func TestEnqueuePlanAuto_TypesAndTriggersThatNeverRunOnTheirOwn(t *testing.T) {
	for _, typ := range []domain.RequestType{domain.RequestTypeSpike, domain.RequestTypeQuestion, domain.RequestTypeHotfix} {
		k := newEnqueueKit()
		req, _ := k.planning(typ)
		if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil || k.jobs.count() != 0 {
			t.Errorf("%s: err=%v jobs=%d", typ, err, k.jobs.count())
		}
	}
	k := newEnqueueKit()
	req, _ := k.planning(domain.RequestTypeBug)
	for _, trigger := range []domain.Trigger{domain.TriggerPlanRevision, domain.TriggerAnalysisReady, domain.TriggerInformationProvided} {
		ev := approvedEvent("ev-"+string(trigger), req)
		ev.Trigger = string(trigger)
		if err := k.enq.Handle(planAutoTenantCtx(), ev); err != nil || k.jobs.count() != 0 {
			t.Errorf("%s: err=%v jobs=%d", trigger, err, k.jobs.count())
		}
	}
	other := approvedEvent("ev-other", req)
	other.To = "analyzing"
	if err := k.enq.Handle(planAutoTenantCtx(), other); err != nil || k.jobs.count() != 0 {
		t.Fatalf("a change to another status started a job: %v", err)
	}
}

func TestEnqueuePlanAuto_ARequestThatAlreadyHasAPlanGetsNoJob(t *testing.T) {
	k := newEnqueueKit()
	req, _ := k.planning(domain.RequestTypeBug)
	r := k.w.requests[req.ID]
	r.PlanTaskID = "11111111-1111-4111-8111-111111111111"
	k.w.requests[req.ID] = r
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil || k.jobs.count() != 0 {
		t.Fatalf("err=%v jobs=%d", err, k.jobs.count())
	}
}

func TestEnqueuePlanAuto_ARequestThatLeftPlanningGetsNoJob(t *testing.T) {
	k := newEnqueueKit()
	req, _ := k.planning(domain.RequestTypeBug)
	r := k.w.requests[req.ID]
	r.Status = domain.RequestStatusCancelled
	k.w.requests[req.ID] = r
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil || k.jobs.count() != 0 {
		t.Fatalf("err=%v jobs=%d", err, k.jobs.count())
	}
}

func TestEnqueuePlanAuto_TypeConfirmationOnlyCountsForTypesWithoutAnAnalysis(t *testing.T) {
	k := newEnqueueKit()
	task, _ := k.planning(domain.RequestTypeTask)
	ev := approvedEvent("ev-ty", task)
	ev.Trigger = string(domain.TriggerTypeConfirmed)
	if err := k.enq.Handle(planAutoTenantCtx(), ev); err != nil {
		t.Fatal(err)
	}
	if job := k.jobs.get(task.ID, "ty-ev-ty"); job.State != domain.PlanAutoPending {
		t.Fatalf("job = %+v", job)
	}

	bug, _ := k.planning(domain.RequestTypeBug)
	ev = approvedEvent("ev-ty2", bug)
	ev.Trigger = string(domain.TriggerTypeConfirmed)
	if err := k.enq.Handle(planAutoTenantCtx(), ev); err != nil || k.jobs.count() != 1 {
		t.Fatalf("a bug confirmation started a job: err=%v jobs=%d", err, k.jobs.count())
	}
}

func TestEnqueuePlanAuto_WithoutAnApprovedAnalysisNothingIsQueued(t *testing.T) {
	k := newEnqueueKit()
	req := k.w.seedRequest(domain.RequestTypeBug, domain.RequestStatusPlanning)
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", k.w.requests[req.ID])); err != nil || k.jobs.count() != 0 {
		t.Fatalf("err=%v jobs=%d", err, k.jobs.count())
	}
}

func TestEnqueuePlanAuto_AUnknownRequestIsAcked(t *testing.T) {
	k := newEnqueueKit()
	ev := PlanAutoEvent{ID: "ev-1", TenantID: "t1", RequestID: "00000000-0000-4000-8000-000000000000", To: "planning", Trigger: string(domain.TriggerAnalysisApproved)}
	if err := k.enq.Handle(planAutoTenantCtx(), ev); err != nil || k.jobs.count() != 0 {
		t.Fatalf("err=%v jobs=%d", err, k.jobs.count())
	}
}

func TestEnqueuePlanAuto_MaxAttemptsFollowThePolicy(t *testing.T) {
	k := newEnqueueKit()
	k.policy.MaxExtraAttempts = 0
	k.build(PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: k.rows})
	req, solutionID := k.planning(domain.RequestTypeBug)
	if err := k.enq.Handle(planAutoTenantCtx(), approvedEvent("ev-1", req)); err != nil {
		t.Fatal(err)
	}
	if got := k.jobs.get(req.ID, solutionID).MaxAttempts; got != 1 {
		t.Fatalf("max attempts = %d", got)
	}
}
