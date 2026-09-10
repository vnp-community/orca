package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// PickByTag picks a live, connected dev server whose group matches tag and
// resolves it down to a Relay-ready connection id — TASK-WF-002-04, added
// to close a real gap discovered while implementing workflow-service's
// TargetKindFleetTag (BE-SOL-002): this service had no "pick one server
// from a group" primitive.
//
// "tag" maps onto DevServerGroup.Name — this service has no separate
// tagging concept (confirmed live: internal/usecase/'s existing group
// files are all management primitives — create/assign/list/grant/revoke —
// none select/load-balance). DevServer.GroupID is the real membership
// field ListDevServerGroups' own callers already rely on.
//
// Selection is deliberately naive: the first group member with a live
// agent connection wins. Load-balancing (round-robin, least-loaded, etc.)
// is explicitly out of scope — BE-SOL-002 already deferred that choice,
// this task keeps deferring it. A correct-but-naive "first match" is an
// acceptable v1.
type PickByTag struct {
	groups     DevServerGroupRepository
	devServers DevServerRepository
	resolver   ConnectionResolver
	agent      DevServerAgentClient
}

func NewPickByTag(groups DevServerGroupRepository, devServers DevServerRepository, resolver ConnectionResolver, agent DevServerAgentClient) *PickByTag {
	return &PickByTag{groups: groups, devServers: devServers, resolver: resolver, agent: agent}
}

func (uc *PickByTag) Execute(ctx context.Context, tag string) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if tag == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "INFRA_PICK_BY_TAG_NO_TAG", "tag is required", nil)
	}

	groups, err := uc.groups.List(ctx, tenantID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "INFRA_LIST_DEV_SERVER_GROUPS_FAILED", "failed to list dev server groups", err)
	}
	groupID := ""
	for _, g := range groups {
		if g.Name == tag {
			groupID = g.ID
			break
		}
	}
	if groupID == "" {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "INFRAFLEET_NO_SERVER_FOR_TAG", "no dev server group matches this tag", nil)
	}

	servers, err := uc.devServers.List(ctx, tenantID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "INFRA_LIST_DEV_SERVERS_FAILED", "failed to list dev servers", err)
	}

	for _, s := range servers {
		if s.GroupID != groupID {
			continue
		}
		if !uc.agent.IsConnected(s.ID) {
			continue
		}
		connected, _, conn, err := uc.resolver.ResolveConnectionByDevServer(ctx, tenantID, s.ID)
		if err != nil || !connected {
			continue
		}
		return conn.ID, nil
	}
	return "", apperrors.New(apperrors.KindFailedPrecondition, "INFRAFLEET_NO_SERVER_FOR_TAG", "no connected server found for this tag", nil)
}
