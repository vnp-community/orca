package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// fakeTenantServiceClient is a minimal test double for
// tenantv1.TenantServiceClient — embeds the (nil) interface so it satisfies
// every method, and overrides only the ones this file's channel handlers
// actually call. Calling an unset method panics on a nil-pointer deref,
// which is fine: no test here should ever reach one. Same pattern as
// fakeInfraFleetClient in channels_test.go.
type fakeTenantServiceClient struct {
	tenantv1.TenantServiceClient

	createTeamFunc       func(ctx context.Context, in *tenantv1.CreateTeamRequest) (*tenantv1.CreateTeamResponse, error)
	listTeamsFunc        func(ctx context.Context, in *tenantv1.ListTeamsRequest) (*tenantv1.ListTeamsResponse, error)
	addTeamMemberFunc    func(ctx context.Context, in *tenantv1.AddTeamMemberRequest) (*tenantv1.AddTeamMemberResponse, error)
	removeTeamMemberFunc func(ctx context.Context, in *tenantv1.RemoveTeamMemberRequest) (*emptypb.Empty, error)
	listTeamMembersFunc  func(ctx context.Context, in *tenantv1.ListTeamMembersRequest) (*tenantv1.ListTeamMembersResponse, error)
	getUserProfileFunc   func(ctx context.Context, in *tenantv1.GetUserProfileRequest) (*tenantv1.GetUserProfileResponse, error)
}

// GetUserProfile — CR-DS-007/CR-DS-008's devServer.listForUser/
// devServer.requestAccess channels (channels_dev_server_access_control.go).
func (f *fakeTenantServiceClient) GetUserProfile(ctx context.Context, in *tenantv1.GetUserProfileRequest, _ ...grpc.CallOption) (*tenantv1.GetUserProfileResponse, error) {
	return f.getUserProfileFunc(ctx, in)
}

func (f *fakeTenantServiceClient) CreateTeam(ctx context.Context, in *tenantv1.CreateTeamRequest, _ ...grpc.CallOption) (*tenantv1.CreateTeamResponse, error) {
	return f.createTeamFunc(ctx, in)
}

func (f *fakeTenantServiceClient) ListTeams(ctx context.Context, in *tenantv1.ListTeamsRequest, _ ...grpc.CallOption) (*tenantv1.ListTeamsResponse, error) {
	return f.listTeamsFunc(ctx, in)
}

func (f *fakeTenantServiceClient) AddTeamMember(ctx context.Context, in *tenantv1.AddTeamMemberRequest, _ ...grpc.CallOption) (*tenantv1.AddTeamMemberResponse, error) {
	return f.addTeamMemberFunc(ctx, in)
}

func (f *fakeTenantServiceClient) RemoveTeamMember(ctx context.Context, in *tenantv1.RemoveTeamMemberRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.removeTeamMemberFunc(ctx, in)
}

func (f *fakeTenantServiceClient) ListTeamMembers(ctx context.Context, in *tenantv1.ListTeamMembersRequest, _ ...grpc.CallOption) (*tenantv1.ListTeamMembersResponse, error) {
	return f.listTeamMembersFunc(ctx, in)
}

func TestTeamCreateChannel_Success(t *testing.T) {
	var gotReq *tenantv1.CreateTeamRequest
	var gotCtx context.Context
	fake := &fakeTenantServiceClient{
		createTeamFunc: func(ctx context.Context, in *tenantv1.CreateTeamRequest) (*tenantv1.CreateTeamResponse, error) {
			gotCtx = ctx
			gotReq = in
			return &tenantv1.CreateTeamResponse{Team: &tenantv1.Team{Id: "team-1", CompanyId: in.CompanyId, Name: in.Name, SettingsJson: in.SettingsJson}}, nil
		},
	}

	r := NewRegistry()
	registerTeamChannels(r, fake)

	args := argsJSON(t, map[string]string{"name": "Platform", "settingsJson": `{"a":1}`})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "team.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	team, ok := result.(*tenantv1.Team)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if team.Id != "team-1" {
		t.Errorf("expected team id team-1, got %q", team.Id)
	}
	if gotReq.CompanyId != "tenant-1" {
		t.Errorf("expected CompanyId=tenant-1 (from Identity.TenantID), got %q", gotReq.CompanyId)
	}
	if gotReq.Name != "Platform" {
		t.Errorf("expected Name=Platform, got %q", gotReq.Name)
	}
	if gotReq.SettingsJson != `{"a":1}` {
		t.Errorf("expected SettingsJson to round-trip, got %q", gotReq.SettingsJson)
	}
	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "tenant-1" || user != "user-1" {
		t.Errorf("expected AttachIdentity to stamp tenant-1/user-1 onto outgoing metadata, got tenant=%q user=%q", tenant, user)
	}
}

func TestTeamListChannel_Success(t *testing.T) {
	fake := &fakeTenantServiceClient{
		listTeamsFunc: func(ctx context.Context, in *tenantv1.ListTeamsRequest) (*tenantv1.ListTeamsResponse, error) {
			return &tenantv1.ListTeamsResponse{Teams: []*tenantv1.Team{
				{Id: "team-1", CompanyId: "tenant-1", Name: "Platform"},
				{Id: "team-2", CompanyId: "tenant-1", Name: "Growth"},
			}}, nil
		},
	}

	r := NewRegistry()
	registerTeamChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "team.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	teams, ok := result.([]*tenantv1.Team)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
}

// TestTeamListChannel_EmptyResult_ReturnsEmptyArrayNotNull is the direct
// regression test for BUG-005 (specs/backend-go/bugs/missing-v2/): an
// empty ListTeamsResponse leaves Teams as a nil slice, normalized to []
// by Registry.Dispatch before it reaches the frontend.
func TestTeamListChannel_EmptyResult_ReturnsEmptyArrayNotNull(t *testing.T) {
	fake := &fakeTenantServiceClient{
		listTeamsFunc: func(ctx context.Context, in *tenantv1.ListTeamsRequest) (*tenantv1.ListTeamsResponse, error) {
			return &tenantv1.ListTeamsResponse{}, nil // Teams left nil
		},
	}

	r := NewRegistry()
	registerTeamChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "team.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("expected [], got %s", b)
	}
}

func TestTeamAddMemberChannel_Success(t *testing.T) {
	var gotReq *tenantv1.AddTeamMemberRequest
	fake := &fakeTenantServiceClient{
		addTeamMemberFunc: func(ctx context.Context, in *tenantv1.AddTeamMemberRequest) (*tenantv1.AddTeamMemberResponse, error) {
			gotReq = in
			return &tenantv1.AddTeamMemberResponse{}, nil
		},
	}

	r := NewRegistry()
	registerTeamChannels(r, fake)

	// role is decoded (must not error) but intentionally dropped — it has
	// nowhere to go on AddTeamMemberRequest.
	args := argsJSON(t, map[string]any{"teamId": "team-1", "userId": "user-1", "role": "admin", "priority": 7})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "team.addMember", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ok, isMap := result.(map[string]bool)
	if !isMap || !ok["ok"] {
		t.Fatalf("expected {ok: true}, got %#v", result)
	}
	if gotReq.TeamId != "team-1" || gotReq.UserId != "user-1" || gotReq.Priority != 7 {
		t.Fatalf("unexpected request: %+v", gotReq)
	}
}

func TestTeamRemoveMemberChannel_Success(t *testing.T) {
	var gotReq *tenantv1.RemoveTeamMemberRequest
	fake := &fakeTenantServiceClient{
		removeTeamMemberFunc: func(ctx context.Context, in *tenantv1.RemoveTeamMemberRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}

	r := NewRegistry()
	registerTeamChannels(r, fake)

	args := argsJSON(t, map[string]string{"teamId": "team-1", "userId": "user-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "team.removeMember", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ok, isMap := result.(map[string]bool)
	if !isMap || !ok["ok"] {
		t.Fatalf("expected {ok: true}, got %#v", result)
	}
	if gotReq.TeamId != "team-1" || gotReq.UserId != "user-1" {
		t.Fatalf("unexpected request: %+v", gotReq)
	}
}

func TestTeamListMembersChannel_Success(t *testing.T) {
	var gotReq *tenantv1.ListTeamMembersRequest
	fake := &fakeTenantServiceClient{
		listTeamMembersFunc: func(ctx context.Context, in *tenantv1.ListTeamMembersRequest) (*tenantv1.ListTeamMembersResponse, error) {
			gotReq = in
			return &tenantv1.ListTeamMembersResponse{Members: []*tenantv1.TeamMember{
				{UserId: "user-1", Priority: 1},
				{UserId: "user-2", Priority: 2},
			}}, nil
		},
	}

	r := NewRegistry()
	registerTeamChannels(r, fake)

	args := argsJSON(t, map[string]string{"teamId": "team-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "team.listMembers", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	members, isSlice := result.([]*tenantv1.TeamMember)
	if !isSlice || len(members) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if gotReq.TeamId != "team-1" {
		t.Fatalf("expected TeamId=team-1, got %q", gotReq.TeamId)
	}
}
