package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListDevServersForUserInput's DepartmentID/TeamIDs are supplied by the
// caller (api-gateway, having resolved them via
// tenant-service.GetResolvedProfile) — see the proto
// ListDevServersForUserRequest message's doc comment for why this service
// never looks them up itself.
type ListDevServersForUserInput struct {
	DepartmentID string
	TeamIDs      []string
}

// ListDevServersForUser is the department/team-filtered view CR-DS-007
// adds — NOT admin-gated (every authenticated tenant user calls this;
// admins wanting the unfiltered view use the existing ListDevServers RPC).
//
// This is also where CR-DS-006's approval_status finally gets enforced —
// only approval_status=approved dev servers are ever returned here (the
// two RPCs Phase 1/Phase 2 shipped before this, RegisterDevServer and
// ListDevServers, still return every dev server regardless of status —
// unchanged, so nothing already depending on them broke).
//
// Access rule (CR-DS-007 §3's recorded decisions): a dev server with no
// group is never returned (ungrouped = admin-only, via ListDevServers). A
// grouped dev server is returned if ANY ancestor group in its tree
// (inclusive of itself) has a grant matching the caller's department OR
// any of the caller's teams (OR, not AND) — see docs/crs/v2/dev-server/
// CR-DS-007-department-based-access-control.md §3.
type ListDevServersForUser struct {
	devServers DevServerRepository
	groups     DevServerGroupRepository
	grants     DevServerGroupGrantRepository
}

func NewListDevServersForUser(devServers DevServerRepository, groups DevServerGroupRepository, grants DevServerGroupGrantRepository) *ListDevServersForUser {
	return &ListDevServersForUser{devServers: devServers, groups: groups, grants: grants}
}

func (uc *ListDevServersForUser) Execute(ctx context.Context, in ListDevServersForUserInput) ([]domain.DevServer, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	allServers, err := uc.devServers.List(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_DEV_SERVERS_FAILED", "failed to list dev servers", err)
	}
	groups, err := uc.groups.List(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_DEV_SERVER_GROUPS_FAILED", "failed to list dev server groups", err)
	}
	grants, err := uc.grants.ListAll(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_GRANTS_FAILED", "failed to list grants", err)
	}

	parentOf := make(map[string]string, len(groups))
	for _, g := range groups {
		parentOf[g.ID] = g.ParentGroupID
	}
	grantsByGroup := make(map[string][]domain.DevServerGroupGrant, len(grants))
	for _, g := range grants {
		grantsByGroup[g.DevServerGroupID] = append(grantsByGroup[g.DevServerGroupID], g)
	}
	teamSet := make(map[string]bool, len(in.TeamIDs))
	for _, id := range in.TeamIDs {
		teamSet[id] = true
	}

	// accessible memoizes the ancestor-chain walk per groupID — every dev
	// server sharing a group re-checks the same chain otherwise.
	accessible := make(map[string]bool)
	var groupGrantsAccess func(groupID string, seen map[string]bool) bool
	groupGrantsAccess = func(groupID string, seen map[string]bool) bool {
		if groupID == "" || seen[groupID] {
			return false // cycle guard: a malformed parent loop must never hang this
		}
		if v, ok := accessible[groupID]; ok {
			return v
		}
		seen[groupID] = true

		result := false
		for _, grant := range grantsByGroup[groupID] {
			if grant.GranteeKind == domain.GranteeKindDepartment && grant.GranteeID == in.DepartmentID && in.DepartmentID != "" {
				result = true
				break
			}
			if grant.GranteeKind == domain.GranteeKindTeam && teamSet[grant.GranteeID] {
				result = true
				break
			}
		}
		if !result {
			if parent, ok := parentOf[groupID]; ok && parent != "" {
				result = groupGrantsAccess(parent, seen)
			}
		}
		accessible[groupID] = result
		return result
	}

	var out []domain.DevServer
	for _, ds := range allServers {
		if ds.Status != domain.DevServerStatusApproved {
			continue
		}
		if ds.GroupID == "" {
			continue // ungrouped — admin-only, see doc comment
		}
		if groupGrantsAccess(ds.GroupID, map[string]bool{}) {
			out = append(out, ds)
		}
	}
	return out, nil
}
