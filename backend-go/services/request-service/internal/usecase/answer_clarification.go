package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type AnswerItem struct {
	QuestionID    string
	Value         json.RawMessage
	AcceptDefault bool
}

type AnswerInput struct {
	ClarificationID string
	Answers         []AnswerItem
	// Complete=false saves a draft; true closes the round.
	Complete        bool
	ExpectedVersion int64
}

type AnswerResult struct {
	Clarification   domain.Clarification
	RequestStatus   domain.RequestStatus
	RequestRevision int
	StillMissing    bool
}

// SolutionSuperseder retires the proposed Solutions of a request whose facts just changed. Approved ones are
// immutable and stay. The default does nothing; the solution feature wires the real one.
type SolutionSuperseder interface {
	SupersedeProposedByRequest(ctx context.Context, requestID string) error
}

type noSolutionSuperseder struct{}

func (noSolutionSuperseder) SupersedeProposedByRequest(context.Context, string) error { return nil }

// AnswerClarification records answers. A completed round writes a new Request revision, supersedes what was
// derived from the older facts, re-checks readiness and moves the request on, all in one transaction.
type AnswerClarification struct {
	repo           RequestRepository
	clarifications ClarificationRepository
	appendRevision *AppendRequestRevision
	transition     RequestTransitioner
	returner       *ReturnRequestToBacklog
	clarify        *RequestClarification
	revisions      RequestRevisionRepository
	decisions      DecisionRepository
	solutions      SolutionSuperseder
	teams          TeamMembershipResolver
	recipients     RecipientResolver
	tx             TxRunner
	outbox         OutboxWriter
	maxRounds      int
	clock          func() time.Time
}

func NewAnswerClarification(repo RequestRepository, clarifications ClarificationRepository, appendRevision *AppendRequestRevision,
	transition RequestTransitioner, returner *ReturnRequestToBacklog, clarify *RequestClarification, revisions RequestRevisionRepository,
	decisions DecisionRepository, tx TxRunner, outbox OutboxWriter, maxRounds int) *AnswerClarification {
	if maxRounds <= 0 {
		maxRounds = domain.DefaultMaxClarificationRounds
	}
	return &AnswerClarification{
		repo: repo, clarifications: clarifications, appendRevision: appendRevision, transition: transition, returner: returner,
		clarify: clarify, revisions: revisions, decisions: decisions, solutions: noSolutionSuperseder{}, tx: tx, outbox: outbox,
		maxRounds: maxRounds, clock: func() time.Time { return time.Now().UTC() },
	}
}

func (uc *AnswerClarification) WithSolutions(s SolutionSuperseder) *AnswerClarification {
	uc.solutions = s
	return uc
}

func (uc *AnswerClarification) WithTeams(t TeamMembershipResolver) *AnswerClarification {
	uc.teams = t
	return uc
}

func (uc *AnswerClarification) WithClock(c func() time.Time) *AnswerClarification {
	uc.clock = c
	return uc
}

func (uc *AnswerClarification) Execute(ctx context.Context, in AnswerInput) (AnswerResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return AnswerResult{}, domain.ErrRequestTenantRequired()
	}
	// There is no reliable way yet to tell a machine from a person by token; ActorType is the best signal available.
	if callerIsMachine(ctx) {
		return AnswerResult{}, domain.ErrClarificationNotAssignee()
	}
	var out AnswerResult
	err := uc.tx.InTx(ctx, func(txCtx context.Context) error {
		res, err := uc.answer(txCtx, in)
		out = res
		return err
	})
	return out, err
}

func (uc *AnswerClarification) answer(ctx context.Context, in AnswerInput) (AnswerResult, error) {
	c, err := uc.clarifications.Get(ctx, in.ClarificationID)
	if err != nil {
		return AnswerResult{}, err
	}
	r, err := uc.repo.Get(ctx, c.RequestID)
	if err != nil {
		return AnswerResult{}, err
	}
	now := uc.clock()
	switch c.Status {
	case domain.ClarificationStatusAnswered:
		if uc.sameAsStored(c, in) {
			return AnswerResult{Clarification: c, RequestStatus: r.Status, RequestRevision: derefInt(c.AnsweredRequestRevision)}, nil
		}
		return AnswerResult{}, domain.ErrClarificationAlreadyAnswered(c.ID)
	case domain.ClarificationStatusOpen:
		if c.IsExpired(now) {
			return AnswerResult{}, domain.ErrClarificationExpired(c.ID) // lazy expiry: the sweep has not caught it yet
		}
	default:
		return AnswerResult{}, domain.ErrClarificationNotOpen(c.ID, c.Status)
	}
	if !uc.canAnswer(ctx, r, c) {
		return AnswerResult{}, domain.ErrClarificationNotAssignee()
	}
	records, err := resolveAnswers(c, in.Answers, callerID(ctx), now)
	if err != nil {
		return AnswerResult{}, err
	}
	merged := mergeAnswers(c.Questions, records)
	missing := missingRequired(merged)

	if !in.Complete {
		if len(records) > 0 {
			if err := uc.clarifications.UpdateAnswers(ctx, c.ID, records, in.ExpectedVersion); err != nil {
				return AnswerResult{}, err
			}
		}
		fresh, err := uc.clarifications.Get(ctx, c.ID)
		return AnswerResult{Clarification: fresh, RequestStatus: r.Status, RequestRevision: r.ContentRevision, StillMissing: len(missing) > 0}, err
	}
	if len(missing) > 0 {
		return AnswerResult{}, domain.ErrClarificationIncomplete(missing)
	}
	if len(records) > 0 || in.ExpectedVersion != 0 {
		if err := uc.clarifications.UpdateAnswers(ctx, c.ID, records, in.ExpectedVersion); err != nil {
			return AnswerResult{}, err
		}
	}
	content, err := domain.ContentFromRequest(r)
	if err != nil {
		return AnswerResult{}, err
	}
	updatedContent, err := domain.ApplyAnswers(content, merged)
	if err != nil {
		return AnswerResult{}, err
	}
	rev, err := uc.appendRevision.AppendWithinTx(ctx, AppendInput{
		RequestID: r.ID, Content: updatedContent, Cause: domain.RevisionCauseClarificationAnswered, ClarificationID: c.ID,
		ActorID: callerID(ctx), ActorKind: domain.ActorKindUser, ExpectedVersion: r.Version,
	})
	if err != nil {
		return AnswerResult{}, err
	}
	newRevision := rev.Request.ContentRevision
	if err := uc.clarifications.MarkAnswered(ctx, c.ID, newRevision, now, 0); err != nil {
		return AnswerResult{}, err
	}

	stillMissing := false
	status := r.Status
	waived := false
	if c.Source == domain.ClarificationSourceReadiness {
		if waived, err = ReadinessWaived(ctx, uc.revisions, r.ID); err != nil {
			return AnswerResult{}, err
		}
		report := domain.ReadinessPolicy{}.Evaluate(r.Type, updatedContent)
		stillMissing = !report.Ready && !waived
		if stillMissing {
			if status, err = uc.nextRoundOrBacklog(ctx, rev.Request, c, report); err != nil {
				return AnswerResult{}, err
			}
		}
	}
	if !stillMissing {
		if c.ResumeStatus == domain.RequestStatusAnalyzing || c.ResumeStatus == domain.RequestStatusPlanning {
			if err := uc.supersedeDerived(ctx, r.ID, now); err != nil {
				return AnswerResult{}, err
			}
		}
		from := domain.RequestStatusAwaitingInformation
		tr, err := uc.transition.Execute(ctx, TransitionInput{
			RequestID: r.ID, Trigger: domain.TriggerInformationProvided, ExpectedFrom: &from, ResumeStatus: c.ResumeStatus,
			ActorID: callerID(ctx), ActorKind: domain.ActorKindUser,
		})
		if err != nil {
			return AnswerResult{}, err
		}
		status = tr.Request.Status
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectClarificationAnswered, answeredPayload{
		ClarificationID: c.ID, RequestID: r.ID, DisplayID: c.DisplayID(r.Number), Revision: newRevision, ResumeStatus: string(c.ResumeStatus), StillMissing: stillMissing,
	})
	if err != nil {
		return AnswerResult{}, err
	}
	ev.OccurredAt = now
	if err := uc.outbox.InsertOutboxEvent(ctx, ev); err != nil {
		return AnswerResult{}, err
	}
	fresh, err := uc.clarifications.Get(ctx, c.ID)
	return AnswerResult{Clarification: fresh, RequestStatus: status, RequestRevision: newRevision, StillMissing: stillMissing}, err
}

type answeredPayload struct {
	ClarificationID string `json:"clarification_id"`
	RequestID       string `json:"request_id"`
	DisplayID       string `json:"display_id"`
	Revision        int    `json:"revision"`
	ResumeStatus    string `json:"resume_status"`
	StillMissing    bool   `json:"still_missing"`
}

// nextRoundOrBacklog handles a round that left the readiness gaps open: ask again, or give up after the last round.
func (uc *AnswerClarification) nextRoundOrBacklog(ctx context.Context, r domain.Request, prev domain.Clarification, report domain.ReadinessReport) (domain.RequestStatus, error) {
	questions := domain.QuestionBuilder{}.Build(report, domain.ReadinessHints{Title: r.Title, Urgency: r.Urgency, Size: r.Size, SourceHints: r.SourceHints})
	if prev.Round < uc.maxRounds && len(questions) > 0 {
		if _, err := uc.clarify.CreateFollowUp(ctx, r, prev, QuestionInputsFrom(questions)); err != nil {
			return "", err
		}
		return domain.RequestStatusAwaitingInformation, nil
	}
	var paths []string
	for _, m := range report.Blocker() {
		paths = append(paths, m.Path)
	}
	back, err := uc.returner.Execute(ctx, ReturnInput{
		RequestID: r.ID, Stage: domain.ReturnStageForResume(prev.Source, prev.ResumeStatus), Category: domain.ReturnCategoryMissingInfo,
		Reason: "thiếu thông tin sau " + itoa(prev.Round) + " vòng: " + strings.Join(paths, ", "), ActorKind: domain.ActorKindSystem,
	})
	if err != nil {
		return "", err
	}
	return back.Status, nil
}

func (uc *AnswerClarification) supersedeDerived(ctx context.Context, requestID string, now time.Time) error {
	if err := uc.solutions.SupersedeProposedByRequest(ctx, requestID); err != nil {
		return err
	}
	superseded, err := uc.decisions.SupersedeLiveByRequest(ctx, requestID)
	if err != nil {
		return err
	}
	for _, d := range superseded {
		if err := uc.decisions.AppendHistory(ctx, domain.DecisionHistory{
			ID: uuid.NewString(), DecisionID: d.ID, Action: domain.DecisionActionSuperseded, OptionID: d.ChosenOptionID, ActorID: callerID(ctx), At: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (uc *AnswerClarification) canAnswer(ctx context.Context, r domain.Request, c domain.Clarification) bool {
	if callerIsAdmin(ctx) {
		return true
	}
	me := callerID(ctx)
	if me == "" {
		return false
	}
	role, _ := tenant.Role(ctx)
	for _, a := range c.Assignees {
		switch a.Kind {
		case domain.PrincipalKindUser:
			if a.ID == me {
				return true
			}
		case domain.PrincipalKindReporter:
			if r.ReporterID == me {
				return true
			}
		case domain.PrincipalKindRole:
			if role != "" && a.ID == role {
				return true
			}
		case domain.PrincipalKindTeam:
			if uc.teams == nil {
				continue
			}
			if members, err := uc.teams.MembersOfTeam(ctx, a.ID); err == nil {
				for _, m := range members {
					if m == me {
						return true
					}
				}
			}
		}
	}
	return false
}

// resolveAnswers validates the submitted items against their questions and returns the records to store.
func resolveAnswers(c domain.Clarification, items []AnswerItem, by string, now time.Time) ([]AnswerRecord, error) {
	byID := map[string]domain.ClarificationQuestion{}
	for _, q := range c.Questions {
		byID[q.ID] = q
	}
	var out []AnswerRecord
	seen := map[string]bool{}
	for _, it := range items {
		q, ok := byID[it.QuestionID]
		if !ok {
			return nil, domain.ErrClarificationInvalidAnswer(it.QuestionID, "no such question in this clarification")
		}
		if seen[q.ID] {
			return nil, domain.ErrClarificationInvalidAnswer(q.QuestionKey, "answered twice in one request")
		}
		seen[q.ID] = true
		rec := AnswerRecord{QuestionID: q.ID, By: by, At: now, Source: domain.AnswerSourceUser}
		if it.AcceptDefault {
			if len(q.SuggestedDefault) == 0 {
				return nil, domain.ErrClarificationInvalidAnswer(q.QuestionKey, "this question has no suggested default to accept")
			}
			rec.Value, rec.Source = q.SuggestedDefault, domain.AnswerSourceDefaultAccepted
		} else {
			rec.Value = it.Value
		}
		if err := domain.ValidateAnswer(q, rec.Value); err != nil {
			return nil, err
		}
		canon, err := domain.CanonicalJSON(rec.Value)
		if err != nil {
			return nil, domain.ErrClarificationInvalidAnswer(q.QuestionKey, "the answer is not valid JSON")
		}
		rec.Value = canon
		out = append(out, rec)
	}
	return out, nil
}

func mergeAnswers(qs []domain.ClarificationQuestion, recs []AnswerRecord) []domain.ClarificationQuestion {
	byID := map[string]AnswerRecord{}
	for _, r := range recs {
		byID[r.QuestionID] = r
	}
	out := make([]domain.ClarificationQuestion, len(qs))
	copy(out, qs)
	for i := range out {
		if r, ok := byID[out[i].ID]; ok {
			out[i].Answer, out[i].AnswerSource, out[i].AnsweredBy = r.Value, r.Source, r.By
		}
	}
	return out
}

func missingRequired(qs []domain.ClarificationQuestion) []string {
	var keys []string
	for _, q := range qs {
		if q.Required && !q.HasAnswer() {
			keys = append(keys, q.QuestionKey)
		}
	}
	return keys
}

// sameAsStored is the redelivery check: the submitted answers equal what the answered clarification holds.
func (uc *AnswerClarification) sameAsStored(c domain.Clarification, in AnswerInput) bool {
	if !in.Complete {
		return false
	}
	stored := map[string]json.RawMessage{}
	for _, q := range c.Questions {
		stored[q.ID] = q.Answer
	}
	for _, it := range in.Answers {
		want := it.Value
		if it.AcceptDefault {
			for _, q := range c.Questions {
				if q.ID == it.QuestionID {
					want = q.SuggestedDefault
				}
			}
		}
		canon, err := domain.CanonicalJSON(want)
		if err != nil || !bytes.Equal(canon, stored[it.QuestionID]) {
			return false
		}
	}
	return true
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func itoa(n int) string { return strconv.Itoa(n) }
