package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeAuthServiceClientForDirectory embeds the nil interface and overrides
// only ListTenantMemberDirectory — the one method
// registerAuthDirectoryChannels' handler calls, same pattern as
// fakeAuthServiceClientForAdmin (channels_admin_users_test.go).
type fakeAuthServiceClientForDirectory struct {
	authv1.AuthServiceClient

	listFunc func(context.Context, *authv1.ListTenantMemberDirectoryRequest) (*authv1.ListTenantMemberDirectoryResponse, error)
}

func (f *fakeAuthServiceClientForDirectory) ListTenantMemberDirectory(ctx context.Context, in *authv1.ListTenantMemberDirectoryRequest, _ ...grpc.CallOption) (*authv1.ListTenantMemberDirectoryResponse, error) {
	return f.listFunc(ctx, in)
}

// TestAuthListTenantMemberDirectoryChannel_DoesNotRequireAdmin is the whole
// point of this channel existing separately from admin.listUsers — an
// ordinary (non-admin) caller must be able to reach it.
func TestAuthListTenantMemberDirectoryChannel_DoesNotRequireAdmin(t *testing.T) {
	fake := &fakeAuthServiceClientForDirectory{
		listFunc: func(context.Context, *authv1.ListTenantMemberDirectoryRequest) (*authv1.ListTenantMemberDirectoryResponse, error) {
			return &authv1.ListTenantMemberDirectoryResponse{
				Members: []*authv1.TenantMemberDirectoryEntry{
					{Id: "u1", Name: "Alice", Email: "alice@example.com"},
				},
			}, nil
		},
	}
	r := NewRegistry()
	registerAuthDirectoryChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1", Role: "user"}, "auth.listTenantMemberDirectory", nil)
	if err != nil {
		t.Fatalf("unexpected error for a non-admin caller: %v", err)
	}
	views, ok := result.([]tenantMemberDirectoryEntryView)
	if !ok || len(views) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if views[0].ID != "u1" || views[0].Name != "Alice" || views[0].Email != "alice@example.com" {
		t.Errorf("unexpected entry: %+v", views[0])
	}
}

// TestAuthListTenantMemberDirectoryChannel_EmptyResultIsAnEmptySliceNotNil
// guards a common JSON-marshaling footgun: a nil Go slice marshals to
// `null`, not `[]`, which a frontend expecting an array to .map() over
// would choke on.
func TestAuthListTenantMemberDirectoryChannel_EmptyResultIsAnEmptySliceNotNil(t *testing.T) {
	fake := &fakeAuthServiceClientForDirectory{
		listFunc: func(context.Context, *authv1.ListTenantMemberDirectoryRequest) (*authv1.ListTenantMemberDirectoryResponse, error) {
			return &authv1.ListTenantMemberDirectoryResponse{}, nil
		},
	}
	r := NewRegistry()
	registerAuthDirectoryChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1", Role: "user"}, "auth.listTenantMemberDirectory", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]tenantMemberDirectoryEntryView)
	if !ok || views == nil {
		t.Fatalf("expected a non-nil empty slice, got %+v (%T)", result, result)
	}
}
