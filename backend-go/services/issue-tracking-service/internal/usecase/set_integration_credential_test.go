package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

type fakeCredentialWriter struct {
	writeRawFunc func(ctx context.Context, tenantID string, provider domain.Provider, token, configJSON string) error
}

func (f *fakeCredentialWriter) WriteRaw(ctx context.Context, tenantID string, provider domain.Provider, token, configJSON string) error {
	return f.writeRawFunc(ctx, tenantID, provider, token, configJSON)
}

func TestSetIntegrationCredential_CallsWriteRawWithOwnerIDConvention(t *testing.T) {
	var gotTenant, gotToken, gotConfig string
	var gotProvider domain.Provider
	writer := &fakeCredentialWriter{writeRawFunc: func(_ context.Context, tenantID string, provider domain.Provider, token, configJSON string) error {
		gotTenant, gotProvider, gotToken, gotConfig = tenantID, provider, token, configJSON
		return nil
	}}
	uc := usecase.NewSetIntegrationCredential(writer)

	err := uc.Execute(context.Background(), usecase.SetIntegrationCredentialInput{
		TenantID: "t1", Provider: domain.ProviderJira, Token: "tok-123", ConfigJSON: `{"baseUrl":"https://x"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTenant != "t1" || gotProvider != domain.ProviderJira || gotToken != "tok-123" || gotConfig != `{"baseUrl":"https://x"}` {
		t.Errorf("WriteRaw called with unexpected args: tenant=%q provider=%v token=%q config=%q", gotTenant, gotProvider, gotToken, gotConfig)
	}
}

func TestSetIntegrationCredential_EmptyToken_Errors(t *testing.T) {
	uc := usecase.NewSetIntegrationCredential(&fakeCredentialWriter{})
	err := uc.Execute(context.Background(), usecase.SetIntegrationCredentialInput{TenantID: "t1"})
	if err == nil {
		t.Fatal("expected ISSUETRACKING_NO_TOKEN error")
	}
}

func TestSetIntegrationCredential_EmptyTenant_ErrorsWithoutCallingWriter(t *testing.T) {
	called := false
	writer := &fakeCredentialWriter{writeRawFunc: func(context.Context, string, domain.Provider, string, string) error {
		called = true
		return nil
	}}
	uc := usecase.NewSetIntegrationCredential(writer)
	err := uc.Execute(context.Background(), usecase.SetIntegrationCredentialInput{Token: "tok-123"})
	if err == nil {
		t.Fatal("expected ISSUETRACKING_NO_TENANT error")
	}
	if called {
		t.Error("expected WriteRaw to never be called when tenant_id is missing")
	}
}
