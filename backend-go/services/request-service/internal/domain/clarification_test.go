package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)

func TestResumeStatusFor_Table(t *testing.T) {
	withAnalysis := flowOf(t, RequestTypeBug)
	noAnalysis := flowOf(t, RequestTypeTask)
	cases := []struct {
		name   string
		flow   FlowDefinition
		source ClarificationSource
		from   RequestStatus
		want   RequestStatus
		bad    bool
	}{
		{"readiness analysis flow", withAnalysis, ClarificationSourceReadiness, RequestStatusAwaitingTypeConfirmation, RequestStatusAnalyzing, false},
		{"readiness no analysis", noAnalysis, ClarificationSourceReadiness, RequestStatusAwaitingTypeConfirmation, RequestStatusPlanning, false},
		{"readiness elsewhere", withAnalysis, ClarificationSourceReadiness, RequestStatusAnalyzing, "", true},
		{"open question analyzing", withAnalysis, ClarificationSourceSolutionOpenQuestion, RequestStatusAnalyzing, RequestStatusAnalyzing, false},
		{"open question awaiting approval", withAnalysis, ClarificationSourceSolutionOpenQuestion, RequestStatusAwaitingAnalysisApproval, RequestStatusAnalyzing, false},
		{"open question executing", withAnalysis, ClarificationSourceSolutionOpenQuestion, RequestStatusExecuting, "", true},
		{"assumption planning", withAnalysis, ClarificationSourcePlanAssumption, RequestStatusPlanning, RequestStatusPlanning, false},
		{"assumption awaiting plan approval", withAnalysis, ClarificationSourcePlanAssumption, RequestStatusAwaitingPlanApproval, RequestStatusPlanning, false},
		{"assumption analyzing", withAnalysis, ClarificationSourcePlanAssumption, RequestStatusAnalyzing, "", true},
		{"task blocked executing", withAnalysis, ClarificationSourceTaskBlocked, RequestStatusExecuting, RequestStatusExecuting, false},
		{"task blocked planning", withAnalysis, ClarificationSourceTaskBlocked, RequestStatusPlanning, "", true},
		{"manual analyzing", withAnalysis, ClarificationSourceManual, RequestStatusAnalyzing, RequestStatusAnalyzing, false},
		{"manual awaiting plan", withAnalysis, ClarificationSourceManual, RequestStatusAwaitingPlanApproval, RequestStatusPlanning, false},
		{"manual executing", withAnalysis, ClarificationSourceManual, RequestStatusExecuting, RequestStatusExecuting, false},
		{"manual awaiting type confirmation", withAnalysis, ClarificationSourceManual, RequestStatusAwaitingTypeConfirmation, "", true},
		{"manual completed", withAnalysis, ClarificationSourceManual, RequestStatusCompleted, "", true},
	}
	for _, c := range cases {
		got, err := ResumeStatusFor(c.flow, c.source, c.from)
		if c.bad {
			if errCode(err) != "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED" {
				t.Errorf("%s: want STATE_NOT_ALLOWED, got %q %v", c.name, got, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: got %q %v want %q", c.name, got, err, c.want)
		}
		if !IsResumeStatus(got) {
			t.Errorf("%s: %q is not a legal resume status", c.name, got)
		}
	}
}

func TestReturnStageForResume(t *testing.T) {
	cases := map[[2]string]ReturnStage{
		{"readiness", "analyzing"}:              ReturnStageClassification,
		{"readiness", "planning"}:               ReturnStageClassification,
		{"solution_open_question", "analyzing"}: ReturnStageAnalysis,
		{"plan_assumption", "planning"}:         ReturnStagePlan,
		{"task_blocked", "executing"}:           ReturnStageTask,
		{"manual", "executing"}:                 ReturnStageTask,
	}
	for k, want := range cases {
		if got := ReturnStageForResume(ClarificationSource(k[0]), RequestStatus(k[1])); got != want {
			t.Errorf("%v: got %s want %s", k, got, want)
		}
	}
}

func TestStageForStatus_AwaitingInformation(t *testing.T) {
	flow := flowOf(t, RequestTypeChangeRequest)
	got, err := StageForStatus(RequestStatusAwaitingInformation, flow, RequestSizeM)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []ReturnStage{ReturnStageClassification, ReturnStageAnalysis, ReturnStagePlan, ReturnStagePhase, ReturnStageTask} {
		if !ContainsStage(got, s) {
			t.Errorf("missing stage %s in %v", s, got)
		}
	}
	noPhase, _ := StageForStatus(RequestStatusAwaitingInformation, flowOf(t, RequestTypeTask), RequestSizeS)
	if ContainsStage(noPhase, ReturnStagePhase) {
		t.Error("a flow without phases must not offer the phase stage")
	}
}

func TestClarification_StateMachine(t *testing.T) {
	open := func() Clarification {
		return Clarification{ID: "c1", Status: ClarificationStatusOpen, DueAt: t0.Add(time.Hour)}
	}
	c := open()
	if err := c.Answer(t0, 3); err != nil || c.Status != ClarificationStatusAnswered || *c.AnsweredRequestRevision != 3 || c.AnsweredAt == nil {
		t.Fatalf("answer: %+v %v", c, err)
	}
	for name, fn := range map[string]func(*Clarification) error{
		"answer": func(c *Clarification) error { return c.Answer(t0, 1) },
		"expire": func(c *Clarification) error { return c.Expire(t0) },
		"cancel": func(c *Clarification) error { return c.Cancel("x", t0) },
	} {
		if err := fn(&c); errCode(err) != "REQUEST_CLARIFICATION_NOT_OPEN" {
			t.Errorf("%s on answered: %v", name, err)
		}
	}
	e := open()
	if err := e.Expire(t0); err != nil || e.Status != ClarificationStatusExpired {
		t.Fatalf("%+v %v", e, err)
	}
	x := open()
	if err := x.Cancel("type_changed", t0); err != nil || x.Status != ClarificationStatusCancelled || x.CancelReason != "type_changed" {
		t.Fatalf("%+v %v", x, err)
	}
	o := open()
	if o.IsExpired(t0) || !o.IsExpired(t0.Add(time.Hour)) || !o.IsExpired(t0.Add(2*time.Hour)) {
		t.Error("IsExpired must flip exactly at due_at")
	}
	if e.IsExpired(t0.Add(time.Hour)) {
		t.Error("only open clarifications can be expired")
	}
	if c.DisplayID(142) != "CLR-142.0" {
		t.Errorf("display id %s", c.DisplayID(142))
	}
}

func TestDefaultDue_Table(t *testing.T) {
	cases := []struct {
		source  ClarificationSource
		urgency Urgency
		want    time.Duration
	}{
		{ClarificationSourceReadiness, UrgencyNormal, 7 * 24 * time.Hour},
		{ClarificationSourceReadiness, UrgencyUrgent, 24 * time.Hour},
		{ClarificationSourceSolutionOpenQuestion, UrgencyNormal, 72 * time.Hour},
		{ClarificationSourceSolutionOpenQuestion, UrgencyUrgent, 8 * time.Hour},
		{ClarificationSourcePlanAssumption, UrgencyNormal, 72 * time.Hour},
		{ClarificationSourcePlanAssumption, UrgencyUrgent, 8 * time.Hour},
		{ClarificationSourceTaskBlocked, UrgencyNormal, 24 * time.Hour},
		{ClarificationSourceTaskBlocked, UrgencyUrgent, 24 * time.Hour},
		{ClarificationSourceManual, UrgencyNormal, 72 * time.Hour},
	}
	for _, c := range cases {
		if got := DefaultDue(c.source, c.urgency, t0).Sub(t0); got != c.want {
			t.Errorf("%s/%s: %v want %v", c.source, c.urgency, got, c.want)
		}
	}
}

func TestSystemOnlySources(t *testing.T) {
	for _, s := range AllClarificationSources() {
		want := s == ClarificationSourceReadiness || s == ClarificationSourceTaskBlocked
		if s.SystemOnly() != want {
			t.Errorf("%s SystemOnly=%v", s, s.SystemOnly())
		}
	}
}

func q(kind QuestionKind, opts ...string) ClarificationQuestion {
	c := ClarificationQuestion{QuestionKey: "k", Kind: kind, Prompt: "p", Reason: "r", Required: true}
	for _, o := range opts {
		c.Options = append(c.Options, QuestionOption{ID: o, Label: o})
	}
	return c
}

func TestValidateQuestion(t *testing.T) {
	if err := ValidateQuestion(q(QuestionKindText)); err != nil {
		t.Fatal(err)
	}
	bad := map[string]ClarificationQuestion{
		"no key":         func() ClarificationQuestion { c := q(QuestionKindText); c.QuestionKey = ""; return c }(),
		"no reason":      func() ClarificationQuestion { c := q(QuestionKindText); c.Reason = " "; return c }(),
		"long prompt":    func() ClarificationQuestion { c := q(QuestionKindText); c.Prompt = strings.Repeat("x", 1001); return c }(),
		"choice no opts": q(QuestionKindSingleChoice),
		"dup options":    q(QuestionKindMultiChoice, "a", "a"),
		"unknown kind":   q("telepathy"),
		"bad default": func() ClarificationQuestion {
			c := q(QuestionKindSingleChoice, "a")
			c.SuggestedDefault = json.RawMessage(`"z"`)
			return c
		}(),
	}
	for name, c := range bad {
		if err := ValidateQuestion(c); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidateAnswer_AllKinds(t *testing.T) {
	file := func(text string) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"filename": "a.log", "mime": "text/plain", "size": len(text), "text": text})
		return b
	}
	vietnamese4000 := json.RawMessage(`"` + strings.Repeat("ế", 4000) + `"`)
	vietnamese4001 := json.RawMessage(`"` + strings.Repeat("ế", 4001) + `"`)
	cases := []struct {
		name  string
		q     ClarificationQuestion
		value json.RawMessage
		ok    bool
	}{
		{"text ok", q(QuestionKindText), json.RawMessage(`"câu trả lời"`), true},
		{"text 4000 runes", q(QuestionKindText), vietnamese4000, true},
		{"text 4001 runes", q(QuestionKindText), vietnamese4001, false},
		{"text required blank", q(QuestionKindText), json.RawMessage(`"  "`), false},
		{"text not a string", q(QuestionKindText), json.RawMessage(`5`), false},
		{"single ok", q(QuestionKindSingleChoice, "a", "b"), json.RawMessage(`"b"`), true},
		{"single outside", q(QuestionKindSingleChoice, "a", "b"), json.RawMessage(`"c"`), false},
		{"single not string", q(QuestionKindSingleChoice, "a"), json.RawMessage(`["a"]`), false},
		{"multi ok", q(QuestionKindMultiChoice, "a", "b"), json.RawMessage(`["a","b"]`), true},
		{"multi outside", q(QuestionKindMultiChoice, "a", "b"), json.RawMessage(`["a","z"]`), false},
		{"multi required empty", q(QuestionKindMultiChoice, "a"), json.RawMessage(`[]`), false},
		{"bool ok", q(QuestionKindBoolean), json.RawMessage(`true`), true},
		{"bool string", q(QuestionKindBoolean), json.RawMessage(`"true"`), false},
		{"file ok", q(QuestionKindFile), file("log line"), true},
		{"file 64KB", q(QuestionKindFile), file(strings.Repeat("a", MaxFileAnswerBytes)), true},
		{"file over 64KB", q(QuestionKindFile), file(strings.Repeat("a", MaxFileAnswerBytes+1)), false},
		{"file no name", q(QuestionKindFile), json.RawMessage(`{"filename":"","text":"x"}`), false},
		{"file extra member", q(QuestionKindFile), json.RawMessage(`{"filename":"a","text":"x","path":"/etc/passwd"}`), false},
	}
	for _, c := range cases {
		err := ValidateAnswer(c.q, c.value)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if !c.ok && errCode(err) != "REQUEST_CLARIFICATION_INVALID_ANSWER" {
			t.Errorf("%s: want INVALID_ANSWER, got %v", c.name, err)
		}
	}
}

func answered(key, target string, kind QuestionKind, value string) ClarificationQuestion {
	return ClarificationQuestion{QuestionKey: key, TargetPath: target, Kind: kind, Answer: json.RawMessage(value)}
}

func TestApplyAnswers_TargetPaths(t *testing.T) {
	c := RequestContent{Title: "t", Body: "đã có", Type: RequestTypeBug, TypeFields: map[string]any{}}
	got, err := ApplyAnswers(c, []ClarificationQuestion{
		answered("repro", "type_fields.repro_steps", QuestionKindText, `"mở trang\nbấm nút\n- lỗi"`),
		answered("sev", "type_fields.severity", QuestionKindSingleChoice, `"high"`),
		answered("actual", "type_fields.actual", QuestionKindText, `"  treo  "`),
		answered("ac", "acceptance_criteria", QuestionKindText, "\"Không treo nữa\\nCó log\""),
		answered("body", "body", QuestionKindText, `"chi tiết thêm"`),
		answered("title", "title", QuestionKindText, `"Tiêu đề mới"`),
		{QuestionKey: "note", Kind: QuestionKindText, Answer: json.RawMessage(`"ghi chú"`)},
		{QuestionKey: "unanswered", TargetPath: "type_fields.expected", Kind: QuestionKindText},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.TypeFields["repro_steps"], []any{"mở trang", "bấm nút", "lỗi"}) || got.TypeFields["severity"] != "high" || got.TypeFields["actual"] != "treo" {
		t.Errorf("type fields = %#v", got.TypeFields)
	}
	if len(got.AcceptanceCriteria.Items) != 2 || got.AcceptanceCriteria.Items[1].ID != "AC-2" {
		t.Errorf("ACs = %+v", got.AcceptanceCriteria)
	}
	if got.Body != "đã có\n\nchi tiết thêm" || got.Title != "Tiêu đề mới" {
		t.Errorf("title/body = %q / %q", got.Title, got.Body)
	}
	if _, has := got.TypeFields["expected"]; has {
		t.Error("an unanswered question must not write anything")
	}
	if len(c.AcceptanceCriteria.Items) != 0 || c.Title != "t" {
		t.Error("ApplyAnswers must not mutate its input")
	}
}

func TestApplyAnswers_UnsupportedPath(t *testing.T) {
	c := RequestContent{Title: "t", Type: RequestTypeBug}
	for _, target := range []string{"status", "type_fields.", "type_fields.a.b", "type_fields.ac_next", "acceptance_criteria.0", "../body"} {
		_, err := ApplyAnswers(c, []ClarificationQuestion{answered("k", target, QuestionKindText, `"x"`)})
		if errCode(err) != "REQUEST_CLARIFICATION_INVALID_ANSWER" {
			t.Errorf("%q: got %v", target, err)
		}
	}
	// A value that does not fit the key's rule is refused rather than coerced.
	_, err := ApplyAnswers(c, []ClarificationQuestion{answered("k", "type_fields.severity", QuestionKindText, `"meh"`)})
	if errCode(err) != "REQUEST_CLARIFICATION_INVALID_ANSWER" {
		t.Errorf("enum mismatch: %v", err)
	}
	_, err = ApplyAnswers(RequestContent{Type: RequestTypeSecurity}, []ClarificationQuestion{answered("k", "type_fields.data_exposed", QuestionKindText, `"yes"`)})
	if errCode(err) != "REQUEST_CLARIFICATION_INVALID_ANSWER" {
		t.Errorf("bool mismatch: %v", err)
	}
}

func TestApplyAnswers_ResultPassesReadyCheck(t *testing.T) {
	c := RequestContent{Title: "Lỗi treo", Body: "Trang treo khi bấm lưu, rất nghiêm trọng.", Type: RequestTypeBug, TypeFields: map[string]any{}}
	report := ReadinessPolicy{}.Evaluate(RequestTypeBug, c)
	if report.Ready {
		t.Fatal("must start unready")
	}
	qs := QuestionBuilder{}.Build(report, ReadinessHints{Title: c.Title})
	for i := range qs {
		switch qs[i].QuestionKey {
		case "type_fields.severity":
			qs[i].Answer = json.RawMessage(`"high"`)
		case "acceptance_criteria":
			qs[i].Answer = json.RawMessage(`"Lưu không treo"`)
		default:
			qs[i].Answer = json.RawMessage(`"một\nhai"`)
		}
		if err := ValidateAnswer(qs[i], qs[i].Answer); err != nil {
			t.Fatalf("%s: %v", qs[i].QuestionKey, err)
		}
	}
	filled, err := ApplyAnswers(c, qs)
	if err != nil {
		t.Fatal(err)
	}
	if after := (ReadinessPolicy{}).Evaluate(RequestTypeBug, filled); !after.Ready {
		t.Fatalf("answers should satisfy the gaps, still missing %+v", after.Missing)
	}
}

func TestReadinessPolicy_AllElevenTypes(t *testing.T) {
	for _, rt := range AllRequestTypes() {
		empty := (ReadinessPolicy{}).Evaluate(rt, RequestContent{Title: "t", Type: rt})
		if empty.Ready {
			t.Errorf("%s: empty content cannot be ready", rt)
		}
		if empty.Type != rt {
			t.Errorf("%s: report type %s", rt, empty.Type)
		}
		full := (ReadinessPolicy{}).Evaluate(rt, contentWith(rt, fullTypeFields(rt)))
		if !full.Ready || len(full.Missing) != 0 {
			t.Errorf("%s: %+v", rt, full)
		}
	}
}

func TestReadinessPolicy_NonBlockingMissingStillReady(t *testing.T) {
	tf := fullTypeFields(RequestTypeChangeRequest)
	delete(tf, "scope_out")
	r := (ReadinessPolicy{}).Evaluate(RequestTypeChangeRequest, contentWith(RequestTypeChangeRequest, tf))
	if !r.Ready || len(r.Missing) != 1 || r.Missing[0].Blocking || r.Missing[0].Path != "/type_fields/scope_out" {
		t.Fatalf("%+v", r)
	}
	if qs := (QuestionBuilder{}).Build(r, ReadinessHints{}); len(qs) != 0 {
		t.Fatalf("a recommended key must not produce a question: %+v", qs)
	}
}

func TestQuestionBuilder_DeterministicOrderAndReasons(t *testing.T) {
	c := RequestContent{Title: "Lỗi", Body: "x", Type: RequestTypeBug, TypeFields: map[string]any{}}
	report := (ReadinessPolicy{}).Evaluate(RequestTypeBug, c)
	hints := ReadinessHints{Title: "Lỗi", Urgency: UrgencyUrgent, SourceHints: SourceHints{Priority: "Medium"}}
	qs := QuestionBuilder{}.Build(report, hints)
	var keys []string
	for _, q := range qs {
		keys = append(keys, q.QuestionKey)
		if strings.TrimSpace(q.Reason) == "" || strings.TrimSpace(q.Prompt) == "" || !q.Required {
			t.Errorf("%s: prompt/reason/required missing", q.QuestionKey)
		}
		if err := ValidateQuestion(q); err != nil {
			t.Errorf("%s: built question is invalid: %v", q.QuestionKey, err)
		}
	}
	want := []string{"body", "type_fields.repro_steps", "type_fields.actual", "type_fields.expected", "type_fields.environment", "type_fields.severity", "acceptance_criteria"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("order = %v", keys)
	}
	for i, q := range qs {
		if q.Seq != i+1 {
			t.Errorf("seq %d at %d", q.Seq, i)
		}
		switch q.QuestionKey {
		case "type_fields.severity":
			if q.Kind != QuestionKindSingleChoice || len(q.Options) != 4 || string(q.SuggestedDefault) != `"high"` {
				t.Errorf("severity question = %+v default %s", q, q.SuggestedDefault)
			}
		case "type_fields.repro_steps":
			if q.Kind != QuestionKindText || q.TargetPath != "type_fields.repro_steps" {
				t.Errorf("repro question = %+v", q)
			}
		case "acceptance_criteria":
			if !strings.Contains(string(q.SuggestedDefault), "Lỗi") {
				t.Errorf("AC default = %s", q.SuggestedDefault)
			}
		}
	}
	again := QuestionBuilder{}.Build(report, hints)
	if !reflect.DeepEqual(qs, again) {
		t.Fatal("builder is not deterministic")
	}
	sec := (ReadinessPolicy{}).Evaluate(RequestTypeSecurity, RequestContent{Title: "t", Type: RequestTypeSecurity})
	for _, q := range (QuestionBuilder{}).Build(sec, ReadinessHints{}) {
		if q.QuestionKey == "type_fields.data_exposed" && q.Kind != QuestionKindBoolean {
			t.Errorf("data_exposed should be a boolean question: %+v", q)
		}
	}
}
