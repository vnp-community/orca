# TASK-133: Implement `project-service` domain/usecase/repository/grpc layers for `ListMembers`/`RemoveMember`/`UpdateMemberRole`

**From Solution:** SOL-020
**Priority:** P1
**Service:** `project-service`
**File:** `internal/domain/membership.go`, `internal/usecase/ports.go`, `internal/usecase/list_members.go` (new), `internal/usecase/remove_member.go` (new), `internal/usecase/update_member_role.go` (new), `internal/adapter/postgres/repository.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-132
**Status:** `[ ]` TODO

---

## Context

`domain/membership.go`'s `ProjectRole` doc comment already flags this as a
"documented follow-up once ListMembers/UpdateMemberRole/the '≥1 owner'
invariant are ported" — this task is that follow-up, built against the
**existing 2-role model** (`member`/`owner`; `project-service.md`'s 3-role
`viewer` extension is out of scope, a separate follow-on).

Authorization reuses the existing `projectActionOwnerOnly`/
`projectActionAnyMember` constants (`internal/usecase/authorization.go`) —
`RemoveMember`/`UpdateMemberRole` are owner-tier, `ListMembers` is
any-member-tier, matching `project-service.md` §9. No new action constant
or `project.rego` change is needed.

The ownerless-guard is a **pure function in `domain/`** (zero I/O, per
`03-clean-architecture-guidelines.md`) — the usecase does the `CountOwners`
read, the domain function decides.

## Changes to make

### Step 1 — `internal/domain/membership.go`: add the ownerless guard

Append:

```go
// ErrProjectWouldBeOwnerless is returned by AssertNotLastOwnerRemoval.
var ErrProjectWouldBeOwnerless = errors.New("domain: project must retain at least one owner")

// AssertNotLastOwnerRemoval enforces project-service.md §4's invariant:
// removing membership or demoting a role must never leave zero owners.
// targetRoleAfter is "" for a removal (no role after), or the new role for
// a demotion/promotion.
func AssertNotLastOwnerRemoval(currentOwnerCount int, targetIsCurrentlyOwner bool, targetRoleAfter ProjectRole) error {
	if targetIsCurrentlyOwner && targetRoleAfter != ProjectRoleOwner && currentOwnerCount <= 1 {
		return ErrProjectWouldBeOwnerless
	}
	return nil
}
```

### Step 2 — `internal/usecase/ports.go`: extend `ProjectRepository`

Find:

```go
	// GetMembership returns the caller's membership row for a project —
	// used by requireProjectAccess (authorization.go) to resolve
	// caller_project_role for the OPA policy check. Returns
	// domain.ErrMembershipNotFound (wrapped) if the user has no membership
	// row for this project; this is the normal "not a member" case, not an
	// error requireProjectAccess treats as a fetch failure.
	GetMembership(ctx context.Context, projectID, userID string) (domain.ProjectMember, error)
}
```

Replace with:

```go
	// GetMembership returns the caller's membership row for a project —
	// used by requireProjectAccess (authorization.go) to resolve
	// caller_project_role for the OPA policy check. Returns
	// domain.ErrMembershipNotFound (wrapped) if the user has no membership
	// row for this project; this is the normal "not a member" case, not an
	// error requireProjectAccess treats as a fetch failure.
	GetMembership(ctx context.Context, projectID, userID string) (domain.ProjectMember, error)
	// ListMembers returns every membership row for a project.
	ListMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error)
	// RemoveMember deletes one membership row. Returns
	// domain.ErrMembershipNotFound if none exists.
	RemoveMember(ctx context.Context, projectID, userID string) error
	// UpdateMemberRole changes one membership row's role. Returns
	// domain.ErrMembershipNotFound if none exists.
	UpdateMemberRole(ctx context.Context, projectID, userID string, role domain.ProjectRole) (domain.ProjectMember, error)
	// CountOwners is the read RemoveMember/UpdateMemberRole use to enforce
	// the "≥1 owner" invariant before mutating.
	CountOwners(ctx context.Context, projectID string) (int, error)
}
```

### Step 3 — `internal/usecase/list_members.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListMembersInput struct {
	ProjectID string
}

// ListMembers requires only membership (member or owner) — any-member tier,
// project-service.md §9's "ListMembers ... require[s] any membership."
type ListMembers struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewListMembers(repo ProjectRepository, opa OPAClient) *ListMembers {
	return &ListMembers{repo: repo, opa: opa}
}

func (uc *ListMembers) Execute(ctx context.Context, in ListMembersInput) ([]domain.ProjectMember, error) {
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionAnyMember); err != nil {
		return nil, err
	}
	members, err := uc.repo.ListMembers(ctx, in.ProjectID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_MEMBERS_FAILED", "failed to list project members", err)
	}
	return members, nil
}
```

### Step 4 — `internal/usecase/remove_member.go` (new)

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RemoveMemberInput struct {
	ProjectID string
	UserID    string
}

// RemoveMember requires owner (or global admin) — same tier as AddMember,
// project-service.md §9 — and enforces the "≥1 owner" invariant before
// mutating.
type RemoveMember struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewRemoveMember(repo ProjectRepository, opa OPAClient) *RemoveMember {
	return &RemoveMember{repo: repo, opa: opa}
}

func (uc *RemoveMember) Execute(ctx context.Context, in RemoveMemberInput) error {
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return err
	}

	target, err := uc.repo.GetMembership(ctx, in.ProjectID, in.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return apperrors.New(apperrors.KindNotFound, "PROJECT_MEMBERSHIP_NOT_FOUND", "membership does not exist", err)
		}
		return apperrors.New(apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED", "failed to look up membership", err)
	}

	owners, err := uc.repo.CountOwners(ctx, in.ProjectID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_COUNT_OWNERS_FAILED", "failed to count project owners", err)
	}
	if err := domain.AssertNotLastOwnerRemoval(owners, target.Role == domain.ProjectRoleOwner, ""); err != nil {
		return apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_WOULD_BE_OWNERLESS", err.Error(), err)
	}

	if err := uc.repo.RemoveMember(ctx, in.ProjectID, in.UserID); err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REMOVE_MEMBER_FAILED", "failed to remove project member", err)
	}
	return nil
}
```

### Step 5 — `internal/usecase/update_member_role.go` (new)

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type UpdateMemberRoleInput struct {
	ProjectID string
	UserID    string
	Role      domain.ProjectRole
}

// UpdateMemberRole requires owner (or global admin), same tier as
// RemoveMember, and enforces the "≥1 owner" invariant against the NEW role
// before mutating (a demotion away from owner is the only way this can
// trip; a promotion or a non-owner's role change never can).
type UpdateMemberRole struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewUpdateMemberRole(repo ProjectRepository, opa OPAClient) *UpdateMemberRole {
	return &UpdateMemberRole{repo: repo, opa: opa}
}

func (uc *UpdateMemberRole) Execute(ctx context.Context, in UpdateMemberRoleInput) (domain.ProjectMember, error) {
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.ProjectMember{}, err
	}
	if !in.Role.Valid() {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_ROLE", "invalid project role", nil)
	}

	target, err := uc.repo.GetMembership(ctx, in.ProjectID, in.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return domain.ProjectMember{}, apperrors.New(apperrors.KindNotFound, "PROJECT_MEMBERSHIP_NOT_FOUND", "membership does not exist", err)
		}
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED", "failed to look up membership", err)
	}

	owners, err := uc.repo.CountOwners(ctx, in.ProjectID)
	if err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_COUNT_OWNERS_FAILED", "failed to count project owners", err)
	}
	if err := domain.AssertNotLastOwnerRemoval(owners, target.Role == domain.ProjectRoleOwner, in.Role); err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_WOULD_BE_OWNERLESS", err.Error(), err)
	}

	member, err := uc.repo.UpdateMemberRole(ctx, in.ProjectID, in.UserID, in.Role)
	if err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_UPDATE_MEMBER_ROLE_FAILED", "failed to update member role", err)
	}
	return member, nil
}
```

### Step 6 — `internal/adapter/postgres/repository.go`: add `ListMembers`/`RemoveMember`/`UpdateMemberRole`/`CountOwners`

Append (after `GetMembership`):

```go
func (r *Repository) ListMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT project_id, user_id, role
		FROM project.project_members
		WHERE project_id = $1
		ORDER BY user_id
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query project members: %w", err)
	}
	defer rows.Close()

	var out []domain.ProjectMember
	for rows.Next() {
		var m domain.ProjectMember
		var role string
		if err := rows.Scan(&m.ProjectID, &m.UserID, &role); err != nil {
			return nil, fmt.Errorf("postgres: scan project member row: %w", err)
		}
		m.Role = domain.ProjectRole(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) RemoveMember(ctx context.Context, projectID, userID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM project.project_members WHERE project_id = $1 AND user_id = $2
	`, projectID, userID)
	if err != nil {
		return fmt.Errorf("postgres: delete project member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMembershipNotFound
	}
	return nil
}

func (r *Repository) UpdateMemberRole(ctx context.Context, projectID, userID string, role domain.ProjectRole) (domain.ProjectMember, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE project.project_members SET role = $3
		WHERE project_id = $1 AND user_id = $2
	`, projectID, userID, string(role))
	if err != nil {
		return domain.ProjectMember{}, fmt.Errorf("postgres: update project member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ProjectMember{}, domain.ErrMembershipNotFound
	}
	return domain.ProjectMember{ProjectID: projectID, UserID: userID, Role: role}, nil
}

// CountOwners is the read RemoveMember/UpdateMemberRole use to enforce the
// "≥1 owner" invariant before mutating — see usecase.AssertNotLastOwnerRemoval.
func (r *Repository) CountOwners(ctx context.Context, projectID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM project.project_members
		WHERE project_id = $1 AND role = $2
	`, projectID, string(domain.ProjectRoleOwner)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres: count project owners: %w", err)
	}
	return count, nil
}
```

### Step 7 — `internal/adapter/grpc/server.go`: register the 3 new RPC handlers

Add 3 fields to `Server`/`Deps`/`New`'s member-adjacent group (near
`addMember`), and a `toProtoRole` inverse of the existing `toDomainRole`:

```go
// Server struct: add, near addMember
	listMembers      *usecase.ListMembers
	removeMember     *usecase.RemoveMember
	updateMemberRole *usecase.UpdateMemberRole
```

Add matching params to `Deps`, `New(deps Deps)`'s struct literal, mechanically
identical to `addMember`'s existing wiring.

```go
func (s *Server) ListMembers(ctx context.Context, req *projectv1.ListMembersRequest) (*projectv1.ListMembersResponse, error) {
	members, err := s.listMembers.Execute(ctx, usecase.ListMembersInput{ProjectID: req.GetProjectId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.Member, 0, len(members))
	for _, m := range members {
		out = append(out, &projectv1.Member{UserId: m.UserID, Role: toProtoRole(m.Role)})
	}
	return &projectv1.ListMembersResponse{Members: out}, nil
}

func (s *Server) RemoveMember(ctx context.Context, req *projectv1.RemoveMemberRequest) (*projectv1.RemoveMemberResponse, error) {
	err := s.removeMember.Execute(ctx, usecase.RemoveMemberInput{
		ProjectID: req.GetProjectId(), UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.RemoveMemberResponse{}, nil
}

func (s *Server) UpdateMemberRole(ctx context.Context, req *projectv1.UpdateMemberRoleRequest) (*projectv1.UpdateMemberRoleResponse, error) {
	member, err := s.updateMemberRole.Execute(ctx, usecase.UpdateMemberRoleInput{
		ProjectID: req.GetProjectId(), UserID: req.GetUserId(), Role: toDomainRole(req.GetRole()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateMemberRoleResponse{Member: &projectv1.Member{UserId: member.UserID, Role: toProtoRole(member.Role)}}, nil
}

// toProtoRole is toDomainRole's inverse, for ListMembers/UpdateMemberRole's
// response mapping.
func toProtoRole(r domain.ProjectRole) projectv1.ProjectRole {
	switch r {
	case domain.ProjectRoleOwner:
		return projectv1.ProjectRole_PROJECT_ROLE_OWNER
	case domain.ProjectRoleMember:
		return projectv1.ProjectRole_PROJECT_ROLE_MEMBER
	default:
		return projectv1.ProjectRole_PROJECT_ROLE_UNSPECIFIED
	}
}
```

### Step 8 — `cmd/server/main.go`: wire the 3 new usecases

Find `addMemberUC := usecase.NewAddMember(repo, opa)` and add right after:

```go
listMembersUC := usecase.NewListMembers(repo, opa)
removeMemberUC := usecase.NewRemoveMember(repo, opa)
updateMemberRoleUC := usecase.NewUpdateMemberRole(repo, opa)
```

Add the 3 new fields to the `projectgrpc.Deps{...}` struct literal, next to
`AddMember: addMemberUC,`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./... && go vet ./...
```

Expected: clean build. `cmd/server/main.go` build failure until Step 8 is
applied is expected mid-task.
