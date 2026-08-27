package usecase

import (
	"context"
	"testing"
)

func TestValidateTenant_ExistingTenant(t *testing.T) {
	repo := newFakeCompanyRepository()
	company := mustCompany(t, "company-1", "Acme", nil)
	_, _ = repo.Create(context.Background(), company)

	uc := NewValidateTenant(repo)
	got, err := uc.Execute(context.Background(), ValidateTenantInput{TenantID: "company-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected exists=true for a known tenant")
	}
}

func TestValidateTenant_UnknownTenant(t *testing.T) {
	uc := NewValidateTenant(newFakeCompanyRepository())
	got, err := uc.Execute(context.Background(), ValidateTenantInput{TenantID: "does-not-exist"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected exists=false for an unknown tenant")
	}
}

func TestValidateTenant_EmptyTenantID(t *testing.T) {
	uc := NewValidateTenant(newFakeCompanyRepository())
	got, err := uc.Execute(context.Background(), ValidateTenantInput{TenantID: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected exists=false for an empty tenant id")
	}
}
