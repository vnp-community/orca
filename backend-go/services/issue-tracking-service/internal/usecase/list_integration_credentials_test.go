package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

type fakeCredentialLister struct {
	listFunc func(ctx context.Context, tenantID string) ([]domain.Provider, error)
}

func (f *fakeCredentialLister) ListConfiguredProviders(ctx context.Context, tenantID string) ([]domain.Provider, error) {
	return f.listFunc(ctx, tenantID)
}

func TestListIntegrationCredentials_ReturnsConfiguredProviders(t *testing.T) {
	lister := &fakeCredentialLister{listFunc: func(_ context.Context, tenantID string) ([]domain.Provider, error) {
		if tenantID != "t1" {
			t.Fatalf("expected tenantID t1, got %q", tenantID)
		}
		return []domain.Provider{domain.ProviderJira, domain.ProviderLinear}, nil
	}}
	uc := usecase.NewListIntegrationCredentials(lister)

	got, err := uc.Execute(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != domain.ProviderJira || got[1] != domain.ProviderLinear {
		t.Errorf("expected [jira linear], got %v", got)
	}
}

func TestListIntegrationCredentials_NoTenant_ErrorsWithoutCallingLister(t *testing.T) {
	called := false
	lister := &fakeCredentialLister{listFunc: func(context.Context, string) ([]domain.Provider, error) {
		called = true
		return nil, nil
	}}
	uc := usecase.NewListIntegrationCredentials(lister)

	if _, err := uc.Execute(context.Background(), ""); err == nil {
		t.Fatal("expected ISSUETRACKING_NO_TENANT error")
	}
	if called {
		t.Error("expected ListConfiguredProviders to never be called when tenant_id is missing")
	}
}
