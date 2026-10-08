package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// QuestionInput is one question as a caller supplies it; ids and sequence are assigned here.
type QuestionInput struct {
	QuestionKey      string
	Kind             domain.QuestionKind
	Prompt           string
	Reason           string
	Options          []domain.QuestionOption
	SuggestedDefault json.RawMessage
	Required         bool
	TargetPath       string
}

type RequestClarificationInput struct {
	RequestID string
	Source    domain.ClarificationSource
	SourceRef string
	Questions []QuestionInput
	// Assignees defaults to the reporter.
	Assignees []domain.Principal
	ActorID   string
	ActorKind domain.ActorKind
}

// ClarificationDeadlines decides when a clarification expires; the default wraps domain.DefaultDue.
type ClarificationDeadlines interface {
	DueAt(source domain.ClarificationSource, urgency domain.Urgency, now time.Time) time.Time
}

type defaultDeadlines struct{}

func (defaultDeadlines) DueAt(s domain.ClarificationSource, u domain.Urgency, now time.Time) time.Time {
	return domain.DefaultDue(s, u, now)
}

// RequestClarification parks a request in awaiting_information behind one open Clarification.
// Every source (readiness, open question, assumption, blocked task, manual) enters through CreateWithinTx.
type RequestClarification struct {
	repo           RequestRepository
	clarifications ClarificationRepository
	transition     RequestTransitioner
	canceller      ApprovalCanceller
	tx             TxRunner
	outbox         OutboxWriter
	recipients     RecipientResolver
	deadlines      ClarificationDeadlines
	clock          func() time.Time
}

func NewRequestClarification(repo RequestRepository, clarifications ClarificationRepository, transition RequestTransitioner,
	canceller ApprovalCanceller, tx TxRunner, outbox OutboxWriter) *RequestClarification {
	return &RequestClarification{
		repo: repo, clarifications: clarifications, transition: transition, canceller: canceller, tx: tx, outbox: outbox,
		recipients: PrincipalRecipients{}, deadlines: defaultDeadlines{}, clock: func() time.Time { return time.Now().UTC() },
	}
}

func (uc *RequestClarification) WithRecipients(r RecipientResolver) *RequestClarification {
	uc.recipients = r
	return uc
}

func (uc *RequestClarification) WithDeadlines(d ClarificationDeadlines) *RequestClarification {
	uc.deadlines = d
	return uc
}

func (uc *RequestClarification) WithClock(c func() time.Time) *RequestClarification {
	uc.clock = c
	return uc
}

func (uc *RequestClarification) Execute(ctx context.Context, in RequestClarificationInput) (domain.Clarification, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Clarification{}, domain.ErrRequestTenantRequired()
	}
	var out domain.Clarification
	err := uc.tx.InTx(ctx, func(txCtx context.Context) error {
		c, err := uc.CreateWithinTx(txCtx, in)
		out = c
		return err
	})
	return out, err
}

// CreateWithinTx joins the caller's transaction.
func (uc *RequestClarification) CreateWithinTx(ctx context.Context, in RequestClarificationInput) (domain.Clarification, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Clarification{}, domain.ErrRequestTenantRequired()
	}
	if _, err := domain.ParseClarificationSource(string(in.Source)); err != nil {
		return domain.Clarification{}, err
	}
	if in.ActorKind == "" {
		in.ActorKind = domain.ActorKindUser
	}
	r, err := uc.repo.Get(ctx, in.RequestID)
	if err != nil {
		return domain.Clarification{}, err
	}
	if err := uc.authorize(ctx, r, in); err != nil {
		return domain.Clarification{}, err
	}
	questions, err := buildQuestions(in.Questions)
	if err != nil {
		return domain.Clarification{}, err
	}

	// An open clarification with the same source, reference and question keys is the same ask (retry or double click).
	if open, err := uc.clarifications.GetOpenByRequest(ctx, r.ID); err != nil {
		return domain.Clarification{}, err
	} else if open != nil {
		if open.Source == in.Source && open.SourceRef == in.SourceRef && sameQuestionKeys(open.Questions, questions) {
			return *open, nil
		}
		return domain.Clarification{}, domain.ErrClarificationStateNotAllowed("this request already has an open clarification")
	}

	if r.Type == "" {
		return domain.Clarification{}, domain.ErrClarificationStateNotAllowed("the request type is not set yet")
	}
	flow, err := domain.FlowFor(r.Type)
	if err != nil {
		return domain.Clarification{}, err
	}
	resume, err := domain.ResumeStatusFor(flow, in.Source, r.Status)
	if err != nil {
		return domain.Clarification{}, err
	}
	seq, err := uc.clarifications.NextSeq(ctx, r.ID)
	if err != nil {
		return domain.Clarification{}, err
	}
	prior, err := uc.clarifications.MaxRound(ctx, r.ID, in.Source, in.SourceRef)
	if err != nil {
		return domain.Clarification{}, err
	}
	now := uc.clock()
	assignees := in.Assignees
	if len(assignees) == 0 {
		assignees = []domain.Principal{{Kind: domain.PrincipalKindReporter}}
	}
	c := domain.Clarification{
		ID: uuid.NewString(), TenantID: r.TenantID, RequestID: r.ID, Seq: seq, Source: in.Source, SourceRef: in.SourceRef,
		Status: domain.ClarificationStatusOpen, ResumeStatus: resume, Round: prior + 1, AskedRequestRevision: r.ContentRevision,
		DueAt: uc.deadlines.DueAt(in.Source, r.Urgency, now), CreatedBy: createdByOf(in), CreatedAt: now, Version: 1,
		Questions: questions, Assignees: assignees,
	}
	// A pending approval would otherwise outlive the facts it was asked about.
	if err := uc.canceller.CancelPending(ctx, r.ID, "information_required"); err != nil {
		return domain.Clarification{}, err
	}
	if err := uc.clarifications.Insert(ctx, c); err != nil {
		return domain.Clarification{}, err
	}
	from := r.Status
	if _, err := uc.transition.Execute(ctx, TransitionInput{
		RequestID: r.ID, Trigger: domain.TriggerInformationRequired, ExpectedFrom: &from,
		ActorID: in.ActorID, ActorKind: in.ActorKind, Reason: fmt.Sprintf("clarification:%d", seq),
	}); err != nil {
		return domain.Clarification{}, err
	}
	users, err := uc.recipients.Resolve(ctx, r, assignees)
	if err != nil {
		return domain.Clarification{}, err
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectClarificationRequested, clarificationRequestedNotice(r, c, users, false))
	if err != nil {
		return domain.Clarification{}, err
	}
	ev.OccurredAt = now
	if err := uc.outbox.InsertOutboxEvent(ctx, ev); err != nil {
		return domain.Clarification{}, err
	}
	return c, nil
}

func createdByOf(in RequestClarificationInput) string {
	if in.ActorKind == domain.ActorKindSystem || in.ActorID == "" {
		return "system"
	}
	return in.ActorID
}

// authorize: system sources come from the platform; the others from a person who may write the request
// (interim rule, README v6 section 8 leaves request-level permissions open).
func (uc *RequestClarification) authorize(ctx context.Context, r domain.Request, in RequestClarificationInput) error {
	if in.ActorKind == domain.ActorKindSystem {
		return nil
	}
	if in.Source.SystemOnly() {
		return errRequestForbidden(fmt.Sprintf("a %s clarification is raised by the platform, not by a caller", in.Source))
	}
	if callerIsMachine(ctx) || !canWriteRequest(ctx, r) {
		return errRequestForbidden("only the reporter or an admin may ask for more information")
	}
	return nil
}

func buildQuestions(in []QuestionInput) ([]domain.ClarificationQuestion, error) {
	if len(in) < 1 || len(in) > domain.MaxQuestions {
		return nil, domain.ErrClarificationInvalidAnswer("", fmt.Sprintf("a clarification needs 1..%d questions", domain.MaxQuestions))
	}
	seen := map[string]bool{}
	out := make([]domain.ClarificationQuestion, 0, len(in))
	for i, q := range in {
		cq := domain.ClarificationQuestion{
			ID: uuid.NewString(), Seq: i + 1, QuestionKey: strings.TrimSpace(q.QuestionKey), Kind: q.Kind,
			Prompt: strings.TrimSpace(domain.NormalizeNFC(q.Prompt)), Reason: strings.TrimSpace(domain.NormalizeNFC(q.Reason)),
			Options: q.Options, SuggestedDefault: q.SuggestedDefault, Required: q.Required, TargetPath: q.TargetPath,
		}
		if err := domain.ValidateQuestion(cq); err != nil {
			return nil, err
		}
		if seen[cq.QuestionKey] {
			return nil, domain.ErrClarificationInvalidAnswer(cq.QuestionKey, "question_key must be unique")
		}
		seen[cq.QuestionKey] = true
		out = append(out, cq)
	}
	return out, nil
}

func sameQuestionKeys(a, b []domain.ClarificationQuestion) bool {
	if len(a) != len(b) {
		return false
	}
	keys := map[string]bool{}
	for _, q := range a {
		keys[q.QuestionKey] = true
	}
	for _, q := range b {
		if !keys[q.QuestionKey] {
			return false
		}
	}
	return true
}

// QuestionInputsFrom turns built readiness questions back into inputs for CreateWithinTx.
func QuestionInputsFrom(qs []domain.ClarificationQuestion) []QuestionInput {
	out := make([]QuestionInput, len(qs))
	for i, q := range qs {
		out[i] = QuestionInput{QuestionKey: q.QuestionKey, Kind: q.Kind, Prompt: q.Prompt, Reason: q.Reason, Options: q.Options,
			SuggestedDefault: q.SuggestedDefault, Required: q.Required, TargetPath: q.TargetPath}
	}
	return out
}

// CreateFollowUp opens the next round of a readiness chain while the request stays in awaiting_information:
// the previous clarification is already answered, so the one-open index admits the new one, and no transition happens.
func (uc *RequestClarification) CreateFollowUp(ctx context.Context, r domain.Request, prev domain.Clarification, questions []QuestionInput) (domain.Clarification, error) {
	built, err := buildQuestions(questions)
	if err != nil {
		return domain.Clarification{}, err
	}
	seq, err := uc.clarifications.NextSeq(ctx, r.ID)
	if err != nil {
		return domain.Clarification{}, err
	}
	now := uc.clock()
	c := domain.Clarification{
		ID: uuid.NewString(), TenantID: r.TenantID, RequestID: r.ID, Seq: seq, Source: prev.Source, SourceRef: prev.SourceRef,
		Status: domain.ClarificationStatusOpen, ResumeStatus: prev.ResumeStatus, Round: prev.Round + 1, AskedRequestRevision: r.ContentRevision,
		DueAt: uc.deadlines.DueAt(prev.Source, r.Urgency, now), CreatedBy: "system", CreatedAt: now, Version: 1,
		Questions: built, Assignees: prev.Assignees,
	}
	if err := uc.clarifications.Insert(ctx, c); err != nil {
		return domain.Clarification{}, err
	}
	users, err := uc.recipients.Resolve(ctx, r, c.Assignees)
	if err != nil {
		return domain.Clarification{}, err
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectClarificationRequested, clarificationRequestedNotice(r, c, users, false))
	if err != nil {
		return domain.Clarification{}, err
	}
	ev.OccurredAt = now
	return c, uc.outbox.InsertOutboxEvent(ctx, ev)
}
