package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// fakeCredentialRevoker is an in-memory CredentialRevoker — mirrors
// fakeCredentialWriter's shape (oauth_flow_test.go) for the same reason:
// record what RevokeAuth called it with, and return a canned error.
type fakeCredentialRevoker struct {
	revokeErr    error
	lastTenant   string
	lastProvider domain.ScmProvider
	calls        int
}

func (f *fakeCredentialRevoker) RevokeByOwner(_ context.Context, tenantID string, provider domain.ScmProvider) error {
	f.calls++
	f.lastTenant, f.lastProvider = tenantID, provider
	return f.revokeErr
}

func TestRevokeAuth_CallsCredentialRevokerForReal(t *testing.T) {
	revoker := &fakeCredentialRevoker{}
	uc := NewRevokeAuth(revoker)

	err := uc.Execute(context.Background(), RevokeAuthInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revoker.calls != 1 {
		t.Fatalf("expected RevokeByOwner to be called exactly once, got %d", revoker.calls)
	}
	if revoker.lastTenant != "tenant-1" || revoker.lastProvider != domain.ScmProviderGitHub {
		t.Errorf("expected RevokeByOwner called with (tenant-1, github), got (%s, %s)", revoker.lastTenant, revoker.lastProvider)
	}
}

func TestRevokeAuth_PropagatesBrokerFailure(t *testing.T) {
	revoker := &fakeCredentialRevoker{revokeErr: errors.New("broker unavailable")}
	uc := NewRevokeAuth(revoker)

	err := uc.Execute(context.Background(), RevokeAuthInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err == nil {
		t.Fatal("expected an error when the broker call fails")
	}
}

func TestRevokeAuth_RequiresTenant(t *testing.T) {
	revoker := &fakeCredentialRevoker{}
	uc := NewRevokeAuth(revoker)

	err := uc.Execute(context.Background(), RevokeAuthInput{Provider: domain.ScmProviderGitHub})
	if err == nil {
		t.Fatal("expected an error for missing tenant_id")
	}
	if revoker.calls != 0 {
		t.Errorf("expected no broker call when tenant_id is missing, got %d", revoker.calls)
	}
}
