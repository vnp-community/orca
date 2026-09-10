package usecase

import (
	"context"
	"testing"
)

func TestListCliTokens_ScopedToCallerUser(t *testing.T) {
	repo := newFakeServiceTokenRepository()
	seedIssuedToken(t, repo, "jti-a1", "user-a")
	seedIssuedToken(t, repo, "jti-a2", "user-a")
	seedIssuedToken(t, repo, "jti-b1", "user-b")

	uc := NewListCliTokens(repo)
	out, err := uc.Execute(context.Background(), ListCliTokensInput{UserID: "user-a", CallerUserID: "user-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d tokens, want 2 (only user-a's)", len(out))
	}
	for _, tok := range out {
		if tok.UserID != "user-a" {
			t.Errorf("got token for user %q, want only user-a's tokens", tok.UserID)
		}
	}
}

func TestListCliTokens_RejectsCallerActingForAnotherUser(t *testing.T) {
	repo := newFakeServiceTokenRepository()
	seedIssuedToken(t, repo, "jti-b1", "user-b")

	uc := NewListCliTokens(repo)
	_, err := uc.Execute(context.Background(), ListCliTokensInput{UserID: "user-b", CallerUserID: "user-a"})
	if err == nil {
		t.Fatal("expected an error when CallerUserID != UserID")
	}
}

func TestListCliTokens_MissingCallerIdentityRejected(t *testing.T) {
	repo := newFakeServiceTokenRepository()

	uc := NewListCliTokens(repo)
	_, err := uc.Execute(context.Background(), ListCliTokensInput{UserID: "user-a", CallerUserID: ""})
	if err == nil {
		t.Fatal("expected an error when CallerUserID is empty")
	}
}
