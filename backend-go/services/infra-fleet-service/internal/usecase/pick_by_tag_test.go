package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestPickByTag_RequiresTenantContext(t *testing.T) {
	uc := NewPickByTag(&fakeDevServerGroupRepository{}, &fakeDevServerRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), "gpu-fleet")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestPickByTag_RequiresTag(t *testing.T) {
	uc := NewPickByTag(&fakeDevServerGroupRepository{}, &fakeDevServerRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected an error when tag is omitted")
	}
}

func TestPickByTag_NoMatchingGroupReturnsNoServerForTag(t *testing.T) {
	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {{ID: "grp-1", TenantID: "tenant-1", Name: "gpu-fleet"}},
	}}
	uc := NewPickByTag(groups, &fakeDevServerRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a tag with no matching group")
	}
}

// TestPickByTag_PicksFirstConnectedGroupMember covers the naive
// first-connected-match selection this task deliberately keeps simple —
// see PickByTag's doc comment for why load-balancing is out of scope.
func TestPickByTag_PicksFirstConnectedGroupMember(t *testing.T) {
	ctx := withTenant(context.Background(), "tenant-1")

	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {{ID: "grp-1", TenantID: "tenant-1", Name: "gpu-fleet"}},
	}}
	dsDisconnected, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "")
	dsDisconnected.GroupID = "grp-1"
	dsConnected, _ := domain.NewDevServer("ds-2", "tenant-1", "10.0.0.2", domain.ConnectionModeDirectWebSocket, "")
	dsConnected.GroupID = "grp-1"
	dsOtherGroup, _ := domain.NewDevServer("ds-3", "tenant-1", "10.0.0.3", domain.ConnectionModeDirectWebSocket, "")
	dsOtherGroup.GroupID = "grp-other"

	devServers := &fakeDevServerRepository{byID: map[string]domain.DevServer{
		"ds-1": dsDisconnected,
		"ds-2": dsConnected,
		"ds-3": dsOtherGroup,
	}}
	agent := &fakeDevServerAgentClient{} // isConnected defaults false; overridden per-call below via a wrapper

	resolver := &fakeConnectionResolver{
		byDevServerID: map[string]domain.DevServer{"ds-2": dsConnected},
		connByDevServer: map[string]domain.Connection{
			"ds-2": {ID: "conn-2", TenantID: "tenant-1", DevServerID: "ds-2"},
		},
	}

	uc := NewPickByTag(groups, devServers, resolver, &connectedOnlyAgentClient{connectedIDs: map[string]bool{"ds-2": true}, fakeDevServerAgentClient: agent})

	connID, err := uc.Execute(ctx, "gpu-fleet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if connID != "conn-2" {
		t.Errorf("expected conn-2 (the connected group member), got %q", connID)
	}
}

func TestPickByTag_NoConnectedMemberReturnsNoServerForTag(t *testing.T) {
	ctx := withTenant(context.Background(), "tenant-1")

	groups := &fakeDevServerGroupRepository{byTenant: map[string][]domain.DevServerGroup{
		"tenant-1": {{ID: "grp-1", TenantID: "tenant-1", Name: "gpu-fleet"}},
	}}
	ds, _ := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "")
	ds.GroupID = "grp-1"
	devServers := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}

	uc := NewPickByTag(groups, devServers, &fakeConnectionResolver{}, &fakeDevServerAgentClient{isConnected: false})

	_, err := uc.Execute(ctx, "gpu-fleet")
	if err == nil {
		t.Fatal("expected an error when no group member has a live connection")
	}
}

func TestPickByTag_GroupListErrorPropagates(t *testing.T) {
	groups := &fakeDevServerGroupRepository{listErr: errors.New("db down")}
	uc := NewPickByTag(groups, &fakeDevServerRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, "gpu-fleet")
	if err == nil {
		t.Fatal("expected the group repository error to propagate")
	}
}

// connectedOnlyAgentClient wraps fakeDevServerAgentClient to answer
// IsConnected per-devServerID (the shared fake only supports one global
// answer) — needed here since TestPickByTag_PicksFirstConnectedGroupMember
// exercises two group members with different connectivity.
type connectedOnlyAgentClient struct {
	*fakeDevServerAgentClient
	connectedIDs map[string]bool
}

func (c *connectedOnlyAgentClient) IsConnected(devServerID string) bool {
	return c.connectedIDs[devServerID]
}
