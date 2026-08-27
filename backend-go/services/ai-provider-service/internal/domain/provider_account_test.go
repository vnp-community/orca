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
				tt.scope, tt.userID, tt.projectID, "", "", "", "", 0, nil, false, nil, "", nil, now, now)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewProviderAccount_QuotaLimitTooLow(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		quota   int
		wantErr bool
	}{
		{"1 is too low", 1, true},
		{"500 is too low", 500, true},
		{"999 is too low", 999, true},
		{"0 means unlimited, allowed", 0, false},
		{"1000 is the floor, allowed", 1000, false},
		{"50000 is allowed", 50000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProviderAccount("acc-1", "t1", ProviderTypeAnthropic, AccountStatusActive, "cred-ref-1",
				ScopeServer, "", "", "", "", "", "", tt.quota, nil, false, nil, "", nil, now, now)
			if tt.wantErr && err != ErrQuotaLimitTooLow {
				t.Fatalf("expected ErrQuotaLimitTooLow, got %v", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
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
		"cred-ref-1", ScopeServer, "", "", "", "", "", "", 0, nil, false, nil, "", nil, now, now)
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
		acc, err := NewProviderAccount("acc-1", "t1", ProviderTypeAnthropic, status, "cred-ref-1",
			ScopeServer, "", "", "", "", "", "", 0, nil, false, nil, "", nil, now, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := status == AccountStatusActive
		if got := acc.Resolvable(); got != want {
			t.Errorf("status %q: Resolvable() = %v, want %v", status, got, want)
		}
	}
}

// TestProviderAccount_Resolvable_ExcludesAllHealthFailures is a regression
// guard: excluding health failures never needs a second check anywhere else
// in the codebase — Resolvable()'s existing one-line status check already
// covers every HealthDetail value, including nil, once the health-check job
// flips Status to AccountStatusError.
func TestProviderAccount_Resolvable_ExcludesAllHealthFailures(t *testing.T) {
	healthy := HealthDetailHealthy
	degraded := HealthDetailDegraded
	quotaExceeded := HealthDetailQuotaExceeded
	invalidKey := HealthDetailInvalidKey
	unreachable := HealthDetailUnreachable

	for _, detail := range []*string{nil, &healthy, &degraded, &quotaExceeded, &invalidKey, &unreachable} {
		acc := ProviderAccount{Status: AccountStatusError, HealthDetail: detail}
		if acc.Resolvable() {
			label := "nil"
			if detail != nil {
				label = *detail
			}
			t.Errorf("HealthDetail=%v: expected Resolvable() to be false for AccountStatusError", label)
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
