package grpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// clarStub serves canned clarifications and, like the real adapters, hides those of other tenants.
type clarStub struct {
	usecase.ClarificationRepository
	byID    map[string]domain.Clarification
	pending []domain.Clarification
	next    string
	touch   error
}

func (s *clarStub) Get(ctx context.Context, id string) (domain.Clarification, error) {
	t, _ := tenant.TenantID(ctx)
	if c, ok := s.byID[id]; ok && c.TenantID == t {
		return c, nil
	}
	return domain.Clarification{}, domain.ErrClarificationNotFound(id)
}

func (s *clarStub) List(_ context.Context, f usecase.ClarificationListFilter) ([]domain.Clarification, error) {
	var out []domain.Clarification
	for _, c := range s.byID {
		if c.RequestID == f.RequestID && c.Seq > f.AfterSeq && (f.Status == "" || c.Status == f.Status) {
			out = append(out, c)
		}
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (s *clarStub) UpdateAnswers(context.Context, string, []usecase.AnswerRecord, int64) error {
	return s.touch
}

func (s *clarStub) ListPendingForUser(_ context.Context, f usecase.PendingFilter) ([]domain.Clarification, string, error) {
	return s.pending, s.next, nil
}

type decStub struct {
	usecase.DecisionRepository
	rows map[string]domain.Decision
}

func (s *decStub) Get(ctx context.Context, id string) (domain.Decision, error) {
	t, _ := tenant.TenantID(ctx)
	if d, ok := s.rows[id]; ok && d.TenantID == t {
		return d, nil
	}
	return domain.Decision{}, domain.ErrDecisionNotFound(id)
}

func (s *decStub) List(_ context.Context, f usecase.DecisionListFilter) ([]domain.Decision, error) {
	var out []domain.Decision
	for _, d := range s.rows {
		if d.RequestID == f.RequestID && d.Seq > f.AfterSeq {
			out = append(out, d)
		}
	}
	return out, nil
}

type clarHarness struct {
	srv   *Server
	art   *artHarness
	clars *clarStub
	decs  *decStub
	open  domain.Clarification
}

func newClarHarness(t *testing.T) *clarHarness {
	t.Helper()
	a := newArtHarness(t)
	q := func(id, key string, kind domain.QuestionKind, opts ...string) domain.ClarificationQuestion {
		cq := domain.ClarificationQuestion{ID: id, Seq: 1, QuestionKey: key, Kind: kind, Prompt: "p", Reason: "r", Required: true, Answer: json.RawMessage(`"bí mật"`)}
		for _, o := range opts {
			cq.Options = append(cq.Options, domain.QuestionOption{ID: o, Label: o})
		}
		return cq
	}
	open := domain.Clarification{
		ID: "c-open", TenantID: artTenant, RequestID: a.req.ID, Seq: 1, Source: domain.ClarificationSourceReadiness, Status: domain.ClarificationStatusOpen,
		ResumeStatus: domain.RequestStatusAnalyzing, Round: 1, DueAt: time.Now().Add(time.Hour), CreatedAt: time.Now(), Version: 3,
		Questions: []domain.ClarificationQuestion{q("q1", "type_fields.severity", domain.QuestionKindSingleChoice, "low", "high"), q("q2", "body", domain.QuestionKindText)},
		Assignees: []domain.Principal{{Kind: domain.PrincipalKindReporter}},
	}
	expired := open
	expired.ID, expired.DueAt = "c-expired", time.Now().Add(-time.Hour)
	answered := open
	answered.ID, answered.Status = "c-answered", domain.ClarificationStatusAnswered
	h := &clarHarness{art: a, open: open, clars: &clarStub{byID: map[string]domain.Clarification{open.ID: open, expired.ID: expired, answered.ID: answered}}}
	h.decs = &decStub{rows: map[string]domain.Decision{"d1": {
		ID: "d1", TenantID: artTenant, RequestID: a.req.ID, Seq: 1, SubjectKind: domain.DecisionSubjectSolutionOption, SubjectID: "sol", Status: domain.DecisionStatusChosen,
		ChosenOptionID: "opt-2", ChooserID: artUser, Options: []domain.DecisionOption{{ID: "opt-2", Label: "Viết lại toàn bộ"}}, RiskLevel: domain.RiskHigh, Version: 1, CreatedAt: time.Now(),
	}}}
	queries := usecase.NewClarificationQueries(a.repo, h.clars)
	clarify := usecase.NewRequestClarification(a.repo, h.clars, nil, nil, noTx{}, noOutbox{})
	answer := usecase.NewAnswerClarification(a.repo, h.clars, nil, nil, nil, clarify, nil, nil, noTx{}, noOutbox{}, 3)
	a.srv.WithClarification(ClarificationUseCases{
		Readiness: usecase.NewGetRequestReadiness(a.repo), Request: clarify, Queries: queries, Answer: answer,
		Decisions: usecase.NewDecisionQueries(a.repo, h.decs), DecisionConf: usecase.NewConfirmDecision(a.repo, h.decs, noTx{}, noOutbox{}),
	})
	h.srv = a.srv
	return h
}

func TestClarificationServer_Auth_NoTenant(t *testing.T) {
	h := newClarHarness(t)
	bare := context.Background()
	_, err := h.srv.GetClarification(bare, &requestv1.GetClarificationRequest{Id: "c-open"})
	wantCode(t, err, codes.InvalidArgument, "GetClarification")
	_, err = h.srv.AnswerClarification(bare, &requestv1.AnswerClarificationRequest{ClarificationId: "c-open"})
	wantCode(t, err, codes.InvalidArgument, "AnswerClarification")
	_, err = h.srv.ListClarifications(bare, &requestv1.ListClarificationsRequest{RequestId: h.art.req.ID})
	wantCode(t, err, codes.InvalidArgument, "ListClarifications")
	_, err = h.srv.GetRequestReadiness(bare, &requestv1.GetRequestReadinessRequest{RequestId: h.art.req.ID})
	wantCode(t, err, codes.InvalidArgument, "GetRequestReadiness")
	_, err = h.srv.GetDecision(bare, &requestv1.GetDecisionRequest{Id: "d1"})
	wantCode(t, err, codes.InvalidArgument, "GetDecision")
	_, err = h.srv.ConfirmDecision(bare, &requestv1.ConfirmDecisionRequest{DecisionId: "d1"})
	wantCode(t, err, codes.InvalidArgument, "ConfirmDecision")
	_, err = h.srv.ListPendingClarificationsForUser(bare, &requestv1.ListPendingClarificationsForUserRequest{})
	wantCode(t, err, codes.InvalidArgument, "ListPendingClarificationsForUser")
}

func TestClarificationServer_CrossTenantNotFound(t *testing.T) {
	h := newClarHarness(t)
	other := tenant.WithUserID(tenant.WithTenantID(context.Background(), "99999999-9999-4999-8999-999999999999"), artUser)
	_, err := h.srv.GetClarification(other, &requestv1.GetClarificationRequest{Id: "c-open"})
	wantCode(t, err, codes.NotFound, "GetClarification")
	_, err = h.srv.AnswerClarification(other, &requestv1.AnswerClarificationRequest{ClarificationId: "c-open", Complete: true})
	wantCode(t, err, codes.NotFound, "AnswerClarification")
	_, err = h.srv.ListClarifications(other, &requestv1.ListClarificationsRequest{RequestId: h.art.req.ID})
	wantCode(t, err, codes.NotFound, "ListClarifications")
	_, err = h.srv.GetRequestReadiness(other, &requestv1.GetRequestReadinessRequest{RequestId: h.art.req.ID})
	wantCode(t, err, codes.NotFound, "GetRequestReadiness")
	_, err = h.srv.GetDecision(other, &requestv1.GetDecisionRequest{Id: "d1"})
	wantCode(t, err, codes.NotFound, "GetDecision")
	_, err = h.srv.ListDecisions(other, &requestv1.ListDecisionsRequest{RequestId: h.art.req.ID})
	wantCode(t, err, codes.NotFound, "ListDecisions")
	_, err = h.srv.ConfirmDecision(other, &requestv1.ConfirmDecisionRequest{DecisionId: "d1", ConfirmationText: "x"})
	wantCode(t, err, codes.NotFound, "ConfirmDecision")
}

func TestClarificationServer_RequestClarification_RejectsSystemOnlySources(t *testing.T) {
	h := newClarHarness(t)
	for _, src := range []string{"readiness", "task_blocked"} {
		_, err := h.srv.RequestClarification(artCtx(artUser), &requestv1.RequestClarificationRequest{RequestId: h.art.req.ID, Source: src})
		wantCode(t, err, codes.FailedPrecondition, src)
		if !contains(status.Convert(err).Message(), "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED") {
			t.Errorf("%s: %v", src, err)
		}
	}
	_, err := h.srv.RequestClarification(artCtx(artUser), &requestv1.RequestClarificationRequest{RequestId: h.art.req.ID, Source: "bogus"})
	wantCode(t, err, codes.FailedPrecondition, "unknown source")
	_, err = h.srv.RequestClarification(artCtx(artUser), &requestv1.RequestClarificationRequest{
		RequestId: h.art.req.ID, Source: "manual", Questions: []*requestv1.ClarificationQuestion{{QuestionKey: "k", Kind: requestv1.QuestionKind_QUESTION_KIND_UNSPECIFIED}},
	})
	wantCode(t, err, codes.InvalidArgument, "unspecified kind")
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestClarificationServer_Answer_MapsErrors(t *testing.T) {
	h := newClarHarness(t)
	ctx := artCtx(artUser)
	cases := []struct {
		name string
		req  *requestv1.AnswerClarificationRequest
		ctx  context.Context
		code codes.Code
		msg  string
	}{
		{"unknown id", &requestv1.AnswerClarificationRequest{ClarificationId: "nope"}, ctx, codes.NotFound, "REQUEST_CLARIFICATION_NOT_FOUND"},
		{"expired", &requestv1.AnswerClarificationRequest{ClarificationId: "c-expired"}, ctx, codes.FailedPrecondition, "REQUEST_CLARIFICATION_EXPIRED"},
		{"already answered, different body", &requestv1.AnswerClarificationRequest{ClarificationId: "c-answered", Complete: true,
			Answers: []*requestv1.AnswerItem{{QuestionId: "q2", ValueJson: `"khác"`}}}, ctx, codes.FailedPrecondition, "REQUEST_CLARIFICATION_ALREADY_ANSWERED"},
		{"not an assignee", &requestv1.AnswerClarificationRequest{ClarificationId: "c-open"}, artCtx(artOther), codes.PermissionDenied, "REQUEST_CLARIFICATION_NOT_ASSIGNEE"},
		{"value outside options", &requestv1.AnswerClarificationRequest{ClarificationId: "c-open",
			Answers: []*requestv1.AnswerItem{{QuestionId: "q1", ValueJson: `"meh"`}}}, ctx, codes.InvalidArgument, "REQUEST_CLARIFICATION_INVALID_ANSWER"},
		{"value is not JSON", &requestv1.AnswerClarificationRequest{ClarificationId: "c-open",
			Answers: []*requestv1.AnswerItem{{QuestionId: "q1", ValueJson: `{`}}}, ctx, codes.InvalidArgument, "REQUEST_CLARIFICATION_INVALID_ANSWER"},
		{"incomplete", &requestv1.AnswerClarificationRequest{ClarificationId: "c-open", Complete: true,
			Answers: []*requestv1.AnswerItem{{QuestionId: "q1", ValueJson: `"low"`}}}, ctx, codes.FailedPrecondition, "REQUEST_CLARIFICATION_INCOMPLETE"},
	}
	// q1/q2 already carry answers in the canned data, so blank them for the incomplete case.
	c := h.clars.byID["c-open"]
	c.Questions = append([]domain.ClarificationQuestion(nil), c.Questions...)
	c.Questions[1].Answer = nil
	h.clars.byID["c-open"] = c
	for _, tc := range cases {
		_, err := h.srv.AnswerClarification(tc.ctx, tc.req)
		wantCode(t, err, tc.code, tc.name)
		if err != nil && !contains(status.Convert(err).Message(), tc.msg) {
			t.Errorf("%s: message %q lacks %s", tc.name, status.Convert(err).Message(), tc.msg)
		}
	}
	h.clars.touch = domain.ErrClarificationVersionConflict("c-open", 9)
	_, err := h.srv.AnswerClarification(ctx, &requestv1.AnswerClarificationRequest{ClarificationId: "c-open", ExpectedVersion: 9,
		Answers: []*requestv1.AnswerItem{{QuestionId: "q1", ValueJson: `"low"`}}})
	wantCode(t, err, codes.FailedPrecondition, "version conflict")
}

func TestClarificationServer_AnswerJSONHiddenFromNonAssignee(t *testing.T) {
	h := newClarHarness(t)
	own, err := h.srv.GetClarification(artCtx(artUser), &requestv1.GetClarificationRequest{Id: "c-open"})
	if err != nil || own.Clarification.Questions[1].AnswerJson != `"bí mật"` {
		t.Fatalf("the reporter sees answers: %+v %v", own, err)
	}
	stranger, err := h.srv.GetClarification(artCtx(artOther), &requestv1.GetClarificationRequest{Id: "c-open"})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range stranger.Clarification.Questions {
		if q.AnswerJson != "" {
			t.Fatalf("answer leaked to a stranger: %+v", q)
		}
	}
	if len(stranger.Clarification.Questions) != 2 || stranger.Clarification.Questions[0].Prompt != "p" {
		t.Fatal("questions stay visible to readers")
	}
	admin := tenant.WithRole(artCtx(artOther), "admin")
	got, _ := h.srv.GetClarification(admin, &requestv1.GetClarificationRequest{Id: "c-open"})
	if got.Clarification.Questions[0].AnswerJson == "" {
		t.Fatal("an admin sees answers")
	}
}

func TestClarificationServer_ListPending_PaginatesAndFiltersByAssignee(t *testing.T) {
	h := newClarHarness(t)
	h.clars.pending, h.clars.next = []domain.Clarification{h.open}, "tok-2"
	res, err := h.srv.ListPendingClarificationsForUser(artCtx(artUser), &requestv1.ListPendingClarificationsForUserRequest{PageSize: 1})
	if err != nil || len(res.Clarifications) != 1 || res.NextPageToken != "tok-2" || res.Clarifications[0].Id != "c-open" || res.Clarifications[0].DisplayId != "CLR-7.1" {
		t.Fatalf("%+v %v", res, err)
	}
	if res.Clarifications[0].Status != requestv1.ClarificationStatus_CLARIFICATION_STATUS_OPEN || res.Clarifications[0].ResumeStatus != "analyzing" {
		t.Fatalf("%+v", res.Clarifications[0])
	}
}

func TestClarificationServer_ListClarifications_StatusFilterAndToken(t *testing.T) {
	h := newClarHarness(t)
	res, err := h.srv.ListClarifications(artCtx(artUser), &requestv1.ListClarificationsRequest{RequestId: h.art.req.ID, Status: requestv1.ClarificationStatus_CLARIFICATION_STATUS_ANSWERED})
	if err != nil || len(res.Clarifications) != 1 || res.Clarifications[0].Id != "c-answered" {
		t.Fatalf("%+v %v", res, err)
	}
	_, err = h.srv.ListClarifications(artCtx(artUser), &requestv1.ListClarificationsRequest{RequestId: h.art.req.ID, PageToken: "x"})
	wantCode(t, err, codes.InvalidArgument, "bad token")
	_, err = h.srv.ListClarifications(artCtx(artUser), &requestv1.ListClarificationsRequest{RequestId: h.art.req.ID, Status: 99})
	wantCode(t, err, codes.FailedPrecondition, "unknown status")
}

func TestDecisionServer_ConfirmDecision_MapsMismatchToFailedPrecondition(t *testing.T) {
	h := newClarHarness(t)
	_, err := h.srv.ConfirmDecision(artCtx(artUser), &requestv1.ConfirmDecisionRequest{DecisionId: "d1", ConfirmationText: "Nâng cấp"})
	wantCode(t, err, codes.FailedPrecondition, "mismatch")
	if !contains(status.Convert(err).Message(), "REQUEST_DECISION_CONFIRMATION_MISMATCH") {
		t.Fatal(err)
	}
	_, err = h.srv.ConfirmDecision(artCtx(artOther), &requestv1.ConfirmDecisionRequest{DecisionId: "d1", ConfirmationText: "Viết lại toàn bộ"})
	wantCode(t, err, codes.PermissionDenied, "not the chooser")
	_, err = h.srv.ConfirmDecision(tenant.WithActorType(artCtx(artUser), tenant.ActorAgent), &requestv1.ConfirmDecisionRequest{DecisionId: "d1", ConfirmationText: "Viết lại toàn bộ"})
	wantCode(t, err, codes.PermissionDenied, "machine")
}

func TestDecisionServer_ListAndGet(t *testing.T) {
	h := newClarHarness(t)
	list, err := h.srv.ListDecisions(artCtx(artUser), &requestv1.ListDecisionsRequest{RequestId: h.art.req.ID})
	if err != nil || len(list.Decisions) != 1 || list.Decisions[0].DisplayId != "DEC-7.1" || list.Decisions[0].Status != requestv1.DecisionStatus_DECISION_STATUS_CHOSEN || list.Decisions[0].RiskLevel != "high" {
		t.Fatalf("%+v %v", list, err)
	}
	got, err := h.srv.GetDecision(artCtx(artUser), &requestv1.GetDecisionRequest{Id: "d1"})
	if err != nil || got.Decision.ChosenOptionId != "opt-2" || got.Decision.OptionsJson == "" {
		t.Fatalf("%+v %v", got, err)
	}
	_, err = h.srv.ListDecisions(artCtx(artUser), &requestv1.ListDecisionsRequest{RequestId: h.art.req.ID, Status: 99})
	if err == nil {
		t.Fatal("unknown status")
	}
}

func TestClarificationMapper_RoundTrip_AllKinds(t *testing.T) {
	var qs []*requestv1.ClarificationQuestion
	for kind := range questionKindToProto {
		qs = append(qs, &requestv1.ClarificationQuestion{
			QuestionKey: "k-" + string(kind), Kind: questionKindToProto[kind], Prompt: "p", Reason: "r", Required: true, TargetPath: "body",
			OptionsJson: `[{"id":"a","label":"A"}]`, SuggestedDefaultJson: `"a"`,
		})
	}
	inputs, err := questionInputsFromProto(qs)
	if err != nil || len(inputs) != len(qs) {
		t.Fatalf("%v", err)
	}
	for i, in := range inputs {
		if questionKindToProto[in.Kind] != qs[i].Kind || in.Options[0].ID != "a" || string(in.SuggestedDefault) != `"a"` || !in.Required || in.TargetPath != "body" {
			t.Errorf("%+v", in)
		}
	}
	if _, err := questionInputsFromProto([]*requestv1.ClarificationQuestion{{QuestionKey: "k", Kind: questionKindToProto[domain.QuestionKindText], OptionsJson: "{"}}); err == nil {
		t.Error("bad options JSON must fail")
	}
	if _, err := questionInputsFromProto([]*requestv1.ClarificationQuestion{{QuestionKey: "k", Kind: questionKindToProto[domain.QuestionKindText], SuggestedDefaultJson: "{"}}); err == nil {
		t.Error("bad default JSON must fail")
	}
	items, err := answerItemsFromProto([]*requestv1.AnswerItem{{QuestionId: "q", ValueJson: `{"a":1}`}, {QuestionId: "r", AcceptDefault: true}})
	if err != nil || len(items) != 2 || !items[1].AcceptDefault || items[1].Value != nil {
		t.Fatalf("%+v %v", items, err)
	}
	for _, st := range domain.AllClarificationStatuses() {
		back, err := clarificationStatusFromProto(clarificationStatusToProto[st])
		if err != nil || back != st {
			t.Errorf("%s -> %s %v", st, back, err)
		}
	}
	for _, st := range []domain.DecisionStatus{domain.DecisionStatusOpen, domain.DecisionStatusChosen, domain.DecisionStatusEffective, domain.DecisionStatusSuperseded} {
		back, err := decisionStatusFromProto(decisionStatusToProto[st])
		if err != nil || back != st {
			t.Errorf("decision %s -> %s %v", st, back, err)
		}
	}
}
