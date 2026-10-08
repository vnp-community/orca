package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type lcLifecycle struct {
	s         *lcStore
	spy       *lcSpyTransitioner
	canceller *lcCanceller
	guard     *lcGuard
	ret       *ReturnRequestToBacklog
	reopen    *ReopenRequest
	cancel    *CancelRequest
}

func newLcLifecycle() *lcLifecycle {
	s := newLcStore()
	spy := &lcSpyTransitioner{inner: newLcTransition(s)}
	c, g := &lcCanceller{}, &lcGuard{}
	return &lcLifecycle{
		s: s, spy: spy, canceller: c, guard: g,
		ret:    NewReturnRequestToBacklog(s, spy, lcHistory{s}, c, g, s, s),
		reopen: NewReopenRequest(s, spy, lcHistory{s}, nil, s),
		cancel: NewCancelRequest(s, spy, lcHistory{s}, c, g, s),
	}
}

func stageFor(st domain.RequestStatus) domain.ReturnStage {
	switch st {
	case domain.RequestStatusClassifying, domain.RequestStatusAwaitingTypeConfirmation:
		return domain.ReturnStageClassification
	case domain.RequestStatusAnalyzing, domain.RequestStatusAwaitingAnalysisApproval:
		return domain.ReturnStageAnalysis
	case domain.RequestStatusPlanning, domain.RequestStatusAwaitingPlanApproval:
		return domain.ReturnStagePlan
	}
	return domain.ReturnStageTask
}

func TestReturn_FromEachStatus_ValidStage_GoesBacklog(t *testing.T) {
	for _, st := range []domain.RequestStatus{
		domain.RequestStatusClassifying, domain.RequestStatusAwaitingTypeConfirmation, domain.RequestStatusAnalyzing,
		domain.RequestStatusAwaitingAnalysisApproval, domain.RequestStatusPlanning, domain.RequestStatusAwaitingPlanApproval, domain.RequestStatusExecuting,
	} {
		l := newLcLifecycle()
		r := l.s.seed(func(r *domain.Request) { r.Status = st; r.Type = domain.RequestTypeTask })
		got, err := l.ret.Execute(lcCtx(), ReturnInput{
			RequestID: r.ID, Stage: stageFor(st), Category: domain.ReturnCategoryMissingInfo, Reason: "need more", ActorID: "u1", ActorKind: domain.ActorKindUser,
		})
		if err != nil {
			t.Fatalf("%s: %v", st, err)
		}
		if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != stageFor(st) ||
			got.ReturnedCategory != domain.ReturnCategoryMissingInfo || got.ReturnReason != "need more" {
			t.Errorf("%s: %+v", st, got)
		}
		if len(l.s.history) != 1 || l.s.history[0].Action != domain.ReturnActionReturned {
			t.Errorf("%s: history %+v", st, l.s.history)
		}
	}
}

func TestReturn_StageMismatchAndPhaseRule(t *testing.T) {
	l := newLcLifecycle()
	r := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusPlanning; r.Type = domain.RequestTypeBug })
	in := ReturnInput{RequestID: r.ID, Stage: domain.ReturnStageAnalysis, Category: domain.ReturnCategoryOther, Reason: "r"}
	if _, err := l.ret.Execute(lcCtx(), in); codeOf(err) != "REQUEST_RETURN_STAGE_INVALID" {
		t.Fatalf("mismatch: %v", err)
	}
	exec := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusExecuting; r.Type = domain.RequestTypeTask })
	in = ReturnInput{RequestID: exec.ID, Stage: domain.ReturnStagePhase, Category: domain.ReturnCategoryOther, Reason: "r"}
	if _, err := l.ret.Execute(lcCtx(), in); codeOf(err) != "REQUEST_RETURN_STAGE_INVALID" {
		t.Fatalf("task type has no phase: %v", err)
	}
	cr := l.s.seed(func(r *domain.Request) {
		r.Status = domain.RequestStatusExecuting
		r.Type = domain.RequestTypeChangeRequest
	})
	in.RequestID = cr.ID
	if _, err := l.ret.Execute(lcCtx(), in); err != nil {
		t.Fatalf("change_request phase: %v", err)
	}
}

func TestReturn_ReasonCategoryAndVersion(t *testing.T) {
	l := newLcLifecycle()
	r := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusPlanning; r.Type = domain.RequestTypeBug })
	base := ReturnInput{RequestID: r.ID, Stage: domain.ReturnStagePlan, Category: domain.ReturnCategoryOther, Reason: "r"}
	bad := base
	bad.Reason = " "
	if _, err := l.ret.Execute(lcCtx(), bad); codeOf(err) != "REQUEST_REASON_REQUIRED" {
		t.Errorf("reason: %v", err)
	}
	bad = base
	bad.Category = "zzz"
	if _, err := l.ret.Execute(lcCtx(), bad); codeOf(err) != "REQUEST_RETURN_CATEGORY_INVALID" {
		t.Errorf("category: %v", err)
	}
	bad = base
	bad.ExpectedVersion = 99
	if _, err := l.ret.Execute(lcCtx(), bad); codeOf(err) != "REQUEST_VERSION_CONFLICT" {
		t.Errorf("version: %v", err)
	}
	if len(l.s.events) != 0 || len(l.s.history) != 0 {
		t.Errorf("rejected calls wrote data")
	}
}

func TestReturn_Idempotent_SecondCallNoHistoryNoEvent(t *testing.T) {
	l := newLcLifecycle()
	r := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusPlanning; r.Type = domain.RequestTypeBug })
	in := ReturnInput{RequestID: r.ID, Stage: domain.ReturnStagePlan, Category: domain.ReturnCategoryOther, Reason: "r"}
	if _, err := l.ret.Execute(lcCtx(), in); err != nil {
		t.Fatal(err)
	}
	events, hist := len(l.s.events), len(l.s.history)
	if _, err := l.ret.Execute(lcCtx(), in); err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(l.s.events) != events || len(l.s.history) != hist {
		t.Fatalf("second call wrote: events %d->%d history %d->%d", events, len(l.s.events), hist, len(l.s.history))
	}
	in.Stage = domain.ReturnStageAnalysis
	if _, err := l.ret.Execute(lcCtx(), in); codeOf(err) != "REQUEST_TRANSITION_NOT_ALLOWED" {
		t.Fatalf("other stage: %v", err)
	}
}

func TestReturn_RejectedUsesRejectedTriggers(t *testing.T) {
	cases := map[domain.RequestStatus]domain.Trigger{
		domain.RequestStatusAwaitingAnalysisApproval: domain.TriggerAnalysisRejected,
		domain.RequestStatusAwaitingPlanApproval:     domain.TriggerPlanRejected,
		domain.RequestStatusPlanning:                 domain.TriggerReturnToBacklog,
	}
	for st, want := range cases {
		l := newLcLifecycle()
		r := l.s.seed(func(r *domain.Request) { r.Status = st; r.Type = domain.RequestTypeBug })
		if _, err := l.ret.Execute(lcCtx(), ReturnInput{RequestID: r.ID, Stage: stageFor(st), Category: domain.ReturnCategoryRejected, Reason: "no"}); err != nil {
			t.Fatalf("%s: %v", st, err)
		}
		if got := l.spy.seen[0].Trigger; got != want {
			t.Errorf("%s: trigger %s, want %s", st, got, want)
		}
	}
}

func TestReturn_ExecutingActiveBlocked(t *testing.T) {
	l := newLcLifecycle()
	l.guard.active = true
	r := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusExecuting; r.Type = domain.RequestTypeTask })
	_, err := l.ret.Execute(lcCtx(), ReturnInput{RequestID: r.ID, Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryOther, Reason: "r"})
	if codeOf(err) != "REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION" {
		t.Fatalf("got %v", err)
	}
}

func TestReturn_CancelsPendingApproval_EmitsReturnedAndStatusChanged(t *testing.T) {
	l := newLcLifecycle()
	r := l.s.seed(func(r *domain.Request) {
		r.Status = domain.RequestStatusAwaitingPlanApproval
		r.Type = domain.RequestTypeBug
	})
	if _, err := l.ret.Execute(lcCtx(), ReturnInput{RequestID: r.ID, Stage: domain.ReturnStagePlan, Category: domain.ReturnCategoryBlockedDependency, Reason: "blocked", ActorID: "u9", ActorKind: domain.ActorKindSystem}); err != nil {
		t.Fatal(err)
	}
	if len(l.canceller.calls) != 1 || l.canceller.calls[0] != r.ID+":returned" {
		t.Errorf("canceller %v", l.canceller.calls)
	}
	got := l.s.subjects()
	if len(got) != 2 || got[0] != "orca.request.request.status_changed" || got[1] != "orca.request.request.returned" {
		t.Fatalf("events %v", got)
	}
	var p ReturnedPayload
	_ = jsonUnmarshal(l.s.events[1].Payload, &p)
	if p != (ReturnedPayload{RequestID: r.ID, Stage: "plan", Category: "blocked_dependency", Reason: "blocked", ActorID: "u9"}) {
		t.Errorf("returned payload %+v", p)
	}
}

func TestReturn_NewAndFinalStatusesRejected(t *testing.T) {
	for _, st := range []domain.RequestStatus{domain.RequestStatusNew, domain.RequestStatusCompleted, domain.RequestStatusCancelled} {
		l := newLcLifecycle()
		r := l.s.seed(func(r *domain.Request) { r.Status = st })
		_, err := l.ret.Execute(lcCtx(), ReturnInput{RequestID: r.ID, Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryOther, Reason: "r"})
		if codeOf(err) != "REQUEST_TRANSITION_NOT_ALLOWED" {
			t.Errorf("%s: %v", st, err)
		}
	}
}
