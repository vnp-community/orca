# TASK-TG-003-01: Real `TeamScopeResolver` — wire `tenant-service`'s `ListTeamsForUser` (name confirmed, not `ListUserTeams`)

**From Solution:** BE-SOL-003
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go` (replace `StubTeamScopeResolver`), `backend-go/services/task-service/cmd/server/main.go` (dial `tenant-service`)
**Depends on:** None
**Status:** `[x]` DONE

---

## Context

BE-SOL-003 itself flags: *"Verify before implementing: confirm
`tenant-service` actually exposes `ListUserTeams` (or the real equivalent
RPC name)... the exact RPC name needs a direct check of `tenant-service`'s
proto before coding."* **That check has now been done — the assumed name is
wrong.** `tenant-service`'s real proto (`backend-go/proto/orca/tenant/v1/tenant.proto`,
confirmed by direct read) has no `ListUserTeams` RPC. The real RPC is:

```protobuf
rpc ListTeamsForUser(ListTeamsForUserRequest) returns (ListTeamsForUserResponse);

message ListTeamsForUserRequest {
  string user_id = 1;
  // company_id intentionally omitted — the scoping company comes from the
  // validated request context (tenant.RequireTenantID), never a
  // client-supplied field, per tenant-service.md §9.
}
message ListTeamsForUserResponse {
  repeated string team_ids = 1;
}
```

(`tenant.proto:221-231`, exact lines confirmed by direct read.) Note the
request has **only `user_id`** — no `tenant_id` field, unlike BE-SOL-003's
sketch which passes `TenantId: tenantID` — the tenant scope is derived
server-side from `tenant-service`'s own auth context on that call, the same
way every other cross-service call in this codebase carries tenant
identity via context metadata (interceptor-injected), not an explicit wire
field. Do not add a `tenant_id` field to the request this task sends.

The real `StubTeamScopeResolver` (`internal/adapter/grpcclient/team_scope_resolver.go:1-26`,
read in full) already documents its own fix in its doc comment (lines
11-17): *"Real wiring needs: a `tenantv1.TenantServiceClient` (gRPC) dialed
to `tenant-service`... Until that's wired, team-scoped grants
(`GrantLevelTeam`) will never match any caller."* This task wires exactly
that, with the corrected RPC name/shape above.

## Changes to make

**1. `internal/adapter/grpcclient/team_scope_resolver.go`** — replace the
stub:

```go
package grpcclient

import (
	"context"
	"fmt"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// TeamScopeResolver implements usecase.TeamScopeResolver against a real
// tenant-service gRPC client — replaces StubTeamScopeResolver (TASK-TG-003-01).
// The real RPC is ListTeamsForUser, NOT ListUserTeams (confirmed by direct
// read of tenant.proto — see this task's Context for the citation trail).
type TeamScopeResolver struct {
	tenant tenantv1.TenantServiceClient
}

func NewTeamScopeResolver(tenant tenantv1.TenantServiceClient) *TeamScopeResolver {
	return &TeamScopeResolver{tenant: tenant}
}

func (r *TeamScopeResolver) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
	// tenantID is accepted for usecase.TeamScopeResolver's port-shape
	// compatibility (ResolvePermission calls ResolveTeams(ctx, tenantID,
	// userID)) but is NOT sent on the wire — tenant-service derives the
	// scoping tenant from the validated request context itself, per
	// ListTeamsForUserRequest's own doc comment (tenant.proto:221-227).
	resp, err := r.tenant.ListTeamsForUser(ctx, &tenantv1.ListTeamsForUserRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("team_scope_resolver: list_teams_for_user: %w", err)
	}
	return resp.GetTeamIds(), nil
}
```

**2. `cmd/server/main.go`** — dial `tenant-service` (find the existing
dial-pattern for another cross-service client this service already has,
e.g. `infra-fleet-service`'s or `ai-provider-service`'s client dial in the
same file, and mirror it exactly — connection string/env var naming
convention, retry/backoff, TLS options); replace
`grpcclient.NewStubTeamScopeResolver()`'s wiring with
`grpcclient.NewTeamScopeResolver(tenantClient)`.

## Test plan

- `TeamScopeResolver.ResolveTeams` against a `tenant-service` test
  double/fake `TenantServiceClient` — asserts the request carries only
  `user_id` (regression test that a `tenant_id` field is never added back
  by accident) and that a caller who IS a member of the returned team IDs
  makes a `scope=team` grant match via `ResolvePermission`'s existing BFS
  walk (`domain.ResolveGrant`, unchanged by this task).
- gRPC error from `tenant-service` propagates as a wrapped error, not a
  silent empty list (regression test versus the stub's previous
  always-succeeds-with-nil behavior).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/grpcclient/... -run TestTeamScopeResolver -v
go test ./services/task-service/internal/usecase/... -run TestResolvePermission -v
```

Expected: clean build; `ResolveTeams` calls `ListTeamsForUser` (not
`ListUserTeams`) with only `user_id` set; a caller with a `team`-level grant
via team membership now resolves correctly through `ResolvePermission`
end-to-end (this was previously impossible — `StubTeamScopeResolver` always
returned `nil, nil`).

## Execution notes (2026-09-09)

Confirmed the live `tenant.proto` matches the task's citation exactly
(`ListTeamsForUser` at line 35, `ListTeamsForUserRequest`/`Response` at
221-231, `user_id`-only request). Replaced
`team_scope_resolver.go`'s `StubTeamScopeResolver` entirely with the real
`TeamScopeResolver` per the task's code sample, using
`withTenantMetadata(ctx)` (the same tenant-forwarding-via-outgoing-metadata
helper `AIProviderContextResolver`/`TechStackDetector` already use) rather
than a wire field — matches the doc comment's "tenant derived from the
validated request context" convention exactly. `NewStubTeamScopeResolver`
had exactly one caller (`main.go`); updated it to dial `tenant-service` (new
`TenantServiceAddr` config field, `TENANT_SERVICE_ADDR` env var defaulting
to `tenant-service:9090`, matching `auth-service`'s existing naming
convention for the same address) and wire the real resolver.

Added `fakeTenantServiceClient` to `grpcclient_test.go` (embeds
`tenantv1.TenantServiceClient`, same "panic on any unimplemented method"
convention as the existing `fakeAiProviderServiceClient`) and 3 new tests:
`TestTeamScopeResolver_ResolveTeams_CallsListTeamsForUserWithOnlyUserID`
(the task's own named regression test — asserts the request carries only
`user_id`), `TestTeamScopeResolver_ResolveErrorPropagates_NotSilentEmptyList`
(a real tenant-service error surfaces as a real error, not the stub's old
always-`nil,nil` behavior), and `TestTeamScopeResolver_NoTenantInContext`.
Did not add a dedicated `ResolvePermission`-end-to-end test with a
team-level grant beyond what already exists —
`TestResolvePermission_UsesTeamScopeResolverForTeamGrants` (pre-existing,
unaffected by this change since `usecase.ResolvePermission` itself has zero
changes here) already covers that BFS-walk behavior against a fake
`TeamScopeResolver` at the usecase layer, and this task's own scope is the
grpcclient adapter, not a new usecase-layer test.

Verify: `go build`/`go vet ./services/task-service/...` both clean; `go
test .../grpcclient/... -run TestTeamScopeResolver` — 3/3 pass; `go test
.../usecase/... -run TestResolvePermission` — all 8 pre-existing cases pass
unchanged; full `go test ./services/task-service/...` passes with no
regressions.
