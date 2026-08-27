package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

type fakeCredentialRevoker struct {
	revokeErr    error
	lastTenant   string
	lastProvider domain.Provider
	calls        int
}

func (f *fakeCredentialRevoker) RevokeByOwner(_ context.Context, tenantID string, provider domain.Provider) error {
	f.calls++
	f.lastTenant, f.lastProvider = tenantID, provider
	return f.revokeErr
}

func TestRevokeAuth_CallsRevokeByOwner(t *testing.T) {
	revoker := &fakeCredentialRevoker{}
	uc := usecase.NewRevokeAuth(revoker)

	err := uc.Execute(context.Background(), usecase.RevokeAuthInput{TenantID: "t1", Provider: domain.ProviderJira})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revoker.calls != 1 {
		t.Fatalf("expected RevokeByOwner to be called exactly once, got %d", revoker.calls)
	}
	if revoker.lastTenant != "t1" || revoker.lastProvider != domain.ProviderJira {
		t.Errorf("expected RevokeByOwner called with (t1, jira), got (%s, %s)", revoker.lastTenant, revoker.lastProvider)
	}
}

func TestRevokeAuth_PropagatesBrokerFailure(t *testing.T) {
	revoker := &fakeCredentialRevoker{revokeErr: errors.New("broker unavailable")}
	uc := usecase.NewRevokeAuth(revoker)

	err := uc.Execute(context.Background(), usecase.RevokeAuthInput{TenantID: "t1", Provider: domain.ProviderLinear})
	if err == nil {
		t.Fatal("expected an error when the broker call fails")
	}
}

func TestRevokeAuth_NoTenant_Errors(t *testing.T) {
	revoker := &fakeCredentialRevoker{}
	uc := usecase.NewRevokeAuth(revoker)

	err := uc.Execute(context.Background(), usecase.RevokeAuthInput{Provider: domain.ProviderJira})
	if err == nil {
		t.Fatal("expected ISSUETRACKING_NO_TENANT error")
	}
	if revoker.calls != 0 {
		t.Errorf("expected no broker call when tenant_id is missing, got %d", revoker.calls)
	}
}
