package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeDevServerListRepository is a minimal DevServerRepository fake for
// ListDevServersForUser's tests — only List is exercised.
type fakeDevServerListRepository struct {
	servers []domain.DevServer
}

func (f *fakeDevServerListRepository) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *fakeDevServerListRepository) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *fakeDevServerListRepository) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	return f.servers, nil
}
func (f *fakeDevServerListRepository) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *fakeDevServerListRepository) FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *fakeDevServerListRepository) UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *fakeDevServerListRepository) AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}

func approvedServer(id, groupID string) domain.DevServer {
	return domain.DevServer{ID: id, TenantID: "tenant-1", Status: domain.DevServerStatusApproved, GroupID: groupID}
}

func TestListDevServersForUser_UngroupedNeverReturned(t *testing.T) {
	devServers := &fakeDevServerListRepository{servers: []domain.DevServer{
		approvedServer("ds1", ""), // ungrouped
	}}
	uc := NewListDevServersForUser(devServers, &fakeDevServerGroupRepository{}, &fakeDevServerGroupGrantRepository{})

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersForUserInput{DepartmentID: "dept1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no dev servers (ungrouped is admin-only), got %+v", got)
	}
}

func TestListDevServersForUser_NonApprovedNeverReturned(t *testing.T) {
	devServers := &fakeDevServerListRepository{servers: []domain.DevServer{
		{ID: "ds1", TenantID: "tenant-1", Status: domain.DevServerStatusPendingApproval, GroupID: "g1"},
	}}
	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {{ID: "g1", TenantID: "tenant-1", Name: "Backend"}},
	}}
	grants := &fakeDevServerGroupGrantRepository{byTenant: map[string][]domain.DevServerGroupGrant{
		"tenant-1": {{ID: "grant1", TenantID: "tenant-1", DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1"}},
	}}
	uc := NewListDevServersForUser(devServers, groups, grants)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersForUserInput{DepartmentID: "dept1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no dev servers (pending_approval, not approved), got %+v", got)
	}
}

func TestListDevServersForUser_DirectDepartmentGrantMatches(t *testing.T) {
	devServers := &fakeDevServerListRepository{servers: []domain.DevServer{approvedServer("ds1", "g1")}}
	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {{ID: "g1", TenantID: "tenant-1", Name: "Backend"}},
	}}
	grants := &fakeDevServerGroupGrantRepository{byTenant: map[string][]domain.DevServerGroupGrant{
		"tenant-1": {{ID: "grant1", TenantID: "tenant-1", DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1"}},
	}}
	uc := NewListDevServersForUser(devServers, groups, grants)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersForUserInput{DepartmentID: "dept1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ds1" {
		t.Errorf("expected ds1 to be returned, got %+v", got)
	}
}

func TestListDevServersForUser_TeamGrantMatches(t *testing.T) {
	devServers := &fakeDevServerListRepository{servers: []domain.DevServer{approvedServer("ds1", "g1")}}
	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {{ID: "g1", TenantID: "tenant-1", Name: "Backend"}},
	}}
	grants := &fakeDevServerGroupGrantRepository{byTenant: map[string][]domain.DevServerGroupGrant{
		"tenant-1": {{ID: "grant1", TenantID: "tenant-1", DevServerGroupID: "g1", GranteeKind: domain.GranteeKindTeam, GranteeID: "team1"}},
	}}
	uc := NewListDevServersForUser(devServers, groups, grants)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersForUserInput{TeamIDs: []string{"team1", "team2"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ds1" {
		t.Errorf("expected ds1 to be returned via team grant, got %+v", got)
	}
}

// TestListDevServersForUser_InheritsGrantFromParentGroup guards CR-DS-007
// §3's recorded "inherit down the tree" decision — a grant on the parent
// group must apply to every dev server in a child group too.
func TestListDevServersForUser_InheritsGrantFromParentGroup(t *testing.T) {
	devServers := &fakeDevServerListRepository{servers: []domain.DevServer{approvedServer("ds1", "child")}}
	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {
			{ID: "parent", TenantID: "tenant-1", Name: "Backend"},
			{ID: "child", TenantID: "tenant-1", Name: "Backend - Staging", ParentGroupID: "parent"},
		},
	}}
	grants := &fakeDevServerGroupGrantRepository{byTenant: map[string][]domain.DevServerGroupGrant{
		"tenant-1": {{ID: "grant1", TenantID: "tenant-1", DevServerGroupID: "parent", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1"}},
	}}
	uc := NewListDevServersForUser(devServers, groups, grants)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersForUserInput{DepartmentID: "dept1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ds1" {
		t.Errorf("expected ds1 to be returned via inherited parent grant, got %+v", got)
	}
}

func TestListDevServersForUser_NoMatchingGrantExcluded(t *testing.T) {
	devServers := &fakeDevServerListRepository{servers: []domain.DevServer{approvedServer("ds1", "g1")}}
	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {{ID: "g1", TenantID: "tenant-1", Name: "Backend"}},
	}}
	grants := &fakeDevServerGroupGrantRepository{byTenant: map[string][]domain.DevServerGroupGrant{
		"tenant-1": {{ID: "grant1", TenantID: "tenant-1", DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept-other"}},
	}}
	uc := NewListDevServersForUser(devServers, groups, grants)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersForUserInput{DepartmentID: "dept1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no dev servers (no matching grant), got %+v", got)
	}
}

// TestListDevServersForUser_FiltersByKind is CR-DS-009 §3.1's regression —
// same "empty kind = no filter" convention as ListDevServers.Execute.
func TestListDevServersForUser_FiltersByKind(t *testing.T) {
	devServer := approvedServer("ds1", "g1")
	devServer.Kind = domain.AgentKindMobileEmulator
	devServers := &fakeDevServerListRepository{servers: []domain.DevServer{devServer}}
	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {{ID: "g1", TenantID: "tenant-1", Name: "Backend"}},
	}}
	grants := &fakeDevServerGroupGrantRepository{byTenant: map[string][]domain.DevServerGroupGrant{
		"tenant-1": {{ID: "grant1", TenantID: "tenant-1", DevServerGroupID: "g1", GranteeKind: domain.GranteeKindDepartment, GranteeID: "dept1"}},
	}}
	uc := NewListDevServersForUser(devServers, groups, grants)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, ListDevServersForUserInput{DepartmentID: "dept1", Kind: domain.AgentKindDevServer})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no dev servers (kind filter excludes mobile-emulator row), got %+v", got)
	}

	got, err = uc.Execute(ctx, ListDevServersForUserInput{DepartmentID: "dept1", Kind: domain.AgentKindMobileEmulator})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ds1" {
		t.Errorf("expected ds1 to be returned when kind filter matches, got %+v", got)
	}
}

func TestListDevServersForUser_RequiresTenantContext(t *testing.T) {
	uc := NewListDevServersForUser(&fakeDevServerListRepository{}, &fakeDevServerGroupRepository{}, &fakeDevServerGroupGrantRepository{})
	_, err := uc.Execute(context.Background(), ListDevServersForUserInput{})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
