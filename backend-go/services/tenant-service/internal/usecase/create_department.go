package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// CreateDepartmentInput mirrors CreateDepartmentRequest 1:1.
type CreateDepartmentInput struct {
	CompanyID string
	Name      string
}

// CreateDepartment creates a Department under a Company. CompanyID comes
// from CreateDepartmentRequest's own bound field, trusted as already
// validated by the caller — tenant-service.md §3: "every request carries
// tenant_id explicitly... via a bound field", the same trust model
// RecordUsageSession's UserID uses in usage-service.
type CreateDepartment struct {
	companies   CompanyRepository
	departments DepartmentRepository
}

func NewCreateDepartment(companies CompanyRepository, departments DepartmentRepository) *CreateDepartment {
	return &CreateDepartment{companies: companies, departments: departments}
}

func (uc *CreateDepartment) Execute(ctx context.Context, in CreateDepartmentInput) (domain.Department, error) {
	exists, err := uc.companies.Exists(ctx, in.CompanyID)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_COMPANY_LOOKUP_FAILED", "failed to check company existence", err)
	}
	if !exists {
		return domain.Department{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
	}

	department, err := domain.NewDepartment(uuid.NewString(), in.CompanyID, in.Name, nil)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_DEPARTMENT", err.Error(), err)
	}

	created, err := uc.departments.Create(ctx, department)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_CREATE_DEPARTMENT_FAILED", "failed to persist department", err)
	}
	return created, nil
}
