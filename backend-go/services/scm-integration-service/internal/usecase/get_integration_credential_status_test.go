package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

type fakeCredentialStatusReader struct {
	getStatusFunc func(ctx context.Context, tenantID string, provider domain.ScmProvider) (bool, string, error)
}

func (f *fakeCredentialStatusReader) GetStatus(ctx context.Context, tenantID string, provider domain.ScmProvider) (bool, string, error) {
	return f.getStatusFunc(ctx, tenantID, provider)
}

func TestGetIntegrationCredentialStatus_NotConfigured(t *testing.T) {
	reader := &fakeCredentialStatusReader{getStatusFunc: func(context.Context, string, domain.ScmProvider) (bool, string, error) {
		return false, "", nil
	}}
	uc := usecase.NewGetIntegrationCredentialStatus(reader)

	got, err := uc.Execute(context.Background(), usecase.GetIntegrationCredentialStatusInput{TenantID: "t1", Provider: domain.ScmProviderGitea})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Configured {
		t.Error("expected Configured=false")
	}
	if got.ConfigJSON != "" {
		t.Errorf("expected empty ConfigJSON when not configured, got %q", got.ConfigJSON)
	}
}

func TestGetIntegrationCredentialStatus_Configured(t *testing.T) {
	reader := &fakeCredentialStatusReader{getStatusFunc: func(context.Context, string, domain.ScmProvider) (bool, string, error) {
		return true, `{"baseUrl":"https://x"}`, nil
	}}
	uc := usecase.NewGetIntegrationCredentialStatus(reader)

	got, err := uc.Execute(context.Background(), usecase.GetIntegrationCredentialStatusInput{TenantID: "t1", Provider: domain.ScmProviderGitea})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Configured {
		t.Error("expected Configured=true")
	}
	if got.ConfigJSON != `{"baseUrl":"https://x"}` {
		t.Errorf("expected ConfigJSON to match, got %q", got.ConfigJSON)
	}
}

func TestGetIntegrationCredentialStatus_NoTenant_Errors(t *testing.T) {
	uc := usecase.NewGetIntegrationCredentialStatus(&fakeCredentialStatusReader{})
	if _, err := uc.Execute(context.Background(), usecase.GetIntegrationCredentialStatusInput{}); err == nil {
		t.Fatal("expected SCM_NO_TENANT error")
	}
}
