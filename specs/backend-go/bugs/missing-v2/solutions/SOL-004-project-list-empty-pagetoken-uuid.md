# SOL-004: Fix BUG-004 — treat an empty `page_token` as "start from the beginning", not a literal UUID cursor

**Resolves:** BUG-004
**Service:** `project-service`
**Affected files:** `internal/adapter/postgres/repository.go` (`Repository.List`), `internal/usecase/list_projects.go` (optional — validation layer, see below)
**Priority:** Medium
**Status:** 🟡 Proposed — not yet implemented

---

## Grounding in `specs/backend-go/tdd/`

`crs/v0/standards/api-design-guidelines.md`'s "Request/response conventions":

> **Pagination: cursor-based (`page_token`/`next_page_token`), not
> offset-based** — stable under concurrent writes, standard for gRPC list
> APIs (matches Google's AIP-158).

[AIP-158](https://google.aip.dev/158) (the standard this doc cites by
name) is explicit that **an empty/unset `page_token` means "return the
first page"** — the convention `ListProjects`/`Repository.List` is
supposed to be implementing, but doesn't: `list_projects.go` passes
`in.PageToken` straight through to a raw `id > $2` comparison with no
empty-string special case, so "first page" (the overwhelmingly common
call shape — no caller has a cursor before their first call) is the one
input that breaks it.

`architecture/07-security-architecture.md`'s "Input validation" section
(*"Every gRPC message validated against `.proto`-declared constraints...
the delivery layer's job per Clean Architecture, not scattered validation
inside business logic"*) is the secondary grounding for *where* the fix
belongs: this is arguably a `protovalidate` gap too (nothing currently
constrains `page_token` to either be empty or a valid UUID before it
reaches the repository layer) — but the primary fix has to be
`Repository.List` actually implementing the stated cursor semantics
correctly, since a valid-shaped-but-nonexistent UUID cursor would need to
resolve to "empty result", not a query error either.

## Design

```go
// internal/adapter/postgres/repository.go — sketch
func (r *Repository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	var rows pgx.Rows
	var err error
	if pageToken == "" {
		// AIP-158: an empty/absent page_token means "from the beginning" —
		// no cursor comparison at all, not id > "".
		rows, err = r.pool.Query(ctx, `
			SELECT `+projectColumns+`
			FROM project.projects
			WHERE tenant_id = $1
			ORDER BY id
			LIMIT $2
		`, tenantID, pageSize)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT `+projectColumns+`
			FROM project.projects
			WHERE tenant_id = $1 AND id > $2
			ORDER BY id
			LIMIT $3
		`, tenantID, pageToken, pageSize)
	}
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query projects: %w", err)
	}
	defer rows.Close()
	// ... unchanged scan loop ...
}
```

### A malformed (non-empty, non-UUID) `page_token` should still fail cleanly

The two-branch fix above only handles the empty case — a client sending a
genuinely garbage cursor (`page_token: "not-a-uuid"`) would still hit the
same raw Postgres `invalid input syntax for type uuid` error, just less
often. Per the "Error model" section in the same `api-design-guidelines.md`
(*"gRPC status codes used per their canonical meaning... not everything
collapsed to `INTERNAL`"*), that case should resolve to `INVALID_ARGUMENT`
(a client error — bad input), not `INTERNAL` (a server error) — currently
both would map to the same `PROJECT_LIST_FAILED`/`Internal` this bug
report already flags as wrong. Recommend validating `page_token` is either
empty or a well-formed UUID **before** the repository call (in
`list_projects.go`'s `Execute`, alongside the existing `pageSize` clamping
— matches this usecase's own existing pattern of doing lightweight
input-shaping there rather than in the repository):

```go
// list_projects.go — sketch addition
if in.PageToken != "" {
	if _, err := uuid.Parse(in.PageToken); err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_PAGE_TOKEN", "page_token must be empty or a valid cursor", err)
	}
}
```

### Same bug class may recur elsewhere — worth a repo-wide check

`Repository.List`'s exact `id > $2`-with-unchecked-`pageToken` shape is
plausibly copy-pasted into other cursor-paginated list methods in this and
other services (`ListWorktrees`, `ListRepos`, etc., if they share this
pagination convention) — this report only confirms `project.list`'s
instance (the one live-reproduced); a `grep -rn "id > \$" --include=*.go`
sweep across `backend-go` for the same unguarded-empty-cursor pattern is
worth doing alongside this fix, not assumed to be the only occurrence.

## Testing Plan

- Unit test: `Repository.List(ctx, tenantID, "", pageSize)` against a
  tenant with N existing projects → returns the first `pageSize` rows, no
  error (the direct regression test for this bug).
- Unit test: `Repository.List` with a real, valid cursor from a prior
  page's `next_page_token` → returns the next page correctly (unchanged
  behavior, guards against the fix breaking the working case).
- Unit test: `ListProjects.Execute` with a garbage (non-UUID, non-empty)
  `PageToken` → returns `PROJECT_INVALID_PAGE_TOKEN`/`InvalidArgument`,
  and the repository is never called (validation short-circuits before
  the DB call, matching Clean Architecture's "delivery layer validates,
  business logic doesn't re-check" split this doc set repeatedly
  emphasizes — though here it's the usecase layer since there's no proto
  `protovalidate` annotation covering this yet).
- Re-run `tests/client/rpc-catalog.spec.ts`'s `project.list` case (called
  with no `pageToken`, its natural default) against the fixed deployment —
  should move from `PROJECT_LIST_FAILED` to a real (possibly empty) list.
