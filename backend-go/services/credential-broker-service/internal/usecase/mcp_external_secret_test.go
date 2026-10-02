package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestMcpExternalSecret_OnlyMcpServiceMayWriteAndResolve(t *testing.T) {
	rec := &callRecorder{}
	meta, audit, store := newFakeMetadataRepo(rec), newFakeAuditRepo(rec), newFakeSecretStore(rec)
	write := NewWriteCredential(store, newFakeTxRunner(rec, meta, audit))
	resolve := NewResolveCredentialByOwner(meta, audit, store)
	ctx := context.Background()
	in := func(svc string) WriteCredentialInput {
		return WriteCredentialInput{TenantID: "t1", OwnerID: "mcp:s:env:TOKEN", Category: domain.CategoryMcpExternalSecret,
			EncryptedEnvelope: []byte("s3cret"), RequestingService: svc}
	}

	for _, svc := range []string{"", "api-gateway", "scm-integration-service"} {
		_, err := write.Execute(ctx, in(svc))
		var ae *apperrors.AppError
		if !asAppError(err, &ae) || ae.Code != "CREDENTIAL_CALLER_NOT_ALLOWED" {
			t.Fatalf("caller %q: want CREDENTIAL_CALLER_NOT_ALLOWED, got %v", svc, err)
		}
	}
	if len(meta.rows) != 0 {
		t.Fatalf("denied writes must not create rows, got %d", len(meta.rows))
	}
	if _, err := write.Execute(ctx, in("mcp-service")); err != nil {
		t.Fatalf("mcp-service write: %v", err)
	}
	if _, err := resolve.Execute(ctx, ResolveCredentialByOwnerInput{TenantID: "t1", OwnerID: "mcp:s:env:TOKEN",
		Category: domain.CategoryMcpExternalSecret, RequestingService: "api-gateway"}); err == nil {
		t.Fatal("other service must not resolve")
	}
	got, err := resolve.Execute(ctx, ResolveCredentialByOwnerInput{TenantID: "t1", OwnerID: "mcp:s:env:TOKEN",
		Category: domain.CategoryMcpExternalSecret, RequestingService: "mcp-service"})
	if err != nil || string(got) != "s3cret" {
		t.Fatalf("mcp-service resolve: %q %v", got, err)
	}
}

func TestOtherCategoriesRemainUnrestricted(t *testing.T) {
	for _, c := range []domain.Category{domain.CategoryScmOAuth, domain.CategoryDevServerAgentToken, domain.CategoryServiceSecret} {
		if !c.AllowsCaller("anyone") || !c.AllowsCaller("") {
			t.Errorf("%s must stay unrestricted", c)
		}
	}
}

func TestMcpExternalSecretMetadataReadsAreGated(t *testing.T) {
	rec := &callRecorder{}
	meta := newFakeMetadataRepo(rec)
	ctx := context.Background()
	if _, err := NewGetCredentialMetadataByOwner(meta).Execute(ctx, GetCredentialMetadataByOwnerInput{
		TenantID: "t1", OwnerID: "o", Category: domain.CategoryMcpExternalSecret, RequestingService: "x"}); err == nil {
		t.Error("metadata-by-owner must be gated")
	}
	if _, err := NewListCredentialsByCategory(meta).Execute(ctx, ListCredentialsByCategoryInput{
		TenantID: "t1", Category: domain.CategoryMcpExternalSecret, RequestingService: "x"}); err == nil {
		t.Error("list-by-category must be gated")
	}
}

func asAppError(err error, target **apperrors.AppError) bool {
	return errors.As(err, target)
}
