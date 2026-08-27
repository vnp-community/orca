# TASK-PRF-03-06: Scope `ListProjects` to membership and add `allowedServerTags` RBAC filter

**From Solution:** SOL-PRF-03
**Priority:** P1
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/list_projects.go`
**Depends on:** TASK-PRF-03-02, TASK-PRF-03-03
**Status:** `[ ]` TODO

---

## Context

`ListProjects.Execute` today calls `uc.repo.List(ctx, tenantID, ...)` — every
tenant project regardless of the caller's membership, a gap independent of
(but compounding) the missing visibility filter. This task closes both:
scope to `ListForMember`, then for non-admin/non-lead callers filter by
`fleet.allowedServerTags` intersection against each project's dev server
tags (per BL-PRF-03's `hasServerAccess`).

## Changes to make

In `backend-go/services/project-service/internal/usecase/list_projects.go`:

```go
type ListProjects struct {
	repo     ProjectRepository
	profiles ProfileResolver // NEW
}

func NewListProjects(repo ProjectRepository, profiles ProfileResolver) *ListProjects {
	return &ListProjects{repo: repo, profiles: profiles}
}

func (uc *ListProjects) Execute(ctx context.Context, in ListProjectsInput) (ListProjectsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	// NEW — membership scope, was previously unscoped (returned every
	// tenant project regardless of caller membership).
	projects, next, err := uc.repo.ListForMember(ctx, tenantID, userID, in.PageToken, pageSize)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_FAILED", "failed to list projects", err)
	}

	role, _ := tenant.Role(ctx) // same known upstream gap as tenant-service's SOL-PRF-01 — "" fails to the SAFER (narrowest-filter) branch below
	if role == "admin" || role == "lead" {
		return ListProjectsOutput{Projects: projects, NextPageToken: next}, nil
	}

	// developer (or unknown/"" role): filter by allowedServerTags. Unlike
	// SOL-PRF-01's authz checks (single yes/no gate, fail closed = deny),
	// this is a per-item filter with no single gate to fail closed on — the
	// safe default for an unknown role is the narrowest membership filter,
	// not an empty result or an error.
	resolved, err := uc.profiles.GetResolvedProfile(ctx, tenantID, userID)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal, "PROJECT_PROFILE_RESOLVE_FAILED", "failed to resolve caller profile for visibility filtering", err)
	}
	allowedTags, hasRestriction := resolved.AllowedServerTags()

	filtered := projects[:0]
	for _, p := range projects {
		if !hasRestriction || p.DevServerID == "" {
			filtered = append(filtered, p)
			continue
		}
		tags, err := uc.profiles.DevServerTags(ctx, tenantID, p.DevServerID)
		if err == nil && tagsIntersect(tags, allowedTags) {
			filtered = append(filtered, p)
		}
	}
	return ListProjectsOutput{Projects: filtered, NextPageToken: next}, nil
}

// tagsIntersect reports whether a and b share at least one element.
func tagsIntersect(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, t := range a {
		set[t] = true
	}
	for _, t := range b {
		if set[t] {
			return true
		}
	}
	return false
}
```

`ListForMember`'s postgres implementation (in
`internal/adapter/postgres/repository.go` or wherever `ProjectRepository` is
implemented) is a join against `project_members` instead of `List`'s bare
`tenant_id` filter — implement alongside this task, following that file's
existing query style.

## Verify

```bash
cd /opt/repos/orca/backend-go
go vet ./services/project-service/internal/usecase/list_projects.go
go build ./services/project-service/internal/adapter/postgres/...
```

Add test cases per SOL-PRF-03's Test plan: fake `ProjectRepository.ListForMember`
returns only the caller's projects; role `"admin"`/`"lead"` -> no
`ProfileResolver` call at all (short-circuit); role `"developer"`/`""` ->
`ProfileResolver.GetResolvedProfile` called once, a project whose dev
server's tags don't intersect `allowedServerTags` is excluded, a project
with no `DevServerID` always passes through regardless of role. Full
build/test lands with TASK-PRF-03-07.
