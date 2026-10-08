package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestExpire_ReturnsRequestToBacklogWithMissingInfo(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	f.now = c.DueAt.Add(time.Minute)
	n, err := f.sweeper.ExpireOnce(context.Background(), 10)
	if err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	got := f.env.s.requests[r.ID]
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryMissingInfo || got.ReturnReason != "clarification_expired" || got.ReturnedFromStage != domain.ReturnStageClassification {
		t.Fatalf("%+v", got)
	}
	if cl := f.env.clars[c.ID]; cl.Status != domain.ClarificationStatusExpired {
		t.Fatalf("%+v", cl)
	}
	seen := f.spy.seen[len(f.spy.seen)-1]
	if seen.ActorKind != domain.ActorKindSystem {
		t.Fatalf("expiry is a system action: %+v", seen)
	}
	evs := f.eventsOf(domain.SubjectClarificationExpired)
	if len(evs) != 1 {
		t.Fatalf("%d expired events", len(evs))
	}
	var n2 ClarificationNotice
	_ = json.Unmarshal(evs[0].Payload, &n2)
	if n2.Title != noticeTitleExpired || len(n2.UserIDs) != 1 || strings.Contains(string(evs[0].Payload), "Hỏi") {
		t.Fatalf("%+v", n2)
	}
	if m, _ := f.sweeper.ExpireOnce(context.Background(), 10); m != 0 {
		t.Fatalf("a second sweep must find nothing, got %d", m)
	}
}

func TestExpire_StageMapping_AllSources(t *testing.T) {
	cases := []struct {
		from   domain.RequestStatus
		source domain.ClarificationSource
		actor  domain.ActorKind
		stage  domain.ReturnStage
	}{
		{domain.RequestStatusAnalyzing, domain.ClarificationSourceSolutionOpenQuestion, domain.ActorKindUser, domain.ReturnStageAnalysis},
		{domain.RequestStatusAwaitingPlanApproval, domain.ClarificationSourcePlanAssumption, domain.ActorKindUser, domain.ReturnStagePlan},
		{domain.RequestStatusExecuting, domain.ClarificationSourceTaskBlocked, domain.ActorKindSystem, domain.ReturnStageTask},
		{domain.RequestStatusPlanning, domain.ClarificationSourceManual, domain.ActorKindUser, domain.ReturnStagePlan},
	}
	for _, c := range cases {
		f := newClFx()
		r := f.seedIn(c.from, nil)
		cl, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{RequestID: r.ID, Source: c.source, Questions: oneQuestion("k"), ActorKind: c.actor})
		if err != nil {
			t.Fatalf("%s: %v", c.source, err)
		}
		f.now = cl.DueAt
		if n, err := f.sweeper.ExpireOnce(context.Background(), 10); err != nil || n != 1 {
			t.Fatalf("%s: %d %v", c.source, n, err)
		}
		if got := f.env.s.requests[r.ID].ReturnedFromStage; got != c.stage {
			t.Errorf("%s: stage %s, want %s", c.source, got, c.stage)
		}
	}
}

func TestExpire_TwoInstances_NoDoubleProcessing(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	f.now = c.DueAt.Add(time.Hour)
	// Another worker already holds the row (SKIP LOCKED): this sweep must leave it alone.
	f.env.locked[c.ID] = true
	// the fake releases locks at the end of each transaction; model the foreign hold by re-locking inside LockOpenDue's view
	held := clarHolding{clarView{f.env}, c.ID}
	other := NewClarificationSweeper(f.env.s, held, f.ret, PrincipalRecipients{}, f.env, f.env.s).WithClock(func() time.Time { return f.now })
	if n, err := other.ExpireOnce(context.Background(), 10); err != nil || n != 0 {
		t.Fatalf("a locked row must be skipped: %d %v", n, err)
	}
	if f.env.clars[c.ID].Status != domain.ClarificationStatusOpen {
		t.Fatal("the skipped clarification must stay open")
	}
	if n, _ := f.sweeper.ExpireOnce(context.Background(), 10); n != 1 {
		t.Fatalf("the free worker takes it: %d", n)
	}
	if n := len(f.eventsOf(domain.SubjectClarificationExpired)); n != 1 {
		t.Fatalf("handled twice: %d events", n)
	}
}

// clarHolding makes LockOpenDue report "held elsewhere" for one id.
type clarHolding struct {
	clarView
	id string
}

func (h clarHolding) LockOpenDue(ctx context.Context, id string, now time.Time) (*domain.Clarification, error) {
	if id == h.id {
		return nil, nil
	}
	return h.clarView.LockOpenDue(ctx, id, now)
}

func TestExpire_ReturnToBacklogFailure_KeepsClarificationOpen(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	f.now = c.DueAt.Add(time.Minute)
	failing := NewReturnRequestToBacklog(f.env.s, failingTransitioner{}, lcHistory{f.env.s}, f.appr, &lcGuard{}, f.env, f.env.s)
	sweeper := NewClarificationSweeper(f.env.s, clarView{f.env}, failing, PrincipalRecipients{}, f.env, f.env.s).WithClock(func() time.Time { return f.now })
	n, err := sweeper.ExpireOnce(context.Background(), 10)
	if err != nil || n != 0 {
		t.Fatalf("a failing item is skipped, not fatal: %d %v", n, err)
	}
	if f.env.clars[c.ID].Status != domain.ClarificationStatusOpen || f.env.s.requests[r.ID].Status != domain.RequestStatusAwaitingInformation {
		t.Fatal("the clarification must be open again for the next sweep")
	}
	if len(f.eventsOf(domain.SubjectClarificationExpired)) != 0 {
		t.Fatal("no event for a failed expiry")
	}
	if n, _ := f.sweeper.ExpireOnce(context.Background(), 10); n != 1 {
		t.Fatalf("the next sweep succeeds: %d", n)
	}
}

func TestExpire_AnswerRacingExpiry_OneWins(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	if _, err := f.answerBy(userAda, c, answerAll(c), true); err != nil {
		t.Fatal(err)
	}
	f.now = c.DueAt.Add(time.Hour)
	if n, _ := f.sweeper.ExpireOnce(context.Background(), 10); n != 0 {
		t.Fatalf("an answered clarification cannot expire: %d", n)
	}
}

func TestRemind_OncePerClarification_AtHalfDue(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	window := c.DueAt.Sub(c.CreatedAt)
	f.now = c.CreatedAt.Add(window/2 - time.Second)
	if n, _ := f.sweeper.RemindOnce(context.Background(), 10); n != 0 {
		t.Fatalf("too early: %d", n)
	}
	f.now = c.CreatedAt.Add(window / 2)
	if n, err := f.sweeper.RemindOnce(context.Background(), 10); err != nil || n != 1 {
		t.Fatalf("at half: %d %v", n, err)
	}
	if n, _ := f.sweeper.RemindOnce(context.Background(), 10); n != 0 {
		t.Fatalf("a second reminder: %d", n)
	}
	evs := f.eventsOf(domain.SubjectClarificationRequested)
	var last ClarificationNotice
	if err := json.Unmarshal(evs[len(evs)-1].Payload, &last); err != nil || !last.Reminder || last.Reason != "reminder" || last.Title != noticeTitleReminder {
		t.Fatalf("%+v %v", last, err)
	}
	f.now = c.DueAt.Add(time.Second)
	if n, _ := f.sweeper.RemindOnce(context.Background(), 10); n != 0 {
		t.Fatal("no reminders after the deadline")
	}
}

// ---- resume

type recordingResume struct {
	solutions []string
	advances  []string
	err       error
}

func (r *recordingResume) StartSolution(_ context.Context, id, feedback, key string) error {
	r.solutions = append(r.solutions, id+"|"+feedback+"|"+key)
	return r.err
}

func (r *recordingResume) Advance(_ context.Context, id string) error {
	r.advances = append(r.advances, id)
	return r.err
}

type memProcessed struct{ seen map[string]bool }

func (m *memProcessed) MarkProcessed(_ context.Context, id, _ string) (bool, error) {
	if m.seen[id] {
		return true, nil
	}
	m.seen[id] = true
	return false, nil
}
func (m *memProcessed) Prune(context.Context, time.Time) (int64, error) { return 0, nil }

// resumeFx wraps the env's transaction so the processed marker rolls back with a failed attempt.
type resumeFx struct {
	f    *clFx
	rec  *recordingResume
	proc *procRollback
	uc   *ResumeAfterClarification
}

type procRollback struct {
	inner *memProcessed
	env   *artEnv
}

func (p *procRollback) MarkProcessed(ctx context.Context, id, subj string) (bool, error) {
	return p.inner.MarkProcessed(ctx, id, subj)
}
func (p *procRollback) Prune(context.Context, time.Time) (int64, error) { return 0, nil }

func newResumeFx(t *testing.T) (*resumeFx, domain.Clarification) {
	f := newClFx()
	_, c := f.parked(t)
	if _, err := f.answerBy(userAda, c, answerAll(c), true); err != nil {
		t.Fatal(err)
	}
	rec := &recordingResume{}
	proc := &procRollback{inner: &memProcessed{seen: map[string]bool{}}, env: f.env}
	uc := NewResumeAfterClarification(clarView{f.env}, &rollbackMarker{proc}, f.env).WithAnalysis(rec).WithExecution(rec)
	return &resumeFx{f: f, rec: rec, proc: proc, uc: uc}, c
}

// rollbackMarker un-marks an event when the surrounding transaction fails, as processed_events does.
type rollbackMarker struct{ p *procRollback }

func (m *rollbackMarker) MarkProcessed(ctx context.Context, id, subj string) (bool, error) {
	return m.p.MarkProcessed(ctx, id, subj)
}
func (m *rollbackMarker) Prune(context.Context, time.Time) (int64, error) { return 0, nil }

func provided(id, to string) ResumeEvent {
	return ResumeEvent{ID: id, TenantID: "t1", RequestID: "req", To: to, Trigger: string(domain.TriggerInformationProvided)}
}

func TestResume_AnalyzingTriggersSolutionWithFeedback(t *testing.T) {
	fx, c := newResumeFx(t)
	ev := provided("e1", "analyzing")
	ev.RequestID = c.RequestID
	if err := fx.uc.Handle(lcCtx(), ev); err != nil {
		t.Fatal(err)
	}
	if len(fx.rec.solutions) != 1 || fx.rec.solutions[0] != c.RequestID+"|clarification:"+c.ID+"|clr-"+c.ID {
		t.Fatalf("%v", fx.rec.solutions)
	}
}

func TestResume_PlanningDoesNotAutoRun_ExecutingAdvances(t *testing.T) {
	fx, c := newResumeFx(t)
	ev := provided("e1", "planning")
	ev.RequestID = c.RequestID
	if err := fx.uc.Handle(lcCtx(), ev); err != nil || len(fx.rec.solutions)+len(fx.rec.advances) != 0 {
		t.Fatalf("planning is started by the person: %+v %v", fx.rec, err)
	}
	ev = provided("e2", "executing")
	ev.RequestID = c.RequestID
	if err := fx.uc.Handle(lcCtx(), ev); err != nil || len(fx.rec.advances) != 1 {
		t.Fatalf("%+v %v", fx.rec, err)
	}
}

func TestResume_OtherTriggersIgnored(t *testing.T) {
	fx, _ := newResumeFx(t)
	for _, trig := range []string{"type_confirmed", "cancel", ""} {
		ev := provided("e-"+trig, "analyzing")
		ev.Trigger = trig
		if err := fx.uc.Handle(lcCtx(), ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(fx.rec.solutions) != 0 || len(fx.proc.inner.seen) != 0 {
		t.Fatal("other triggers must be ignored without a trace")
	}
}

func TestResume_RedeliveryNoSecondRun(t *testing.T) {
	fx, c := newResumeFx(t)
	ev := provided("same-event", "analyzing")
	ev.RequestID = c.RequestID
	for i := 0; i < 3; i++ {
		if err := fx.uc.Handle(lcCtx(), ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(fx.rec.solutions) != 1 {
		t.Fatalf("redelivery started %d runs", len(fx.rec.solutions))
	}
}

func TestResume_TransientErrorIsRetried(t *testing.T) {
	fx, c := newResumeFx(t)
	ev := provided("e1", "analyzing")
	ev.RequestID = c.RequestID
	fx.rec.err = errBoom
	if err := fx.uc.Handle(lcCtx(), ev); err == nil {
		t.Fatal("a transient failure must surface so the event is redelivered")
	}
	fx.rec.err = nil
	fx.proc.inner.seen = map[string]bool{} // the marker rolled back with the failed transaction
	if err := fx.uc.Handle(lcCtx(), ev); err != nil || len(fx.rec.solutions) != 2 {
		t.Fatalf("retry: %v %v", err, fx.rec.solutions)
	}
}

func TestResume_SkippedErrorDoesNotLoop(t *testing.T) {
	fx, c := newResumeFx(t)
	ev := provided("e1", "analyzing")
	ev.RequestID = c.RequestID
	fx.rec.err = ErrResumeSkipped
	if err := fx.uc.Handle(lcCtx(), ev); err != nil {
		t.Fatalf("a skipped resume is final, not a retry: %v", err)
	}
}

func TestResume_AutoRegenerateOff(t *testing.T) {
	fx, c := newResumeFx(t)
	fx.uc.WithAutoRegenerate(false)
	ev := provided("e1", "analyzing")
	ev.RequestID = c.RequestID
	if err := fx.uc.Handle(lcCtx(), ev); err != nil || len(fx.rec.solutions) != 0 {
		t.Fatalf("%v %v", err, fx.rec.solutions)
	}
}

func TestResume_WaiverWithoutClarificationUsesEventAsKey(t *testing.T) {
	f := newClFx()
	rec := &recordingResume{}
	uc := NewResumeAfterClarification(clarView{f.env}, &memProcessed{seen: map[string]bool{}}, f.env).WithAnalysis(rec)
	ev := provided("ev-9", "analyzing")
	ev.RequestID = "req-without-clarifications"
	if err := uc.Handle(lcCtx(), ev); err != nil || len(rec.solutions) != 1 || !strings.HasSuffix(rec.solutions[0], "clr-ev-9") {
		t.Fatalf("%v %v", err, rec.solutions)
	}
}

// ---- queries

func TestClarificationQueries_AnswersHiddenFromReadersWhoAreNotAssignees(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	q := questionByKey(c, "type_fields.actual")
	if _, err := f.answerBy(userAda, c, []AnswerItem{{QuestionID: q.ID, Value: json.RawMessage(`"bí mật"`)}}, false); err != nil {
		t.Fatal(err)
	}
	v, err := f.queries.Get(asUser(userAda), c.ID)
	if err != nil || !v.AnswersVisible {
		t.Fatalf("the assignee sees answers: %+v %v", v, err)
	}
	v, _ = f.queries.Get(asUser(userBob), c.ID)
	if v.AnswersVisible {
		t.Fatal("a stranger must not see answers")
	}
	v, _ = f.queries.Get(asAdmin(), c.ID)
	if !v.AnswersVisible {
		t.Fatal("an admin sees answers")
	}
	if _, err := f.queries.Get(asUser(userAda), "missing"); err == nil {
		t.Fatal("unknown id")
	}
}

func TestClarificationQueries_ListAndPending(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	page, err := f.queries.List(asUser(userBob), r.ID, "", 0, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].RequestNumber != r.Number {
		t.Fatalf("%+v %v", page, err)
	}
	if open, _ := f.queries.List(asUser(userBob), r.ID, domain.ClarificationStatusAnswered, 0, 10); len(open.Items) != 0 {
		t.Fatal("status filter")
	}
	pending, err := f.queries.ListPending(asUser(userAda), 10, "")
	if err != nil || len(pending.Items) != 1 || pending.Items[0].Clarification.ID != c.ID {
		t.Fatalf("the reporter sees what is waiting for them: %+v %v", pending, err)
	}
	other, _ := f.queries.ListPending(asUser(userBob), 10, "")
	if len(other.Items) != 0 {
		t.Fatal("a stranger has nothing pending")
	}
	admin, _ := f.queries.ListPending(asAdmin(), 10, "")
	if len(admin.Items) != 1 {
		t.Fatal("an admin sees all open clarifications")
	}
	if _, err := f.queries.ListPending(lcCtx(), 10, ""); err == nil {
		t.Fatal("a user identity is required")
	}
}

// deferredStarter records when the worker would start relative to the commit.
type deferredStarter struct {
	recordingResume
	after int
}

func (d *deferredStarter) StartSolutionDeferred(ctx context.Context, requestID, feedback, key string) (func(), error) {
	if err := d.recordingResume.StartSolution(ctx, requestID, feedback, key); err != nil {
		return nil, err
	}
	return func() { d.after++ }, nil
}

func TestResume_DeferredStarterSpawnsOnlyAfterCommit(t *testing.T) {
	fx, c := newResumeFx(t)
	ds := &deferredStarter{}
	fx.uc.WithAnalysis(ds)
	ev := provided("e-deferred", "analyzing")
	ev.RequestID = c.RequestID

	// A failed start means nothing was committed, so the worker must not run.
	fail := &deferredStarter{}
	fail.recordingResume.err = errBoom
	failing, _ := newResumeFx(t)
	failing.uc.WithAnalysis(fail)
	fev := provided("e-deferred-fail", "analyzing")
	if err := failing.uc.Handle(lcCtx(), fev); err == nil || fail.after != 0 {
		t.Fatalf("failed start: err=%v after=%d", err, fail.after)
	}
	if err := fx.uc.Handle(lcCtx(), ev); err != nil {
		t.Fatal(err)
	}
	if ds.after != 1 || len(ds.recordingResume.solutions) != 1 {
		t.Fatalf("after=%d solutions=%d", ds.after, len(ds.recordingResume.solutions))
	}
	if err := fx.uc.Handle(lcCtx(), ev); err != nil || ds.after != 1 {
		t.Fatalf("redelivery must not start again: err=%v after=%d", err, ds.after)
	}
}
