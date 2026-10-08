package contracttest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type clarKit struct {
	env      ArtifactEnv
	clock    *testClock
	tr       *usecase.TransitionRequest
	closer   *usecase.OpenClarificationCanceller
	ret      *usecase.ReturnRequestToBacklog
	cancel   *usecase.CancelRequest
	change   *usecase.ChangeRequestType
	clarify  *usecase.RequestClarification
	confirm  *usecase.ConfirmRequestType
	answer   *usecase.AnswerClarification
	waive    *usecase.WaiveReadiness
	sweeper  *usecase.ClarificationSweeper
	appendRv *usecase.AppendRequestRevision
	gates    *usecase.ApprovalGates
	recorder *usecase.RecordDecision
	confirmD *usecase.ConfirmDecision
	resume   *usecase.ResumeAfterClarification
	started  *startRecorder
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Set(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
}

type startRecorder struct {
	mu       sync.Mutex
	runs     []string
	advances []string
}

func (s *startRecorder) StartSolution(_ context.Context, id, feedback, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = append(s.runs, id+"|"+feedback+"|"+key)
	return nil
}

func (s *startRecorder) Advance(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.advances = append(s.advances, id)
	return nil
}

type approvalSpy struct {
	mu        sync.Mutex
	cancelled []string
}

func (a *approvalSpy) CancelPending(_ context.Context, _, why string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancelled = append(a.cancelled, why)
	return nil
}
func (a *approvalSpy) RequestTypeApproval(context.Context, string) error { return nil }
func (a *approvalSpy) Approve(context.Context, string, string) error     { return nil }

type typeHistoryAdapter struct {
	usecase.RequestTypeHistoryRepository
}

func newClarKit(env ArtifactEnv) *clarKit {
	k := &clarKit{env: env, clock: &testClock{now: time.Now().UTC().Truncate(time.Microsecond)}, started: &startRecorder{}}
	scope := env.Tx.(usecase.TxScope)
	k.tr = usecase.NewTransitionRequest(env.Requests, scope, env.Outbox)
	spy := &approvalSpy{}
	k.closer = usecase.NewOpenClarificationCanceller(env.Clarifications, env.Requests, env.Outbox)
	k.ret = usecase.NewReturnRequestToBacklog(env.Requests, k.tr, env.Returns, spy, &toggleGuard{}, env.Tx, env.Outbox).WithClarifications(k.closer)
	k.cancel = usecase.NewCancelRequest(env.Requests, k.tr, env.Returns, spy, &toggleGuard{}, env.Tx).WithClarifications(k.closer)
	k.change = usecase.NewChangeRequestType(env.Requests, env.History, k.tr, spy, &toggleGuard{}, env.Tx, env.Outbox).WithClarifications(k.closer)
	k.clarify = usecase.NewRequestClarification(env.Requests, env.Clarifications, k.tr, spy, env.Tx, env.Outbox).WithClock(k.clock.Now)
	gate := usecase.NewReadinessGate(env.Requests, k.clarify, env.Clarifications, env.Revisions, k.tr, 3)
	k.confirm = usecase.NewConfirmRequestType(env.Requests, env.History, k.tr, spy, env.Tx, env.Outbox).WithReadiness(gate)
	k.appendRv = artAppend(env)
	k.answer = usecase.NewAnswerClarification(env.Requests, env.Clarifications, k.appendRv, k.tr, k.ret, k.clarify, env.Revisions, env.Decisions, env.Tx, env.Outbox, 3).WithClock(k.clock.Now)
	k.waive = usecase.NewWaiveReadiness(env.Requests, env.Clarifications, k.appendRv, k.tr, env.Tx, env.Outbox)
	k.sweeper = usecase.NewClarificationSweeper(env.Requests, env.Clarifications, k.ret, usecase.PrincipalRecipients{}, env.Tx, env.Outbox).WithClock(k.clock.Now)
	k.gates = usecase.NewApprovalGates(env.Decisions, env.Clarifications)
	k.recorder = usecase.NewRecordDecision(env.Decisions, env.Outbox)
	k.confirmD = usecase.NewConfirmDecision(env.Requests, env.Decisions, env.Tx, env.Outbox)
	k.resume = usecase.NewResumeAfterClarification(env.Clarifications, env.Processed, env.Tx).WithAnalysis(k.started).WithExecution(k.started)
	return k
}

func adminCtx(tenantID string) context.Context {
	return tenant.WithRole(userIn(tenantID, uuid.NewString()), "admin")
}

// RunClarificationContract covers CR-REQ-028 on the dialect's real database.
func RunClarificationContract(t *testing.T, newEnv func(t *testing.T) ArtifactEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, k *clarKit)
	}{
		{"BugMissingDataGoesAwaitingInformation", clBugAwaiting},
		{"TwoConcurrentRequestsOneOpenWins", clTwoConcurrent},
		{"AnswerCompleteWritesRevisionAndResumes", clAnswerResume},
		{"ResumeEventRedeliveryOneRun", clResumeRedelivery},
		{"StillMissingRound2ThenBacklogAfterThree", clRounds},
		{"ExpiryReturnsToBacklogMissingInfo", clExpiry},
		{"TwoSweepersNoDoubleProcessing", clTwoSweepers},
		{"AnswerRacingExpiryOneWins", clAnswerVsExpiry},
		{"TypeChangeFromAwaitingInformationCancels", clTypeChange},
		{"CancelRequestCancelsClarification", clCancelRequest},
		{"WaiveForbiddenForSecurityAndAdminOnly", clWaive},
		{"ListPendingForUserSameOnBothDialects", clPendingForUser},
		{"DecisionHighRiskBlocksUntilConfirmed", clDecisionFlow},
		{"DecisionOneLivePerSubject", clDecisionLive},
		{"AnswerRollsBackAcrossTables", clAnswerRollback},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, newClarKit(env)) })
	}
}

func clSeed(t *testing.T, k *clarKit, ctx context.Context, status domain.RequestStatus, mod func(r *domain.Request)) domain.Request {
	t.Helper()
	return artSeed(t, k.env, ctx, func(r *domain.Request) {
		r.Status, r.Type, r.Size = status, domain.RequestTypeBug, domain.RequestSizeM
		c, _ := domain.ContentFromRequest(*r)
		c.Body = "Mô tả đủ dài để vượt qua ngưỡng hai mươi ký tự của Definition of Ready."
		*r, _ = r.WithContent(c)
		if mod != nil {
			mod(r)
		}
	})
}

func clConfirm(t *testing.T, k *clarKit, ctx context.Context, r domain.Request) domain.Request {
	t.Helper()
	out, err := k.confirm.Execute(ctx, usecase.ConfirmInput{RequestID: r.ID, Type: "bug", Size: "M", ActorID: uuid.NewString(), ActorKind: domain.ActorKindUser})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	return out
}

func clOpen(t *testing.T, k *clarKit, ctx context.Context, requestID string) domain.Clarification {
	t.Helper()
	c, err := k.env.Clarifications.GetOpenByRequest(ctx, requestID)
	if err != nil || c == nil {
		t.Fatalf("no open clarification: %v", err)
	}
	return *c
}

func clAnswerItems(c domain.Clarification) []usecase.AnswerItem {
	values := map[string]string{
		"type_fields.repro_steps": `"mở trang\nbấm lưu"`, "type_fields.actual": `"treo"`, "type_fields.expected": `"lưu được"`,
		"type_fields.environment": `"prod"`, "type_fields.severity": `"high"`, "acceptance_criteria": `"Lưu không treo"`, "body": `"Trang treo mỗi khi bấm lưu bản ghi mới."`,
	}
	var out []usecase.AnswerItem
	for _, q := range c.Questions {
		out = append(out, usecase.AnswerItem{QuestionID: q.ID, Value: json.RawMessage(values[q.QuestionKey])})
	}
	return out
}

func clBugAwaiting(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.ReporterID = reporter })
	out := clConfirm(t, k, ctx, r)
	if out.Status != domain.RequestStatusAwaitingInformation {
		t.Fatalf("status %s", out.Status)
	}
	c := clOpen(t, k, ctx, r.ID)
	if c.Source != domain.ClarificationSourceReadiness || c.Round != 1 || c.Seq != 1 || len(c.Questions) != 6 || c.ResumeStatus != domain.RequestStatusAnalyzing {
		t.Fatalf("%+v", c)
	}
	if len(c.Assignees) != 1 || c.Assignees[0].Kind != domain.PrincipalKindReporter || c.Assignees[0].ID != "" {
		t.Fatalf("assignees %+v", c.Assignees)
	}
	for _, q := range c.Questions {
		if q.Reason == "" || q.Prompt == "" || !q.Required {
			t.Errorf("question %s lacks prompt, reason or required", q.QuestionKey)
		}
		if q.QuestionKey == "type_fields.severity" && (len(q.Options) != 4 || string(q.SuggestedDefault) == "") {
			t.Errorf("severity: %+v", q)
		}
	}
	subjects := k.env.OutboxSubjects(t, tenantID)
	if indexOf(subjects, domain.SubjectRequestTypeConfirmed) < 0 || indexOf(subjects, domain.SubjectClarificationRequested) < 0 {
		t.Fatalf("events %v", subjects)
	}
	pend, next, err := k.env.Clarifications.ListPendingForUser(ctx, usecase.PendingFilter{UserID: reporter, PageSize: 10})
	if err != nil || len(pend) != 1 || next != "" {
		t.Fatalf("the reporter's pending list: %+v %v", pend, err)
	}
}

func clTwoConcurrent(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	ctx := userIn(tenantID, uuid.NewString())
	r := clSeed(t, k, ctx, domain.RequestStatusAnalyzing, nil)
	var wg sync.WaitGroup
	type res struct {
		c   domain.Clarification
		err error
	}
	out := make(chan res, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			c, err := k.clarify.Execute(adminCtx(tenantID), usecase.RequestClarificationInput{
				RequestID: r.ID, Source: domain.ClarificationSourceManual, ActorKind: domain.ActorKindUser,
				Questions: []usecase.QuestionInput{{QuestionKey: fmt.Sprintf("q%d", i), Kind: domain.QuestionKindText, Prompt: "p", Reason: "r", Required: true}},
			})
			out <- res{c, err}
		}()
	}
	close(start)
	wg.Wait()
	close(out)
	wins := 0
	for o := range out {
		if o.err == nil {
			wins++
			continue
		}
		code := errCode(o.err)
		if code != "REQUEST_CLARIFICATION_STATE_NOT_ALLOWED" && code != "REQUEST_VERSION_CONFLICT" && code != "REQUEST_STATE_STALE" && !containsAny(o.err.Error(), "1213", "deadlock detected", "could not serialize") {
			t.Fatalf("unexpected loss: %v", o.err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d winners, want exactly one", wins)
	}
	open := 0
	list, _ := k.env.Clarifications.List(ctx, usecase.ClarificationListFilter{RequestID: r.ID, Limit: 10})
	for _, c := range list {
		if c.Status == domain.ClarificationStatusOpen {
			open++
		}
	}
	if open != 1 {
		t.Fatalf("%d open clarifications, the unique index allows one", open)
	}
	if got, _ := k.env.Requests.Get(ctx, r.ID); got.Status != domain.RequestStatusAwaitingInformation {
		t.Fatalf("status %s", got.Status)
	}
}

func clAnswerResume(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.ReporterID = reporter })
	clConfirm(t, k, ctx, r)
	c := clOpen(t, k, ctx, r.ID)
	// a draft first
	q := c.Questions[1]
	draft, err := k.answer.Execute(ctx, usecase.AnswerInput{ClarificationID: c.ID, Answers: []usecase.AnswerItem{{QuestionID: q.ID, Value: json.RawMessage(`"treo"`)}}})
	if err != nil || !draft.StillMissing {
		t.Fatalf("%+v %v", draft, err)
	}
	if got, _ := k.env.Requests.Get(ctx, r.ID); got.ContentRevision != 1 {
		t.Fatal("a draft wrote a revision")
	}
	res, err := k.answer.Execute(ctx, usecase.AnswerInput{ClarificationID: c.ID, Answers: clAnswerItems(c), Complete: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.RequestStatus != domain.RequestStatusAnalyzing || res.RequestRevision != 2 || res.Clarification.Status != domain.ClarificationStatusAnswered {
		t.Fatalf("%+v", res)
	}
	got, _ := k.env.Requests.Get(ctx, r.ID)
	if got.Status != domain.RequestStatusAnalyzing || got.ContentRevision != 2 {
		t.Fatalf("%+v", got)
	}
	rev, err := k.env.Revisions.Get(ctx, r.ID, 2)
	if err != nil || rev.Cause != domain.RevisionCauseClarificationAnswered || rev.ClarificationID != c.ID {
		t.Fatalf("%+v %v", rev, err)
	}
	stored, _ := k.env.Clarifications.Get(ctx, c.ID)
	if stored.AnsweredRequestRevision == nil || *stored.AnsweredRequestRevision != 2 || stored.AnsweredAt == nil {
		t.Fatalf("%+v", stored)
	}
	for _, sq := range stored.Questions {
		if !sq.HasAnswer() || sq.AnswerSource != domain.AnswerSourceUser {
			t.Errorf("question %s: %+v", sq.QuestionKey, sq)
		}
	}
	subjects := k.env.OutboxSubjects(t, tenantID)
	if indexOf(subjects, domain.SubjectClarificationAnswered) < 0 {
		t.Fatalf("events %v", subjects)
	}
	// an exact repeat is answered from the stored result
	again, err := k.answer.Execute(ctx, usecase.AnswerInput{ClarificationID: c.ID, Answers: clAnswerItems(c), Complete: true})
	if err != nil || again.RequestRevision != 2 {
		t.Fatalf("%+v %v", again, err)
	}
	if after, _ := k.env.Requests.Get(ctx, r.ID); after.ContentRevision != 2 {
		t.Fatal("a repeat wrote another revision")
	}
}

func clResumeRedelivery(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.ReporterID = reporter })
	clConfirm(t, k, ctx, r)
	c := clOpen(t, k, ctx, r.ID)
	if _, err := k.answer.Execute(ctx, usecase.AnswerInput{ClarificationID: c.ID, Answers: clAnswerItems(c), Complete: true}); err != nil {
		t.Fatal(err)
	}
	ev := usecase.ResumeEvent{ID: uuid.NewString(), TenantID: tenantID, RequestID: r.ID, To: "analyzing", Trigger: string(domain.TriggerInformationProvided)}
	for i := 0; i < 3; i++ {
		if err := k.resume.Handle(CtxForTenant(tenantID), ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(k.started.runs) != 1 || k.started.runs[0] != r.ID+"|clarification:"+c.ID+"|clr-"+c.ID {
		t.Fatalf("runs %v", k.started.runs)
	}
	if n := k.env.CountRows(t, "processed_events", tenantID); n != 1 {
		t.Fatalf("%d processed markers", n)
	}
}

func clRounds(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) {
		r.ReporterID, r.Body = reporter, "ngắn"
		c, _ := domain.ContentFromRequest(*r)
		c.TypeFields = map[string]any{"repro_steps": []any{"a"}, "actual": "b", "expected": "c", "environment": "d", "severity": "high"}
		_, _ = c.AcceptanceCriteria.Add("tiêu chí", "test")
		*r, _ = r.WithContent(c)
	})
	clConfirm(t, k, ctx, r)
	for round := 1; round <= 3; round++ {
		c := clOpen(t, k, ctx, r.ID)
		if c.Round != round {
			t.Fatalf("round %d, want %d", c.Round, round)
		}
		var body usecase.AnswerItem
		for _, q := range c.Questions {
			if q.QuestionKey == "body" {
				body = usecase.AnswerItem{QuestionID: q.ID, Value: json.RawMessage(`"ok"`)}
			}
		}
		res, err := k.answer.Execute(ctx, usecase.AnswerInput{ClarificationID: c.ID, Answers: []usecase.AnswerItem{body}, Complete: true})
		if err != nil {
			t.Fatal(err)
		}
		if round < 3 && (!res.StillMissing || res.RequestStatus != domain.RequestStatusAwaitingInformation) {
			t.Fatalf("round %d: %+v", round, res)
		}
	}
	got, _ := k.env.Requests.Get(ctx, r.ID)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryMissingInfo || got.ReturnedFromStage != domain.ReturnStageClassification {
		t.Fatalf("after three rounds: %+v", got)
	}
	if c, _ := k.env.Clarifications.GetOpenByRequest(ctx, r.ID); c != nil {
		t.Fatal("no clarification stays open")
	}
}

func clParkedWithDeadline(t *testing.T, k *clarKit, tenantID string) (domain.Request, domain.Clarification, context.Context) {
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.ReporterID = reporter })
	// the kit clock decides due_at; pin it to "now" so a later Set can pass the deadline
	k.clock.Set(time.Now().UTC().Truncate(time.Microsecond))
	clConfirm(t, k, ctx, r)
	return r, clOpen(t, k, ctx, r.ID), ctx
}

func clExpiry(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	r, c, ctx := clParkedWithDeadline(t, k, tenantID)
	// The sweep is cross-tenant and the database is shared by the scenarios, so judge only this tenant's rows.
	k.clock.Set(c.DueAt.Add(-time.Minute))
	if _, err := k.sweeper.ExpireOnce(context.Background(), 500); err != nil {
		t.Fatal(err)
	}
	if stored, _ := k.env.Clarifications.Get(ctx, c.ID); stored.Status != domain.ClarificationStatusOpen {
		t.Fatalf("expired before its deadline: %+v", stored)
	}
	k.clock.Set(c.DueAt.Add(time.Minute))
	if _, err := k.answer.Execute(ctx, usecase.AnswerInput{ClarificationID: c.ID, Answers: clAnswerItems(c), Complete: true}); errCode(err) != "REQUEST_CLARIFICATION_EXPIRED" {
		t.Fatalf("a late answer before the sweep is refused lazily: %v", err)
	}
	// the sweep is cross-tenant, so only assert on this tenant's rows
	n, err := k.sweeper.ExpireOnce(context.Background(), 500)
	if err != nil || n < 1 {
		t.Fatalf("%d %v", n, err)
	}
	got, _ := k.env.Requests.Get(ctx, r.ID)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryMissingInfo || got.ReturnedFromStage != domain.ReturnStageClassification || got.ReturnReason != "clarification_expired" {
		t.Fatalf("%+v", got)
	}
	stored, _ := k.env.Clarifications.Get(ctx, c.ID)
	if stored.Status != domain.ClarificationStatusExpired {
		t.Fatalf("%+v", stored)
	}
	if indexOf(k.env.OutboxSubjects(t, tenantID), domain.SubjectClarificationExpired) < 0 {
		t.Fatal("no expired event")
	}
}

func clTwoSweepers(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	_, c, ctx := clParkedWithDeadline(t, k, tenantID)
	k.clock.Set(c.DueAt.Add(time.Hour))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = retryDeadlock(func() error { _, err := k.sweeper.ExpireOnce(context.Background(), 500); return err })
		}()
	}
	close(start)
	wg.Wait()
	events := 0
	for _, s := range k.env.OutboxSubjects(t, tenantID) {
		if s == domain.SubjectClarificationExpired {
			events++
		}
	}
	if events != 1 {
		t.Fatalf("handled %d times, want once", events)
	}
	if stored, _ := k.env.Clarifications.Get(ctx, c.ID); stored.Status != domain.ClarificationStatusExpired {
		t.Fatalf("%+v", stored)
	}
}

func clAnswerVsExpiry(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	r, c, ctx := clParkedWithDeadline(t, k, tenantID)
	// the answer lands just before the deadline; the sweep then finds nothing to expire
	k.clock.Set(c.DueAt.Add(-time.Second))
	if _, err := k.answer.Execute(ctx, usecase.AnswerInput{ClarificationID: c.ID, Answers: clAnswerItems(c), Complete: true}); err != nil {
		t.Fatal(err)
	}
	k.clock.Set(c.DueAt.Add(time.Hour))
	_, _ = k.sweeper.ExpireOnce(context.Background(), 500)
	got, _ := k.env.Requests.Get(ctx, r.ID)
	if got.Status != domain.RequestStatusAnalyzing {
		t.Fatalf("an answered clarification must not send the request back: %+v", got)
	}
	if stored, _ := k.env.Clarifications.Get(ctx, c.ID); stored.Status != domain.ClarificationStatusAnswered {
		t.Fatalf("%+v", stored)
	}
}

func clTypeChange(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.ReporterID = reporter })
	clConfirm(t, k, ctx, r)
	c := clOpen(t, k, ctx, r.ID)
	got, err := k.change.Execute(ctx, usecase.ChangeInput{RequestID: r.ID, NewType: "change_request", Reason: "thực ra là thay đổi", ActorID: reporter, ActorKind: domain.ActorKindUser})
	if err != nil || got.Status != domain.RequestStatusAwaitingTypeConfirmation {
		t.Fatalf("%+v %v", got, err)
	}
	stored, _ := k.env.Clarifications.Get(ctx, c.ID)
	if stored.Status != domain.ClarificationStatusCancelled || stored.CancelReason != "type_changed" {
		t.Fatalf("%+v", stored)
	}
	if open, _ := k.env.Clarifications.GetOpenByRequest(ctx, r.ID); open != nil {
		t.Fatal("still open")
	}
}

func clCancelRequest(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.ReporterID = reporter })
	clConfirm(t, k, ctx, r)
	c := clOpen(t, k, ctx, r.ID)
	if _, err := k.cancel.Execute(ctx, usecase.CancelInput{RequestID: r.ID, Reason: "bỏ", ActorID: reporter}); err != nil {
		t.Fatal(err)
	}
	if stored, _ := k.env.Clarifications.Get(ctx, c.ID); stored.Status != domain.ClarificationStatusCancelled || stored.CancelReason != "request_cancelled" {
		t.Fatalf("%+v", stored)
	}
}

func clWaive(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	sec := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.Type = domain.RequestTypeSecurity })
	_, err := k.waive.Execute(adminCtx(tenantID), usecase.WaiveInput{RequestID: sec.ID, Reason: "gấp"})
	requireCode(t, err, "REQUEST_READINESS_WAIVE_FORBIDDEN")
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.ReporterID = reporter })
	_, err = k.waive.Execute(ctx, usecase.WaiveInput{RequestID: r.ID, Reason: "x"})
	requireCode(t, err, "REQUEST_READINESS_WAIVE_FORBIDDEN")
	clConfirm(t, k, ctx, r)
	res, err := k.waive.Execute(adminCtx(tenantID), usecase.WaiveInput{RequestID: r.ID, Reason: "khách lớn"})
	if err != nil || res.Request.Status != domain.RequestStatusAnalyzing {
		t.Fatalf("%+v %v", res, err)
	}
	rev, err := k.env.Revisions.Get(ctx, r.ID, res.RequestRevision)
	if err != nil || rev.Cause != domain.RevisionCauseEdited {
		t.Fatalf("%+v %v", rev, err)
	}
	if ok, err := usecase.ReadinessWaived(ctx, k.env.Revisions, r.ID); err != nil || !ok {
		t.Fatalf("the waiver must be found after a round trip through JSON: %v %v", ok, err)
	}
}

func clPendingForUser(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter, member, roleUser, stranger := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	ctx := userIn(tenantID, reporter)
	mk := func(assignees []domain.Principal) domain.Clarification {
		r := clSeed(t, k, ctx, domain.RequestStatusAnalyzing, func(r *domain.Request) { r.ReporterID = reporter })
		c, err := k.clarify.Execute(adminCtx(tenantID), usecase.RequestClarificationInput{
			RequestID: r.ID, Source: domain.ClarificationSourceManual, Assignees: assignees, ActorKind: domain.ActorKindUser,
			Questions: []usecase.QuestionInput{{QuestionKey: "k", Kind: domain.QuestionKindText, Prompt: "p", Reason: "r", Required: true}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	forReporter := mk([]domain.Principal{{Kind: domain.PrincipalKindReporter}})
	forMember := mk([]domain.Principal{{Kind: domain.PrincipalKindUser, ID: member}})
	forTeam := mk([]domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "team-7"}})
	forRole := mk([]domain.Principal{{Kind: domain.PrincipalKindRole, ID: "reviewer"}})
	ids := func(f usecase.PendingFilter) map[string]bool {
		list, _, err := k.env.Clarifications.ListPendingForUser(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, c := range list {
			out[c.ID] = true
		}
		return out
	}
	if got := ids(usecase.PendingFilter{UserID: reporter}); len(got) != 1 || !got[forReporter.ID] {
		t.Fatalf("reporter: %v", got)
	}
	if got := ids(usecase.PendingFilter{UserID: member}); len(got) != 1 || !got[forMember.ID] {
		t.Fatalf("named user: %v", got)
	}
	if got := ids(usecase.PendingFilter{UserID: roleUser, Teams: []string{"team-7"}, Roles: []string{"reviewer"}}); len(got) != 2 || !got[forTeam.ID] || !got[forRole.ID] {
		t.Fatalf("team and role: %v", got)
	}
	if got := ids(usecase.PendingFilter{UserID: stranger}); len(got) != 0 {
		t.Fatalf("stranger: %v", got)
	}
	if got := ids(usecase.PendingFilter{UserID: stranger, IsAdmin: true}); len(got) != 4 {
		t.Fatalf("admin: %v", got)
	}
	if got := ids(usecase.PendingFilter{UserID: reporter, Teams: []string{"other"}}); len(got) != 1 {
		t.Fatalf("an unrelated team adds nothing: %v", got)
	}
	// keyset pagination is stable and complete
	seen := map[string]bool{}
	token := ""
	for page := 0; page < 10; page++ {
		list, next, err := k.env.Clarifications.ListPendingForUser(ctx, usecase.PendingFilter{UserID: stranger, IsAdmin: true, PageSize: 1, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range list {
			if seen[c.ID] {
				t.Fatalf("%s listed twice", c.ID)
			}
			seen[c.ID] = true
		}
		if next == "" {
			break
		}
		token = next
	}
	if len(seen) != 4 {
		t.Fatalf("paginated %d of 4", len(seen))
	}
	// another tenant sees none of it
	if list, _, _ := k.env.Clarifications.ListPendingForUser(CtxForTenant(newTenant()), usecase.PendingFilter{UserID: reporter, IsAdmin: true}); len(list) != 0 {
		t.Fatal("tenant leak")
	}
}

const contractSolutionDoc = `{"schema_version":1,"options":[
 {"id":"opt-1","title":"Nâng cấp từng bước","summary":"s","approach":"a","effort":{"size":"M","hours_estimate":8},"risk":{"level":"low","description":"d"},"affected_areas":[]},
 {"id":"opt-2","title":"Viết lại toàn bộ","summary":"s","approach":"a","breaking_change":true,"effort":{"size":"L","hours_estimate":80},"risk":{"level":"high","description":"d"},"affected_areas":[]}],
 "recommendation":{"option_id":"opt-1","reason":"ít rủi ro"}}`

func clDecisionFlow(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	chooser := uuid.NewString()
	ctx := userIn(tenantID, chooser)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingAnalysisApproval, nil)
	solution := uuid.NewString()
	record := func(option, rationale, digest string) (domain.Decision, error) {
		var out domain.Decision
		err := k.env.Tx.InTx(ctx, func(txCtx context.Context) error {
			d, err := k.recorder.Execute(txCtx, usecase.RecordDecisionInput{
				RequestID: r.ID, RequestNumber: r.Number, SolutionID: solution, OptionsJSON: []byte(contractSolutionDoc), ChosenOption: option,
				ChooserID: chooser, Rationale: rationale, SubjectDigest: digest, ReporterID: r.ReporterID, SelfChoiceAllowed: false,
			})
			out = d
			return err
		})
		return out, err
	}
	if err := k.gates.CheckDecision(ctx, solution, "d1"); errCode(err) != "REQUEST_DECISION_NOT_EFFECTIVE" {
		t.Fatalf("no decision yet: %v", err)
	}
	_, err := record("opt-2", "", "d1")
	requireCode(t, err, "REQUEST_DECISION_RATIONALE_REQUIRED")
	if n := k.env.CountRows(t, "decisions", tenantID); n != 0 {
		t.Fatalf("a refused choice left %d decision rows", n)
	}
	d, err := record("opt-2", "kiến trúc cũ hết đường", "d1")
	if err != nil || d.Status != domain.DecisionStatusChosen || d.RiskLevel != domain.RiskHigh {
		t.Fatalf("%+v %v", d, err)
	}
	requireCode(t, k.gates.CheckDecision(ctx, solution, "d1"), "REQUEST_DECISION_NOT_EFFECTIVE")
	_, err = k.confirmD.Execute(ctx, usecase.ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "Nâng cấp từng bước"})
	requireCode(t, err, "REQUEST_DECISION_CONFIRMATION_MISMATCH")
	confirmed, err := k.confirmD.Execute(ctx, usecase.ConfirmDecisionInput{DecisionID: d.ID, ConfirmationText: "  viết lại   TOÀN BỘ "})
	if err != nil || confirmed.Status != domain.DecisionStatusEffective || confirmed.ConfirmedBy != chooser {
		t.Fatalf("%+v %v", confirmed, err)
	}
	if err := k.gates.CheckDecision(ctx, solution, "d1"); err != nil {
		t.Fatalf("effective: %v", err)
	}
	requireCode(t, k.gates.CheckDecision(ctx, solution, "other"), "REQUEST_DECISION_NOT_EFFECTIVE")
	again, err := record("opt-1", "", "d2")
	if err != nil || again.ID != d.ID || again.ConfirmedBy != "" || again.Status != domain.DecisionStatusEffective {
		t.Fatalf("choosing again reuses the decision and clears the confirmation: %+v %v", again, err)
	}
	history, _ := k.env.Decisions.ListHistory(ctx, d.ID)
	var actions []domain.DecisionAction
	for _, h := range history {
		actions = append(actions, h.Action)
	}
	if len(actions) != 3 || actions[0] != domain.DecisionActionChosen || actions[1] != domain.DecisionActionConfirmed || actions[2] != domain.DecisionActionRechosen {
		t.Fatalf("history %v", actions)
	}
	list, _ := k.env.Decisions.List(ctx, usecase.DecisionListFilter{RequestID: r.ID, Limit: 10})
	if len(list) != 1 || list[0].Seq != 1 {
		t.Fatalf("%+v", list)
	}
}

func clDecisionLive(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	ctx := userIn(tenantID, uuid.NewString())
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingAnalysisApproval, nil)
	mk := func(status domain.DecisionStatus, seq int) domain.Decision {
		return domain.Decision{
			ID: uuid.NewString(), TenantID: tenantID, RequestID: r.ID, Seq: seq, SubjectKind: domain.DecisionSubjectSolutionOption, SubjectID: "sol-live",
			Options: []domain.DecisionOption{{ID: "opt-1", Label: "A"}}, Status: status, RiskLevel: domain.RiskNormal, Version: 1, CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		}
	}
	first := mk(domain.DecisionStatusEffective, 1)
	if err := k.env.Decisions.Insert(ctx, first); err != nil {
		t.Fatal(err)
	}
	for _, st := range []domain.DecisionStatus{domain.DecisionStatusOpen, domain.DecisionStatusChosen, domain.DecisionStatusEffective} {
		if err := k.env.Decisions.Insert(ctx, mk(st, 2)); errCode(err) != "REQUEST_DECISION_STATE_INVALID" {
			t.Fatalf("a second live decision (%s) must be refused: %v", st, err)
		}
	}
	superseded, err := k.env.Decisions.SupersedeLiveBySubject(ctx, domain.DecisionSubjectSolutionOption, "sol-live")
	if err != nil || len(superseded) != 1 || superseded[0].Status != domain.DecisionStatusSuperseded {
		t.Fatalf("%+v %v", superseded, err)
	}
	if err := k.env.Decisions.Insert(ctx, mk(domain.DecisionStatusOpen, 2)); err != nil {
		t.Fatalf("after superseding, a new decision fits: %v", err)
	}
	if again, _ := k.env.Decisions.SupersedeLiveByRequest(ctx, r.ID); len(again) != 1 {
		t.Fatalf("by request: %+v", again)
	}
	_, err = k.env.Decisions.Update(ctx, first, 99)
	requireCode(t, err, "REQUEST_DECISION_VERSION_CONFLICT")
}

func clAnswerRollback(t *testing.T, k *clarKit) {
	tenantID := newTenant()
	reporter := uuid.NewString()
	ctx := userIn(tenantID, reporter)
	r := clSeed(t, k, ctx, domain.RequestStatusAwaitingTypeConfirmation, func(r *domain.Request) { r.ReporterID = reporter })
	clConfirm(t, k, ctx, r)
	c := clOpen(t, k, ctx, r.ID)
	failing := usecase.NewAnswerClarification(k.env.Requests, k.env.Clarifications, k.appendRv, failTransition{}, k.ret, k.clarify, k.env.Revisions, k.env.Decisions, k.env.Tx, k.env.Outbox, 3).WithClock(k.clock.Now)
	if _, err := failing.Execute(ctx, usecase.AnswerInput{ClarificationID: c.ID, Answers: clAnswerItems(c), Complete: true}); err == nil {
		t.Fatal("expected failure")
	}
	got, _ := k.env.Requests.Get(ctx, r.ID)
	stored, _ := k.env.Clarifications.Get(ctx, c.ID)
	if got.ContentRevision != 1 || got.Status != domain.RequestStatusAwaitingInformation || stored.Status != domain.ClarificationStatusOpen {
		t.Fatalf("not rolled back: %+v / %+v", got, stored)
	}
	for _, q := range stored.Questions {
		if q.HasAnswer() {
			t.Fatalf("question %s kept its answer", q.QuestionKey)
		}
	}
	if _, err := k.env.Revisions.Get(ctx, r.ID, 2); errCode(err) != "REQUEST_REVISION_NOT_FOUND" {
		t.Fatalf("revision 2 must not exist: %v", err)
	}
}

type failTransition struct{}

func (failTransition) Execute(context.Context, usecase.TransitionInput) (usecase.TransitionResult, error) {
	return usecase.TransitionResult{}, fmt.Errorf("forced transition failure")
}
