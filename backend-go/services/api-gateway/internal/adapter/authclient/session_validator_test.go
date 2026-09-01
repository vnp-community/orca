package authclient

import (
	"context"
	"testing"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// TestValidateToken_PropagatesRole guards the exact bug CR-DS-006 Phase 2
// fixed: auth-service's ValidateSession response carries a real Role, but
// it used to be silently dropped when converting to wscompat.Identity.
func TestValidateToken_PropagatesRole(t *testing.T) {
	tests := []struct {
		name      string
		protoRole authv1.Role
		wantRole  string
	}{
		{"admin", authv1.Role_ROLE_ADMIN, "admin"},
		{"user", authv1.Role_ROLE_USER, "user"},
		{"unspecified", authv1.Role_ROLE_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeAuthServiceClient{
				validateSessionFunc: func(ctx context.Context, in *authv1.ValidateSessionRequest) (*authv1.ValidateSessionResponse, error) {
					return &authv1.ValidateSessionResponse{
						Valid: true,
						User:  &authv1.User{Id: "u1", TenantId: "t1", Role: tt.protoRole},
					}, nil
				},
			}
			v := New(fake)

			got, err := v.ValidateToken(context.Background(), "sometoken")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.TenantID != "t1" || got.UserID != "u1" {
				t.Errorf("unexpected identity: %+v", got)
			}
			if got.Role != tt.wantRole {
				t.Errorf("want Role=%q, got %q", tt.wantRole, got.Role)
			}
		})
	}
}

func TestValidateToken_InvalidSessionErrors(t *testing.T) {
	fake := &fakeAuthServiceClient{
		validateSessionFunc: func(ctx context.Context, in *authv1.ValidateSessionRequest) (*authv1.ValidateSessionResponse, error) {
			return &authv1.ValidateSessionResponse{Valid: false}, nil
		},
	}
	v := New(fake)

	if _, err := v.ValidateToken(context.Background(), "bad-token"); err == nil {
		t.Fatal("expected an error for an invalid session")
	}
}
