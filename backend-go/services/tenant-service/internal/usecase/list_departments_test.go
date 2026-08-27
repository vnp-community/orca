package usecase

import (
	"context"
	"testing"
)

func TestListDepartments_ScopedByCompany(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-2", "company-1", "Sales", nil))
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-3", "company-2", "Other Co", nil))

	uc := NewListDepartments(departments)

	got, err := uc.Execute(context.Background(), ListDepartmentsInput{CompanyID: "company-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 departments scoped to company-1, got %d: %+v", len(got), got)
	}
	for _, d := range got {
		if d.CompanyID != "company-1" {
			t.Errorf("expected every department scoped to company-1, got %+v", d)
		}
	}
}
