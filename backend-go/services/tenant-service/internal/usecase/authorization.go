package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

const (
	actionCompanyEdit    = "company_edit"
	actionDepartmentEdit = "department_edit"
)

// requireCompanyAdmin gates UpdateCompany — admin role only, per
// BL-PRF-01's Error Cases table ("Not admin (company edit) -> 403").
func requireCompanyAdmin(ctx context.Context, opa OPAClient) error {
	return decide(ctx, opa, actionCompanyEdit, false)
}

// requireDepartmentAccess gates UpdateDepartment/CreateDepartment — admin,
// or lead of the SAME department. sameDepartment is precomputed by the
// caller (it already has the actor's UserProfile.DepartmentID and the
// target department's id in hand) — OPA never does its own department
// lookup, per this file's doc comment.
func requireDepartmentAccess(ctx context.Context, opa OPAClient, sameDepartment bool) error {
	return decide(ctx, opa, actionDepartmentEdit, sameDepartment)
}

func decide(ctx context.Context, opa OPAClient, action string, sameDepartment bool) error {
	if _, ok := tenant.UserID(ctx); !ok {
		return apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_ACTOR", "no authenticated user in request context", nil)
	}
	role, _ := tenant.Role(ctx) // "" until the upstream claim-propagation gap closes — fails closed below
	allowed, err := opa.Decision(ctx, role, action, sameDepartment)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_POLICY_EVAL_FAILED", "failed to evaluate authorization policy", err)
	}
	if !allowed {
		return apperrors.New(apperrors.KindPermissionDenied, "TENANT_NOT_AUTHORIZED", "caller is not authorized for this action", nil)
	}
	return nil
}
