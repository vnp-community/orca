# TASK-TG-03-03: Real `TeamScopeResolver` — dial `tenant-service`, replace the stub

**From Solution:** SOL-TG-03
**Priority:** P0 — the stub silently makes every `GrantLevelTeam` grant permanently inert with no error anywhere; correctness bug with no visible symptom short of an audit
**Service:** `task-service` (client) + `tenant-service` (server, see `TASK-TG-03-02`)
**File:** `backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go`
**Depends on:** TASK-TG-03-02
**Status:** `[x]` DONE — TeamScopeResolver now dials tenant-service.ListTeamsForUser for real (StubTeamScopeResolver kept but unwired); main.go wired with a new tenant-service client dial + TenantServiceAddr config. go test ./internal/adapter/grpcclient/... -run TestTeamScopeResolver and ./internal/usecase/... -run TestResolvePermission both pass.

---

## Context

`StubTeamScopeResolver.ResolveTeams` always returns `nil, nil` — an empty
team list, never an error. It satisfies the *interface* contract but
silently violates the *semantic* contract every `GrantLevelTeam` grant
depends on: a Lead grants "Backend Team: execute," the write succeeds, and
the grant is permanently unreachable because no caller's `CallerIdentity`
ever has that team in `TeamIDs`. This task replaces it with a real
`tenant-service` client call, preserving `ResolvePermission`'s existing
fail-closed posture (a resolver error already fails the whole request via
`apperrors.KindInternal` — that stays unchanged).

## Changes to make

Replace `backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go`:

```go
package grpcclient

import (
	"context"
	"fmt"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// TeamScopeResolver implements usecase.TeamScopeResolver for real, replacing
// StubTeamScopeResolver — calls tenant-service's ListTeamsForUser RPC
// (TASK-TG-03-02), never reading tenant-service's team_members table
// directly (task-service.md §2/§9's bounded-context rule).
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
```

Keep `StubTeamScopeResolver` in the same file for now (tests may still
reference it) but its doc comment should be updated to note it's no longer
wired into `main.go` — actual removal is a follow-up cleanup once nothing
references it.

Update `backend-go/services/task-service/cmd/server/main.go`: replace

```go
teamScopeResolver := taskgrpcclient.NewStubTeamScopeResolver()
```

with a real `tenant-service` dial (mirrors the existing
`infraFleetConn`/`aiProviderConn` pattern at `main.go:89-105`):

```go
tenantConn, err := taskgrpcclient.Dial(cfg.TenantServiceAddr)
if err != nil {
	return fmt.Errorf("dialing tenant-service: %w", err)
}
defer func() { _ = tenantConn.Close() }()
tenantClient := tenantv1.NewTenantServiceClient(tenantConn)
teamScopeResolver := taskgrpcclient.NewTeamScopeResolver(tenantClient)
```

Add `tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"` to
`main.go`'s imports, and `TenantServiceAddr string` to
`backend-go/services/task-service/internal/config`'s config struct
(mirroring `InfraFleetServiceAddr`/`AIProviderServiceAddr`'s existing
env-var-loading pattern) if not already present.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/grpcclient/... -run TestTeamScopeResolver -v
go test ./services/task-service/internal/usecase/... -run TestResolvePermission -v
```

Expected: new `team_scope_resolver_test.go` — fake `TenantServiceClient`:
`ResolveTeams` returns the RPC's team IDs verbatim; an RPC error propagates
as a wrapped error, not an empty list (regression guard against silently
reintroducing the stub's always-empty behavior). `TestResolvePermission_UsesTeamScopeResolverForTeamGrants`
(existing) still passes unchanged (it already exercises this port via a
fake, unaffected by the real implementation swap).
