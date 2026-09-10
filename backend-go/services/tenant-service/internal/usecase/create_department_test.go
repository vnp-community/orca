package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestCreateDepartment_RequiresExistingCompany(t *testing.T) {
	uc := NewCreateDepartment(newFakeCompanyRepository(), newFakeDepartmentRepository(), newFakeOPAClient(true), nil)
	_, err := uc.Execute(deptAdminCtx("missing", "admin-1"), CreateDepartmentInput{CompanyID: "missing", Name: "Engineering"})
	if err == nil {
		t.Fatal("expected an error when the company doesn't exist")
	}
}

func TestCreateDepartment_PersistsUnderCompany(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	departments := newFakeDepartmentRepository()

	uc := NewCreateDepartment(companies, departments, newFakeOPAClient(true), nil)
	got, err := uc.Execute(deptAdminCtx("company-1", "admin-1"), CreateDepartmentInput{CompanyID: "company-1", Name: "Engineering"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CompanyID != "company-1" || got.Name != "Engineering" {
		t.Errorf("unexpected department: %+v", got)
	}
	if len(departments.byKey) != 1 {
		t.Fatalf("expected 1 persisted department, got %d", len(departments.byKey))
	}
}

func TestCreateDepartment_DeniesNonAdminUnconditionally(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	departments := newFakeDepartmentRepository()
	opa := newFakeOPAClient(false)

	uc := NewCreateDepartment(companies, departments, opa, nil)
	ctx := withRole(withActor(withTenant(context.Background(), "company-1"), "lead-1"), "lead")

	_, err := uc.Execute(ctx, CreateDepartmentInput{CompanyID: "company-1", Name: "Engineering"})
	assertAppError(t, err, apperrors.KindPermissionDenied)
	if len(opa.calls) != 1 || opa.calls[0].sameDepartment {
		t.Errorf("expected requireDepartmentAccess called with sameDepartment=false, got %+v", opa.calls)
	}
	if len(departments.byKey) != 0 {
		t.Error("expected no department persisted on deny")
	}
}

func TestCreateDepartment_NameTaken_ShortCircuitsBeforeCreate(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	departments := newFakeDepartmentRepository()
	departments.existsByName = true

	uc := NewCreateDepartment(companies, departments, newFakeOPAClient(true), nil)
	_, err := uc.Execute(deptAdminCtx("company-1", "admin-1"), CreateDepartmentInput{CompanyID: "company-1", Name: "Engineering"})
	assertAppError(t, err, apperrors.KindInvalidArgument)
	if len(departments.byKey) != 0 {
		t.Error("expected no department persisted when name is taken")
	}
}

func TestCreateDepartment_PublishesAuditEventOnSuccess(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	departments := newFakeDepartmentRepository()
	audit := newFakeAuditPublisher()

	uc := NewCreateDepartment(companies, departments, newFakeOPAClient(true), audit)
	if _, err := uc.Execute(deptAdminCtx("company-1", "admin-1"), CreateDepartmentInput{CompanyID: "company-1", Name: "Engineering"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(audit.calls) != 1 || audit.calls[0].action != "department.created" {
		t.Errorf("expected 1 audit event with action department.created, got %+v", audit.calls)
	}
}
