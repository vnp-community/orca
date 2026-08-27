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
			_, err := NewAuditEntry(tt.id, tt.tenantID, "actor-1", tt.action, "user", "target-1", nil, "", tt.occurredAt)
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
	entry, err := NewAuditEntry("a1", "t1", "", "session.expired", "session", "session-1", nil, "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ActorID != "" {
		t.Errorf("expected empty ActorID, got %q", entry.ActorID)
	}
}

func TestNewAuditEntry_AllowsEmptyTargetTypeAndID(t *testing.T) {
	// A system-initiated event with no single resource target (e.g. the
	// session reaper's batch purge) is a valid domain state.
	entry, err := NewAuditEntry("a1", "t1", "", "session.reap_batch", "", "", nil, "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.TargetType != "" || entry.TargetID != "" {
		t.Errorf("expected empty TargetType/TargetID, got %q/%q", entry.TargetType, entry.TargetID)
	}
}

func TestNewAuditEntry_NilMetadataNormalizesToEmptyMap(t *testing.T) {
	entry, err := NewAuditEntry("a1", "t1", "actor-1", "user.login", "user", "u1", nil, "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Metadata == nil {
		t.Fatal("expected nil metadata to be normalized to an empty map, got nil")
	}
	if len(entry.Metadata) != 0 {
		t.Errorf("expected an empty map, got %+v", entry.Metadata)
	}
}

func TestNewAuditEntry_PreservesGivenMetadata(t *testing.T) {
	entry, err := NewAuditEntry("a1", "t1", "actor-1", "user.role_updated", "user", "u1",
		map[string]any{"from": "user", "to": "admin"}, "203.0.113.7", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Metadata["from"] != "user" || entry.Metadata["to"] != "admin" {
		t.Errorf("expected metadata to round-trip, got %+v", entry.Metadata)
	}
	if entry.IPAddress != "203.0.113.7" {
		t.Errorf("expected IPAddress to round-trip, got %q", entry.IPAddress)
	}
}
