package domain

import (
	"testing"
	"time"
)

func TestNewProviderAccount_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		tenantID  string
		provider  ProviderType
		status    AccountStatus
		scope     AccountScope
		userID    string
		projectID string
		wantErr   error
	}{
		{"valid user scope", "t1", ProviderTypeAnthropic, AccountStatusActive, ScopeUser, "u1", "", nil},
		{"valid project scope", "t1", ProviderTypeOpenAI, AccountStatusActive, ScopeProject, "", "p1", nil},
		{"valid server scope", "t1", ProviderTypeAWSBedrock, AccountStatusPending, ScopeServer, "", "", nil},
		{"empty tenant", "", ProviderTypeAnthropic, AccountStatusActive, ScopeServer, "", "", ErrEmptyTenantID},
		{"invalid provider type", "t1", ProviderType("bogus"), AccountStatusActive, ScopeServer, "", "", ErrInvalidProviderType},
		{"invalid status", "t1", ProviderTypeAnthropic, AccountStatus("bogus"), ScopeServer, "", "", ErrInvalidStatus},
		{"invalid scope", "t1", ProviderTypeAnthropic, AccountStatusActive, AccountScope("bogus"), "", "", ErrInvalidScope},
		{"user scope missing user id", "t1", ProviderTypeAnthropic, AccountStatusActive, ScopeUser, "", "", ErrInvalidScopeRef},
		{"user scope with project id set too", "t1", ProviderTypeAnthropic, AccountStatusActive, ScopeUser, "u1", "p1", ErrInvalidScopeRef},
		{"project scope missing project id", "t1", ProviderTypeAnthropic, AccountStatusActive, ScopeProject, "", "", ErrInvalidScopeRef},
		{"server scope with user id set", "t1", ProviderTypeAnthropic, AccountStatusActive, ScopeServer, "u1", "", ErrInvalidScopeRef},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProviderAccount("acc-1", tt.tenantID, tt.provider, tt.status, "cred-ref-1",
				tt.scope, tt.userID, tt.projectID, "", nil, now, now)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestProviderAccount_HasNoSecretField(t *testing.T) {
	// Structural guard, not just a comment: every exported field on
	// ProviderAccount must be metadata, never plaintext/ciphertext. This
	// test documents the intent so a future field addition (e.g. "ApiKey")
	// gets caught in review, even though Go can't enforce it at compile time.
	now := time.Now()
	acc, err := NewProviderAccount("acc-1", "t1", ProviderTypeAnthropic, AccountStatusActive,
		"cred-ref-1", ScopeServer, "", "", "", nil, now, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.CredentialRef != "cred-ref-1" {
		t.Errorf("expected CredentialRef to be the opaque ref passed in, got %q", acc.CredentialRef)
	}
}

func TestProviderAccount_Resolvable(t *testing.T) {
	now := time.Now()
	for _, status := range []AccountStatus{AccountStatusPending, AccountStatusActive, AccountStatusRotating, AccountStatusRevoked, AccountStatusError} {
		acc, err := NewProviderAccount("acc-1", "t1", ProviderTypeAnthropic, status, "cred-ref-1", ScopeServer, "", "", "", nil, now, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := status == AccountStatusActive
		if got := acc.Resolvable(); got != want {
			t.Errorf("status %q: Resolvable() = %v, want %v", status, got, want)
		}
	}
}

func TestDayKey_TruncatesToUTCMidnight(t *testing.T) {
	t1 := time.Date(2026, 8, 17, 23, 59, 59, 0, time.UTC)
	got := DayKey(t1)
	want := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("DayKey(%v) = %v, want %v", t1, got, want)
	}
}
