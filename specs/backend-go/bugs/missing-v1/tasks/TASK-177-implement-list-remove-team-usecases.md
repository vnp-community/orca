# TASK-177: Implement `ListTeams`/`RemoveTeamMember` usecases + Postgres + gRPC wiring

**From Solution:** SOL-028
**Priority:** P1
**Service:** `tenant-service`
**File:** `internal/usecase/ports.go`, `internal/usecase/list_teams.go` (new), `internal/usecase/remove_team_member.go` (new), `internal/adapter/postgres/team_repository.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-176
**Status:** `[x]` DONE — implemented in worktree `agent-aa8bd8599a599323a` (team/terminal/workflow/worktree pass, merged into `integration/missing-v1` as commit `baa34819a`); this task doc's own Status line was never updated by that implementing pass (a task-doc-capture gap, not a missing-code gap) — verified against the current merged code+tests during a later re-audit: build/vet/test clean.

---

## Context

`TeamRepository` (`internal/usecase/ports.go:50-60`) has `Create`/`Get`/
`AddMember`/`ListMembers`/`ListUserTeamLayers` but no `ListByCompany` or
`RemoveMember`. This task adds both, the two usecases that use them, and
wires the new RPCs through `internal/adapter/grpc/server.go` and
`cmd/server/main.go`.

## Changes to make

### Step 1 — `internal/usecase/ports.go`: extend `TeamRepository`

Find:

```go
type TeamRepository interface {
	Create(ctx context.Context, team domain.Team) (domain.Team, error)
	Get(ctx context.Context, companyID, id string) (domain.Team, bool, error)
	AddMember(ctx context.Context, member domain.TeamMember) error
	ListMembers(ctx context.Context, teamID string) ([]domain.TeamMember, error)
	ListUserTeamLayers(ctx context.Context, companyID, userID string) ([]domain.TeamSettingsLayer, error)
}
```

Replace with:

```go
type TeamRepository interface {
	Create(ctx context.Context, team domain.Team) (domain.Team, error)
	Get(ctx context.Context, companyID, id string) (domain.Team, bool, error)
	// ListByCompany backs ListTeams — every team row scoped to companyID,
	// same not-found-not-wrong-company posture as Get (tenant-service.md §9).
	ListByCompany(ctx context.Context, companyID string) ([]domain.Team, error)
	AddMember(ctx context.Context, member domain.TeamMember) error
	// RemoveMember deletes one (team_id, user_id) row — backs
	// RemoveTeamMember. Returns found=false (not an error) when no such row
	// existed, so the usecase can treat "already removed" as an idempotent
	// no-op, matching DELETE semantics elsewhere in this codebase.
	RemoveMember(ctx context.Context, teamID, userID string) (bool, error)
	ListMembers(ctx context.Context, teamID string) ([]domain.TeamMember, error)
	ListUserTeamLayers(ctx context.Context, companyID, userID string) ([]domain.TeamSettingsLayer, error)
}
```

### Step 2 — `internal/adapter/postgres/team_repository.go`: implement both methods

Add after `Get` (before `AddMember`):

```go
// ListByCompany backs usecase.ListTeams — every tenant.teams row scoped to
// companyID.
func (r *TeamRepository) ListByCompany(ctx context.Context, companyID string) ([]domain.Team, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, company_id, name, settings_json FROM tenant.teams WHERE company_id = $1
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query teams: %w", err)
	}
	defer rows.Close()

	var out []domain.Team
	for rows.Next() {
		var t domain.Team
		var settingsJSON string
		if err := rows.Scan(&t.ID, &t.CompanyID, &t.Name, &settingsJSON); err != nil {
			return nil, fmt.Errorf("postgres: scan team row: %w", err)
		}
		settings, err := unmarshalSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("postgres: unmarshal team settings: %w", err)
		}
		t.Settings = settings
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate team rows: %w", err)
	}
	return out, nil
}
```

Add after `AddMember` (before `ListMembers`):

```go
// RemoveMember deletes one tenant.team_members row — backs
// usecase.RemoveTeamMember. tag.RowsAffected()==0 means the row was
// already gone (idempotent no-op), not an error.
func (r *TeamRepository) RemoveMember(ctx context.Context, teamID, userID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM tenant.team_members WHERE team_id = $1 AND user_id = $2
	`, teamID, userID)
	if err != nil {
		return false, fmt.Errorf("postgres: delete team member: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
```

### Step 3 — `internal/usecase/list_teams.go` (new file)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// ListTeams lists every team in the caller's company — the missing read
// half of team.* CRUD (create/get exist; list never did). See
// tenant-service.md §3.
type ListTeams struct {
	teams TeamRepository
}

func NewListTeams(teams TeamRepository) *ListTeams { return &ListTeams{teams: teams} }

func (uc *ListTeams) Execute(ctx context.Context) ([]domain.Team, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}
	teams, err := uc.teams.ListByCompany(ctx, companyID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_TEAMS_FAILED", "failed to list teams", err)
	}
	return teams, nil
}
```

### Step 4 — `internal/usecase/remove_team_member.go` (new file)

Mirrors `AddTeamMember.Execute`'s shape (`internal/usecase/add_team_member.go`)
exactly, including its cache-invalidation obligation: tenant-service.md §8
requires every team-membership edit to invalidate that user's cached
`ResolvedProfile` — a removal is exactly as invalidation-relevant as an add.

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RemoveTeamMemberInput mirrors RemoveTeamMemberRequest 1:1.
type RemoveTeamMemberInput struct {
	TeamID string
	UserID string
}

// RemoveTeamMember deletes one team-membership row — the documented gap
// from services/tenant-service/README.md:101.
type RemoveTeamMember struct {
	teams        TeamRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
}

// NewRemoveTeamMember wires cache invalidation. invalidation may be nil
// (NATS unreachable at startup), same convention as NewAddTeamMember.
func NewRemoveTeamMember(teams TeamRepository, cache ProfileCache, invalidation CacheInvalidationPublisher) *RemoveTeamMember {
	return &RemoveTeamMember{teams: teams, cache: cache, invalidation: invalidation}
}

func (uc *RemoveTeamMember) Execute(ctx context.Context, in RemoveTeamMemberInput) error {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}
	if _, found, err := uc.teams.Get(ctx, companyID, in.TeamID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_TEAM_LOOKUP_FAILED", "failed to look up team", err)
	} else if !found {
		return apperrors.New(apperrors.KindNotFound, "TENANT_TEAM_NOT_FOUND", "team does not exist", nil)
	}
	if _, err := uc.teams.RemoveMember(ctx, in.TeamID, in.UserID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_REMOVE_TEAM_MEMBER_FAILED", "failed to remove team member", err)
	}
	// Same invalidation obligation as AddTeamMember.Execute (§8) — the
	// removed member's team-layer contribution to ResolveProfile changes.
	if uc.cache != nil {
		uc.cache.Invalidate(ctx, in.UserID)
	}
	if uc.invalidation != nil {
		_ = uc.invalidation.PublishProfileInvalidated(ctx, companyID, in.UserID)
	}
	return nil
}
```

### Step 5 — `internal/adapter/grpc/server.go`: wire both RPCs

Add two fields to `Server` and two params to `New` (alongside the existing
`listTeamMembers *usecase.ListTeamMembers` field/param):

```go
	listTeamMembers   *usecase.ListTeamMembers
	listTeams         *usecase.ListTeams
	removeTeamMember  *usecase.RemoveTeamMember
```

Update every call site of `grpc.New(...)` (only `cmd/server/main.go`) to
pass the two new usecases positionally at the end — see Step 6.

Add the two new handler methods (mirroring `ListTeamMembers`'s shape
exactly):

```go
func (s *Server) ListTeams(ctx context.Context, req *tenantv1.ListTeamsRequest) (*tenantv1.ListTeamsResponse, error) {
	teams, err := s.listTeams.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*tenantv1.Team, 0, len(teams))
	for _, t := range teams {
		proto, err := toProtoTeam(t)
		if err != nil {
			return nil, apperrors.ToGRPCStatus(err)
		}
		out = append(out, proto)
	}
	return &tenantv1.ListTeamsResponse{Teams: out}, nil
}

func (s *Server) RemoveTeamMember(ctx context.Context, req *tenantv1.RemoveTeamMemberRequest) (*emptypb.Empty, error) {
	err := s.removeTeamMember.Execute(ctx, usecase.RemoveTeamMemberInput{
		TeamID: req.GetTeamId(),
		UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}
```

Add the `emptypb` import to `server.go`'s import block:

```go
	"google.golang.org/protobuf/types/known/emptypb"
```

### Step 6 — `cmd/server/main.go`: construct and wire the two new usecases

Find where `addTeamMemberUC`/`listTeamMembersUC` (or equivalently-named
usecases) are constructed and passed into `tenantgrpc.New(...)`. Add,
alongside them:

```go
listTeamsUC := usecase.NewListTeams(teamRepo)
removeTeamMemberUC := usecase.NewRemoveTeamMember(teamRepo, profileCache, invalidationPublisher)
```

(`teamRepo`, `profileCache`, `invalidationPublisher` are whatever variable
names the existing `AddTeamMember`/`CreateTeam` construction already uses in
this file — reuse the same instances, do not construct new ones.)

Add both to the `tenantgrpc.New(...)` call's argument list, in the same
position `Server`'s struct/constructor now expects them (end of the
argument list, matching Step 5's field order).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/tenant-service
go build ./... && go vet ./...
```
