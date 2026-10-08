package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestReopen_FromBacklog_GoesClassifying_ClearsReturnColumns(t *testing.T) {
	l := newLcLifecycle()
	r := l.s.seed(func(r *domain.Request) {
		r.Status, r.ReturnedFromStage, r.ReturnedCategory, r.ReturnReason = domain.RequestStatusRequestBacklog, domain.ReturnStageAnalysis, domain.ReturnCategoryOther, "x"
		r.Type = domain.RequestTypeBug
	})
	got, err := l.reopen.Execute(lcCtx(), ReopenInput{RequestID: r.ID, Note: "retry", ActorID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.RequestStatusClassifying || got.ReturnedFromStage != "" || got.ReturnedCategory != "" || got.ReturnReason != "" || got.Type != domain.RequestTypeBug {
		t.Fatalf("%+v", got)
	}
	if len(l.s.history) != 1 || l.s.history[0].Action != domain.ReturnActionReopened || l.s.history[0].Reason != "retry" {
		t.Fatalf("history %+v", l.s.history)
	}
	// Only status_changed: no dedicated reopened event exists.
	if sub := l.s.subjects(); len(sub) != 1 || sub[0] != "orca.request.request.status_changed" {
		t.Fatalf("events %v", sub)
	}
	if p := decodeStatusChanged(t, l.s.events[0]); p.Trigger != "reopen" || p.To != "classifying" {
		t.Fatalf("payload %+v", p)
	}
}

type lcAttempts struct{ calls []string }

func (a *lcAttempts) ResetClassificationAttempts(_ context.Context, id string) error {
	a.calls = append(a.calls, id)
	return nil
}

func TestReopen_ResetsClassificationAttemptsWhenWired(t *testing.T) {
	l := newLcLifecycle()
	a := &lcAttempts{}
	uc := NewReopenRequest(l.s, l.spy, lcHistory{l.s}, a, l.s)
	r := l.s.seed(func(r *domain.Request) {
		r.Status, r.ReturnedFromStage, r.ReturnedCategory = domain.RequestStatusRequestBacklog, domain.ReturnStagePlan, domain.ReturnCategoryOther
	})
	if _, err := uc.Execute(lcCtx(), ReopenInput{RequestID: r.ID}); err != nil || len(a.calls) != 1 || a.calls[0] != r.ID {
		t.Fatalf("err=%v calls=%v", err, a.calls)
	}
}

func TestReopen_FromOtherStatus_Rejected(t *testing.T) {
	for _, st := range domain.AllRequestStatuses() {
		if st == domain.RequestStatusRequestBacklog {
			continue
		}
		l := newLcLifecycle()
		r := l.s.seed(func(r *domain.Request) { r.Status = st })
		if _, err := l.reopen.Execute(lcCtx(), ReopenInput{RequestID: r.ID}); codeOf(err) != "REQUEST_REOPEN_NOT_ALLOWED" {
			t.Errorf("%s: %v", st, err)
		}
	}
}

func TestReopen_VersionConflict(t *testing.T) {
	l := newLcLifecycle()
	r := l.s.seed(func(r *domain.Request) {
		r.Status, r.ReturnedFromStage, r.ReturnedCategory = domain.RequestStatusRequestBacklog, domain.ReturnStagePlan, domain.ReturnCategoryOther
	})
	if _, err := l.reopen.Execute(lcCtx(), ReopenInput{RequestID: r.ID, ExpectedVersion: 7}); codeOf(err) != "REQUEST_VERSION_CONFLICT" {
		t.Fatalf("got %v", err)
	}
}

func TestCancel_FromBacklogAndAnalyzing_OK(t *testing.T) {
	for _, st := range []domain.RequestStatus{domain.RequestStatusRequestBacklog, domain.RequestStatusAnalyzing, domain.RequestStatusNew} {
		l := newLcLifecycle()
		r := l.s.seed(func(r *domain.Request) {
			r.Status = st
			if st == domain.RequestStatusRequestBacklog {
				r.ReturnedFromStage, r.ReturnedCategory = domain.ReturnStagePlan, domain.ReturnCategoryOther
			}
		})
		res, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: r.ID, Reason: "dup", ActorID: "u1"})
		if err != nil || !res.Applied || res.Request.Status != domain.RequestStatusCancelled || res.Request.ReturnedFromStage != "" {
			t.Fatalf("%s: %+v %v", st, res, err)
		}
		if len(l.canceller.calls) != 1 || l.canceller.calls[0] != r.ID+":cancelled" {
			t.Errorf("%s: canceller %v", st, l.canceller.calls)
		}
		if len(l.s.history) != 1 || l.s.history[0].Action != domain.ReturnActionCancelled {
			t.Errorf("%s: history %+v", st, l.s.history)
		}
	}
}

func TestCancel_Completed_Rejected_Twice_SuccessNoApplied(t *testing.T) {
	l := newLcLifecycle()
	done := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusCompleted })
	if _, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: done.ID, Reason: "x"}); codeOf(err) != "REQUEST_CANCEL_NOT_ALLOWED" {
		t.Fatalf("completed: %v", err)
	}
	r := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusPlanning; r.Type = domain.RequestTypeBug })
	if _, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: r.ID, Reason: "x"}); err != nil {
		t.Fatal(err)
	}
	events, hist := len(l.s.events), len(l.s.history)
	res, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: r.ID, Reason: "x"})
	if err != nil || res.Applied || res.Request.Status != domain.RequestStatusCancelled {
		t.Fatalf("second: %+v %v", res, err)
	}
	if len(l.s.events) != events || len(l.s.history) != hist {
		t.Fatal("second cancel wrote data")
	}
}

func TestCancel_ExecutingActive_Blocked_ReasonRequired(t *testing.T) {
	l := newLcLifecycle()
	l.guard.active = true
	r := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusExecuting; r.Type = domain.RequestTypeTask })
	if _, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: r.ID, Reason: "x"}); codeOf(err) != "REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION" {
		t.Fatalf("blocked: %v", err)
	}
	l.guard.active = false
	if _, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: r.ID, Reason: ""}); codeOf(err) != "REQUEST_REASON_REQUIRED" {
		t.Fatalf("reason: %v", err)
	}
}

func TestCancel_DoesNotTouchChildren(t *testing.T) {
	l := newLcLifecycle()
	parent := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusAnalyzing; r.Type = domain.RequestTypeBug })
	child := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusAnalyzing; r.Type = domain.RequestTypeBug })
	l.s.links = append(l.s.links, domain.RequestLink{ParentRequestID: parent.ID, ChildRequestID: child.ID, Reason: domain.LinkReasonEscalation})
	if _, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: parent.ID, Reason: "x"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := l.s.Get(lcCtx(), child.ID); got.Status != domain.RequestStatusAnalyzing || got.Version != 1 {
		t.Fatalf("child touched: %+v", got)
	}
}
