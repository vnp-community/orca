package usecase

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type confirmFixture struct {
	s    *memStore
	appr *recordingApprovals
	uc   *ConfirmRequestType
}

func newConfirmFixture() *confirmFixture {
	s := newMemStore()
	a := &recordingApprovals{}
	return &confirmFixture{s: s, appr: a, uc: NewConfirmRequestType(s, historyView{s}, &memTransitioner{s: s}, a, s, s)}
}

func (f *confirmFixture) awaiting(t *testing.T, mod func(r *domain.Request)) domain.Request {
	return f.s.seed(t, func(r *domain.Request) {
		r.Status = domain.RequestStatusAwaitingTypeConfirmation
		if mod != nil {
			mod(r)
		}
	})
}

func aiProposed(typ domain.RequestType, size domain.RequestSize) func(r *domain.Request) {
	return func(r *domain.Request) {
		c := 0.8
		r.Type, r.Size, r.TypeSource, r.Confidence = typ, size, domain.TypeSourceAI, &c
	}
}

func userIn(id, typ string) ConfirmInput {
	return ConfirmInput{RequestID: id, Type: typ, ActorID: testReporter, ActorKind: domain.ActorKindUser}
}

func TestConfirm_AcceptAIProposal_NoHistoryRow_TypeSourceAI(t *testing.T) {
	f := newConfirmFixture()
	r := f.awaiting(t, aiProposed(domain.RequestTypeBug, domain.RequestSizeM))
	in := userIn(r.ID, "bug")
	in.Size = "M"
	out, err := f.uc.Execute(tctx(), in)
	if err != nil {
		t.Fatal(err)
	}
	got := f.s.requests[r.ID]
	if got.TypeSource != domain.TypeSourceAI || len(f.s.history) != 0 || out.Status != domain.RequestStatusAnalyzing || f.appr.approved != 1 {
		t.Fatalf("%+v history=%d approved=%d", got, len(f.s.history), f.appr.approved)
	}
}

func TestConfirm_Override_HistoryRowAndTypeSourceHuman(t *testing.T) {
	f := newConfirmFixture()
	r := f.awaiting(t, aiProposed(domain.RequestTypeBug, domain.RequestSizeM))
	in := userIn(r.ID, "task")
	in.Reason = "it is small"
	if _, err := f.uc.Execute(tctx(), in); err != nil {
		t.Fatal(err)
	}
	got := f.s.requests[r.ID]
	if got.TypeSource != domain.TypeSourceHuman || got.Type != domain.RequestTypeTask || len(f.s.history) != 1 {
		t.Fatalf("%+v", got)
	}
	h := f.s.history[0]
	if h.FromType != domain.RequestTypeBug || h.ToType != domain.RequestTypeTask || h.ActorKind != domain.ActorKindUser || h.Reason != "it is small" {
		t.Fatalf("%+v", h)
	}
}

func TestConfirm_NoAIProposal_ManualPick_HistoryRow(t *testing.T) {
	f := newConfirmFixture()
	r := f.awaiting(t, nil)
	if _, err := f.uc.Execute(tctx(), userIn(r.ID, "question")); err != nil {
		t.Fatal(err)
	}
	if len(f.s.history) != 1 || f.s.history[0].FromType != "" || f.s.requests[r.ID].TypeSource != domain.TypeSourceHuman {
		t.Fatalf("%+v", f.s.history)
	}
}

func TestConfirm_RuleRejections(t *testing.T) {
	cases := map[string]struct {
		in   ConfirmInput
		want string
	}{
		"bug without size":      {ConfirmInput{Type: "bug"}, "REQUEST_SIZE_REQUIRED"},
		"refactor without size": {ConfirmInput{Type: "refactor"}, "REQUEST_SIZE_REQUIRED"},
		"hotfix normal":         {ConfirmInput{Type: "hotfix", Reason: "x"}, "REQUEST_HOTFIX_REQUIRES_URGENT"},
		"security no reason":    {ConfirmInput{Type: "security"}, "REQUEST_REASON_REQUIRED"},
		"unknown type":          {ConfirmInput{Type: "epic"}, "REQUEST_INVALID_TYPE"},
		"bad size":              {ConfirmInput{Type: "task", Size: "XL"}, "REQUEST_INVALID_SIZE"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newConfirmFixture()
			r := f.awaiting(t, nil)
			c.in.RequestID, c.in.ActorID, c.in.ActorKind = r.ID, testReporter, domain.ActorKindUser
			_, err := f.uc.Execute(tctx(), c.in)
			mustCode(t, err, c.want)
			if f.s.requests[r.ID].Status != domain.RequestStatusAwaitingTypeConfirmation {
				t.Fatal("rejected confirmation must not change status")
			}
		})
	}
}

func TestConfirm_UrgencyOmittedKeepsAIUrgency(t *testing.T) {
	f := newConfirmFixture()
	r := f.awaiting(t, func(r *domain.Request) {
		aiProposed(domain.RequestTypeHotfix, domain.RequestSizeS)(r)
		r.Urgency = domain.UrgencyUrgent
	})
	in := userIn(r.ID, "hotfix")
	in.Reason = "prod is down"
	if _, err := f.uc.Execute(tctx(), in); err != nil {
		t.Fatal(err)
	}
	if f.s.requests[r.ID].Urgency != domain.UrgencyUrgent {
		t.Fatal("urgency was erased")
	}
}

func TestConfirm_GoesToAnalyzingOrPlanning(t *testing.T) {
	want := map[string]domain.RequestStatus{"task": domain.RequestStatusPlanning, "docs": domain.RequestStatusPlanning, "ops_request": domain.RequestStatusPlanning, "change_request": domain.RequestStatusAnalyzing, "question": domain.RequestStatusAnalyzing}
	for typ, st := range want {
		f := newConfirmFixture()
		r := f.awaiting(t, nil)
		out, err := f.uc.Execute(tctx(), userIn(r.ID, typ))
		if err != nil || out.Status != st {
			t.Errorf("%s: status %s err %v, want %s", typ, out.Status, err, st)
		}
	}
}

func TestConfirm_IdempotentSecondCall_NoExtraEvent(t *testing.T) {
	f := newConfirmFixture()
	r := f.awaiting(t, aiProposed(domain.RequestTypeBug, domain.RequestSizeM))
	in := userIn(r.ID, "bug")
	in.Size = "M"
	if _, err := f.uc.Execute(tctx(), in); err != nil {
		t.Fatal(err)
	}
	events, updates := len(f.s.events), f.s.updateCalls
	out, err := f.uc.Execute(tctx(), in)
	if err != nil || out.Status != domain.RequestStatusAnalyzing || len(f.s.events) != events || f.s.updateCalls != updates {
		t.Fatalf("second call must be a no-op: %+v err=%v", out, err)
	}
}

func TestConfirm_StaleVersion_WrongStatus_NonUserActor(t *testing.T) {
	f := newConfirmFixture()
	r := f.awaiting(t, nil)
	in := userIn(r.ID, "task")
	in.ExpectedVersion = r.Version + 5
	_, err := f.uc.Execute(tctx(), in)
	mustCode(t, err, "REQUEST_VERSION_CONFLICT")

	other := f.s.seed(t, func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
	_, err = f.uc.Execute(tctx(), userIn(other.ID, "task"))
	mustCode(t, err, "REQUEST_TRANSITION_NOT_ALLOWED")

	bad := userIn(r.ID, "task")
	bad.ActorKind = domain.ActorKindAgent
	_, err = f.uc.Execute(tctx(), bad)
	mustCode(t, err, "REQUEST_ACTOR_NOT_ALLOWED")

	_, err = f.uc.Execute(tctx(), userIn(uuid.NewString(), "task"))
	mustCode(t, err, "REQUEST_NOT_FOUND")
}

func TestConfirm_EmitsTypeConfirmedWithApprovalInSameTx(t *testing.T) {
	f := newConfirmFixture()
	r := f.awaiting(t, nil)
	f.s.failOutbox = func(s string) error {
		if s == domain.SubjectRequestTypeConfirmed {
			return errFake
		}
		return nil
	}
	if _, err := f.uc.Execute(tctx(), userIn(r.ID, "task")); err == nil {
		t.Fatal("want error")
	}
	if got := f.s.requests[r.ID]; got.Type != "" || got.Status != domain.RequestStatusAwaitingTypeConfirmation || len(f.s.history) != 0 {
		t.Fatalf("whole confirmation must roll back: %+v", got)
	}
}
