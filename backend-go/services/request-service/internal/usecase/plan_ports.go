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

// PlanActorAuthorizer decides who may generate, revise and commit a Plan. AuthorizePlan is the first generation and the
// commit (the reporter or an admin); AuthorizePlanRevise also lets the approvers of the pending Plan gate ask for changes.
type PlanActorAuthorizer interface {
	AuthorizePlan(ctx context.Context, req domain.Request) error
	AuthorizePlanRevise(ctx context.Context, req domain.Request) error
}

// planActorFromSolution reuses the Solution authorizer and reports its refusal under the Plan's own code.
type planActorFromSolution struct {
	inner SolutionActorAuthorizer
	runs  AnalysisRunStore
	// approversMayRevise is true only while a server-side worker commits the revised Plan: without it an approver's
	// revision would leave a Plan nobody commits, because CommitPlan stays with the reporter and admins.
	approversMayRevise bool
}

func NewPlanActorAuthorizer(inner SolutionActorAuthorizer, runs AnalysisRunStore, approversMayRevise bool) PlanActorAuthorizer {
	return planActorFromSolution{inner: inner, runs: runs, approversMayRevise: approversMayRevise}
}

func (a planActorFromSolution) AuthorizePlan(ctx context.Context, req domain.Request) error {
	return planForbidden(a.inner.AuthorizeGenerate(ctx, req))
}

// planForbidden reports a Solution refusal under the Plan's own code; any other error passes through unchanged.
func planForbidden(err error) error {
	if err != nil && errorHasCode(err, "REQUEST_SOLUTION_FORBIDDEN") {
		return domain.ErrPlanForbidden()
	}
	return err
}

// errorCode is the REQUEST_* code of an application error, or "" for any other error.
func errorCode(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}
