package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
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
	opa         OPAClient      // NEW
	audit       AuditPublisher // NEW
}

func NewCreateDepartment(companies CompanyRepository, departments DepartmentRepository, opa OPAClient, audit AuditPublisher) *CreateDepartment {
	return &CreateDepartment{companies: companies, departments: departments, opa: opa, audit: audit}
}

func (uc *CreateDepartment) Execute(ctx context.Context, in CreateDepartmentInput) (domain.Department, error) {
	// sameDepartment is always false here — a lead can't create a department
	// that doesn't exist yet to be "their own", so only caller_role=="admin"
	// can pass this gate (matches BL-PRF-01's flow, which only shows Admin
	// as the actor for department creation).
	if err := requireDepartmentAccess(ctx, uc.opa, false); err != nil {
		return domain.Department{}, err
	}

	exists, err := uc.companies.Exists(ctx, in.CompanyID)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_COMPANY_LOOKUP_FAILED", "failed to check company existence", err)
	}
	if !exists {
		return domain.Department{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
	}

	nameTaken, err := uc.departments.ExistsByName(ctx, in.CompanyID, in.Name)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_DEPARTMENT_NAME_LOOKUP_FAILED", "failed to check department name uniqueness", err)
	}
	if nameTaken {
		return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_DEPARTMENT_NAME_TAKEN", "a department with this name already exists", nil)
	}

	department, err := domain.NewDepartment(uuid.NewString(), in.CompanyID, in.Name, nil)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_DEPARTMENT", err.Error(), err)
	}

	created, err := uc.departments.Create(ctx, department)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_CREATE_DEPARTMENT_FAILED", "failed to persist department", err)
	}

	if uc.audit != nil {
		actorID, _ := tenant.UserID(ctx)
		_ = uc.audit.PublishAuditEvent(ctx, in.CompanyID, actorID, "department.created", created.ID)
	}
	return created, nil
}
