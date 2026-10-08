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

// parked confirms the type of an empty bug request, which opens a readiness clarification.
func (f *clFx) parked(t *testing.T) (domain.Request, domain.Clarification) {
	t.Helper()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, nil)
	if _, err := f.confirm.Execute(asUser(userAda), confirmIn(r)); err != nil {
		t.Fatal(err)
	}
	return r, f.onlyClarification(t)
}

func (f *clFx) onlyClarification(t *testing.T) domain.Clarification {
	t.Helper()
	var open *domain.Clarification
	for _, c := range f.env.clars {
		if c.Status == domain.ClarificationStatusOpen {
			cp := cloneClar(*c)
			open = &cp
		}
	}
	if open == nil {
		t.Fatal("no open clarification")
	}
	return *open
}

func questionByKey(c domain.Clarification, key string) domain.ClarificationQuestion {
	for _, q := range c.Questions {
		if q.QuestionKey == key {
			return q
		}
	}
	panic("no question " + key)
}

func answerAll(c domain.Clarification) []AnswerItem {
	values := map[string]string{
		"type_fields.repro_steps": `"mở trang\nbấm lưu"`, "type_fields.actual": `"treo"`, "type_fields.expected": `"lưu được"`,
		"type_fields.environment": `"prod"`, "type_fields.severity": `"high"`, "acceptance_criteria": `"Lưu không treo"`, "body": `"Trang treo mỗi khi bấm lưu bản ghi mới."`,
	}
	var out []AnswerItem
	for _, q := range c.Questions {
		out = append(out, AnswerItem{QuestionID: q.ID, Value: json.RawMessage(values[q.QuestionKey])})
	}
	return out
}

func (f *clFx) answerBy(user string, c domain.Clarification, items []AnswerItem, complete bool) (AnswerResult, error) {
	return f.answer.Execute(asUser(user), AnswerInput{ClarificationID: c.ID, Answers: items, Complete: complete})
}

func TestAnswer_DraftOnlySavesAnswers_NoRequestChange(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	before := f.env.s.requests[r.ID]
	events := len(f.env.s.events)
	q := questionByKey(c, "type_fields.actual")
	res, err := f.answerBy(userAda, c, []AnswerItem{{QuestionID: q.ID, Value: json.RawMessage(`"treo"`)}}, false)
	if err != nil || !res.StillMissing || res.Clarification.Status != domain.ClarificationStatusOpen {
		t.Fatalf("%+v %v", res, err)
	}
	after := f.env.s.requests[r.ID]
	if after.Version != before.Version || after.ContentRevision != 1 || after.Status != domain.RequestStatusAwaitingInformation || len(f.env.s.events) != events {
		t.Fatal("a draft must not touch the request or emit events")
	}
	if got := questionByKey(res.Clarification, "type_fields.actual"); string(got.Answer) != `"treo"` || got.AnswerSource != domain.AnswerSourceUser {
		t.Fatalf("draft not stored: %+v", got)
	}
}

func TestAnswer_CompleteMissingRequired_Incomplete(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	items := answerAll(c)
	_, err := f.answerBy(userAda, c, items[:2], true)
	mustCode(t, err, "REQUEST_CLARIFICATION_INCOMPLETE")
	if !strings.Contains(err.Error(), "type_fields.expected") {
		t.Fatalf("the error should list what is missing: %v", err)
	}
	if f.env.s.requests[c.RequestID].ContentRevision != 1 {
		t.Fatal("an incomplete answer wrote a revision")
	}
}

func TestAnswer_InvalidKindValues_InvalidAnswer(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	sev := questionByKey(c, "type_fields.severity")
	_, err := f.answerBy(userAda, c, []AnswerItem{{QuestionID: sev.ID, Value: json.RawMessage(`"meh"`)}}, false)
	mustCode(t, err, "REQUEST_CLARIFICATION_INVALID_ANSWER")
	_, err = f.answerBy(userAda, c, []AnswerItem{{QuestionID: "nope", Value: json.RawMessage(`"x"`)}}, false)
	mustCode(t, err, "REQUEST_CLARIFICATION_INVALID_ANSWER")
	_, err = f.answerBy(userAda, c, []AnswerItem{{QuestionID: sev.ID, Value: json.RawMessage(`"low"`)}, {QuestionID: sev.ID, Value: json.RawMessage(`"high"`)}}, false)
	mustCode(t, err, "REQUEST_CLARIFICATION_INVALID_ANSWER")
}

func TestAnswer_AcceptDefault_SetsAnswerSource(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	ac := questionByKey(c, "acceptance_criteria")
	if len(ac.SuggestedDefault) == 0 {
		t.Fatal("the AC question should carry a suggested default")
	}
	res, err := f.answerBy(userAda, c, []AnswerItem{{QuestionID: ac.ID, AcceptDefault: true}}, false)
	if err != nil {
		t.Fatal(err)
	}
	got := questionByKey(res.Clarification, "acceptance_criteria")
	if got.AnswerSource != domain.AnswerSourceDefaultAccepted || string(got.Answer) != string(ac.SuggestedDefault) {
		t.Fatalf("%+v", got)
	}
	repro := questionByKey(c, "type_fields.repro_steps")
	_, err = f.answerBy(userAda, c, []AnswerItem{{QuestionID: repro.ID, AcceptDefault: true}}, false)
	mustCode(t, err, "REQUEST_CLARIFICATION_INVALID_ANSWER")
}

func TestAnswer_Complete_WritesRevisionAnsweredAndTransitions(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	res, err := f.answerBy(userAda, c, answerAll(c), true)
	if err != nil {
		t.Fatal(err)
	}
	if res.RequestStatus != domain.RequestStatusAnalyzing || res.RequestRevision != 2 || res.StillMissing || res.Clarification.Status != domain.ClarificationStatusAnswered {
		t.Fatalf("%+v", res)
	}
	got := f.env.s.requests[r.ID]
	if got.Status != domain.RequestStatusAnalyzing || got.ContentRevision != 2 {
		t.Fatalf("request %+v", got)
	}
	revs := f.env.revisions[r.ID]
	if len(revs) != 2 || revs[1].Cause != domain.RevisionCauseClarificationAnswered || revs[1].ClarificationID != c.ID || revs[1].ActorID != userAda {
		t.Fatalf("revisions %+v", revs)
	}
	content := mustContent(got)
	if content.TypeFields["severity"] != "high" || len(content.AcceptanceCriteria.ActiveIDs()) != 1 || report(content).Ready == false {
		t.Fatalf("content after answer: %+v", content)
	}
	if res.Clarification.AnsweredRequestRevision == nil || *res.Clarification.AnsweredRequestRevision != 2 {
		t.Fatal("answered_request_revision")
	}
	seen := f.spy.seen[len(f.spy.seen)-1]
	if seen.Trigger != domain.TriggerInformationProvided || seen.ResumeStatus != domain.RequestStatusAnalyzing || *seen.ExpectedFrom != domain.RequestStatusAwaitingInformation {
		t.Fatalf("transition %+v", seen)
	}
	answered := f.eventsOf(domain.SubjectClarificationAnswered)
	if len(answered) != 1 || strings.Contains(string(answered[0].Payload), "mở trang") || strings.Contains(string(answered[0].Payload), "treo") {
		t.Fatalf("answered event: %v", answered)
	}
	if len(f.superseded) != 1 || f.superseded[0] != r.ID {
		t.Fatalf("proposed solutions must be superseded once: %v", f.superseded)
	}
	changed := f.eventsOf(domain.SubjectRequestStatusChanged)
	var p StatusChangedPayload
	_ = json.Unmarshal(changed[len(changed)-1].Payload, &p)
	if p.Trigger != "information_provided" || p.ResumeStatus != "analyzing" || p.To != "analyzing" {
		t.Fatalf("status_changed payload: %+v", p)
	}
}

func report(c domain.RequestContent) domain.ReadinessReport {
	return domain.ReadinessPolicy{}.Evaluate(domain.RequestTypeBug, c)
}

// shortBody parks a request that is ready except for a body too short to analyse.
func (f *clFx) parkedShortBody(t *testing.T, round int) (domain.Request, domain.Clarification) {
	t.Helper()
	r := f.seedIn(domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.Type = ""; r.Body = "ngắn" })
	c := fillBugKeepBody(f, r)
	if _, err := f.confirm.Execute(asUser(userAda), confirmIn(c)); err != nil {
		t.Fatal(err)
	}
	cl := f.onlyClarification(t)
	if round > 1 {
		f.env.clars[cl.ID].Round = round
		cl.Round = round
	}
	return c, cl
}

func fillBugKeepBody(f *clFx, r domain.Request) domain.Request {
	c := mustContent(r)
	c.Type = domain.RequestTypeBug
	c.TypeFields = map[string]any{"repro_steps": []any{"mở trang"}, "actual": "treo", "expected": "lưu được", "environment": "prod", "severity": "high"}
	_, _ = c.AcceptanceCriteria.Add("Lưu không treo", "test")
	next, _ := r.WithContent(c)
	f.env.s.requests[r.ID] = next
	return next
}

func TestAnswer_StillMissing_CreatesRound2_RequestStaysAwaitingInformation(t *testing.T) {
	f := newClFx()
	r, c := f.parkedShortBody(t, 1)
	body := questionByKey(c, "body")
	res, err := f.answerBy(userAda, c, []AnswerItem{{QuestionID: body.ID, Value: json.RawMessage(`"vẫn ngắn"`)}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.StillMissing || res.RequestStatus != domain.RequestStatusAwaitingInformation || f.env.s.requests[r.ID].Status != domain.RequestStatusAwaitingInformation {
		t.Fatalf("%+v", res)
	}
	var rounds []int
	for _, x := range f.env.clars {
		rounds = append(rounds, x.Round)
		if x.Round == 2 && (x.Status != domain.ClarificationStatusOpen || x.Source != domain.ClarificationSourceReadiness || x.Seq != 2 || x.ResumeStatus != c.ResumeStatus) {
			t.Fatalf("round 2: %+v", x)
		}
	}
	if len(rounds) != 2 {
		t.Fatalf("rounds %v", rounds)
	}
	if got := f.env.revisions[r.ID]; len(got) != 2 {
		t.Fatalf("the partial answer is still a revision: %d", len(got))
	}
	if n := len(f.eventsOf(domain.SubjectClarificationRequested)); n != 2 {
		t.Fatalf("a new request event for round 2, got %d", n)
	}
	if len(f.superseded) != 0 {
		t.Fatal("nothing is superseded while the request stays parked")
	}
}

func TestAnswer_AtMaxRounds_ReturnsToBacklogMissingInfo(t *testing.T) {
	f := newClFx()
	r, c := f.parkedShortBody(t, 3)
	body := questionByKey(c, "body")
	res, err := f.answerBy(userAda, c, []AnswerItem{{QuestionID: body.ID, Value: json.RawMessage(`"vẫn ngắn"`)}}, true)
	if err != nil {
		t.Fatal(err)
	}
	got := f.env.s.requests[r.ID]
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryMissingInfo || got.ReturnedFromStage != domain.ReturnStageClassification ||
		!strings.Contains(got.ReturnReason, "/body") || res.RequestStatus != domain.RequestStatusRequestBacklog {
		t.Fatalf("%+v", got)
	}
	if len(f.env.clars) != 1 {
		t.Fatal("no fourth round")
	}
}

func TestAnswer_SupersedesDecisions_WhenResumingAnalyzing(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	f.env.decisions["d1"] = &domain.Decision{ID: "d1", RequestID: r.ID, SubjectKind: domain.DecisionSubjectSolutionOption, SubjectID: "sol-1", Status: domain.DecisionStatusEffective, ChosenOptionID: "opt-1", Version: 1}
	if _, err := f.answerBy(userAda, c, answerAll(c), true); err != nil {
		t.Fatal(err)
	}
	if f.env.decisions["d1"].Status != domain.DecisionStatusSuperseded {
		t.Fatal("the decision of the old facts must be superseded")
	}
	if len(f.env.decHistory) != 1 || f.env.decHistory[0].Action != domain.DecisionActionSuperseded || f.env.decHistory[0].DecisionID != "d1" {
		t.Fatalf("history %+v", f.env.decHistory)
	}
}

func TestAnswer_ExecutingResume_SupersedesNothing(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusExecuting, nil)
	c, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{RequestID: r.ID, Source: domain.ClarificationSourceTaskBlocked, SourceRef: "task-1", Questions: oneQuestion("type_fields.actual"), ActorKind: domain.ActorKindSystem})
	if err != nil {
		t.Fatal(err)
	}
	f.env.decisions["d1"] = &domain.Decision{ID: "d1", RequestID: r.ID, Status: domain.DecisionStatusEffective}
	items := []AnswerItem{{QuestionID: c.Questions[0].ID, Value: json.RawMessage(`"đã rõ"`)}}
	res, err := f.answerBy(userAda, c, items, true)
	if err != nil || res.RequestStatus != domain.RequestStatusExecuting {
		t.Fatalf("%+v %v", res, err)
	}
	if len(f.superseded) != 0 || f.env.decisions["d1"].Status != domain.DecisionStatusEffective {
		t.Fatal("executing work is not invalidated by an answer")
	}
}

func TestAnswer_RedeliveryIdempotent_SameBodyReturnsExisting(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	items := answerAll(c)
	first, err := f.answerBy(userAda, c, items, true)
	if err != nil {
		t.Fatal(err)
	}
	events, revs := len(f.env.s.events), len(f.env.revisions[r.ID])
	again, err := f.answerBy(userAda, c, items, true)
	if err != nil || again.RequestRevision != first.RequestRevision || again.Clarification.Status != domain.ClarificationStatusAnswered {
		t.Fatalf("%+v %v", again, err)
	}
	if len(f.env.s.events) != events || len(f.env.revisions[r.ID]) != revs || len(f.superseded) != 1 {
		t.Fatal("a repeat created another revision, event or supersede")
	}
	changed := append([]AnswerItem(nil), items...)
	changed[0].Value = json.RawMessage(`"khác hẳn"`)
	_, err = f.answerBy(userAda, c, changed, true)
	mustCode(t, err, "REQUEST_CLARIFICATION_ALREADY_ANSWERED")
}

func TestAnswer_AfterDueAt_Expired(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	f.now = c.DueAt
	_, err := f.answerBy(userAda, c, answerAll(c), true)
	mustCode(t, err, "REQUEST_CLARIFICATION_EXPIRED")
	f.now = c.DueAt.Add(-time.Second)
	if _, err := f.answerBy(userAda, c, answerAll(c), true); err != nil {
		t.Fatalf("one second before the deadline still works: %v", err)
	}
}

func TestAnswer_WhoMayAnswer(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	_, err := f.answerBy(userBob, c, answerAll(c), true)
	mustCode(t, err, "REQUEST_CLARIFICATION_NOT_ASSIGNEE")
	machine := tenant.WithActorType(asUser(userAda), tenant.ActorAgent)
	_, err = f.answer.Execute(machine, AnswerInput{ClarificationID: c.ID, Answers: answerAll(c), Complete: true})
	mustCode(t, err, "REQUEST_CLARIFICATION_NOT_ASSIGNEE")
	if _, err := f.answer.Execute(asAdmin(), AnswerInput{ClarificationID: c.ID, Answers: answerAll(c), Complete: true}); err != nil {
		t.Fatalf("an admin may answer: %v", err)
	}
	f2 := newClFx()
	_, c2 := f2.parked(t)
	f2.env.clars[c2.ID].Assignees = []domain.Principal{{Kind: domain.PrincipalKindUser, ID: userBob}}
	if _, err := f2.answerBy(userBob, c2, answerAll(c2), true); err != nil {
		t.Fatalf("a named user may answer: %v", err)
	}
	f3 := newClFx()
	_, c3 := f3.parked(t)
	f3.env.clars[c3.ID].Assignees = []domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "team-1"}}
	f3.answer.WithTeams(teamStub{"team-1": {userBob}})
	if _, err := f3.answerBy(userBob, c3, answerAll(c3), true); err != nil {
		t.Fatalf("a team member may answer: %v", err)
	}
}

type failingTransitioner struct{}

func (failingTransitioner) Execute(context.Context, TransitionInput) (TransitionResult, error) {
	return TransitionResult{}, errBoom
}

func TestAnswer_TransactionRollsBackOnTransitionFailure(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	f.answer = NewAnswerClarification(f.env.s, clarView{f.env}, f.appendRev, failingTransitioner{}, f.ret, f.clarify, revisionsView{f.env}, decView{f.env}, f.env, f.env.s, 3).WithClock(func() time.Time { return f.now })
	if _, err := f.answerBy(userAda, c, answerAll(c), true); err == nil {
		t.Fatal("expected failure")
	}
	got := f.env.s.requests[r.ID]
	if got.ContentRevision != 1 || got.Status != domain.RequestStatusAwaitingInformation || len(f.env.revisions[r.ID]) != 1 || f.env.clars[c.ID].Status != domain.ClarificationStatusOpen {
		t.Fatal("a failed transition must undo the revision and the answered mark")
	}
	if q := f.env.clars[c.ID].Questions[0]; q.HasAnswer() {
		t.Fatal("stored answers must roll back too")
	}
	if len(f.eventsOf(domain.SubjectClarificationAnswered)) != 0 {
		t.Fatal("no answered event")
	}
}

func TestAnswer_StaleExpectedVersion(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	_, err := f.answer.Execute(asUser(userAda), AnswerInput{ClarificationID: c.ID, Answers: answerAll(c), Complete: true, ExpectedVersion: c.Version + 7})
	mustCode(t, err, "REQUEST_CLARIFICATION_VERSION_CONFLICT")
	if f.env.s.requests[c.RequestID].ContentRevision != 1 {
		t.Fatal("a stale answer wrote a revision")
	}
}

func TestAnswer_FileAnswerLimit(t *testing.T) {
	f := newClFx()
	r := f.seedIn(domain.RequestStatusAnalyzing, nil)
	c, err := f.clarify.Execute(asUser(userAda), RequestClarificationInput{RequestID: r.ID, Source: domain.ClarificationSourceManual, ActorKind: domain.ActorKindUser,
		Questions: []QuestionInput{{QuestionKey: "log", Kind: domain.QuestionKindFile, Prompt: "Đính kèm log", Reason: "Để tái hiện", Required: true}}})
	if err != nil {
		t.Fatal(err)
	}
	big, _ := json.Marshal(map[string]any{"filename": "a.log", "mime": "text/plain", "size": 70000, "text": strings.Repeat("x", domain.MaxFileAnswerBytes+1)})
	_, err = f.answerBy(userAda, c, []AnswerItem{{QuestionID: c.Questions[0].ID, Value: big}}, true)
	mustCode(t, err, "REQUEST_CLARIFICATION_INVALID_ANSWER")
}

// ---- cancel and the hooks on the other doors

func TestCancelClarification_ReturnsRequestToBacklog(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	got, err := f.cancelCl.Execute(asUser(userAda), CancelClarificationInput{ClarificationID: c.ID, Reason: "không còn cần"})
	if err != nil || got.Status != domain.ClarificationStatusCancelled || got.CancelReason != "không còn cần" {
		t.Fatalf("%+v %v", got, err)
	}
	req := f.env.s.requests[r.ID]
	if req.Status != domain.RequestStatusRequestBacklog || req.ReturnedCategory != domain.ReturnCategoryMissingInfo || req.ReturnedFromStage != domain.ReturnStageClassification {
		t.Fatalf("%+v", req)
	}
	if n := len(f.eventsOf(domain.SubjectClarificationCancelled)); n != 1 {
		t.Fatalf("one cancelled event, got %d", n)
	}
}

func TestCancelClarification_Guards(t *testing.T) {
	f := newClFx()
	_, c := f.parked(t)
	_, err := f.cancelCl.Execute(asUser(userAda), CancelClarificationInput{ClarificationID: c.ID, Reason: "  "})
	mustCode(t, err, "REQUEST_REASON_REQUIRED")
	_, err = f.cancelCl.Execute(asUser(userBob), CancelClarificationInput{ClarificationID: c.ID, Reason: "x"})
	mustCode(t, err, "REQUEST_FORBIDDEN")
	if _, err := f.cancelCl.Execute(asAdmin(), CancelClarificationInput{ClarificationID: c.ID, Reason: "x"}); err != nil {
		t.Fatal(err)
	}
	_, err = f.cancelCl.Execute(asAdmin(), CancelClarificationInput{ClarificationID: c.ID, Reason: "x"})
	mustCode(t, err, "REQUEST_CLARIFICATION_NOT_OPEN")
}

func TestChangeRequestType_CancelsOpenClarification(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	got, err := f.change.Execute(asUser(userAda), ChangeInput{RequestID: r.ID, NewType: "change_request", Reason: "thực ra là thay đổi lớn", ActorID: userAda, ActorKind: domain.ActorKindUser})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.RequestStatusAwaitingTypeConfirmation {
		t.Fatalf("status %s", got.Status)
	}
	if cl := f.env.clars[c.ID]; cl.Status != domain.ClarificationStatusCancelled || cl.CancelReason != "type_changed" {
		t.Fatalf("%+v", cl)
	}
}

func TestCancelRequest_CancelsOpenClarification(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	if _, err := f.cancelRq.Execute(asUser(userAda), CancelInput{RequestID: r.ID, Reason: "bỏ", ActorID: userAda}); err != nil {
		t.Fatal(err)
	}
	if cl := f.env.clars[c.ID]; cl.Status != domain.ClarificationStatusCancelled || cl.CancelReason != "request_cancelled" {
		t.Fatalf("%+v", cl)
	}
	if f.env.s.requests[r.ID].Status != domain.RequestStatusCancelled {
		t.Fatal("request not cancelled")
	}
}

func TestReturnToBacklog_FromAwaitingInformation_CancelsClarification(t *testing.T) {
	f := newClFx()
	r, c := f.parked(t)
	if _, err := f.ret.Execute(asUser(userAda), ReturnInput{RequestID: r.ID, Stage: domain.ReturnStageClassification, Category: domain.ReturnCategoryOther, Reason: "dừng", ActorID: userAda, ActorKind: domain.ActorKindUser}); err != nil {
		t.Fatal(err)
	}
	if cl := f.env.clars[c.ID]; cl.Status != domain.ClarificationStatusCancelled || cl.CancelReason != "returned_to_backlog" {
		t.Fatalf("%+v", cl)
	}
	if f.env.s.requests[r.ID].Status != domain.RequestStatusRequestBacklog {
		t.Fatal("not returned")
	}
}

func TestAwaitingInformation_AllExitsLeaveNoOpenClarification(t *testing.T) {
	// whatever door the request leaves through, the one-open rule must not keep a stale open clarification behind
	exits := map[string]func(f *clFx, r domain.Request) error{
		"type change": func(f *clFx, r domain.Request) error {
			_, err := f.change.Execute(asUser(userAda), ChangeInput{RequestID: r.ID, NewType: "change_request", Reason: "x", ActorKind: domain.ActorKindUser})
			return err
		},
		"cancel": func(f *clFx, r domain.Request) error {
			_, err := f.cancelRq.Execute(asUser(userAda), CancelInput{RequestID: r.ID, Reason: "x"})
			return err
		},
		"return": func(f *clFx, r domain.Request) error {
			_, err := f.ret.Execute(asUser(userAda), ReturnInput{RequestID: r.ID, Stage: domain.ReturnStageClassification, Category: domain.ReturnCategoryOther, Reason: "x", ActorKind: domain.ActorKindUser})
			return err
		},
	}
	for name, exit := range exits {
		f := newClFx()
		r, _ := f.parked(t)
		if err := exit(f, r); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, c := range f.env.clars {
			if c.Status == domain.ClarificationStatusOpen {
				t.Errorf("%s left an open clarification", name)
			}
		}
	}
}
