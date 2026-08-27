# TASK-009: Tests for `Repository.List`'s empty-`page_token` fix and `ListProjects`'s new validation

**From Solution:** SOL-004
**Priority:** P1
**Service:** `project-service`
**File:** `internal/adapter/postgres/repository_test.go` (integration, real Postgres), `internal/usecase/list_projects_test.go` (new — no test file currently exists for this usecase)
**Depends on:** TASK-008
**Status:** `[ ]` TODO

---

## Context

`list_projects.go` currently has **no dedicated test file** at all
(confirmed — only `list_projects.go` exists in `internal/usecase/`, no
`list_projects_test.go`), and `Repository.List` has no test in
`repository_test.go` either (only `CreateAndGet`, `Get_FiltersByTenant`,
`UpdateDevServerID` exist there). Both gaps predate this bug — TASK-008's
fix needs both closed as part of landing it, not left for a future pass.

## Changes to make

### Step 1 — `internal/adapter/postgres/repository_test.go`: real-Postgres regression test

Add, following the file's existing `setupPool(t)` pattern (integration
build tag, real testcontainer):

```go
func TestRepository_List_EmptyPageToken_ReturnsFirstPage(t *testing.T) {
	pool := setupPool(t)
	repo := New(pool) // match this file's existing constructor call — check TestRepository_CreateAndGet_RoundTrips for the exact New(...) signature this package uses
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := repo.Create(ctx, domain.Project{
			ID: uuid.NewString(), TenantID: "tenant-list-1", Name: fmt.Sprintf("project-%d", i),
		})
		if err != nil {
			t.Fatalf("seeding project %d: %v", i, err)
		}
	}

	// The actual regression test for BUG-004: this previously errored with
	// "invalid input syntax for type uuid" because pageToken="" was bound
	// straight into `id > $2`.
	got, _, err := repo.List(ctx, "tenant-list-1", "", 10)
	if err != nil {
		t.Fatalf("List with empty pageToken: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 projects, got %d", len(got))
	}
}

func TestRepository_List_ValidCursor_ReturnsNextPage(t *testing.T) {
	pool := setupPool(t)
	repo := New(pool)
	ctx := context.Background()

	var ids []string
	for i := 0; i < 3; i++ {
		p, err := repo.Create(ctx, domain.Project{
			ID: uuid.NewString(), TenantID: "tenant-list-2", Name: fmt.Sprintf("project-%d", i),
		})
		if err != nil {
			t.Fatalf("seeding project %d: %v", i, err)
		}
		ids = append(ids, p.ID)
	}

	firstPage, next, err := repo.List(ctx, "tenant-list-2", "", 2)
	if err != nil {
		t.Fatalf("List (first page): %v", err)
	}
	if len(firstPage) != 2 || next == "" {
		t.Fatalf("expected 2 results and a non-empty cursor, got %d results, next=%q", len(firstPage), next)
	}

	// Guards the fix didn't break the already-working cursor path.
	secondPage, _, err := repo.List(ctx, "tenant-list-2", next, 2)
	if err != nil {
		t.Fatalf("List (second page, real cursor): %v", err)
	}
	if len(secondPage) != 1 {
		t.Errorf("expected 1 remaining project on the second page, got %d", len(secondPage))
	}
}
```

Check the exact repository constructor name/signature (`New(pool)` is a
guess matching this package's likely convention — confirm against
`TestRepository_CreateAndGet_RoundTrips`'s own setup code before using),
and add `"fmt"`/`"github.com/google/uuid"` to this file's imports if not
already present.

### Step 2 — `internal/usecase/list_projects_test.go` (new file): validation + saga-order tests

```go
package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestListProjects_EmptyPageToken_Succeeds(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, ListProjectsInput{})
	if err != nil {
		t.Fatalf("unexpected error with empty PageToken: %v", err)
	}
}

func TestListProjects_MalformedPageToken_ReturnsInvalidArgument(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, ListProjectsInput{PageToken: "not-a-uuid"})
	if err == nil {
		t.Fatal("expected an error for a malformed page_token")
	}
	appErr, ok := err.(*apperrors.Error) // adjust to this package's real apperrors type name/assertion shape if different
	if !ok {
		t.Fatalf("expected an *apperrors.Error, got %T", err)
	}
	if appErr.Code != "PROJECT_INVALID_PAGE_TOKEN" {
		t.Errorf("expected code PROJECT_INVALID_PAGE_TOKEN, got %q", appErr.Code)
	}
}

func TestListProjects_ValidUUIDPageToken_ReachesRepository(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	// A syntactically valid but nonexistent cursor should reach the
	// repository (fake or real) rather than being rejected by validation —
	// only non-UUID-shaped tokens should be.
	_, err := uc.Execute(ctx, ListProjectsInput{PageToken: "00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatalf("unexpected error with a well-formed (if nonexistent) cursor: %v", err)
	}
}
```

Check `apperrors`'s actual exported error type/field names in this
codebase (`grep -n 'type.*struct' common/apperrors/*.go`) before assuming
`*apperrors.Error{Code}` — adjust the type assertion to match; every other
usecase test file in this package presumably already has this pattern
somewhere (`grep -rn 'apperrors\.' internal/usecase/*_test.go` for a
precedent to copy instead of guessing).

## Verify

```bash
cd backend-go/services/project-service
go test ./internal/usecase/... -count=1 -v -run 'TestListProjects'
go test -tags=integration ./internal/adapter/postgres/... -count=1 -v -run 'TestRepository_List'
```

Expected: all new tests pass. The integration run requires Docker
available for `testcontainers-go` per this package's existing convention —
skip locally if unavailable and rely on CI, matching how the rest of this
package's `//go:build integration` tests are already handled.
