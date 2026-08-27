package domain

import (
	"testing"
	"time"
)

func TestNewUser_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		id       string
		tenantID string
		email    string
		role     Role
		wantErr  error
	}{
		{"valid", "u1", "t1", "a@example.com", RoleUser, nil},
		{"valid admin", "u1", "t1", "a@example.com", RoleAdmin, nil},
		{"empty id", "", "t1", "a@example.com", RoleUser, ErrEmptyID},
		{"empty tenant", "u1", "", "a@example.com", RoleUser, ErrEmptyTenant},
		{"empty email", "u1", "t1", "", RoleUser, ErrEmptyEmail},
		{"email missing @", "u1", "t1", "not-an-email", RoleUser, ErrInvalidEmail},
		{"invalid role", "u1", "t1", "a@example.com", Role("superuser"), ErrInvalidRole},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewUser(tt.id, tt.tenantID, tt.email, "Alice", tt.role, true, now)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewUser_PopulatesFields(t *testing.T) {
	now := time.Now()
	u, err := NewUser("u1", "t1", "a@example.com", "Alice", RoleAdmin, false, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != "u1" || u.TenantID != "t1" || u.Email != "a@example.com" || u.Name != "Alice" {
		t.Errorf("unexpected fields: %+v", u)
	}
	if u.Role != RoleAdmin {
		t.Errorf("expected role admin, got %v", u.Role)
	}
	if u.IsActive {
		t.Errorf("expected IsActive=false")
	}
}

func TestRole_Valid(t *testing.T) {
	if !RoleUser.Valid() || !RoleAdmin.Valid() {
		t.Error("expected RoleUser and RoleAdmin to be valid")
	}
	if Role("bogus").Valid() {
		t.Error("expected an unknown role to be invalid")
	}
}
