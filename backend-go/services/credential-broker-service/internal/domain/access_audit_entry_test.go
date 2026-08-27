package domain

import (
	"testing"
	"time"
)

func TestNewAccessAuditEntry_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name         string
		credentialID string
		accessor     string
		action       Action
		wantErr      error
	}{
		{"valid", "c1", "scm-integration-service", ActionResolve, nil},
		{"empty credential id", "", "scm-integration-service", ActionResolve, ErrEmptyCredentialID},
		{"empty accessor", "c1", "", ActionResolve, ErrEmptyAccessorService},
		{"invalid action", "c1", "scm-integration-service", Action("bogus"), ErrInvalidAction},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAccessAuditEntry(tt.credentialID, tt.accessor, tt.action, now)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAction_Valid(t *testing.T) {
	for _, a := range []Action{ActionWrite, ActionResolve, ActionRotate, ActionRevoke} {
		if !a.Valid() {
			t.Errorf("expected %s to be valid", a)
		}
	}
	if Action("push").Valid() {
		t.Error("expected an action outside this service's actual RPC surface to be invalid")
	}
}
