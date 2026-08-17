package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// SetUserDepartmentInput mirrors SetUserDepartmentRequest 1:1.
type SetUserDepartmentInput struct {
	UserID       string
	DepartmentID string
}

// SetUserDepartment assigns a user to a department, upserting their
// tenant.user_profiles row. SetUserDepartmentRequest has no company_id
// bound field (tenant.proto), so the scoping company comes from the
// request context (tenant.RequireTenantID), populated by the gRPC
// tenant-extraction interceptor from validated metadata (common/grpcmw) —
// see tenant-service.md §9's cross-tenant isolation rule.
type SetUserDepartment struct {
	departments DepartmentRepository
	profiles    UserProfileRepository
	cache       ProfileCache
}

func NewSetUserDepartment(departments DepartmentRepository, profiles UserProfileRepository, cache ProfileCache) *SetUserDepartment {
	return &SetUserDepartment{departments: departments, profiles: profiles, cache: cache}
}

func (uc *SetUserDepartment) Execute(ctx context.Context, in SetUserDepartmentInput) error {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	department, found, err := uc.departments.Get(ctx, companyID, in.DepartmentID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_DEPARTMENT_LOOKUP_FAILED", "failed to look up department", err)
	}
	if !found {
		// A department_id from another company resolves as not-found here
		// too, since Get is already scoped by companyID — tenant-service.md §9.
		return apperrors.New(apperrors.KindNotFound, "TENANT_DEPARTMENT_NOT_FOUND", "department does not exist", nil)
	}

	existing, _, err := uc.profiles.Get(ctx, companyID, in.UserID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up user profile", err)
	}

	profile, err := domain.NewUserProfile(in.UserID, companyID, department.ID, existing.Settings)
	if err != nil {
		return apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_PROFILE", err.Error(), err)
	}
	if err := uc.profiles.Upsert(ctx, profile); err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_SET_DEPARTMENT_FAILED", "failed to persist user profile", err)
	}

	// Invalidation correctness (tenant-service.md §8): this write changes
	// exactly one user's department layer, so invalidate exactly that
	// user's cached ResolvedProfile before returning success.
	if uc.cache != nil {
		uc.cache.Invalidate(ctx, in.UserID)
	}
	return nil
}
