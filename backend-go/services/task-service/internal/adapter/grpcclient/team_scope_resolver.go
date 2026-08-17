// Package grpcclient holds task-service's outbound gRPC-client adapters for
// the three cross-service ports usecase/ports.go defines
// (TeamScopeResolver, SimpleExecutor, ComplexExecutor). All three are STUBS
// in this scaffold — see each file's doc comment for what real wiring
// needs, and this service's README for the consolidated list.
package grpcclient

import "context"

// StubTeamScopeResolver implements usecase.TeamScopeResolver as a stub that
// always reports no team memberships. Real wiring needs: a
// tenantv1.TenantServiceClient (gRPC) dialed to tenant-service, calling
// whatever RPC resolves "which teams is this user a member of, within this
// tenant" (task-service.md §2/§9 — task-service never reads
// tenant-service's team_members table directly). Until that's wired,
// team-scoped grants (GrantLevelTeam) will never match any caller.
type StubTeamScopeResolver struct{}

func NewStubTeamScopeResolver() *StubTeamScopeResolver {
	return &StubTeamScopeResolver{}
}

func (s *StubTeamScopeResolver) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
	return nil, nil
}
