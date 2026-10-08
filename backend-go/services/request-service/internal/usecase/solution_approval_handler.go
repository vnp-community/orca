package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// analysisApprovalHandler is the SubjectHandler behind solution, findings and answer approvals. They differ only in
// which Solution kinds they cover and whether a choice is required, so the constructors below share one body.
type analysisApprovalHandler struct {
	subject       domain.SubjectType
	kinds         []domain.SolutionKind
	requireChoice bool
	requests      RequestRepository
	solutions     SolutionStore
	returner      BacklogReturner
	transition    RequestTransitioner
	outbox        OutboxWriter
	gates         SolutionApprovalGates
	coverage      ChosenOptionCoverage
	decisions     DecisionSuperseder
}

// DecisionSuperseder retires the Decisions of Solutions that no longer apply.
type DecisionSuperseder interface {
	SupersedeDecisions(ctx context.Context, solutionIDs []string) error
}

// ChosenOptionCoverage refuses an approval whose chosen option leaves an acceptance criterion unanswered.
type ChosenOptionCoverage interface {
	CheckChosen(req domain.Request, doc []byte, optionID string) error
}

// SolutionApprovalGates are the CR-REQ-028 checks a chosen solution must pass before the approval goes through.
type SolutionApprovalGates interface {
	CheckSolution(ctx context.Context, requestID, solutionID string, optionsJSON []byte, digest string) error
}

var _ SubjectHandler = (*analysisApprovalHandler)(nil)

type AnalysisApprovalHandlerDeps struct {
	Requests   RequestRepository
	Solutions  SolutionStore
	Returner   BacklogReturner
	Transition RequestTransitioner
	Outbox     OutboxWriter
	// Gates is optional: nil skips the decision and blocking-question checks.
	Gates SolutionApprovalGates
	// Coverage and Decisions are optional like Gates.
	Coverage  ChosenOptionCoverage
	Decisions DecisionSuperseder
}

func newAnalysisApprovalHandler(d AnalysisApprovalHandlerDeps, subject domain.SubjectType, requireChoice bool, kinds ...domain.SolutionKind) *analysisApprovalHandler {
	return &analysisApprovalHandler{subject: subject, kinds: kinds, requireChoice: requireChoice, requests: d.Requests, solutions: d.Solutions,
		returner: d.Returner, transition: d.Transition, outbox: d.Outbox, gates: d.Gates, coverage: d.Coverage, decisions: d.Decisions}
}

// NewSolutionApprovalHandler serves subject_type=solution: both kind=solution (a choice is required) and kind=diagnosis (none).
func NewSolutionApprovalHandler(d AnalysisApprovalHandlerDeps) SubjectHandler {
	return newAnalysisApprovalHandler(d, domain.SubjectSolution, true, domain.SolutionKindSolution, domain.SolutionKindDiagnosis)
}

func (h *analysisApprovalHandler) covers(k domain.SolutionKind) bool {
	for _, x := range h.kinds {
		if x == k {
			return true
		}
	}
	return false
}

// needsChoice is per Solution, not per handler: a diagnosis shares the solution subject but has nothing to choose.
func (h *analysisApprovalHandler) needsChoice(s domain.Solution) bool {
	return h.requireChoice && s.Kind == domain.SolutionKindSolution
}

func (h *analysisApprovalHandler) digest(s domain.Solution) (string, error) {
	return domain.DigestOptions(s.OptionsJSON, s.ChosenOption)
}

// ValidateForRequest picks the request's proposed Solution of a covered kind and returns the digest the Approval will bind to.
func (h *analysisApprovalHandler) ValidateForRequest(ctx, tx context.Context, req domain.Request, st domain.SubjectType) (string, string, error) {
	if st != h.subject {
		return "", "", domain.ErrApprovalSubjectTypeInvalid
	}
	if !domain.ApprovalAllowedInStatus(st, req.Status) {
		return "", "", domain.ErrApprovalStageMismatch
	}
	all, err := h.solutions.ListByRequest(tx, SolutionListFilter{RequestID: req.ID, Status: domain.SolutionStatusProposed})
	if err != nil {
		return "", "", err
	}
	for i := len(all) - 1; i >= 0; i-- {
		s := all[i]
		if !h.covers(s.Kind) {
			continue
		}
		// A solution is proposed before anyone chooses, so the approval opens with the no-choice digest;
		// ChooseSolutionOption refreshes it and OnApproved refuses an approval without a choice.
		d, err := h.digest(s)
		if err != nil {
			return "", "", err
		}
		return s.ID, d, nil
	}
	return "", "", domain.ErrSolutionNotProposed(req.ID)
}

// loadForDecision reads the Solution an Approval refers to and proves it is still the content that was approved.
func (h *analysisApprovalHandler) loadForDecision(tx context.Context, a domain.Approval) (domain.Solution, error) {
	s, err := h.solutions.Get(tx, a.SubjectID)
	if errorHasCode(err, "SOLUTION_NOT_FOUND") {
		return domain.Solution{}, domain.ErrRequestSolutionNotFound(a.SubjectID)
	}
	if err != nil {
		return domain.Solution{}, err
	}
	if s.RequestID != a.RequestID || !h.covers(s.Kind) {
		return domain.Solution{}, domain.ErrRequestSolutionNotFound(a.SubjectID)
	}
	return s, nil
}

func (h *analysisApprovalHandler) OnApproved(ctx, tx context.Context, a domain.Approval) error {
	s, err := h.loadForDecision(tx, a)
	if err != nil {
		return err
	}
	if s.Status == domain.SolutionStatusApproved {
		return nil // redelivery
	}
	if s.Status != domain.SolutionStatusProposed {
		return domain.ErrSolutionNotProposed(s.ID)
	}
	if h.needsChoice(s) && s.ChosenOption == nil {
		return domain.ErrSolutionOptionNotChosen()
	}
	// The content may have changed after the approval opened; the digest on the Approval is the proof of what was reviewed.
	cur, err := h.digest(s)
	if err != nil {
		return err
	}
	if a.SubjectDigest != "" && a.SubjectDigest != cur {
		return domain.ErrApprovalDigestMismatch
	}
	if h.coverage != nil && h.needsChoice(s) {
		if err := h.checkChosenCoverage(tx, s); err != nil {
			return err
		}
	}
	if h.gates != nil && h.needsChoice(s) {
		if err := h.gates.CheckSolution(tx, s.RequestID, s.ID, s.OptionsJSON, cur); err != nil {
			return err
		}
	}
	if err := s.Approve(); err != nil {
		return err
	}
	saved, err := h.solutions.Update(tx, s, s.Version)
	if err != nil {
		return err
	}
	ev, err := NewOutboxEvent(tx, domain.SubjectSolutionApproved, solutionApprovedPayload{
		RequestID: saved.RequestID, SolutionID: saved.ID, Kind: string(saved.Kind), ChosenOption: saved.ChosenOption,
	})
	if err != nil {
		return err
	}
	if err := h.outbox.InsertOutboxEvent(tx, ev); err != nil {
		return err
	}
	from := domain.RequestStatusAwaitingAnalysisApproval
	_, err = h.transition.Execute(tx, TransitionInput{
		RequestID: a.RequestID, Trigger: domain.TriggerAnalysisApproved, ExpectedFrom: &from, ActorID: derefOr(a.DecidedBy), ActorKind: domain.ActorKindUser,
	})
	return err
}

func (h *analysisApprovalHandler) checkChosenCoverage(tx context.Context, s domain.Solution) error {
	opts, err := domain.ParseSolutionOptions(s.OptionsJSON)
	if err != nil || s.ChosenOption == nil || *s.ChosenOption >= len(opts.Options) {
		return err
	}
	req, err := h.requests.Get(tx, s.RequestID)
	if err != nil {
		return err
	}
	if err := h.coverage.CheckChosen(req, s.OptionsJSON, opts.Options[*s.ChosenOption].ID); err != nil {
		return domain.ErrRequestContentInvalid(err.Error())
	}
	return nil
}

func (h *analysisApprovalHandler) OnRejected(ctx, tx context.Context, a domain.Approval) error {
	s, err := h.loadForDecision(tx, a)
	if err != nil {
		return err
	}
	if s.Status == domain.SolutionStatusRejected {
		return nil // redelivery
	}
	if err := s.Reject(); err != nil {
		return err
	}
	if _, err := h.solutions.Update(tx, s, s.Version); err != nil {
		return err
	}
	reason := a.Comment
	if reason == "" {
		reason = "analysis rejected"
	}
	_, err = h.returner.Execute(tx, ReturnInput{
		RequestID: a.RequestID, Stage: domain.ReturnStageAnalysis, Category: domain.ReturnCategoryRejected, Reason: reason,
		ActorID: derefOr(a.DecidedBy), ActorKind: domain.ActorKindUser,
	})
	return err
}

// OnClosedWithoutDecision supersedes the Solution only when the type changed; an expiry or cancel leaves it proposed.
func (h *analysisApprovalHandler) OnClosedWithoutDecision(ctx, tx context.Context, a domain.Approval, why string) error {
	if why != "type_changed" {
		return nil
	}
	s, err := h.loadForDecision(tx, a)
	if errorHasCode(err, "REQUEST_SOLUTION_NOT_FOUND") {
		return nil
	}
	if err != nil {
		return err
	}
	if s.Status != domain.SolutionStatusProposed {
		return nil
	}
	if err := s.Supersede(); err != nil {
		return err
	}
	if _, err = h.solutions.Update(tx, s, s.Version); err != nil {
		return err
	}
	if h.decisions != nil {
		return h.decisions.SupersedeDecisions(tx, []string{s.ID})
	}
	return nil
}

func derefOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// SupersedeForTypeChange retires the open Solutions of a request when its type changes (CR-REQ-005 calls this in its transaction).
func SupersedeForTypeChange(ctx context.Context, solutions SolutionStore, requestID string) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ErrRequestTenantRequired()
	}
	for _, kind := range []domain.SolutionKind{domain.SolutionKindSolution, domain.SolutionKindDiagnosis, domain.SolutionKindFindings, domain.SolutionKindAnswer} {
		if _, err := solutions.SupersedeOpen(ctx, requestID, kind, ""); err != nil {
			return err
		}
	}
	return nil
}
