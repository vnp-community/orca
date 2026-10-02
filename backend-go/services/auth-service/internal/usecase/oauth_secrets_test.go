package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
)

// Authorization codes, refresh tokens, access tokens and PKCE verifiers must
// not reach logs, audit entries or error text, on success or failure paths.
func TestSecretsNeverLoggedOrAudited(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	h := newOAuthHarness(t)
	ctx := context.Background()
	var errTexts []string
	note := func(err error) {
		if err != nil {
			errTexts = append(errTexts, err.Error())
		}
	}

	a := h.authorize()
	out := h.tokens(a)
	_, err := h.exchange.Execute(ctx, h.codeInput(a)) // replay
	note(err)
	_, err = h.exchange.Execute(ctx, h.refreshInput(out.RefreshToken)) // reuse-after-revoke path
	note(err)
	bad := h.codeInput(h.authorize())
	bad.CodeVerifier = a.verifier // wrong verifier
	_, err = h.exchange.Execute(ctx, bad)
	note(err)
	note(h.revoke.Execute(ctx, OAuthRevokeTokenInput{Token: out.AccessToken}))
	note(h.revoke.Execute(ctx, OAuthRevokeTokenInput{Token: out.RefreshToken}))

	auditJSON, _ := json.Marshal(h.audit.entries)
	haystacks := map[string]string{
		"logs": logs.String(), "audit": string(auditJSON), "errors": fmt.Sprint(errTexts),
	}
	secrets := map[string]string{
		"authorization code": a.code, "code verifier": a.verifier,
		"refresh token": out.RefreshToken, "access token": out.AccessToken,
	}
	for hn, hay := range haystacks {
		for sn, s := range secrets {
			if s != "" && bytes.Contains([]byte(hay), []byte(s)) {
				t.Errorf("%s leaked into %s", sn, hn)
			}
		}
	}
	if len(h.audit.entries) == 0 {
		t.Fatal("expected audit entries to have been recorded")
	}
}
