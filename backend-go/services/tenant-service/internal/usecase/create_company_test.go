package usecase

import (
	"context"
	"testing"
)

func TestCreateCompany_PersistsAndReturnsCompany(t *testing.T) {
	repo := newFakeCompanyRepository()
	uc := NewCreateCompany(repo)

	got, err := uc.Execute(context.Background(), CreateCompanyInput{Name: "Acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Acme" {
		t.Errorf("expected Name=Acme, got %q", got.Name)
	}
	if got.ID == "" {
		t.Error("expected a generated ID")
	}
	if len(repo.byID) != 1 {
		t.Fatalf("expected 1 persisted company, got %d", len(repo.byID))
	}
}

func TestCreateCompany_RejectsEmptyName(t *testing.T) {
	uc := NewCreateCompany(newFakeCompanyRepository())
	_, err := uc.Execute(context.Background(), CreateCompanyInput{Name: ""})
	if err == nil {
		t.Fatal("expected an error for an empty name")
	}
}

func TestCreateCompany_RepositoryFailurePropagates(t *testing.T) {
	repo := newFakeCompanyRepository()
	repo.createErr = errFakeRepository
	uc := NewCreateCompany(repo)

	_, err := uc.Execute(context.Background(), CreateCompanyInput{Name: "Acme"})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
