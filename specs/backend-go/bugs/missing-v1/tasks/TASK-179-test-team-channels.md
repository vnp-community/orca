# TASK-179: Tests for `team.*` usecases, Postgres repository, and `wscompat` channels

**From Solution:** SOL-028
**Priority:** P1
**Service:** `tenant-service` + `api-gateway`
**File:** `internal/usecase/list_teams_test.go` (new), `internal/usecase/remove_team_member_test.go` (new), `internal/adapter/postgres/team_repository_test.go` (extend), `services/api-gateway/internal/adapter/wscompat/channels_team_test.go` (new)
**Depends on:** TASK-176, TASK-177, TASK-178
**Status:** `[x]` DONE — implemented in worktree `agent-aa8bd8599a599323a` (team/terminal/workflow/worktree pass, merged into `integration/missing-v1` as commit `baa34819a`); this task doc's own Status line was never updated by that implementing pass (a task-doc-capture gap, not a missing-code gap) — verified against the current merged code+tests during a later re-audit: build/vet/test clean.

---

## Context

Covers `ListTeams`/`RemoveTeamMember`'s usecase logic, the two new
`TeamRepository` methods against a real Postgres (testcontainers-go), and
all 5 `team.*` `wscompat` channels against a fake `TenantServiceClient`.

## Changes to make

### `services/tenant-service/internal/usecase/list_teams_test.go` (new)

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
	"github.com/stablyai/orca-go/services/tenant-service/internal/usecase"
)

type fakeTeamRepoForList struct {
	byCompany map[string][]domain.Team
}

func (f *fakeTeamRepoForList) Create(context.Context, domain.Team) (domain.Team, error) { panic("unused") }
func (f *fakeTeamRepoForList) Get(context.Context, string, string) (domain.Team, bool, error) {
	panic("unused")
}
func (f *fakeTeamRepoForList) ListByCompany(_ context.Context, companyID string) ([]domain.Team, error) {
	return f.byCompany[companyID], nil
}
func (f *fakeTeamRepoForList) AddMember(context.Context, domain.TeamMember) error { panic("unused") }
func (f *fakeTeamRepoForList) RemoveMember(context.Context, string, string) (bool, error) {
	panic("unused")
}
func (f *fakeTeamRepoForList) ListMembers(context.Context, string) ([]domain.TeamMember, error) {
	panic("unused")
}
func (f *fakeTeamRepoForList) ListUserTeamLayers(context.Context, string, string) ([]domain.TeamSettingsLayer, error) {
	panic("unused")
}

func TestListTeams_ScopesByCompany(t *testing.T) {
	repo := &fakeTeamRepoForList{byCompany: map[string][]domain.Team{
		"company-a": {{ID: "team-1", CompanyID: "company-a"}, {ID: "team-2", CompanyID: "company-a"}},
		"company-b": {{ID: "team-3", CompanyID: "company-b"}},
	}}
	uc := usecase.NewListTeams(repo)

	ctx := tenant.WithTenantID(context.Background(), "company-a")
	teams, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams for company-a, got %d", len(teams))
	}
	for _, tm := range teams {
		if tm.CompanyID != "company-a" {
			t.Fatalf("cross-company leak: got team %+v while scoped to company-a", tm)
		}
	}
}

func TestListTeams_NoTenant_ReturnsUnauthenticated(t *testing.T) {
	repo := &fakeTeamRepoForList{}
	uc := usecase.NewListTeams(repo)
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected error for missing tenant in context")
	}
}
```

(Adjust `tenant.WithTenantID`'s exact name/signature to whatever
`common/tenant` actually exports — check `add_team_member_test.go` or
`create_team_test.go` for the real test helper used to inject a tenant ID
into `context.Context` in this package's existing tests, and use that
instead if the name above doesn't match.)

### `services/tenant-service/internal/usecase/remove_team_member_test.go` (new)

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
	"github.com/stablyai/orca-go/services/tenant-service/internal/usecase"
)

type fakeTeamRepoForRemove struct {
	teams          map[string]domain.Team // keyed by "companyID/teamID"
	removeCalled   bool
	removeFound    bool
	invalidated    []string
}

func (f *fakeTeamRepoForRemove) Create(context.Context, domain.Team) (domain.Team, error) { panic("unused") }
func (f *fakeTeamRepoForRemove) Get(_ context.Context, companyID, id string) (domain.Team, bool, error) {
	t, ok := f.teams[companyID+"/"+id]
	return t, ok, nil
}
func (f *fakeTeamRepoForRemove) ListByCompany(context.Context, string) ([]domain.Team, error) { panic("unused") }
func (f *fakeTeamRepoForRemove) AddMember(context.Context, domain.TeamMember) error            { panic("unused") }
func (f *fakeTeamRepoForRemove) RemoveMember(_ context.Context, teamID, userID string) (bool, error) {
	f.removeCalled = true
	return f.removeFound, nil
}
func (f *fakeTeamRepoForRemove) ListMembers(context.Context, string) ([]domain.TeamMember, error) {
	panic("unused")
}
func (f *fakeTeamRepoForRemove) ListUserTeamLayers(context.Context, string, string) ([]domain.TeamSettingsLayer, error) {
	panic("unused")
}

type fakeProfileCache struct{ invalidated []string }

func (f *fakeProfileCache) Get(context.Context, string) (domain.ResolvedProfile, bool) { return domain.ResolvedProfile{}, false }
func (f *fakeProfileCache) Set(context.Context, string, domain.ResolvedProfile, time.Duration) {}
func (f *fakeProfileCache) Invalidate(_ context.Context, userID string) {
	f.invalidated = append(f.invalidated, userID)
}

func TestRemoveTeamMember_RemovesExistingMember_InvalidatesCache(t *testing.T) {
	repo := &fakeTeamRepoForRemove{
		teams:       map[string]domain.Team{"company-a/team-1": {ID: "team-1", CompanyID: "company-a"}},
		removeFound: true,
	}
	cache := &fakeProfileCache{}
	uc := usecase.NewRemoveTeamMember(repo, cache, nil)

	ctx := tenant.WithTenantID(context.Background(), "company-a")
	err := uc.Execute(ctx, usecase.RemoveTeamMemberInput{TeamID: "team-1", UserID: "user-1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !repo.removeCalled {
		t.Fatal("expected RemoveMember to be called")
	}
	if len(cache.invalidated) != 1 || cache.invalidated[0] != "user-1" {
		t.Fatalf("expected cache invalidation for user-1, got %v", cache.invalidated)
	}
}

func TestRemoveTeamMember_NonMember_IsIdempotentNoOp(t *testing.T) {
	repo := &fakeTeamRepoForRemove{
		teams:       map[string]domain.Team{"company-a/team-1": {ID: "team-1", CompanyID: "company-a"}},
		removeFound: false, // RemoveMember found nothing to delete
	}
	uc := usecase.NewRemoveTeamMember(repo, &fakeProfileCache{}, nil)

	ctx := tenant.WithTenantID(context.Background(), "company-a")
	if err := uc.Execute(ctx, usecase.RemoveTeamMemberInput{TeamID: "team-1", UserID: "not-a-member"}); err != nil {
		t.Fatalf("expected no error for a no-op removal, got: %v", err)
	}
}

func TestRemoveTeamMember_TeamInAnotherCompany_ReturnsNotFound(t *testing.T) {
	repo := &fakeTeamRepoForRemove{
		teams: map[string]domain.Team{"company-b/team-1": {ID: "team-1", CompanyID: "company-b"}},
	}
	uc := usecase.NewRemoveTeamMember(repo, &fakeProfileCache{}, nil)

	ctx := tenant.WithTenantID(context.Background(), "company-a") // wrong company
	err := uc.Execute(ctx, usecase.RemoveTeamMemberInput{TeamID: "team-1", UserID: "user-1"})
	if err == nil {
		t.Fatal("expected TENANT_TEAM_NOT_FOUND for a team owned by a different company")
	}
}
```

Add `"time"` to this file's imports (needed by `fakeProfileCache.Set`'s
signature). Confirm `domain.ResolvedProfile` is the correct return type for
`ProfileCache.Get`/`Set` by checking `internal/domain/profile_resolution.go`
before finalizing — adjust the fake if the real port signature differs.

### `services/tenant-service/internal/adapter/postgres/team_repository_test.go`

Add two testcontainers-go cases following this file's existing pattern
(spin up Postgres, run migrations, exercise the repository against it):

- `TestTeamRepository_ListByCompany`: insert 2 teams for company A, 1 for
  company B; `ListByCompany(ctx, "company-a-id")` returns exactly the 2
  company-A teams.
- `TestTeamRepository_RemoveMember`: insert a team + member row;
  `RemoveMember` returns `(true, nil)` and the row is gone; calling it
  again on the same pair returns `(false, nil)`, not an error.

### `services/api-gateway/internal/adapter/wscompat/channels_team_test.go` (new)

One test per `team.*` channel against a fake `tenantv1.TenantServiceClient`
(embed `tenantv1.UnimplementedTenantServiceServer`-shaped fake or a
hand-rolled struct implementing the client interface — follow whatever
pattern `channels_test.go` already uses for `annotationv1`/`taskv1` fakes
in this package). Cases:

- `team.create` → calls `CreateTeam` with `CompanyId: id.TenantID`, returns
  the created team.
- `team.list` → calls `ListTeams`, returns the team slice.
- `team.addMember` → calls `AddTeamMember` with the decoded
  `teamId`/`userId`/`priority`; assert the `role` field decodes without
  error and is silently dropped (not forwarded to `AddTeamMemberRequest`).
- `team.removeMember` → calls `RemoveTeamMember`, returns `{"ok": true}`.
- `team.listMembers` → calls `ListTeamMembers`, returns the members slice.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/tenant-service
go test ./internal/usecase/... ./internal/adapter/postgres/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -count=1 -v
```
