package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReadinessGate decides, right after a person confirmed the type, whether the request is ready for analysis
// or must first collect missing information (CR-REQ-028 section 2.3).
type ReadinessGate struct {
	repo           RequestRepository
	clarify        *RequestClarification
	clarifications ClarificationRepository
	revisions      RequestRevisionRepository
	transition     RequestTransitioner
	maxRounds      int
}

func NewReadinessGate(repo RequestRepository, clarify *RequestClarification, clarifications ClarificationRepository,
	revisions RequestRevisionRepository, transition RequestTransitioner, maxRounds int) *ReadinessGate {
	if maxRounds <= 0 {
		maxRounds = domain.DefaultMaxClarificationRounds
	}
	return &ReadinessGate{repo: repo, clarify: clarify, clarifications: clarifications, revisions: revisions, transition: transition, maxRounds: maxRounds}
}

// GateOutcome.Handled is false for a ready request: the caller continues with type_confirmed as before.
type GateOutcome struct {
	Request domain.Request
	Handled bool
}

func (g *ReadinessGate) GateTypeConfirmed(ctx context.Context, r domain.Request, actorID string) (GateOutcome, error) {
	content, err := domain.ContentFromRequest(r)
	if err != nil {
		return GateOutcome{}, err
	}
	report := domain.ReadinessPolicy{}.Evaluate(r.Type, content)
	if report.Ready {
		return GateOutcome{}, nil
	}
	if waived, err := ReadinessWaived(ctx, g.revisions, r.ID); err != nil {
		return GateOutcome{}, err
	} else if waived {
		return GateOutcome{}, nil
	}
	prior, err := g.clarifications.MaxRound(ctx, r.ID, domain.ClarificationSourceReadiness, "")
	if err != nil {
		return GateOutcome{}, err
	}
	questions := domain.QuestionBuilder{}.Build(report, domain.ReadinessHints{Title: r.Title, Urgency: r.Urgency, Size: r.Size, SourceHints: r.SourceHints})
	if prior >= g.maxRounds || len(questions) == 0 {
		out, err := g.returnToBacklog(ctx, r, report)
		return GateOutcome{Request: out, Handled: true}, err
	}
	if _, err := g.clarify.CreateWithinTx(ctx, RequestClarificationInput{
		RequestID: r.ID, Source: domain.ClarificationSourceReadiness, Questions: QuestionInputsFrom(questions),
		Assignees: []domain.Principal{{Kind: domain.PrincipalKindReporter}}, ActorID: actorID, ActorKind: domain.ActorKindSystem,
	}); err != nil {
		return GateOutcome{}, err
	}
	out, err := g.repo.Get(ctx, r.ID)
	return GateOutcome{Request: out, Handled: true}, err
}

func (g *ReadinessGate) returnToBacklog(ctx context.Context, r domain.Request, report domain.ReadinessReport) (domain.Request, error) {
	var paths []string
	for _, m := range report.Blocker() {
		paths = append(paths, m.Path)
	}
	from := r.Status
	res, err := g.transition.Execute(ctx, TransitionInput{
		RequestID: r.ID, Trigger: domain.TriggerReturnToBacklog, ExpectedFrom: &from, ActorKind: domain.ActorKindSystem,
		Stage: domain.ReturnStageClassification, Category: domain.ReturnCategoryMissingInfo,
		Reason: "thiếu thông tin: " + strings.Join(paths, ", "),
	})
	if err != nil {
		return domain.Request{}, err
	}
	return res.Request, nil
}

// ReadinessWaived reports whether an admin waived the Definition of Ready for this request; the waiver
// lives in a revision's meta so it needs no column of its own.
func ReadinessWaived(ctx context.Context, revisions RequestRevisionRepository, requestID string) (bool, error) {
	after := 0
	for {
		batch, err := revisions.List(ctx, requestID, after, 200)
		if err != nil {
			return false, err
		}
		for _, rev := range batch {
			var snap struct {
				Meta struct {
					Waiver struct {
						Kind string `json:"kind"`
					} `json:"waiver"`
				} `json:"meta"`
			}
			if json.Unmarshal(rev.Snapshot, &snap) == nil && snap.Meta.Waiver.Kind == "readiness" {
				return true, nil
			}
			after = rev.Revision
		}
		if len(batch) < 200 {
			return false, nil
		}
	}
}

// GetRequestReadiness is the read-only "what is still missing" view; it creates nothing.
type GetRequestReadiness struct {
	repo RequestRepository
}

func NewGetRequestReadiness(repo RequestRepository) *GetRequestReadiness {
	return &GetRequestReadiness{repo: repo}
}

type ReadinessView struct {
	Report          domain.ReadinessReport
	ContentRevision int
}

func (uc *GetRequestReadiness) Execute(ctx context.Context, requestID string) (ReadinessView, error) {
	r, err := loadReadableRequest(ctx, uc.repo, requestID)
	if err != nil {
		return ReadinessView{}, err
	}
	if r.Type == "" {
		return ReadinessView{}, domain.ErrClarificationStateNotAllowed(fmt.Sprintf("request %s has no type yet", r.ID))
	}
	c, err := domain.ContentFromRequest(r)
	if err != nil {
		return ReadinessView{}, err
	}
	return ReadinessView{Report: domain.ReadinessPolicy{}.Evaluate(r.Type, c), ContentRevision: r.ContentRevision}, nil
}
