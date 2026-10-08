package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func newLcTransition(s *lcStore) *TransitionRequest { return NewTransitionRequest(s, s, s) }

func decodeStatusChanged(t *testing.T, ev domain.OutboxEvent) StatusChangedPayload {
	t.Helper()
	var p StatusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTransition_HappyPath_WritesStatusChangedEvent(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) {
		r.Status = domain.RequestStatusAwaitingTypeConfirmation
		r.Type = domain.RequestTypeBug
	})
	res, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{
		RequestID: r.ID, Trigger: domain.TriggerTypeConfirmed, ExpectedFrom: lcStatus(r.Status), ActorID: "u1", ActorKind: domain.ActorKindUser,
	})
	if err != nil || !res.Applied || res.Request.Status != domain.RequestStatusAnalyzing {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(s.events) != 1 || s.events[0].Subject != "orca.request.request.status_changed" {
		t.Fatalf("events %v", s.subjects())
	}
	p := decodeStatusChanged(t, s.events[0])
	want := StatusChangedPayload{RequestID: r.ID, ProjectID: r.ProjectID, Number: r.Number, From: "awaiting_type_confirmation", To: "analyzing",
		Trigger: "type_confirmed", Type: "bug", ActorID: "u1", ActorKind: "user", Version: 2,
		SourceProvider: string(r.SourceProvider), SourceSite: r.SourceSite, SourceRef: r.SourceRef, ReporterID: r.ReporterID}
	p.At = ""
	if p != want {
		t.Fatalf("payload\n got  %+v\n want %+v", p, want)
	}
}

func TestTransition_Completed_EmitsTwoEvents(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusExecuting; r.Type = domain.RequestTypeTask })
	if _, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerExecutionFinished}); err != nil {
		t.Fatal(err)
	}
	got := s.subjects()
	if len(got) != 2 || got[0] != "orca.request.request.status_changed" || got[1] != "orca.request.request.completed" {
		t.Fatalf("events %v", got)
	}
}

func TestTransition_IdempotentRedelivery(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	uc := newLcTransition(s)
	in := TransitionInput{RequestID: r.ID, Trigger: domain.TriggerProposalReady, ExpectedFrom: lcStatus(domain.RequestStatusClassifying)}
	first, err := uc.Execute(lcCtx(), in)
	if err != nil || !first.Applied {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := uc.Execute(lcCtx(), in)
	if err != nil || second.Applied {
		t.Fatalf("second: %+v %v", second, err)
	}
	if len(s.events) != 1 || s.updateCalls != 1 {
		t.Fatalf("redelivery wrote again: events=%d updates=%d", len(s.events), s.updateCalls)
	}
}

func TestTransition_StateStale(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusAnalyzing; r.Type = domain.RequestTypeBug })
	_, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{
		RequestID: r.ID, Trigger: domain.TriggerProposalReady, ExpectedFrom: lcStatus(domain.RequestStatusClassifying),
	})
	if codeOf(err) != "REQUEST_STATE_STALE" {
		t.Fatalf("got %v", err)
	}
}

func TestTransition_ReasonRequired(t *testing.T) {
	cases := map[domain.Trigger]domain.RequestStatus{
		domain.TriggerReturnToBacklog:  domain.RequestStatusPlanning,
		domain.TriggerCancel:           domain.RequestStatusPlanning,
		domain.TriggerAnalysisRejected: domain.RequestStatusAwaitingAnalysisApproval,
		domain.TriggerPlanRejected:     domain.RequestStatusAwaitingPlanApproval,
	}
	for trig, st := range cases {
		s := newLcStore()
		r := s.seed(func(r *domain.Request) { r.Status = st; r.Type = domain.RequestTypeBug })
		_, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{
			RequestID: r.ID, Trigger: trig, Stage: domain.ReturnStagePlan, Category: domain.ReturnCategoryOther, Reason: "  ",
		})
		if codeOf(err) != "REQUEST_REASON_REQUIRED" {
			t.Errorf("%s: %v", trig, err)
		}
	}
}

func TestTransition_TypeNotSet(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusAwaitingTypeConfirmation })
	_, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerTypeConfirmed})
	if codeOf(err) != "REQUEST_TYPE_NOT_SET" {
		t.Fatalf("got %v", err)
	}
}

func TestTransition_UntypedRequestCanReturnAndCancel(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	res, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{
		RequestID: r.ID, Trigger: domain.TriggerReturnToBacklog, Stage: domain.ReturnStageClassification, Category: domain.ReturnCategoryMissingInfo, Reason: "unclear",
	})
	if err != nil || res.Request.Status != domain.RequestStatusRequestBacklog || res.Request.ReturnedFromStage != domain.ReturnStageClassification {
		t.Fatalf("res=%+v err=%v", res.Request, err)
	}
}

func TestTransition_LeavingBacklogClearsStageCategoryAndReason(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) {
		r.Status, r.ReturnedFromStage, r.ReturnedCategory, r.ReturnReason = domain.RequestStatusRequestBacklog, domain.ReturnStagePlan, domain.ReturnCategoryOther, "x"
	})
	res, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerReopen})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Request
	if got.Status != domain.RequestStatusClassifying || got.ReturnedFromStage != "" || got.ReturnedCategory != "" || got.ReturnReason != "" {
		t.Fatalf("not cleared: %+v", got)
	}
}

func TestTransition_BacklogNeedsValidCategoryAndStage(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusPlanning; r.Type = domain.RequestTypeBug })
	uc := newLcTransition(s)
	_, err := uc.Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerReturnToBacklog, Stage: domain.ReturnStagePlan, Reason: "r"})
	if codeOf(err) != "REQUEST_RETURN_CATEGORY_INVALID" {
		t.Fatalf("category: %v", err)
	}
	_, err = uc.Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerReturnToBacklog, Stage: domain.ReturnStageAnalysis, Category: domain.ReturnCategoryOther, Reason: "r"})
	if codeOf(err) != "REQUEST_RETURN_STAGE_INVALID" {
		t.Fatalf("stage: %v", err)
	}
	// Rejection triggers fix the stage themselves.
	s.requests[r.ID] = func() domain.Request {
		x := s.requests[r.ID]
		x.Status = domain.RequestStatusAwaitingPlanApproval
		return x
	}()
	res, err := uc.Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerPlanRejected, Category: domain.ReturnCategoryRejected, Reason: "no"})
	if err != nil || res.Request.ReturnedFromStage != domain.ReturnStagePlan {
		t.Fatalf("plan_rejected: %+v %v", res.Request, err)
	}
	p := decodeStatusChanged(t, s.events[len(s.events)-1])
	if p.Stage != "plan" || p.Category != "rejected" || p.Reason != "no" {
		t.Fatalf("payload %+v", p)
	}
}

func TestTransition_RetriesOnVersionConflictWhenOutermost(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	s.conflicts = 2
	res, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerProposalReady})
	if err != nil || !res.Applied || s.updateCalls != 3 || len(s.events) != 1 {
		t.Fatalf("res=%+v err=%v updates=%d events=%d", res, err, s.updateCalls, len(s.events))
	}
	s.conflicts = 5
	r2 := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	if _, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{RequestID: r2.ID, Trigger: domain.TriggerProposalReady}); codeOf(err) != "REQUEST_VERSION_CONFLICT" {
		t.Fatalf("after 3 attempts: %v", err)
	}
}

func TestTransition_NoRetryWhenNested(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	s.conflicts = 1
	err := s.InTx(lcCtx(), func(ctx context.Context) error {
		_, err := newLcTransition(s).Execute(ctx, TransitionInput{RequestID: r.ID, Trigger: domain.TriggerProposalReady})
		return err
	})
	if codeOf(err) != "REQUEST_VERSION_CONFLICT" || s.updateCalls != 1 {
		t.Fatalf("err=%v updates=%d", err, s.updateCalls)
	}
}

func TestTransition_OutboxFailureRollsBackStatus(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	s.failOutbox = errors.New("outbox down")
	if _, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerProposalReady}); err == nil {
		t.Fatal("want error")
	}
	if got, _ := s.Get(lcCtx(), r.ID); got.Status != domain.RequestStatusClassifying || got.Version != 1 {
		t.Fatalf("status leaked: %+v", got)
	}
}

func TestTransition_RequiresTenant(t *testing.T) {
	s := newLcStore()
	_, err := newLcTransition(s).Execute(context.Background(), TransitionInput{RequestID: "x", Trigger: domain.TriggerCancel})
	if codeOf(err) != "REQUEST_TENANT_REQUIRED" {
		t.Fatalf("got %v", err)
	}
}
