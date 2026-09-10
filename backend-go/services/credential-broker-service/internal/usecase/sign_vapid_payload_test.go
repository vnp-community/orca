package usecase

import (
	"context"
	"testing"
)

// TestSignVapidPayload_SignsViaTransit is a regression guard for a real bug
// found while implementing TASK-BE-NOTIF-010: this usecase used to call
// store.TransitEncrypt (Vault's transit/encrypt — opaque ciphertext, not a
// verifiable signature) instead of store.TransitSign (transit/sign — an
// actual asymmetric-key signature, what RFC 8292 VAPID needs). The fake's
// TransitSign output ("sig:...") is deliberately shaped differently from
// TransitEncrypt's ("vault:v1:...") so this test fails loudly if the
// usecase ever regresses back to calling the wrong method.
func TestSignVapidPayload_SignsViaTransit(t *testing.T) {
	rec := &callRecorder{}
	store := newFakeSecretStore(rec)

	uc := NewSignVapidPayload(store)
	sig, err := uc.Execute(context.Background(), SignVapidPayloadInput{
		TenantID: "tenant-1", Payload: []byte("vapid-jwt-payload"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "sig:vapid-signing-tenant-1:vapid-jwt-payload"
	if sig != want {
		t.Errorf("got %q, want %q", sig, want)
	}
	if len(rec.snapshot()) != 1 || rec.snapshot()[0] != "store.TransitSign" {
		t.Errorf("expected exactly one store.TransitSign call, got %v", rec.snapshot())
	}
}

func TestSignVapidPayload_RequiresTenantAndPayload(t *testing.T) {
	rec := &callRecorder{}
	store := newFakeSecretStore(rec)
	uc := NewSignVapidPayload(store)

	if _, err := uc.Execute(context.Background(), SignVapidPayloadInput{Payload: []byte("x")}); err == nil {
		t.Error("expected an error for missing tenant_id")
	}
	if _, err := uc.Execute(context.Background(), SignVapidPayloadInput{TenantID: "tenant-1"}); err == nil {
		t.Error("expected an error for empty payload")
	}
}

func TestSignVapidPayload_TransitErrorWrapped(t *testing.T) {
	rec := &callRecorder{}
	store := newFakeSecretStore(rec)
	store.signErr = context.DeadlineExceeded

	uc := NewSignVapidPayload(store)
	_, err := uc.Execute(context.Background(), SignVapidPayloadInput{TenantID: "tenant-1", Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected the Transit error to propagate")
	}
}
