// Package grpcclient holds task-service's outbound gRPC-client adapters:
// ComplexExecutor is still a STUB (see its own file's doc comment, and this
// service's README for the consolidated list); SimpleExecutor,
// ProjectExecutionResolver, AICompleter, AIProviderContextResolver,
// TechStackDetector, and (as of TASK-TG-003-01) TeamScopeResolver are real,
// dialed against infra-fleet-service, ai-provider-service, git-gateway-service,
// and tenant-service respectively.
package grpcclient

import (
	"context"
	"fmt"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// TeamScopeResolver implements usecase.TeamScopeResolver against a real
// tenant-service gRPC client — replaces the former StubTeamScopeResolver
// (TASK-TG-003-01). The real RPC is ListTeamsForUser, NOT ListUserTeams as
// BE-SOL-003's own sketch guessed — confirmed by direct read of
// tenant.proto:35,221-231.
type TeamScopeResolver struct {
	tenant tenantv1.TenantServiceClient
}

func NewTeamScopeResolver(tenant tenantv1.TenantServiceClient) *TeamScopeResolver {
	return &TeamScopeResolver{tenant: tenant}
}

// ResolveTeams calls tenant-service's ListTeamsForUser. tenantID is accepted
// for usecase.TeamScopeResolver's port-shape compatibility (ResolvePermission
// calls ResolveTeams(ctx, tenantID, userID)) but is NOT sent on the wire —
// ListTeamsForUserRequest has no tenant_id field; tenant-service derives the
// scoping company from the validated request context itself, per that
// message's own doc comment (tenant.proto:221-227). Do not add a tenant_id
// field to this request.
func (r *TeamScopeResolver) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := r.tenant.ListTeamsForUser(ctx, &tenantv1.ListTeamsForUserRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("team_scope_resolver: list_teams_for_user: %w", err)
	}
	return resp.GetTeamIds(), nil
}
