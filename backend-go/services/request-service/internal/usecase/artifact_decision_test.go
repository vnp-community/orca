package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const solutionDoc = `{"schema_version":1,
 "options":[
  {"id":"opt-1","title":"Nâng cấp từng bước","summary":"s","approach":"a","effort":{"size":"M","hours_estimate":8},"risk":{"level":"low","description":"d"},"affected_areas":[{"name":"gateway","kind":"service"}]},
  {"id":"opt-2","title":"Viết lại toàn bộ","summary":"s","approach":"a","breaking_change":true,"effort":{"size":"L","hours_estimate":80},"risk":{"level":"high","description":"d"},"affected_areas":[]},
  {"id":"opt-3","title":"Tách ba dịch vụ","summary":"s","approach":"a","effort":{"size":"L","hours_estimate":40},"risk":{"level":"medium","description":"d"},
   "affected_areas":[{"name":"a","kind":"service"},{"name":"b","kind":"service"},{"name":"c","kind":"service"}]}],
 "recommendation":{"option_id":"opt-1","reason":"ít rủi ro"},
 "open_questions":[{"id":"Q-1","text":"Có MFA?","blocking":true},{"id":"Q-2","text":"Có SLA?","blocking":false}]}`

type decFx struct {
	*artFx
	rec     *RecordDecision
	confirm *ConfirmDecision
	gates   *ApprovalGates
	req     domain.Request
}

func newDecFx() *decFx {
	f := &decFx{artFx: newArtFx()}
	f.req = f.seed(func(r *domain.Request) { r.Status = domain.RequestStatusAwaitingAnalysisApproval })
	f.rec = NewRecordDecision(decView{f.env}, f.env.s).WithClock(func() time.Time { return fixedNow })
	f.confirm = NewConfirmDecision(f.env.s, decView{f.env}, f.env, f.env.s)
	f.gates = NewApprovalGates(decView{f.env}, clarView{f.env})
	return f
}

func (f *decFx) record(ctx context.Context, option, rationale, digest string) (domain.Decision, error) {
	var out domain.Decision
	err := f.env.InTx(ctx, func(c context.Context) error {
		d, err := f.rec.Execute(c, RecordDecisionInput{
			RequestID: f.req.ID, RequestNumber: f.req.Number, SolutionID: "sol-1", OptionsJSON: []byte(solutionDoc), ChosenOption: option,
			ChooserID: userBob, Rationale: rationale, SubjectDigest: digest, ReporterID: f.req.ReporterID, SelfChoiceAllowed: false,
		})
		out = d
		return err
	})
	return out, err
}

func TestRecordDecision_FirstChoose_NormalRisk_Effective(t *testing.T) {
	f := newDecFx()
	d, err := f.record(asUser(userBob), "opt-1", "", "dig1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != domain.DecisionStatusEffective || d.RiskLevel != domain.RiskNormal || d.RecommendedOptionID != "opt-1" || d.ChooserID != userBob || d.SubjectDigest != "dig1" || d.Seq != 1 {
		t.Fatalf("%+v", d)
	}
	if len(d.Options) != 3 || d.Options[1].Risk.Level != "high" || d.Options[1].Risk.Reasons[0] != "breaking_change" {
		t.Fatalf("options %+v", d.Options)
	}
	if len(f.env.decHistory) != 1 || f.env.decHistory[0].Action != domain.DecisionActionChosen {
		t.Fatalf("%+v", f.env.decHistory)
	}
	if !strings.HasPrefix(d.Question, "Chọn phương án cho REQ-") {
		t.Fatalf("question %q", d.Question)
	}
}

func TestRecordDecision_BreakingChange_StaysChosen(t *testing.T) {
	f := newDecFx()
	d, err := f.record(asUser(userBob), "opt-2", "kiến trúc cũ đã hết đường", "dig")
	if err != nil || d.Status != domain.DecisionStatusChosen || d.RiskLevel != domain.RiskHigh {
		t.Fatalf("%+v %v", d, err)
	}
	d3, err := newDecFx().record(asUser(userBob), "opt-3", "ba dịch vụ tách được", "dig")
	if err != nil || d3.RiskLevel != domain.RiskHigh {
		t.Fatalf("three services is high risk: %+v %v", d3, err)
	}
}

func TestRecordDecision_NotRecommendedWithoutRationale_Rejected(t *testing.T) {
	f := newDecFx()
	_, err := f.record(asUser(userBob), "opt-3", "  ", "dig")
	mustCode(t, err, "REQUEST_DECISION_RATIONALE_REQUIRED")
	if len(f.env.decisions) != 0 || len(f.env.decHistory) != 0 || len(f.eventsOf(domain.SubjectDecisionRecorded)) != 0 {
		t.Fatal("a refused choice must leave no decision, history or event behind")
	}
}

func TestRecordDecision_RechooseWritesHistoryAndClearsConfirmation(t *testing.T) {
	f := newDecFx()
	if _, err := f.record(asUser(userBob), "opt-2", "lý do", "d1"); err != nil {
		t.Fatal(err)
	}
	d := f.onlyDecision(t)
	if _, err := f.confirm.Execute(asUser(userBob), ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "viết lại toàn bộ"}); err != nil {
		t.Fatal(err)
	}
	again, err := f.record(asUser(userBob), "opt-1", "", "d2")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != d.ID || again.ConfirmedBy != "" || again.ConfirmedAt != nil || again.Status != domain.DecisionStatusEffective || again.SubjectDigest != "d2" {
		t.Fatalf("%+v", again)
	}
	var actions []domain.DecisionAction
	for _, h := range f.env.decHistory {
		actions = append(actions, h.Action)
	}
	if len(actions) != 3 || actions[0] != domain.DecisionActionChosen || actions[1] != domain.DecisionActionConfirmed || actions[2] != domain.DecisionActionRechosen {
		t.Fatalf("history %v", actions)
	}
	if len(f.env.decisions) != 1 {
		t.Fatal("choosing again reuses the live decision")
	}
}

func (f *decFx) onlyDecision(t *testing.T) domain.Decision {
	t.Helper()
	if len(f.env.decisions) != 1 {
		t.Fatalf("%d decisions", len(f.env.decisions))
	}
	for _, d := range f.env.decisions {
		return *d
	}
	panic("unreachable")
}

func TestRecordDecision_PayloadHasNoRationale(t *testing.T) {
	f := newDecFx()
	if _, err := f.record(asUser(userBob), "opt-3", "LÝ-DO-NỘI-BỘ-NHẠY-CẢM", "d"); err != nil {
		t.Fatal(err)
	}
	evs := f.eventsOf(domain.SubjectDecisionRecorded)
	if len(evs) != 1 || strings.Contains(string(evs[0].Payload), "NHẠY-CẢM") {
		t.Fatalf("%v", evs)
	}
	var p DecisionPayload
	if err := json.Unmarshal(evs[0].Payload, &p); err != nil || p.SubjectID != "sol-1" || p.RiskLevel != "high" || p.Status != "chosen" {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestRecordDecision_SelfChoiceForbidden_EvenForAdmin(t *testing.T) {
	f := newDecFx()
	in := RecordDecisionInput{RequestID: f.req.ID, RequestNumber: 1, SolutionID: "sol-1", OptionsJSON: []byte(solutionDoc), ChosenOption: "opt-1",
		ChooserID: f.req.ReporterID, ReporterID: f.req.ReporterID, SelfChoiceAllowed: false}
	_, err := f.rec.Execute(asAdmin(), in)
	mustCode(t, err, "REQUEST_DECISION_SELF_CHOICE_FORBIDDEN")
	in.SelfChoiceAllowed = true
	if _, err := f.rec.Execute(asAdmin(), in); err != nil {
		t.Fatalf("allowed when the approval setting says so: %v", err)
	}
}

func TestRecordDecision_UnknownOptionAndBadDocument(t *testing.T) {
	f := newDecFx()
	_, err := f.record(asUser(userBob), "opt-9", "x", "d")
	mustCode(t, err, "REQUEST_CLARIFICATION_INVALID_ANSWER")
	_, err = f.rec.Execute(asUser(userBob), RecordDecisionInput{RequestID: f.req.ID, SolutionID: "s", OptionsJSON: []byte(`[`), ChosenOption: "opt-1"})
	if err == nil {
		t.Fatal("bad options JSON must fail")
	}
}

func TestConfirmDecision_ExactTitleVietnameseCaseInsensitive(t *testing.T) {
	f := newDecFx()
	if _, err := f.record(asUser(userBob), "opt-2", "lý do", "d"); err != nil {
		t.Fatal(err)
	}
	d := f.onlyDecision(t)
	_, err := f.confirm.Execute(asUser(userBob), ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "viet lai toan bo"})
	mustCode(t, err, "REQUEST_DECISION_CONFIRMATION_MISMATCH")
	got, err := f.confirm.Execute(asUser(userBob), ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "  VIẾT LẠI   TOÀN BỘ "})
	if err != nil || got.Status != domain.DecisionStatusEffective || got.ConfirmedBy != userBob || got.ConfirmedAt == nil {
		t.Fatalf("%+v %v", got, err)
	}
	if n := len(f.eventsOf(domain.SubjectDecisionConfirmed)); n != 1 {
		t.Fatalf("%d confirmed events", n)
	}
}

func TestConfirmDecision_MachineForbidden_OnlyChooserOrAdmin(t *testing.T) {
	f := newDecFx()
	_, _ = f.record(asUser(userBob), "opt-2", "lý do", "d")
	d := f.onlyDecision(t)
	in := ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "Viết lại toàn bộ"}
	_, err := f.confirm.Execute(tenant.WithActorType(asUser(userBob), tenant.ActorAgent), in)
	mustCode(t, err, "REQUEST_DECISION_AGENT_FORBIDDEN")
	_, err = f.confirm.Execute(asUser(userAda), in)
	mustCode(t, err, "REQUEST_FORBIDDEN")
	if _, err := f.confirm.Execute(asAdmin(), in); err != nil {
		t.Fatalf("an admin may confirm: %v", err)
	}
}

func TestConfirmDecision_IdempotentWhenAlreadyEffective_StaleVersion(t *testing.T) {
	f := newDecFx()
	_, _ = f.record(asUser(userBob), "opt-2", "lý do", "d")
	d := f.onlyDecision(t)
	in := ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "Viết lại toàn bộ"}
	first, err := f.confirm.Execute(asUser(userBob), in)
	if err != nil {
		t.Fatal(err)
	}
	events := len(f.env.s.events)
	again, err := f.confirm.Execute(asUser(userBob), in)
	if err != nil || again.Version != first.Version || len(f.env.s.events) != events {
		t.Fatalf("a repeat by the same person is a no-op: %+v %v", again, err)
	}
	g := newDecFx()
	_, _ = g.record(asUser(userBob), "opt-2", "lý do", "d")
	d2 := g.onlyDecision(t)
	_, err = g.confirm.Execute(asUser(userBob), ConfirmDecisionInput{DecisionID: d2.ID, ConfirmationText: "Viết lại toàn bộ", ExpectedVersion: d2.Version + 4})
	mustCode(t, err, "REQUEST_DECISION_VERSION_CONFLICT")
	_, err = g.confirm.Execute(asUser(userBob), ConfirmDecisionInput{DecisionID: "missing", ConfirmationText: "x"})
	mustCode(t, err, "REQUEST_DECISION_NOT_FOUND")
}

func TestConfirmDecision_NormalRiskNeedsNoConfirmation(t *testing.T) {
	f := newDecFx()
	_, _ = f.record(asUser(userBob), "opt-1", "", "d")
	d := f.onlyDecision(t)
	_, err := f.confirm.Execute(asUser(userAda), ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "Nâng cấp từng bước"})
	mustCode(t, err, "REQUEST_FORBIDDEN")
	_, err = f.confirm.Execute(asUser(userBob), ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "Nâng cấp từng bước"})
	if err != nil {
		t.Fatalf("already effective, same chooser: %v", err)
	}
	if err := d.Confirm("x", "y", fixedNow); err == nil {
		t.Fatal("effective decisions cannot be confirmed again by Decision.Confirm")
	}
}

func TestApprovalGates_DecisionMustBeEffectiveAndMatchDigest(t *testing.T) {
	f := newDecFx()
	err := f.gates.CheckDecision(lcCtx(), "sol-1", "d")
	mustCode(t, err, "REQUEST_DECISION_NOT_EFFECTIVE")
	_, _ = f.record(asUser(userBob), "opt-2", "lý do", "d1")
	err = f.gates.CheckDecision(lcCtx(), "sol-1", "d1")
	mustCode(t, err, "REQUEST_DECISION_NOT_EFFECTIVE")
	if !strings.Contains(err.Error(), "confirm") {
		t.Fatalf("the message should say what to do: %v", err)
	}
	d := f.onlyDecision(t)
	_, _ = f.confirm.Execute(asUser(userBob), ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "Viết lại toàn bộ"})
	if err := f.gates.CheckDecision(lcCtx(), "sol-1", "d1"); err != nil {
		t.Fatalf("effective and matching: %v", err)
	}
	mustCode(t, f.gates.CheckDecision(lcCtx(), "sol-1", "other-digest"), "REQUEST_DECISION_NOT_EFFECTIVE")
	// choosing again changes the digest and drops the confirmation
	_, _ = f.record(asUser(userBob), "opt-3", "ba dịch vụ", "d2")
	mustCode(t, f.gates.CheckDecision(lcCtx(), "sol-1", "d2"), "REQUEST_DECISION_NOT_EFFECTIVE")
}

func answeredClarification(f *decFx, source domain.ClarificationSource, ref string, keys ...string) {
	c := &domain.Clarification{ID: newID(), RequestID: f.req.ID, Seq: len(f.env.clars) + 1, Source: source, SourceRef: ref, Status: domain.ClarificationStatusAnswered}
	for i, k := range keys {
		c.Questions = append(c.Questions, domain.ClarificationQuestion{ID: newID(), Seq: i + 1, QuestionKey: k, Answer: json.RawMessage(`"đã rõ"`)})
	}
	f.env.clars[c.ID] = c
}

func TestApprovalGates_SolutionBlockingQuestions(t *testing.T) {
	f := newDecFx()
	_, _ = f.record(asUser(userBob), "opt-1", "", "d")
	err := f.gates.CheckSolution(lcCtx(), f.req.ID, "sol-1", []byte(solutionDoc), "d")
	mustCode(t, err, "REQUEST_SOLUTION_BLOCKING_QUESTIONS")
	if !strings.Contains(err.Error(), "Q-1") || strings.Contains(err.Error(), "Q-2") {
		t.Fatalf("only the blocking question is named: %v", err)
	}
	answeredClarification(f, domain.ClarificationSourceSolutionOpenQuestion, "other-solution", "open_question:Q-1")
	mustCode(t, f.gates.CheckSolution(lcCtx(), f.req.ID, "sol-1", []byte(solutionDoc), "d"), "REQUEST_SOLUTION_BLOCKING_QUESTIONS")
	answeredClarification(f, domain.ClarificationSourceSolutionOpenQuestion, "sol-1", "open_question:Q-1")
	if err := f.gates.CheckSolution(lcCtx(), f.req.ID, "sol-1", []byte(solutionDoc), "d"); err != nil {
		t.Fatalf("answered through a clarification: %v", err)
	}
	// legacy documents (string questions) never block
	if err := f.gates.CheckSolution(lcCtx(), f.req.ID, "sol-1", []byte(`{"open_questions":["a string"]}`), "d"); err != nil {
		t.Fatalf("legacy: %v", err)
	}
}

func TestApprovalGates_PlanUnconfirmedAssumptions(t *testing.T) {
	f := newDecFx()
	plan := []byte(`{"assumptions":[{"id":"A-1","text":"IdP có sẵn","needs_confirmation":true},{"id":"A-2","text":"x","needs_confirmation":false}]}`)
	err := f.gates.CheckPlan(lcCtx(), f.req.ID, "plan-1", plan)
	mustCode(t, err, "REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS")
	if !strings.Contains(err.Error(), "A-1") || strings.Contains(err.Error(), "A-2") {
		t.Fatalf("%v", err)
	}
	answeredClarification(f, domain.ClarificationSourcePlanAssumption, "plan-1", "assumption:A-1")
	if err := f.gates.CheckPlan(lcCtx(), f.req.ID, "plan-1", plan); err != nil {
		t.Fatalf("%v", err)
	}
	if err := f.gates.CheckPlan(lcCtx(), f.req.ID, "plan-1", []byte(`{"goal":"g"}`)); err != nil {
		t.Fatalf("no assumptions, nothing to confirm: %v", err)
	}
}

func TestDecisionQueries(t *testing.T) {
	f := newDecFx()
	_, _ = f.record(asUser(userBob), "opt-1", "", "d")
	q := NewDecisionQueries(f.env.s, decView{f.env})
	page, err := q.List(lcCtx(), f.req.ID, "", 0, 10)
	if err != nil || len(page.Items) != 1 || page.RequestNumber != f.req.Number {
		t.Fatalf("%+v %v", page, err)
	}
	got, number, err := q.Get(lcCtx(), page.Items[0].ID)
	if err != nil || got.ID != page.Items[0].ID || number != f.req.Number {
		t.Fatalf("%+v %v", got, err)
	}
	if only, _ := q.List(lcCtx(), f.req.ID, domain.DecisionStatusSuperseded, 0, 10); len(only.Items) != 0 {
		t.Fatal("status filter")
	}
	if _, _, err := q.Get(context.Background(), page.Items[0].ID); err == nil {
		t.Fatal("tenant required")
	}
}
