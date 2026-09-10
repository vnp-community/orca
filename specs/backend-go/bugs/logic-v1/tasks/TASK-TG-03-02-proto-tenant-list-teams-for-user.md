# TASK-TG-03-02: Proto — add `ListTeamsForUser` to `tenant.proto` (scope addition to `tenant-service`)

**From Solution:** SOL-TG-03
**Priority:** P1
**Service:** `tenant-service`
**File:** `backend-go/proto/orca/tenant/v1/tenant.proto`
**Depends on:** none
**Status:** `[x]` DONE — ListTeamsForUser RPC added to tenant.proto (buf generate + buf breaking vs origin/main clean); ListTeamsForUser usecase reuses TeamRepository.ListUserTeamLayers, wired into tenant-service grpc server + main.go. go test ./internal/usecase/... -run TestListTeamsForUser passes.

---

## Context

`tenant-service`'s current RPC surface has `ListTeams` (company→teams) and
`ListTeamMembers` (team→members) but no user→teams query — the direction
`TeamScopeResolver` actually needs. Resolving it via the existing surface
would mean `task-service` calling `ListTeams` then `ListTeamMembers` per
team and filtering client-side — an N+1 fan-out on task-service's
grant-resolution hot path (`task-service.md §8`). `tenant-service.md:164`
already documents `idx_team_members_user(user_id)` existing specifically
"for cascade team-layer resolution" — this RPC is the direct, indexed query
that index was built for. Flagged as a scope addition to
`tenant-service.md`'s RPC surface, same as `SOL-009` flagged its own proto
extension.

## Changes to make

In `backend-go/proto/orca/tenant/v1/tenant.proto`, add to the
`TenantService` service block (after the existing `ListTeamMembers` line):

```protobuf
  rpc ListTeamsForUser(ListTeamsForUserRequest) returns (ListTeamsForUserResponse);
```

Append new messages (near `ListTeamMembersRequest`/`ListTeamMembersResponse`):

```protobuf
message ListTeamsForUserRequest {
  string tenant_id = 1;
  string user_id = 2;
}
message ListTeamsForUserResponse {
  repeated string team_ids = 1;
}
```

## Server-side implementation (same task — small enough not to split)

`tenant-service` already has the exact reverse-lookup query this RPC needs:
`TeamRepository.ListUserTeamLayers(ctx, companyID, userID)`
(`internal/adapter/postgres/team_repository.go:148-154`) joins
`tenant.team_members` to `tenant.teams` filtered by `tm.user_id` and
`t.company_id` — built for `ResolveProfile`'s team-layer fetch, but its
`SELECT` already returns exactly the team IDs `ListTeamsForUser` needs (via
`t.id`). Add a thin usecase reusing it rather than a new query:

Create `backend-go/services/tenant-service/internal/usecase/list_teams_for_user.go`:

```go
package usecase

import "context"

type ListTeamsForUser struct {
	teams TeamRepository
}

func NewListTeamsForUser(teams TeamRepository) *ListTeamsForUser {
	return &ListTeamsForUser{teams: teams}
}

func (uc *ListTeamsForUser) Execute(ctx context.Context, companyID, userID string) ([]string, error) {
	if userID == "" {
		return nil, nil
	}
	layers, err := uc.teams.ListUserTeamLayers(ctx, companyID, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(layers))
	for _, l := range layers {
		ids = append(ids, l.TeamID) // confirm domain.TeamSettingsLayer's exact field name at implementation time
	}
	return ids, nil
}
```

Wire the new usecase into `tenant-service`'s gRPC server
(`internal/adapter/grpc/server.go`, following `ListTeamMembers`'s existing
handler shape) and composition root (`cmd/server/main.go`).

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/... ./services/tenant-service/...
go test ./services/tenant-service/internal/usecase/... -run TestListTeamsForUser -v
```

Expected: clean build; `buf breaking` reports only an addition (new RPC, new
messages, no existing field/RPC changed); `ListTeamsForUser` returns the
right team IDs for a multi-team user and `nil` for an empty `user_id`.
