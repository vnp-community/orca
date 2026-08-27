// Package grpcclient holds task-service's outbound gRPC-client adapters:
// ComplexExecutor is still a STUB (see its own file's doc comment for what
// real wiring needs, and this service's README for the consolidated list);
// SimpleExecutor, ProjectExecutionResolver, AICompleter,
// AIProviderContextResolver, and (as of TASK-TG-03-03) TeamScopeResolver
// are real, dialed against infra-fleet-service, ai-provider-service, and
// tenant-service respectively.
package grpcclient

import (
	"context"
	"fmt"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// TeamScopeResolver implements usecase.TeamScopeResolver for real,
// replacing StubTeamScopeResolver — calls tenant-service's
// ListTeamsForUser RPC (TASK-TG-03-02), never reading tenant-service's
// team_members table directly (task-service.md §2/§9's bounded-context
// rule).
type TeamScopeResolver struct {
	tenant tenantv1.TenantServiceClient
}

func NewTeamScopeResolver(client tenantv1.TenantServiceClient) *TeamScopeResolver {
	return &TeamScopeResolver{tenant: client}
}

func (r *TeamScopeResolver) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
	if userID == "" {
		return nil, nil // anonymous/system callers have no team membership — not an error
	}
	resp, err := r.tenant.ListTeamsForUser(ctx, &tenantv1.ListTeamsForUserRequest{TenantId: tenantID, UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: resolve team membership: %w", err)
	}
	return resp.GetTeamIds(), nil
}

// StubTeamScopeResolver implements usecase.TeamScopeResolver as a stub that
// always reports no team memberships. No longer wired into main.go as of
// TASK-TG-03-03 (superseded by the real TeamScopeResolver above) — kept
// here only because tests may still reference it directly; removal is a
// follow-up cleanup once nothing does.
type StubTeamScopeResolver struct{}

func NewStubTeamScopeResolver() *StubTeamScopeResolver {
	return &StubTeamScopeResolver{}
}

func (s *StubTeamScopeResolver) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
