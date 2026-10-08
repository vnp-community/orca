package usecase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var fixedNow = time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)

type memTypeHistory struct{ rows []domain.RequestTypeChange }

func (h *memTypeHistory) Append(_ context.Context, c domain.RequestTypeChange) error {
	h.rows = append(h.rows, c)
	return nil
}
func (h *memTypeHistory) List(context.Context, string) ([]domain.RequestTypeChange, error) {
	return h.rows, nil
}

// clFx wires the clarification use cases over the in-memory world with the real state machine.
type clFx struct {
	*artFx
	now        time.Time
	appr       *recordingApprovals
	spy        *lcSpyTransitioner
	ret        *ReturnRequestToBacklog
	cancelRq   *CancelRequest
	change     *ChangeRequestType
	clarify    *RequestClarification
	confirm    *ConfirmRequestType
	answer     *AnswerClarification
	cancelCl   *CancelClarification
	waive      *WaiveReadiness
	sweeper    *ClarificationSweeper
	queries    *ClarificationQueries
	gate       *ReadinessGate
	superseded []string
}

type recordingSuperseder struct{ fx *clFx }

func (r recordingSuperseder) SupersedeProposedByRequest(_ context.Context, id string) error {
	r.fx.superseded = append(r.fx.superseded, id)
	return nil
}

func newClFx() *clFx {
	f := &clFx{artFx: newArtFx(), now: fixedNow, appr: &recordingApprovals{}}
	env := f.env
	f.spy = &lcSpyTransitioner{inner: f.tr}
	clarifs := clarView{env}
	closer := NewOpenClarificationCanceller(clarifs, env.s, env.s)
	f.ret = NewReturnRequestToBacklog(env.s, f.spy, lcHistory{env.s}, f.appr, &lcGuard{}, env, env.s).WithClarifications(closer)
	f.cancelRq = NewCancelRequest(env.s, f.spy, lcHistory{env.s}, f.appr, &lcGuard{}, env).WithClarifications(closer)
	f.change = NewChangeRequestType(env.s, &memTypeHistory{}, f.spy, f.appr, f.appr, env, env.s).WithClarifications(closer)
	clock := func() time.Time { return f.now }
	f.clarify = NewRequestClarification(env.s, clarifs, f.spy, f.appr, env, env.s).WithClock(clock)
	f.gate = NewReadinessGate(env.s, f.clarify, clarifs, revisionsView{env}, f.spy, 3)
	f.confirm = NewConfirmRequestType(env.s, &memTypeHistory{}, f.spy, f.appr, env, env.s).WithReadiness(f.gate)
	f.answer = NewAnswerClarification(env.s, clarifs, f.appendRev, f.spy, f.ret, f.clarify, revisionsView{env}, decView{env}, env, env.s, 3).
		WithClock(clock).WithSolutions(recordingSuperseder{f})
	f.cancelCl = NewCancelClarification(env.s, clarifs, f.ret, env, env.s)
	f.waive = NewWaiveReadiness(env.s, clarifs, f.appendRev, f.spy, env, env.s)
	f.sweeper = NewClarificationSweeper(env.s, clarifs, f.ret, PrincipalRecipients{}, env, env.s).WithClock(clock)
	f.queries = NewClarificationQueries(env.s, clarifs)
	return f
}

// seedIn stores a bug request in status st whose content is complete enough to be ready.
func (f *clFx) seedIn(st domain.RequestStatus, mod func(r *domain.Request)) domain.Request {
	return f.seed(func(r *domain.Request) {
		r.Status, r.Type, r.Urgency = st, domain.RequestTypeBug, domain.UrgencyNormal
		if st == domain.RequestStatusRequestBacklog {
			r.ReturnedFromStage, r.ReturnedCategory = domain.ReturnStageAnalysis, domain.ReturnCategoryOther
		}
		if mod != nil {
			mod(r)
		}
	})
}

func oneQuestion(key string) []QuestionInput {
	return []QuestionInput{{QuestionKey: key, Kind: domain.QuestionKindText, Prompt: "Hỏi gì đó?", Reason: "Cần để tiếp tục", Required: true, TargetPath: "type_fields.actual"}}
}

func (f *clFx) ask(t *testing.T, r domain.Request, source domain.ClarificationSource, ref string) domain.Clarification {
	t.Helper()
	c, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{
		RequestID: r.ID, Source: source, SourceRef: ref, Questions: oneQuestion("type_fields.actual"), ActorID: userAda, ActorKind: domain.ActorKindUser,
	})
	if err != nil {
		t.Fatalf("ask %s: %v", source, err)
	}
	return c
}

func TestRequestClarification_FromEachOfSixStates_GoesAwaitingInformation(t *testing.T) {
	for from, source := range map[domain.RequestStatus]domain.ClarificationSource{
		domain.RequestStatusAnalyzing:                domain.ClarificationSourceSolutionOpenQuestion,
		domain.RequestStatusAwaitingAnalysisApproval: domain.ClarificationSourceSolutionOpenQuestion,
		domain.RequestStatusPlanning:                 domain.ClarificationSourcePlanAssumption,
		domain.RequestStatusAwaitingPlanApproval:     domain.ClarificationSourcePlanAssumption,
		domain.RequestStatusExecuting:                domain.ClarificationSourceManual,
	} {
		f := newClFx()
		r := f.seedIn(from, nil)
		c := f.ask(t, r, source, "ref-1")
		got := f.env.s.requests[r.ID]
		if got.Status != domain.RequestStatusAwaitingInformation {
			t.Errorf("%s: status %s", from, got.Status)
		}
		wantResume := map[domain.RequestStatus]domain.RequestStatus{
			domain.RequestStatusAnalyzing: domain.RequestStatusAnalyzing, domain.RequestStatusAwaitingAnalysisApproval: domain.RequestStatusAnalyzing,
			domain.RequestStatusPlanning: domain.RequestStatusPlanning, domain.RequestStatusAwaitingPlanApproval: domain.RequestStatusPlanning,
			domain.RequestStatusExecuting: domain.RequestStatusExecuting,
		}[from]
		if c.ResumeStatus != wantResume || c.Status != domain.ClarificationStatusOpen || c.Round != 1 || c.Seq != 1 || c.AskedRequestRevision != 1 {
			t.Errorf("%s: %+v", from, c)
		}
		if !c.DueAt.Equal(fixedNow.Add(72 * time.Hour)) {
			t.Errorf("%s: due %v", from, c.DueAt)
		}
	}
	// The sixth source state, awaiting_type_confirmation, is reached through readiness.
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	c, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{
		RequestID: r.ID, Source: domain.ClarificationSourceReadiness, Questions: oneQuestion("type_fields.actual"), ActorKind: domain.ActorKindSystem,
	})
	if err != nil || c.ResumeStatus != domain.RequestStatusAnalyzing || !c.DueAt.Equal(fixedNow.Add(7*24*time.Hour)) {
		t.Fatalf("%+v %v", c, err)
	}
	if f.env.s.requests[r.ID].Status != domain.RequestStatusAwaitingInformation {
		t.Fatal("not parked")
	}
}

func TestRequestClarification_FromTerminalOrBacklog_Rejected(t *testing.T) {
	for _, st := range []domain.RequestStatus{domain.RequestStatusNew, domain.RequestStatusClassifying, domain.RequestStatusCompleted, domain.RequestStatusCancelled, domain.RequestStatusRequestBacklog} {
		f := newClFx()
		r := f.seedIn(st, nil)
		_, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{RequestID: r.ID, Source: domain.ClarificationSourceManual, Questions: oneQuestion("k"), ActorKind: domain.ActorKindUser})
		mustCode(t, err, "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED")
		if len(f.env.clars) != 0 || f.env.s.requests[r.ID].Status != st {
			t.Errorf("%s: a refused ask changed something", st)
		}
	}
}

func TestRequestClarification_CancelsPendingApproval(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingAnalysisApproval, nil)
	f.ask(t, r, domain.ClarificationSourceSolutionOpenQuestion, "sol-1")
	if len(f.appr.cancelled) != 1 || f.appr.cancelled[0] != "information_required" {
		t.Fatalf("cancelled = %v", f.appr.cancelled)
	}
}

func TestRequestClarification_Idempotent_SameKeys(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAnalyzing, nil)
	first := f.ask(t, r, domain.ClarificationSourceManual, "")
	again, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{
		RequestID: r.ID, Source: domain.ClarificationSourceManual, Questions: oneQuestion("type_fields.actual"), ActorKind: domain.ActorKindUser,
	})
	if err != nil || again.ID != first.ID || len(f.env.clars) != 1 {
		t.Fatalf("a repeat must return the open one: %+v %v", again, err)
	}
	_, err = f.clarify.Execute(asUser(userAda), RequestClarificationInput{
		RequestID: r.ID, Source: domain.ClarificationSourceManual, Questions: oneQuestion("something.else"), ActorKind: domain.ActorKindUser,
	})
	mustCode(t, err, "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED")
}

func TestRequestClarification_SecondOpenOneLoses(t *testing.T) {
	// Two callers race: the repository's one-open rule is the last line; the loser's ask must fail and leave the request untouched.
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAnalyzing, nil)
	f.env.clars["rival"] = &domain.Clarification{ID: "rival", RequestID: r.ID, Seq: 1, Status: domain.ClarificationStatusOpen}
	f.env.s.requests[r.ID] = func() domain.Request { x := f.env.s.requests[r.ID]; return x }()
	// GetOpenByRequest sees the rival, so the idempotent path is skipped and the ask is refused outright.
	_, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{RequestID: r.ID, Source: domain.ClarificationSourceManual, Questions: oneQuestion("k"), ActorKind: domain.ActorKindUser})
	mustCode(t, err, "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED")
	if f.env.s.requests[r.ID].Status != domain.RequestStatusAnalyzing {
		t.Fatal("the request must not move")
	}
}

func TestRequestClarification_PayloadHasNoQuestionText(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAnalyzing, nil)
	_, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{
		RequestID: r.ID, Source: domain.ClarificationSourceManual, ActorKind: domain.ActorKindUser,
		Questions: []QuestionInput{{QuestionKey: "k", Kind: domain.QuestionKindText, Prompt: "MẬT-KHẨU-LÀ-GÌ-CỦA-KHÁCH", Reason: "LÝ-DO-RIÊNG-TƯ", Required: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	evs := f.eventsOf(domain.SubjectClarificationRequested)
	if len(evs) != 1 {
		t.Fatalf("%d events", len(evs))
	}
	if strings.Contains(string(evs[0].Payload), "MẬT-KHẨU") || strings.Contains(string(evs[0].Payload), "LÝ-DO") {
		t.Fatalf("payload leaks question text: %s", evs[0].Payload)
	}
	var n ClarificationNotice
	if err := json.Unmarshal(evs[0].Payload, &n); err != nil || len(n.UserIDs) != 1 || n.UserIDs[0] != userAda || n.RequestID != r.ID || !strings.HasPrefix(n.DisplayID, "REQ-") {
		t.Fatalf("%+v %v", n, err)
	}
}

func TestRequestClarification_ActorNotAllowedPerSource(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusExecuting, nil)
	in := func(src domain.ClarificationSource, kind domain.ActorKind) RequestClarificationInput {
		return RequestClarificationInput{RequestID: r.ID, Source: src, Questions: oneQuestion("k"), ActorKind: kind}
	}
	_, err := f.clarify.Execute(asUser(userAda), in(domain.ClarificationSourceTaskBlocked, domain.ActorKindUser))
	mustCode(t, err, "REQUEST_FORBIDDEN")
	_, err = f.clarify.Execute(asUser(userAda), in(domain.ClarificationSourceReadiness, domain.ActorKindUser))
	mustCode(t, err, "REQUEST_FORBIDDEN")
	_, err = f.clarify.Execute(asUser(userBob), in(domain.ClarificationSourceManual, domain.ActorKindUser))
	mustCode(t, err, "REQUEST_FORBIDDEN")
	machine := tenant.WithActorType(asUser(userAda), tenant.ActorAgent)
	_, err = f.clarify.Execute(machine, in(domain.ClarificationSourceManual, domain.ActorKindUser))
	mustCode(t, err, "REQUEST_FORBIDDEN")
	if _, err := f.clarify.Execute(asUser(userAda), in(domain.ClarificationSourceTaskBlocked, domain.ActorKindSystem)); err != nil {
		t.Fatalf("the platform may raise task_blocked: %v", err)
	}
}

func TestRequestClarification_ValidatesQuestions(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAnalyzing, nil)
	ask := func(qs []QuestionInput) error {
		_, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{RequestID: r.ID, Source: domain.ClarificationSourceManual, Questions: qs, ActorKind: domain.ActorKindUser})
		return err
	}
	mustCode(t, ask(nil), "REQUEST_CLARIFICATION_INVALID_ANSWER")
	dup := append(oneQuestion("a"), oneQuestion("a")...)
	mustCode(t, ask(dup), "REQUEST_CLARIFICATION_INVALID_ANSWER")
	noReason := []QuestionInput{{QuestionKey: "k", Kind: domain.QuestionKindText, Prompt: "p"}}
	mustCode(t, ask(noReason), "REQUEST_CLARIFICATION_INVALID_ANSWER")
	many := make([]QuestionInput, domain.MaxQuestions+1)
	for i := range many {
		many[i] = oneQuestion("k" + string(rune('a'+i)))[0]
	}
	mustCode(t, ask(many), "REQUEST_CLARIFICATION_INVALID_ANSWER")
	if len(f.env.clars) != 0 {
		t.Fatal("nothing may be stored for invalid questions")
	}
}

// ---- readiness at type confirmation

func confirmIn(r domain.Request) ConfirmInput {
	return ConfirmInput{RequestID: r.ID, Type: "bug", Size: "M", ActorID: userAda, ActorKind: domain.ActorKindUser}
}

func TestConfirmRequestType_BugMissingReproAndAC_GoesAwaitingInformation(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.Type, r.TypeSource = "", domain.TypeSourceAI })
	res, err := f.confirm.Execute(asUser(userAda), confirmIn(r))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.RequestStatusAwaitingInformation {
		t.Fatalf("status %s", res.Status)
	}
	var open []domain.Clarification
	for _, c := range f.env.clars {
		open = append(open, *c)
	}
	if len(open) != 1 || open[0].Source != domain.ClarificationSourceReadiness || open[0].Status != domain.ClarificationStatusOpen {
		t.Fatalf("clarifications %+v", open)
	}
	var keys []string
	for _, q := range open[0].Questions {
		keys = append(keys, q.QuestionKey)
	}
	want := []string{"type_fields.repro_steps", "type_fields.actual", "type_fields.expected", "type_fields.environment", "type_fields.severity", "acceptance_criteria"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("one question per missing key, in order: %v", keys)
	}
	subjects := f.env.s.subjects()
	if !containsString(subjects, domain.SubjectRequestTypeConfirmed) || !containsString(subjects, domain.SubjectClarificationRequested) {
		t.Fatalf("events %v", subjects)
	}
	if f.appr.approved != 1 {
		t.Fatalf("the request_type approval must still be approved, got %d", f.appr.approved)
	}
	if f.env.s.requests[r.ID].Type != domain.RequestTypeBug {
		t.Fatal("the confirmed type must be stored")
	}
	// A repeat confirm while parked is a no-op, not an error.
	if _, err := f.confirm.Execute(asUser(userAda), confirmIn(r)); err != nil {
		t.Fatalf("repeat confirm: %v", err)
	}
	if len(f.env.clars) != 1 {
		t.Fatal("a repeat confirm must not open another clarification")
	}
}

func mustContent(r domain.Request) domain.RequestContent {
	c, err := domain.ContentFromRequest(r)
	if err != nil {
		panic(err)
	}
	return c
}

func fillBug(f *clFx, r domain.Request) domain.Request {
	c := mustContent(r)
	c.TypeFields = map[string]any{"repro_steps": []any{"mở trang"}, "actual": "treo", "expected": "lưu được", "environment": "prod", "severity": "high"}
	_, _ = c.AcceptanceCriteria.Add("Lưu không treo", "test")
	next, _ := r.WithContent(c)
	next.ContentRevision = 1
	f.env.s.requests[r.ID] = next
	return next
}

func TestConfirmRequestType_ReadyRequest_GoesAnalyzing_AsBefore(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	fillBug(f, r)
	res, err := f.confirm.Execute(asUser(userAda), confirmIn(r))
	if err != nil || res.Status != domain.RequestStatusAnalyzing || len(f.env.clars) != 0 {
		t.Fatalf("%+v %v clarifications=%d", res.Status, err, len(f.env.clars))
	}
	if containsString(f.env.s.subjects(), domain.SubjectClarificationRequested) {
		t.Fatal("no clarification event for a ready request")
	}
}

func TestConfirmRequestType_NonBlockingMissing_StillReady(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.Type = domain.RequestTypeChangeRequest })
	c := mustContent(r)
	c.Type = domain.RequestTypeChangeRequest
	c.TypeFields = map[string]any{"goal": "SSO", "value": "ít ma sát", "scope_in": []any{"web"}} // no scope_out
	_, _ = c.AcceptanceCriteria.Add("Đăng nhập một lần", "test")
	next, _ := r.WithContent(c)
	f.env.s.requests[r.ID] = next
	in := confirmIn(r)
	in.Type = "change_request"
	res, err := f.confirm.Execute(asUser(userAda), in)
	if err != nil || res.Status != domain.RequestStatusAnalyzing {
		t.Fatalf("%s %v", res.Status, err)
	}
}

func TestConfirmRequestType_ExceedsMaxRounds_ReturnsToBacklogMissingInfo(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	f.env.clars["old"] = &domain.Clarification{ID: "old", RequestID: r.ID, Seq: 1, Source: domain.ClarificationSourceReadiness, Status: domain.ClarificationStatusAnswered, Round: 3}
	res, err := f.confirm.Execute(asUser(userAda), confirmIn(r))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.RequestStatusRequestBacklog || res.ReturnedCategory != domain.ReturnCategoryMissingInfo || res.ReturnedFromStage != domain.ReturnStageClassification ||
		!strings.Contains(res.ReturnReason, "/type_fields/repro_steps") {
		t.Fatalf("%+v", res)
	}
	if len(f.env.clars) != 1 {
		t.Fatal("no new clarification after the last round")
	}
}

func TestConfirmRequestType_WaivedReadinessMovesOn(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	if _, err := f.waive.Execute(asAdmin(), WaiveInput{RequestID: r.ID, Reason: "khách hàng lớn, thiếu dữ liệu chấp nhận được"}); err != nil {
		t.Fatal(err)
	}
	res, err := f.confirm.Execute(asUser(userAda), confirmIn(r))
	if err != nil || res.Status != domain.RequestStatusAnalyzing || len(f.env.clars) != 0 {
		t.Fatalf("%s %v", res.Status, err)
	}
}

func TestGetRequestReadiness_ReadOnly_NoWrites(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	events, revs := len(f.env.s.events), len(f.env.revisions[r.ID])
	v, err := NewGetRequestReadiness(f.env.s).Execute(lcCtx(), r.ID)
	if err != nil || v.Report.Ready || len(v.Report.Missing) == 0 || v.ContentRevision != 1 {
		t.Fatalf("%+v %v", v, err)
	}
	if len(f.env.s.events) != events || len(f.env.revisions[r.ID]) != revs || len(f.env.clars) != 0 || f.env.s.requests[r.ID].Version != r.Version {
		t.Fatal("GetRequestReadiness wrote something")
	}
	untyped := f.seedIn(domain.RequestStatusClassifying, func(r *domain.Request) { r.Type = "" })
	if _, err := NewGetRequestReadiness(f.env.s).Execute(lcCtx(), untyped.ID); err == nil {
		t.Fatal("readiness needs a type")
	}
}

// ---- waive

func TestWaiveReadiness_AdminOnly(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	_, err := f.waive.Execute(asUser(userAda), WaiveInput{RequestID: r.ID, Reason: "x"})
	mustCode(t, err, "REQUEST_READINESS_WAIVE_FORBIDDEN")
	machine := tenant.WithActorType(asAdmin(), tenant.ActorAgent)
	_, err = f.waive.Execute(machine, WaiveInput{RequestID: r.ID, Reason: "x"})
	mustCode(t, err, "REQUEST_READINESS_WAIVE_FORBIDDEN")
}

func TestWaiveReadiness_ForbiddenForHotfixSecurityOps(t *testing.T) {
	for _, typ := range []domain.RequestType{domain.RequestTypeHotfix, domain.RequestTypeSecurity, domain.RequestTypeOpsRequest} {
		f := newClFx()
		r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.Type = typ })
		_, err := f.waive.Execute(asAdmin(), WaiveInput{RequestID: r.ID, Reason: "gấp"})
		mustCode(t, err, "REQUEST_READINESS_WAIVE_FORBIDDEN")
	}
}

func TestWaiveReadiness_ReasonRequiredAndWrongState(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	_, err := f.waive.Execute(asAdmin(), WaiveInput{RequestID: r.ID, Reason: "   "})
	mustCode(t, err, "REQUEST_REASON_REQUIRED")
	run := f.seedIn(domain.RequestStatusAnalyzing, nil)
	_, err = f.waive.Execute(asAdmin(), WaiveInput{RequestID: run.ID, Reason: "x"})
	mustCode(t, err, "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED")
}

func TestWaiveReadiness_CancelsOpenClarification_AndResumes_WithRevision(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	if _, err := f.confirm.Execute(asUser(userAda), confirmIn(r)); err != nil {
		t.Fatal(err)
	}
	res, err := f.waive.Execute(asAdmin(), WaiveInput{RequestID: r.ID, Reason: "khách lớn"})
	if err != nil {
		t.Fatal(err)
	}
	got := f.env.s.requests[r.ID]
	if got.Status != domain.RequestStatusAnalyzing || res.Request.Status != domain.RequestStatusAnalyzing {
		t.Fatalf("status %s", got.Status)
	}
	for _, c := range f.env.clars {
		if c.Status != domain.ClarificationStatusCancelled || c.CancelReason != "waived" {
			t.Fatalf("%+v", c)
		}
	}
	revs := f.env.revisions[r.ID]
	last := revs[len(revs)-1]
	if last.Cause != domain.RevisionCauseEdited || !strings.Contains(string(last.Snapshot), `"waiver"`) || !strings.Contains(string(last.Snapshot), "khách lớn") || res.RequestRevision != 2 {
		t.Fatalf("waiver revision: %+v rev=%d", last, res.RequestRevision)
	}
	if ok, _ := ReadinessWaived(lcCtx(), revisionsView{f.env}, r.ID); !ok {
		t.Fatal("the waiver must be discoverable")
	}
	tr := f.spy.seen[len(f.spy.seen)-1]
	if tr.Trigger != domain.TriggerInformationProvided || tr.ResumeStatus != domain.RequestStatusAnalyzing {
		t.Fatalf("transition %+v", tr)
	}
}

// ---- golden notification payloads

func goldenPayload(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "notification", name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

func TestClarificationNotices_MatchNotificationServiceGolden(t *testing.T) {
	r := domain.Request{ID: "req-1", Number: 12}
	c := domain.Clarification{ID: "clr-1", Seq: 1, Source: "analyzing", DueAt: time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC), ResumeStatus: domain.RequestStatusAnalyzing, Round: 1}
	cases := map[string]ClarificationNotice{
		"clarification_requested.json":          clarificationRequestedNotice(r, c, []string{"u-reporter", "u-admin"}, false),
		"clarification_requested_reminder.json": clarificationRequestedNotice(r, c, []string{"u-reporter"}, true),
		"clarification_expired.json":            clarificationExpiredNotice(r, c, []string{"u-reporter"}),
	}
	for file, notice := range cases {
		want, got := goldenPayload(t, file), asMap(t, notice)
		for k, v := range want {
			if !reflect.DeepEqual(got[k], v) {
				t.Errorf("%s: %s = %v, golden has %v", file, k, got[k], v)
			}
		}
		for k := range got {
			if _, inGolden := want[k]; !inGolden && k != "clarification_display_id" && k != "resume_status" && k != "round" && k != "due_at" {
				t.Errorf("%s: unexpected extra member %s", file, k)
			}
		}
	}
}

func TestPrincipalRecipients(t *testing.T) {
	r := domain.Request{ReporterID: "rep"}
	p := PrincipalRecipients{Teams: teamStub{"t1": {"m1", "rep"}}, Admins: adminStub{"a1", "m1"}}
	got, err := p.Resolve(tctxT1(), r, []domain.Principal{
		{Kind: domain.PrincipalKindReporter}, {Kind: domain.PrincipalKindUser, ID: "u9"}, {Kind: domain.PrincipalKindTeam, ID: "t1"},
		{Kind: domain.PrincipalKindRole, ID: "admin"}, {Kind: domain.PrincipalKindRole, ID: "dev"},
	})
	if err != nil || !reflect.DeepEqual(got, []string{"rep", "u9", "m1", "a1"}) {
		t.Fatalf("%v %v", got, err)
	}
	none, _ := PrincipalRecipients{}.Resolve(tctxT1(), r, []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "t1"}})
	if len(none) != 0 {
		t.Fatalf("unwired directories resolve nobody: %v", none)
	}
}

func tctxT1() context.Context { return tenant.WithTenantID(context.Background(), "t1") }

type teamStub map[string][]string

func (t teamStub) MembersOfTeam(_ context.Context, id string) ([]string, error) { return t[id], nil }
func (t teamStub) TeamsForUser(context.Context, string) ([]string, error)       { return nil, nil }

type adminStub []string

func (a adminStub) ListAdmins(context.Context, string) ([]string, error) { return a, nil }
