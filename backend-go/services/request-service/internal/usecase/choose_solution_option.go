package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ChooseSolutionOptionInput struct {
	RequestID  string
	SolutionID string
	OptionID   string
	// Rationale is required by the Decision record when the pick differs from the recommendation (CR-REQ-028).
	Rationale string
}

type ChooseSolutionOptionResult struct {
	Solution domain.Solution
	// ApprovalDigest is what the UI sends back as expected_digest when it approves.
	ApprovalDigest string
	// DecisionStatus is open|chosen|effective; empty when no Decision record is wired.
	DecisionStatus string
	// RequiresConfirmation is true while a high-risk pick still waits for ConfirmDecision.
	RequiresConfirmation bool
}

// SolutionDecisionRecorder is RecordDecision as ChooseSolutionOption sees it.
type SolutionDecisionRecorder interface {
	Execute(ctx context.Context, in RecordDecisionInput) (domain.Decision, error)
}

// SelfChoicePolicy says whether the reporter may pick an option on their own request.
type SelfChoicePolicy func(ctx context.Context, req domain.Request) (bool, error)

// ChooseSolutionOption records the reviewer's pick and refreshes the pending Approval's digest in one
// transaction, so an approval can never be granted for a choice other than the one on screen.
type ChooseSolutionOption struct {
	requests  RequestRepository
	solutions SolutionStore
	digests   PendingDigestUpdater
	auth      SolutionActorAuthorizer
	tx        TxRunner
	decisions SolutionDecisionRecorder
	selfPick  SelfChoicePolicy
}

// WithDecisions records every pick as a Decision in the same transaction, so a refused pick (a missing
// rationale, say) leaves nothing behind. selfPick may be nil, which allows the reporter.
func (uc *ChooseSolutionOption) WithDecisions(rec SolutionDecisionRecorder, selfPick SelfChoicePolicy) *ChooseSolutionOption {
	uc.decisions, uc.selfPick = rec, selfPick
	return uc
}

func NewChooseSolutionOption(requests RequestRepository, solutions SolutionStore, digests PendingDigestUpdater, auth SolutionActorAuthorizer, tx TxRunner) *ChooseSolutionOption {
	return &ChooseSolutionOption{requests: requests, solutions: solutions, digests: digests, auth: auth, tx: tx}
}

func (uc *ChooseSolutionOption) Execute(ctx context.Context, in ChooseSolutionOptionInput) (ChooseSolutionOptionResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ChooseSolutionOptionResult{}, domain.ErrRequestTenantRequired()
	}
	// Authorise before the transaction: approver checks may call the tenant directory, and no gRPC runs inside a transaction.
	if pre, err := uc.requests.Get(ctx, in.RequestID); err != nil {
		if isNotFound(err) {
			return ChooseSolutionOptionResult{}, domain.ErrSolutionRequestNotFound(in.RequestID)
		}
		return ChooseSolutionOptionResult{}, err
	} else if err := uc.auth.AuthorizeChoose(ctx, pre); err != nil {
		return ChooseSolutionOptionResult{}, err
	}
	var out ChooseSolutionOptionResult
	err = uc.tx.InTx(ctx, func(ctx context.Context) error {
		// Request first, then Approval: the same lock order the Approve path uses.
		req, err := uc.requests.Get(ctx, in.RequestID)
		if err != nil {
			if isNotFound(err) {
				return domain.ErrSolutionRequestNotFound(in.RequestID)
			}
			return err
		}
		sol, err := uc.solutions.Get(ctx, in.SolutionID)
		if errorHasCode(err, "SOLUTION_NOT_FOUND") {
			return domain.ErrRequestSolutionNotFound(in.SolutionID)
		}
		if err != nil {
			return err
		}
		if sol.RequestID != req.ID {
			return domain.ErrRequestSolutionNotFound(in.SolutionID) // never reveal a solution of another request
		}
		if sol.Kind != domain.SolutionKindSolution {
			return domain.ErrSolutionKindNotAllowed(req.Type)
		}
		if sol.Status != domain.SolutionStatusProposed {
			return domain.ErrSolutionNotProposed(sol.ID)
		}
		opts, err := domain.ParseSolutionOptions(sol.OptionsJSON)
		if err != nil {
			return err
		}
		idx, ok := opts.IndexOf(in.OptionID)
		if !ok {
			return domain.ErrSolutionOptionNotFound(in.OptionID)
		}
		if sol.ChosenOption == nil || *sol.ChosenOption != idx {
			if err := sol.Choose(idx, len(opts.Options)); err != nil {
				return err
			}
			won, err := uc.solutions.Choose(ctx, sol.ID, idx, sol.Version)
			if err != nil {
				return err
			}
			if !won {
				return domain.ErrSolutionVersionConflict(sol.ID, sol.Version)
			}
			sol, err = uc.solutions.Get(ctx, sol.ID)
			if err != nil {
				return err
			}
		}
		digest, err := domain.DigestOptions(sol.OptionsJSON, sol.ChosenOption)
		if err != nil {
			return err
		}
		// false just means no approval is pending yet; the digest is recomputed when one opens.
		if _, err := uc.digests.UpdatePendingDigest(ctx, tenantID, domain.SubjectSolution, sol.ID, digest); err != nil {
			return err
		}
		out = ChooseSolutionOptionResult{Solution: sol, ApprovalDigest: digest}
		if uc.decisions == nil {
			return nil
		}
		selfAllowed := true
		if uc.selfPick != nil {
			if selfAllowed, err = uc.selfPick(ctx, req); err != nil {
				return err
			}
		}
		actor, _ := tenant.UserID(ctx)
		d, err := uc.decisions.Execute(ctx, RecordDecisionInput{
			RequestID: req.ID, RequestNumber: req.Number, SolutionID: sol.ID, OptionsJSON: sol.OptionsJSON, ChosenOption: in.OptionID,
			ChooserID: actor, Rationale: in.Rationale, SubjectDigest: digest, ReporterID: req.ReporterID, SelfChoiceAllowed: selfAllowed,
		})
		if err != nil {
			return err
		}
		out.DecisionStatus, out.RequiresConfirmation = string(d.Status), d.Status == domain.DecisionStatusChosen
		return nil
	})
	return out, err
}
