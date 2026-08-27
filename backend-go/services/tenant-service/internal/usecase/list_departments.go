package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// ListDepartmentsInput mirrors ListDepartmentsRequest 1:1.
type ListDepartmentsInput struct {
	CompanyID string
}

type ListDepartments struct {
	departments DepartmentRepository
}

func NewListDepartments(departments DepartmentRepository) *ListDepartments {
	return &ListDepartments{departments: departments}
}

func (uc *ListDepartments) Execute(ctx context.Context, in ListDepartmentsInput) ([]domain.Department, error) {
	depts, err := uc.departments.List(ctx, in.CompanyID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_DEPARTMENTS_FAILED", "failed to list departments", err)
	}
	return depts, nil
}
