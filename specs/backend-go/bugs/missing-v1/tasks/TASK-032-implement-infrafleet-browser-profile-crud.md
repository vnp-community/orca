# TASK-032: Implement `BrowserProfile` migration, domain, repository, usecases, and gRPC wiring

**From Solution:** SOL-006 (Group C — metadata CRUD half)
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `migrations/0004_browser_profiles.up.sql` (new), `internal/domain/browser_profile.go` (new), `internal/usecase/{ports.go,list_browser_profiles.go,create_browser_profile.go,delete_browser_profile.go}` (new), `internal/adapter/postgres/browser_profile_repository.go` (new), `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-031
**Status:** `[x]` DONE (verified — `go build`/`go vet` clean for `services/infra-fleet-service/...`; usecase-level tests added in TASK-035 pass, e.g. `TestListBrowserProfiles_*`/`TestCreateBrowserProfile_*`/`TestDeleteBrowserProfile_*`. Postgres persistence split into a new `BrowserProfileStore` type in `internal/adapter/postgres/browser_profile_repository.go` rather than methods on `Repository` directly — `Repository.List`/`Create` already exist with different signatures for `DevServerRepository`/`ConnectionRepository`, same method-name-collision reasoning `SshTargetStore` already uses. No live Postgres in this environment, so the migration/SQL itself is unexercised beyond `go build`.)

---

## Context

Implements SOL-006's Group C SQL sketch and usecase pattern verbatim, plus
the gRPC handlers TASK-031's new RPCs need. A profile is tenant/dev-server-
scoped metadata (name, source browser, default flag) — never cookie/session
data itself, which stays entirely on the dev server's filesystem (see
TASK-034 for the 3 live-agent profile operations).

---

## Changes to make

### Step 1 — `migrations/0004_browser_profiles.up.sql` (new)

```sql
CREATE TABLE infra.browser_profiles (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  dev_server_id  UUID NOT NULL REFERENCES infra.dev_servers(id),
  name           TEXT NOT NULL,
  source_browser TEXT,               -- e.g. "chrome", "firefox" — set by profileImportFromBrowser
  is_default     BOOLEAN NOT NULL DEFAULT false,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_browser_profiles_tenant_dev_server ON infra.browser_profiles (tenant_id, dev_server_id);
```

`migrations/0004_browser_profiles.down.sql` (new):

```sql
DROP TABLE infra.browser_profiles;
```

### Step 2 — `internal/domain/browser_profile.go` (new)

```go
package domain

import "time"

// BrowserProfile is tenant/dev-server-scoped browser profile metadata — a
// profile's actual browser-data directory (cookies, local storage, etc.)
// lives on the dev server's filesystem, never in this struct or this
// service's database. See SOL-006 Group C.
type BrowserProfile struct {
	ID            string
	TenantID      string
	DevServerID   string
	Name          string
	SourceBrowser string // e.g. "chrome", "firefox" — empty if manually created
	IsDefault     bool
	CreatedAt     time.Time
}
```

### Step 3 — `internal/usecase/ports.go`: add `BrowserProfileRepository`

```go
// BrowserProfileRepository is the persistence port for browser profile
// metadata (infra.browser_profiles, TASK-032) — Postgres-only; the 3
// live-agent profile operations (profileClearDefaultCookies/
// profileDetectBrowsers/profileImportFromBrowser) do NOT go through this
// port, they relay via DevServerAgentClient/Relay instead (see TASK-034).
type BrowserProfileRepository interface {
	List(ctx context.Context, tenantID, devServerID string) ([]domain.BrowserProfile, error)
	Create(ctx context.Context, profile domain.BrowserProfile) (domain.BrowserProfile, error)
	Delete(ctx context.Context, tenantID, id string) error
}
```

### Step 4 — usecases (new files)

`list_browser_profiles.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type ListBrowserProfiles struct {
	repo BrowserProfileRepository
}

func NewListBrowserProfiles(repo BrowserProfileRepository) *ListBrowserProfiles {
	return &ListBrowserProfiles{repo: repo}
}

func (uc *ListBrowserProfiles) Execute(ctx context.Context, devServerID string) ([]domain.BrowserProfile, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if devServerID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_NO_DEV_SERVER", "dev_server_id is required", nil)
	}
	return uc.repo.List(ctx, tenantID, devServerID)
}
```

`create_browser_profile.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type CreateBrowserProfileInput struct {
	DevServerID   string
	Name          string
	SourceBrowser string
	IsDefault     bool
}

type CreateBrowserProfile struct {
	repo  BrowserProfileRepository
	newID func() string
}

func NewCreateBrowserProfile(repo BrowserProfileRepository, newID func() string) *CreateBrowserProfile {
	return &CreateBrowserProfile{repo: repo, newID: newID}
}

func (uc *CreateBrowserProfile) Execute(ctx context.Context, in CreateBrowserProfileInput) (domain.BrowserProfile, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.BrowserProfile{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if in.DevServerID == "" || in.Name == "" {
		return domain.BrowserProfile{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_BROWSER_PROFILE_INVALID", "dev_server_id and name are required", nil)
	}
	profile := domain.BrowserProfile{
		ID: uc.newID(), TenantID: tenantID, DevServerID: in.DevServerID,
		Name: in.Name, SourceBrowser: in.SourceBrowser, IsDefault: in.IsDefault,
	}
	return uc.repo.Create(ctx, profile)
}
```

`delete_browser_profile.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type DeleteBrowserProfile struct {
	repo BrowserProfileRepository
}

func NewDeleteBrowserProfile(repo BrowserProfileRepository) *DeleteBrowserProfile {
	return &DeleteBrowserProfile{repo: repo}
}

func (uc *DeleteBrowserProfile) Execute(ctx context.Context, id string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if id == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "INFRA_NO_BROWSER_PROFILE_ID", "id is required", nil)
	}
	return uc.repo.Delete(ctx, tenantID, id)
}
```

### Step 5 — `internal/adapter/postgres/browser_profile_repository.go` (new)

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

var _ usecase.BrowserProfileRepository = (*Repository)(nil)

func (r *Repository) List(ctx context.Context, tenantID, devServerID string) ([]domain.BrowserProfile, error) {
	const q = `
		SELECT id, tenant_id, dev_server_id, name, source_browser, is_default, created_at
		FROM infra.browser_profiles
		WHERE tenant_id = $1 AND dev_server_id = $2
		ORDER BY created_at`
	rows, err := r.pool.Query(ctx, q, tenantID, devServerID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing browser profiles: %w", err)
	}
	defer rows.Close()

	var profiles []domain.BrowserProfile
	for rows.Next() {
		var p domain.BrowserProfile
		var sourceBrowser *string
		if err := rows.Scan(&p.ID, &p.TenantID, &p.DevServerID, &p.Name, &sourceBrowser, &p.IsDefault, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scanning browser profile: %w", err)
		}
		if sourceBrowser != nil {
			p.SourceBrowser = *sourceBrowser
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (r *Repository) Create(ctx context.Context, profile domain.BrowserProfile) (domain.BrowserProfile, error) {
	const q = `
		INSERT INTO infra.browser_profiles (id, tenant_id, dev_server_id, name, source_browser, is_default)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		RETURNING id, tenant_id, dev_server_id, name, COALESCE(source_browser, ''), is_default, created_at`
	row := r.pool.QueryRow(ctx, q, profile.ID, profile.TenantID, profile.DevServerID, profile.Name, profile.SourceBrowser, profile.IsDefault)
	var p domain.BrowserProfile
	if err := row.Scan(&p.ID, &p.TenantID, &p.DevServerID, &p.Name, &p.SourceBrowser, &p.IsDefault, &p.CreatedAt); err != nil {
		return domain.BrowserProfile{}, fmt.Errorf("postgres: creating browser profile: %w", err)
	}
	return p, nil
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	const q = `DELETE FROM infra.browser_profiles WHERE tenant_id = $1 AND id = $2`
	tag, err := r.pool.Exec(ctx, q, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: deleting browser profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: browser profile %s not found", id)
	}
	return nil
}
```

### Step 6 — `internal/adapter/grpc/server.go`: wire the 3 RPCs

Add `listBrowserProfiles *usecase.ListBrowserProfiles`,
`createBrowserProfile *usecase.CreateBrowserProfile`,
`deleteBrowserProfile *usecase.DeleteBrowserProfile` fields to `Server`,
thread them through `New(...)`'s parameter list, and add 3 gRPC methods
translating request/response, following `CreateSshTarget`'s handler in the
same file for the exact shape.

### Step 7 — `cmd/server/main.go`: construct and wire the 3 usecases

Add `usecase.NewListBrowserProfiles(repo)`,
`usecase.NewCreateBrowserProfile(repo, uuid.NewString)`,
`usecase.NewDeleteBrowserProfile(repo)` alongside the other usecase
constructions, pass into `grpc.New(...)`'s extended parameter list.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go build ./... && go vet ./...
```

Expected: clean build. Usecase-level tests are added in TASK-035.
