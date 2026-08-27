# TASK-008: `Repository.List` treats an empty `page_token` as "first page"; `ListProjects` validates a non-empty one is a UUID

**From Solution:** SOL-004
**Priority:** P1
**Service:** `project-service`
**File:** `internal/adapter/postgres/repository.go`, `internal/usecase/list_projects.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`Repository.List`'s query binds `pageToken` straight into `id > $2`
(`id` is `UUID PRIMARY KEY`) with no empty-string check. Every first-page
call (no prior cursor — the overwhelmingly common case) passes
`pageToken=""`, which Postgres rejects as `invalid input syntax for type
uuid: ""`, surfacing as `PROJECT_LIST_FAILED` (BUG-004). AIP-158 (cited by
`crs/v0/standards/api-design-guidelines.md`) defines an empty `page_token`
as "start from the beginning."

## Changes to make

### Step 1 — `internal/adapter/postgres/repository.go`: branch on empty `pageToken`

Current code (`repository.go:74-104`):

```go
func (r *Repository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+projectColumns+`
		FROM project.projects
		WHERE tenant_id = $1 AND id > $2
		ORDER BY id
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query projects: %w", err)
	}
	defer rows.Close()

	var out []domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan project row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate project rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}
```

Replace with:

```go
func (r *Repository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	var rows pgx.Rows
	var err error
	if pageToken == "" {
		// AIP-158: an empty/absent page_token means "from the beginning" —
		// no cursor comparison at all. See specs/backend-go/bugs/missing-v2/BUG-004:
		// binding "" into `id > $2` (id is UUID) previously errored on
		// every first-page call.
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

	var out []domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan project row: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate project rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}
```

(`github.com/jackc/pgx/v5` is already imported in this file — `var rows
pgx.Rows` needs no new import.)

Per SOL-004's "same bug class may recur elsewhere" note: run
`grep -rn 'id > \$' --include=*.go internal/adapter/postgres/` in this
service (and consider the same grep across other services' `postgres/`
packages) to check for the identical unguarded-empty-cursor pattern in
other list methods before considering this fix complete — fix any other
occurrence found the same way, as part of this task, not a separate one.

### Step 2 — `internal/usecase/list_projects.go`: reject a malformed non-empty `page_token` as `InvalidArgument`

Current code (`list_projects.go:29-43`) is otherwise unchanged; add
validation between the tenant check and the repository call:

```go
func (uc *ListProjects) Execute(ctx context.Context, in ListProjectsInput) (ListProjectsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	if in.PageToken != "" {
		if _, err := uuid.Parse(in.PageToken); err != nil {
			return ListProjectsOutput{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_PAGE_TOKEN", "page_token must be empty or a valid cursor", err)
		}
	}

	pageSize := in.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	projects, next, err := uc.repo.List(ctx, tenantID, in.PageToken, pageSize)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_FAILED", "failed to list projects", err)
	}
	return ListProjectsOutput{Projects: projects, NextPageToken: next}, nil
}
```

Add `"github.com/google/uuid"` to this file's imports — confirmed not
currently imported in `list_projects.go` (it only imports `context`,
`common/apperrors`, `common/tenant`, and this service's own `domain`
package today); already a module dependency via other files in this
package (e.g. `create_project.go` uses `uuid.NewString()`), so no `go.mod`
change is needed, just the import line in this file.

## Verify

```bash
cd backend-go
go build ./services/project-service/...
go vet ./services/project-service/...
go test ./services/project-service/... -count=1
```

Expected: clean build, all existing tests pass. TASK-009 adds the tests
specific to this fix.
