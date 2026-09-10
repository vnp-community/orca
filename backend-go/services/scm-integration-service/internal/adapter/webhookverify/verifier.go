// Package webhookverify implements usecase.WebhookVerifier — GitHub's
// X-Hub-Signature-256 (HMAC-SHA256 over the raw request body) and GitLab's
// X-Gitlab-Token (constant-time string compare), each checked against ONE
// shared secret per provider for this deployment. See
// usecase.WebhookVerifier's doc comment (ports.go) for why this isn't
// per-tenant yet — no per-tenant webhook URL/secret registration surface
// exists in this service.
package webhookverify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// Verifier implements usecase.WebhookVerifier.
type Verifier struct {
	githubSecret string
	gitlabToken  string
}

// New constructs a Verifier. An empty secret for a provider means every
// webhook for that provider is rejected — never a silently-open check.
func New(githubSecret, gitlabToken string) *Verifier {
	return &Verifier{githubSecret: githubSecret, gitlabToken: gitlabToken}
}

var _ usecase.WebhookVerifier = (*Verifier)(nil)

func (v *Verifier) Verify(_ context.Context, provider domain.ScmProvider, rawBody []byte, signatureHeader string) bool {
	switch provider {
	case domain.ScmProviderGitHub:
		return verifyGitHubSignature(v.githubSecret, rawBody, signatureHeader)
	case domain.ScmProviderGitLab:
		return verifyGitLabToken(v.gitlabToken, signatureHeader)
	default:
		// No other provider's webhook shape is understood by
		// webhook_parse.go yet (BUG-PI-03's scope: GitHub + GitLab merge
		// events only) — reject rather than accept-and-ignore.
		return false
	}
}

// verifyGitHubSignature checks GitHub's "sha256=<hex hmac>" signature
// header (https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries).
func verifyGitHubSignature(secret string, rawBody []byte, signatureHeader string) bool {
	if secret == "" || signatureHeader == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	want, err := hex.DecodeString(strings.TrimPrefix(signatureHeader, prefix))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	got := mac.Sum(nil)
	return hmac.Equal(got, want)
}

// verifyGitLabToken checks GitLab's X-Gitlab-Token header — a plain shared
// secret (no HMAC), constant-time compared to avoid a timing side channel.
func verifyGitLabToken(token, headerValue string) bool {
	if token == "" || headerValue == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(headerValue)) == 1
}
