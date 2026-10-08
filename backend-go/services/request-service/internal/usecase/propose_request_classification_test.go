package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type proposeFixture struct {
	s    *memStore
	cl   *fakeClassifier
	appr *recordingApprovals
	uc   *ProposeRequestClassification
}

func newProposeFixture(out func(int) (domain.ClassificationProposal, error)) *proposeFixture {
	s := newMemStore()
	f := &proposeFixture{s: s, cl: &fakeClassifier{out: out}, appr: &recordingApprovals{}}
	f.uc = NewProposeRequestClassification(s, historyView{s}, s, f.cl, &memTransitioner{s: s}, f.appr, runView{s}, s, s)
	return f
}

func (f *proposeFixture) classifying(t *testing.T) domain.Request {
	return f.s.seed(t, func(r *domain.Request) { r.Status = domain.RequestStatusClassifying })
}

func (f *proposeFixture) classifiedEvents() []map[string]any {
	var out []map[string]any
	for _, e := range f.s.events {
		if e.Subject == domain.SubjectRequestClassified {
			m := map[string]any{}
			_ = json.Unmarshal(e.Payload, &m)
			out = append(out, m)
		}
	}
	return out
}

func TestPropose_Success_WritesAllAndTransitions(t *testing.T) {
	f := newProposeFixture(okProposal(domain.RequestTypeBug))
	r := f.classifying(t)
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, EventID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	got := f.s.requests[r.ID]
	if got.Type != domain.RequestTypeBug || got.Size != domain.RequestSizeM || got.Urgency != domain.UrgencyNormal || got.Confidence == nil || *got.Confidence != 0.9 ||
		got.ClassificationReason != "because" || got.TypeSource != domain.TypeSourceAI || got.ClassificationAttempts != 1 || got.Status != domain.RequestStatusAwaitingTypeConfirmation {
		t.Fatalf("unexpected request: %+v", got)
	}
	if len(f.s.history) != 1 || f.s.history[0].ActorKind != domain.ActorKindAgent || f.s.history[0].ToType != domain.RequestTypeBug {
		t.Fatalf("history = %+v", f.s.history)
	}
	ev := f.classifiedEvents()
	if len(ev) != 1 || ev[0]["failed"] != false || ev[0]["type"] != "bug" || f.appr.opened != 1 {
		t.Fatalf("events=%v approvals=%d", ev, f.appr.opened)
	}
}

func TestPropose_ClassifierFailsTwice_StillProposalReady(t *testing.T) {
	f := newProposeFixture(failProposal(domain.ErrProposalInvalid))
	r := f.classifying(t)
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID}); err != nil {
		t.Fatal(err)
	}
	got := f.s.requests[r.ID]
	if got.Status != domain.RequestStatusAwaitingTypeConfirmation || got.Type != "" || got.ClassificationReason != "invalid classifier output" || got.ClassificationAttempts != 1 {
		t.Fatalf("%+v", got)
	}
	ev := f.classifiedEvents()
	if len(ev) != 1 || ev[0]["failed"] != true || f.appr.opened != 0 || len(f.s.history) != 0 {
		t.Fatalf("events=%v approvals=%d history=%d", ev, f.appr.opened, len(f.s.history))
	}
}

func TestPropose_NoDevServer_FailurePath(t *testing.T) {
	f := newProposeFixture(failProposal(ErrNoDevServer))
	r := f.classifying(t)
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID}); err != nil {
		t.Fatal(err)
	}
	if got := f.s.requests[r.ID]; got.ClassificationReason != "no dev server connected" || got.Type != "" {
		t.Fatalf("%+v", got)
	}
}

func TestPropose_RedeliveryProcessedOnce(t *testing.T) {
	f := newProposeFixture(okProposal(domain.RequestTypeBug))
	r := f.classifying(t)
	ev := uuid.NewString()
	for i := 0; i < 2; i++ {
		if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, EventID: ev}); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.s.requests[r.ID]; got.ClassificationAttempts != 1 || len(f.s.history) != 1 || len(f.classifiedEvents()) != 1 {
		t.Fatalf("attempts=%d history=%d events=%d", got.ClassificationAttempts, len(f.s.history), len(f.classifiedEvents()))
	}
}

func TestPropose_StatusMovedMeanwhile_NoWrite(t *testing.T) {
	var f *proposeFixture
	var id string
	f = newProposeFixture(func(int) (domain.ClassificationProposal, error) {
		// The request is cancelled while the AI is thinking.
		r := f.s.requests[id]
		r.Status = domain.RequestStatusCancelled
		f.s.requests[id] = r
		return domain.ClassificationProposal{Type: domain.RequestTypeBug, Size: domain.RequestSizeS, Urgency: domain.UrgencyNormal, Confidence: 1}, nil
	})
	id = f.classifying(t).ID
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: id}); err != nil {
		t.Fatal(err)
	}
	if got := f.s.requests[id]; got.Type != "" || got.ClassificationAttempts != 0 || len(f.s.events) != 0 {
		t.Fatalf("stale result must be dropped: %+v events=%d", got, len(f.s.events))
	}
}

func TestPropose_RerunKeepsPreviousProposalOnFailure(t *testing.T) {
	f := newProposeFixture(failProposal(ErrClassifierTimeout))
	conf := 0.7
	r := f.s.seed(t, func(r *domain.Request) {
		r.Status, r.Type, r.TypeSource, r.Confidence, r.ClassificationReason, r.ClassificationAttempts = domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeTask, domain.TypeSourceAI, &conf, "old reason", 1
	})
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, Manual: true}); err != nil {
		t.Fatal(err)
	}
	got := f.s.requests[r.ID]
	if got.Type != domain.RequestTypeTask || got.ClassificationReason != "old reason" || *got.Confidence != 0.7 || got.ClassificationAttempts != 2 || got.Status != domain.RequestStatusAwaitingTypeConfirmation {
		t.Fatalf("%+v", got)
	}
	if ev := f.classifiedEvents(); len(ev) != 1 || ev[0]["failed"] != true {
		t.Fatalf("%v", ev)
	}
}

func TestPropose_RerunSuccessRecordsPreviousTypeInHistory(t *testing.T) {
	f := newProposeFixture(okProposal(domain.RequestTypeBug))
	r := f.s.seed(t, func(r *domain.Request) {
		r.Status, r.Type, r.TypeSource = domain.RequestStatusAwaitingTypeConfirmation, domain.RequestTypeTask, domain.TypeSourceAI
	})
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, Manual: true}); err != nil {
		t.Fatal(err)
	}
	if len(f.s.history) != 1 || f.s.history[0].FromType != domain.RequestTypeTask || f.s.history[0].ToType != domain.RequestTypeBug {
		t.Fatalf("%+v", f.s.history)
	}
}

func TestPropose_LimitReached_ManualRejected(t *testing.T) {
	f := newProposeFixture(failProposal(ErrNoDevServer))
	r := f.classifying(t)
	for i := 0; i < domain.MaxClassificationAttempts; i++ {
		if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, Manual: true}); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, Manual: true})
	mustCode(t, err, "REQUEST_CLASSIFICATION_LIMIT")
	if f.cl.calls != domain.MaxClassificationAttempts {
		t.Fatalf("AI called %d times, want %d", f.cl.calls, domain.MaxClassificationAttempts)
	}
}

func TestPropose_LimitReached_ConsumerGoesFailurePath(t *testing.T) {
	f := newProposeFixture(okProposal(domain.RequestTypeBug))
	r := f.s.seed(t, func(r *domain.Request) {
		r.Status, r.ClassificationAttempts = domain.RequestStatusClassifying, domain.MaxClassificationAttempts
	})
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, EventID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	got := f.s.requests[r.ID]
	if f.cl.calls != 0 || got.Status != domain.RequestStatusAwaitingTypeConfirmation || got.ClassificationReason != "classification limit" || got.Type != "" {
		t.Fatalf("calls=%d %+v", f.cl.calls, got)
	}
}

func TestPropose_NotClassifiable(t *testing.T) {
	f := newProposeFixture(okProposal(domain.RequestTypeBug))
	r := f.s.seed(t, func(r *domain.Request) { r.Status = domain.RequestStatusAnalyzing })
	mustCode(t, f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, Manual: true}), "REQUEST_NOT_CLASSIFIABLE")
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID}); err != nil || f.cl.calls != 0 {
		t.Fatalf("consumer path must skip quietly: err=%v calls=%d", err, f.cl.calls)
	}
}

func TestPropose_InjectedContentCannotChangeOutputBeyondEnum(t *testing.T) {
	// The real classifier parses model text; a hostile reply never becomes a proposal.
	_, err := domain.ParseClassificationProposal([]byte(`{"type":"rm -rf /","size":"M","urgency":"normal","confidence":1,"reason":"x"}`))
	if !errors.Is(err, domain.ErrProposalInvalid) {
		t.Fatal(err)
	}
	f := newProposeFixture(failProposal(err))
	r := f.classifying(t)
	if e := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID}); e != nil {
		t.Fatal(e)
	}
	if got := f.s.requests[r.ID]; got.Type != "" {
		t.Fatalf("type must stay unset: %q", got.Type)
	}
}

func TestPropose_ApprovalFailureRollsBackResult(t *testing.T) {
	f := newProposeFixture(okProposal(domain.RequestTypeBug))
	r := f.classifying(t)
	f.s.failOutbox = func(s string) error {
		if s == domain.SubjectRequestClassified {
			return errors.New("outbox down")
		}
		return nil
	}
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, EventID: uuid.NewString()}); err == nil {
		t.Fatal("want error")
	}
	got := f.s.requests[r.ID]
	if got.Type != "" || got.Status != domain.RequestStatusClassifying || len(f.s.processed) != 0 || len(f.s.history) != 0 {
		t.Fatalf("result and processed marker must roll back together: %+v processed=%d", got, len(f.s.processed))
	}
}

func TestPropose_FinishesRunInSameTransaction(t *testing.T) {
	f := newProposeFixture(okProposal(domain.RequestTypeBug))
	r := f.classifying(t)
	run := domain.ClassificationRun{ID: uuid.NewString(), TenantID: testTenant, RequestID: r.ID, Status: domain.ClassificationRunRunning, LeaseExpiresAt: time.Now().Add(time.Minute)}
	f.s.runs[run.ID] = run
	if err := f.uc.Execute(tctx(), ProposeInput{RequestID: r.ID, RunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	if f.s.runs[run.ID].Status != domain.ClassificationRunSucceeded {
		t.Fatalf("run = %+v", f.s.runs[run.ID])
	}
}

var _ = context.Background
