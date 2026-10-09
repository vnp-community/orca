package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanPolicies hands out the per-type plan rules; *domain.PolicyRegistry satisfies it.
type PlanPolicies interface {
	PolicyFor(t domain.RequestType) domain.TypePolicy
}

// PlanTreeInput is one Plan ready to be written to task-service, already validated.
type PlanTreeInput struct {
	Request domain.Request
	Shape   domain.PlanShape
	// Proposal is the validated, redacted proposal; AIPlanJSON is the same document kept on the Plan task.
	Proposal   domain.PlanProposal
	AIPlanJSON string
	CreatorID  string
	// SupersedesPlanID is the active Plan a replan replaces; empty on a first commit.
	SupersedesPlanID string
}

// PlanTreeResult is what task-service created (or already had).
type PlanTreeResult struct {
	PlanTaskID    string
	PhaseTaskIDs  []string
	TaskIDs       []string
	AlreadyExists bool
}

// TaskPlanWriter writes a whole Plan in one task-service transaction. Calling it again for the same Request
// returns the existing Plan with AlreadyExists, so a retried commit never duplicates tasks.
type TaskPlanWriter interface {
	CreatePlanTree(ctx context.Context, in PlanTreeInput) (PlanTreeResult, error)
}

// PlanActorAuthorizer decides who may generate and commit a Plan (the reporter or an admin, like Solution steering).
type PlanActorAuthorizer interface {
	AuthorizePlan(ctx context.Context, req domain.Request) error
}

// planActorFromSolution reuses the Solution authorizer and reports its refusal under the Plan's own code.
type planActorFromSolution struct{ inner SolutionActorAuthorizer }

func NewPlanActorAuthorizer(inner SolutionActorAuthorizer) PlanActorAuthorizer {
	return planActorFromSolution{inner: inner}
}

func (a planActorFromSolution) AuthorizePlan(ctx context.Context, req domain.Request) error {
	if err := a.inner.AuthorizeGenerate(ctx, req); err != nil {
		if errorHasCode(err, "REQUEST_SOLUTION_FORBIDDEN") {
			return domain.ErrPlanForbidden()
		}
		return err
	}
	return nil
}

// errorCode is the REQUEST_* code of an application error, or "" for any other error.
func errorCode(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}
