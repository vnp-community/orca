package usecase

import (
	"context"
	"testing"
)

func TestCreateDepartment_RequiresExistingCompany(t *testing.T) {
	uc := NewCreateDepartment(newFakeCompanyRepository(), newFakeDepartmentRepository())
	_, err := uc.Execute(context.Background(), CreateDepartmentInput{CompanyID: "missing", Name: "Engineering"})
	if err == nil {
		t.Fatal("expected an error when the company doesn't exist")
	}
}

func TestCreateDepartment_PersistsUnderCompany(t *testing.T) {
	companies := newFakeCompanyRepository()
	_, _ = companies.Create(context.Background(), mustCompany(t, "company-1", "Acme", nil))
	departments := newFakeDepartmentRepository()

	uc := NewCreateDepartment(companies, departments)
	got, err := uc.Execute(context.Background(), CreateDepartmentInput{CompanyID: "company-1", Name: "Engineering"})
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
