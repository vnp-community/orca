package usecase

import (
	"context"
	"testing"
)

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
	want := "vault:v1:vapid-signing-tenant-1:vapid-jwt-payload"
	if sig != want {
		t.Errorf("got %q, want %q", sig, want)
	}
	if len(rec.snapshot()) != 1 || rec.snapshot()[0] != "store.TransitEncrypt" {
		t.Errorf("expected exactly one store.TransitEncrypt call, got %v", rec.snapshot())
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
	store.encryptErr = context.DeadlineExceeded

	uc := NewSignVapidPayload(store)
	_, err := uc.Execute(context.Background(), SignVapidPayloadInput{TenantID: "tenant-1", Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected the Transit error to propagate")
	}
}
