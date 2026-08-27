package webhookverify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func githubSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifier_GitHub_ValidSignatureAccepted(t *testing.T) {
	v := New("shh", "")
	body := []byte(`{"action":"closed"}`)
	ok := v.Verify(context.Background(), domain.ScmProviderGitHub, body, githubSignature("shh", body))
	if !ok {
		t.Fatal("expected a valid HMAC signature to be accepted")
	}
}

func TestVerifier_GitHub_WrongSecretRejected(t *testing.T) {
	v := New("shh", "")
	body := []byte(`{"action":"closed"}`)
	ok := v.Verify(context.Background(), domain.ScmProviderGitHub, body, githubSignature("wrong-secret", body))
	if ok {
		t.Fatal("expected a signature computed with the wrong secret to be rejected")
	}
}

func TestVerifier_GitHub_TamperedBodyRejected(t *testing.T) {
	v := New("shh", "")
	sig := githubSignature("shh", []byte(`{"action":"closed"}`))
	ok := v.Verify(context.Background(), domain.ScmProviderGitHub, []byte(`{"action":"tampered"}`), sig)
	if ok {
		t.Fatal("expected a signature over a different body to be rejected")
	}
}

func TestVerifier_GitHub_EmptyConfiguredSecretAlwaysRejects(t *testing.T) {
	v := New("", "")
	body := []byte(`{}`)
	ok := v.Verify(context.Background(), domain.ScmProviderGitHub, body, githubSignature("anything", body))
	if ok {
		t.Fatal("expected an empty configured secret to always reject, never silently accept")
	}
}

func TestVerifier_GitLab_MatchingTokenAccepted(t *testing.T) {
	v := New("", "gitlab-token")
	ok := v.Verify(context.Background(), domain.ScmProviderGitLab, []byte(`{}`), "gitlab-token")
	if !ok {
		t.Fatal("expected a matching X-Gitlab-Token to be accepted")
	}
}

func TestVerifier_GitLab_MismatchedTokenRejected(t *testing.T) {
	v := New("", "gitlab-token")
	ok := v.Verify(context.Background(), domain.ScmProviderGitLab, []byte(`{}`), "wrong-token")
	if ok {
		t.Fatal("expected a mismatched token to be rejected")
	}
}

func TestVerifier_UnsupportedProviderRejected(t *testing.T) {
	v := New("shh", "gitlab-token")
	ok := v.Verify(context.Background(), domain.ScmProviderBitbucket, []byte(`{}`), "anything")
	if ok {
		t.Fatal("expected an unsupported provider to be rejected")
	}
}
