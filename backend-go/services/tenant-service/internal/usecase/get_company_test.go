package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestGetCompany_ReturnsExistingCompany(t *testing.T) {
	repo := newFakeCompanyRepository()
	repo.byID["co-1"] = domain.Company{ID: "co-1", Name: "Acme"}
	uc := NewGetCompany(repo)

	got, err := uc.Execute(context.Background(), "co-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Acme" {
		t.Errorf("expected Name=Acme, got %q", got.Name)
	}
}

func TestGetCompany_NotFound(t *testing.T) {
	uc := NewGetCompany(newFakeCompanyRepository())

	_, err := uc.Execute(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected an error for a missing company")
	}
}
