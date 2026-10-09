package usecase

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// TASK-E2E-017: only "no connection / unknown credential" is NOT_CONNECTED; outages stay 5xx.
func TestCredentialResolveError_Classification(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantKind apperrors.Kind
		wantCode string
	}{
		{"no connection row", fmt.Errorf("credential: resolving: %w", ErrConnectionNotFound), apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED"},
		{"broker unknown credential", fmt.Errorf("credential: %w", status.Error(codes.NotFound, "gone")), apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED"},
		{"broker down", fmt.Errorf("credential: %w", status.Error(codes.Unavailable, "down")), apperrors.KindUnavailable, "ISSUETRACKING_CREDENTIAL_LOOKUP_FAILED"},
		{"database failure", errors.New("postgres: get credential id: connection refused"), apperrors.KindInternal, "ISSUETRACKING_CREDENTIAL_LOOKUP_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := credentialResolveError(tc.err, "no credential")
			if got.Kind != tc.wantKind || got.Code != tc.wantCode {
				t.Errorf("got kind=%v code=%s, want kind=%v code=%s", got.Kind, got.Code, tc.wantKind, tc.wantCode)
			}
		})
	}
}

func TestGetIssue_ConnectionLookupOutageIsNotReportedAsNotConnected(t *testing.T) {
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: &fakeIssueTrackerProvider{}}
	credentials := &fakeCredentialResolver{resolveErr: errors.New("postgres: connection refused")}
	_, err := NewGetIssue(registry, credentials).Execute(withUser(withTenant(t.Context(), "tenant-1"), "user-1"), GetIssueInput{Provider: domain.ProviderJira, IssueID: "X-1"})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperrors.KindInternal {
		t.Fatalf("want Internal AppError, got %v", err)
	}
}
