package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestListCompanies_ReturnsAllCompanies(t *testing.T) {
	repo := newFakeCompanyRepository()
	repo.byID["co-1"] = domain.Company{ID: "co-1", Name: "Acme"}
	repo.byID["co-2"] = domain.Company{ID: "co-2", Name: "Globex"}
	uc := NewListCompanies(repo)

	got, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 companies, got %d", len(got))
	}
}

func TestListCompanies_EmptyRepositoryReturnsEmptySlice(t *testing.T) {
	uc := NewListCompanies(newFakeCompanyRepository())

	got, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Error("expected an empty slice, not nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 companies, got %d", len(got))
	}
}

func TestListCompanies_RepositoryErrorPropagates(t *testing.T) {
	repo := newFakeCompanyRepository()
	repo.listErr = errors.New("boom")
	uc := NewListCompanies(repo)

	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error when the repository fails")
	}
}
