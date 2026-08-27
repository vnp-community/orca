package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// CreateCompanyInput mirrors CreateCompanyRequest 1:1 — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable.
type CreateCompanyInput struct {
	Name string
}

// CreateCompany originates a new tenant. Deliberately does NOT require a
// tenant already in context — this is the operation that creates one
// (tenant-service.md §1: "the service that originates tenant_id").
type CreateCompany struct {
	companies CompanyRepository
}

func NewCreateCompany(companies CompanyRepository) *CreateCompany {
	return &CreateCompany{companies: companies}
}

func (uc *CreateCompany) Execute(ctx context.Context, in CreateCompanyInput) (domain.Company, error) {
	company, err := domain.NewCompany(uuid.NewString(), in.Name, nil)
	if err != nil {
		return domain.Company{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_COMPANY", err.Error(), err)
	}

	created, err := uc.companies.Create(ctx, company)
	if err != nil {
		return domain.Company{}, apperrors.New(apperrors.KindInternal, "TENANT_CREATE_COMPANY_FAILED", "failed to persist company", err)
	}
	return created, nil
}
