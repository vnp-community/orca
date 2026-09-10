package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestSubmitReview_EmptyCommentsFailsBeforeProviderCall(t *testing.T) {
	github := &fakeProvider{}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	uc := NewSubmitReview(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), SubmitReviewParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r", PRNumber: 1,
		ReviewType: domain.ReviewTypeApprove, Comments: nil,
	})
	if err == nil {
		t.Fatal("expected an error for zero comments (BR-PI-10)")
	}
	if github.calls != 0 {
		t.Fatalf("expected no provider call for an empty comment set, got %d calls", github.calls)
	}
}

// TestSubmitReview_UnspecifiedReviewTypeResolvesToRequestChanges is
// BR-PI-11's regression guard — the fake provider must receive the
// resolved type, never the unspecified one.
func TestSubmitReview_UnspecifiedReviewTypeResolvesToRequestChanges(t *testing.T) {
	captured := &capturingReviewProvider{fakeProvider: &fakeProvider{}}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: captured}}
	uc := NewSubmitReview(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), SubmitReviewParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r", PRNumber: 1,
		ReviewType: domain.ReviewTypeUnspecified,
		Comments:   []domain.ReviewComment{{Path: "a.go", Line: 1, Body: "nit"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.lastReviewInput.Type != domain.ReviewTypeRequestChanges {
		t.Fatalf("expected REVIEW_TYPE_UNSPECIFIED to resolve to RequestChanges, got %q", captured.lastReviewInput.Type)
	}
}

func TestSubmitReview_ExplicitReviewTypePassedThroughUnchanged(t *testing.T) {
	captured := &capturingReviewProvider{fakeProvider: &fakeProvider{}}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: captured}}
	uc := NewSubmitReview(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), SubmitReviewParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r", PRNumber: 1,
		ReviewType: domain.ReviewTypeApprove,
		Comments:   []domain.ReviewComment{{Path: "a.go", Line: 1, Body: "nit"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.lastReviewInput.Type != domain.ReviewTypeApprove {
		t.Fatalf("expected the explicit review type to pass through unchanged, got %q", captured.lastReviewInput.Type)
	}
}

// capturingReviewProvider wraps fakeProvider, overriding SubmitReview to
// record the exact domain.ReviewInput it was called with.
type capturingReviewProvider struct {
	*fakeProvider
	lastReviewInput domain.ReviewInput
}

func (p *capturingReviewProvider) SubmitReview(ctx context.Context, cred Credential, repo string, prNumber int32, in domain.ReviewInput) (domain.Review, error) {
	p.lastReviewInput = in
	return p.fakeProvider.SubmitReview(ctx, cred, repo, prNumber, in)
}
