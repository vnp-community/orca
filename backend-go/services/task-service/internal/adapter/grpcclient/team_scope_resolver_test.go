package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// fakeTenantServiceClient implements tenantv1.TenantServiceClient directly
// — same fake-the-generated-client-port convention as
// fakeInfraFleetServiceClient.
type fakeTenantServiceClient struct {
	tenantv1.TenantServiceClient // embed: panics on any unimplemented method, intentional for these tests

	listTeamsForUserResp *tenantv1.ListTeamsForUserResponse
	listTeamsForUserErr  error
	gotListTeamsForUser  *tenantv1.ListTeamsForUserRequest
}

func (f *fakeTenantServiceClient) ListTeamsForUser(ctx context.Context, in *tenantv1.ListTeamsForUserRequest, _ ...grpc.CallOption) (*tenantv1.ListTeamsForUserResponse, error) {
	f.gotListTeamsForUser = in
	if f.listTeamsForUserErr != nil {
		return nil, f.listTeamsForUserErr
	}
	return f.listTeamsForUserResp, nil
}

func TestTeamScopeResolver_ReturnsTeamIDsVerbatim(t *testing.T) {
	fake := &fakeTenantServiceClient{listTeamsForUserResp: &tenantv1.ListTeamsForUserResponse{TeamIds: []string{"team-a", "team-b"}}}
	r := NewTeamScopeResolver(fake)

	got, err := r.ResolveTeams(context.Background(), "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "team-a" || got[1] != "team-b" {
		t.Errorf("expected team IDs to pass through verbatim, got %+v", got)
	}
	if fake.gotListTeamsForUser.GetTenantId() != "tenant-1" || fake.gotListTeamsForUser.GetUserId() != "user-1" {
		t.Errorf("unexpected request: %+v", fake.gotListTeamsForUser)
	}
}

// TestTeamScopeResolver_RPCError_PropagatesNotEmptyList is the core
// regression guard against silently reintroducing StubTeamScopeResolver's
// always-empty behavior: an RPC error must surface as a real error, never
// as a silent empty team list.
func TestTeamScopeResolver_RPCError_PropagatesNotEmptyList(t *testing.T) {
	fake := &fakeTenantServiceClient{listTeamsForUserErr: errors.New("tenant-service unavailable")}
	r := NewTeamScopeResolver(fake)

	got, err := r.ResolveTeams(context.Background(), "tenant-1", "user-1")
	if err == nil {
		t.Fatal("expected an error to propagate from the RPC failure, not a silent empty list")
	}
	if got != nil {
		t.Errorf("expected a nil result alongside the error, got %+v", got)
	}
}

func TestTeamScopeResolver_EmptyUserID_SkipsRPCReturnsNilNoError(t *testing.T) {
	fake := &fakeTenantServiceClient{}
	r := NewTeamScopeResolver(fake)

	got, err := r.ResolveTeams(context.Background(), "tenant-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for an empty user_id, got %+v", got)
	}
	if fake.gotListTeamsForUser != nil {
		t.Error("expected the RPC not to be called for an empty user_id")
	}
}
