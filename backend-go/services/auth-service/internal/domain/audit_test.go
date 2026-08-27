package domain

import (
	"testing"
	"time"
)

func TestNewAuditEntry_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		id         string
		tenantID   string
		action     string
		occurredAt time.Time
		wantErr    error
	}{
		{"valid", "a1", "t1", "user.login", now, nil},
		{"empty id", "", "t1", "user.login", now, ErrEmptyID},
		{"empty tenant", "a1", "", "user.login", now, ErrEmptyTenant},
		{"empty action", "a1", "t1", "", now, ErrEmptyAction},
		{"zero occurred at", "a1", "t1", "user.login", time.Time{}, ErrZeroOccurredAt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAuditEntry(tt.id, tt.tenantID, "actor-1", tt.action, "target-1", tt.occurredAt)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewAuditEntry_AllowsEmptyActorID(t *testing.T) {
	// A system-initiated event (e.g. the session reaper) has no actor —
	// that's a valid domain state, not an invariant violation.
	entry, err := NewAuditEntry("a1", "t1", "", "session.expired", "session-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ActorID != "" {
		t.Errorf("expected empty ActorID, got %q", entry.ActorID)
	}
}
