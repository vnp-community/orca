# TASK-142: Add `project.project_host_setups` migration + `domain.HostSetup`

**From Solution:** SOL-022
**Priority:** P1
**Service:** `project-service`
**File:** `migrations/0007_project_host_setups.up.sql`/`.down.sql` (new; `0006` is TASK-138's — bump to `0007` if that task hasn't landed yet, otherwise use the next free number), `internal/domain/host_setup.go` (new)
**Depends on:** none (independent of TASK-141; both can land in either order — TASK-143 depends on both)
**Status:** `[x]` DONE — migration `0007_project_host_setups.{up,down}.sql` + `domain.HostSetup` implemented. Worktree `agent-a9271c5b2d89347e7`, uncommitted.

---

## Context

New table in `project-service`'s own database (schema `project`,
`05-data-architecture.md`'s database-per-service rule — no new database),
RLS-scoped by `tenant_id` matching every other table in this schema
(mirrors `migrations/0005_project_groups.up.sql`'s exact RLS pattern).

## Changes to make

### Step 1 — `migrations/0007_project_host_setups.up.sql` (new)

```sql
-- project.project_host_setups — the pre-project wizard record
-- projectHostSetup.* manages: name a dev server + an existing folder path
-- on it, validate, then finalize into a real Project + Repo. dev_server_id
-- is a logical FK -> infra-fleet-service.dev_servers, validated via gRPC
-- at create/setupExistingFolder time, never joined in SQL
-- (05-data-architecture.md's "no cross-database FK" rule).
CREATE TABLE project.project_host_setups (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    dev_server_id UUID NOT NULL,
    folder_path   TEXT NOT NULL,
    display_name  TEXT,
    status        TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'validated', 'completed', 'failed')),
    project_id    UUID REFERENCES project.projects (id) ON DELETE SET NULL,
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_project_host_setups_tenant ON project.project_host_setups (tenant_id);

ALTER TABLE project.project_host_setups ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.project_host_setups
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

### Step 2 — `migrations/0007_project_host_setups.down.sql` (new)

```sql
DROP TABLE IF EXISTS project.project_host_setups;
```

### Step 3 — `internal/domain/host_setup.go` (new)

```go
package domain

import "errors"

// HostSetupStatus is projectHostSetup's wizard-record lifecycle state.
type HostSetupStatus string

const (
	HostSetupPending   HostSetupStatus = "pending"
	HostSetupValidated HostSetupStatus = "validated"
	HostSetupCompleted HostSetupStatus = "completed"
	HostSetupFailed    HostSetupStatus = "failed"
)

// Valid reports whether s is one of the known status values.
func (s HostSetupStatus) Valid() bool {
	switch s {
	case HostSetupPending, HostSetupValidated, HostSetupCompleted, HostSetupFailed:
		return true
	default:
		return false
	}
}

var (
	// ErrHostSetupNotFound is the sentinel adapter/postgres returns
	// (wrapped) when a lookup/mutation targets a setup that doesn't exist.
	ErrHostSetupNotFound = errors.New("domain: project host setup not found")
	// ErrFolderNotFoundOnHost is returned by usecase.SetupExistingFolder
	// when the Dev Server Agent reports folder_path doesn't exist or isn't
	// a directory.
	ErrFolderNotFoundOnHost = errors.New("domain: folder not found on dev server host")
	// ErrHostSetupAlreadyCompleted guards SetupExistingFolder against
	// re-finalizing a setup that already produced a project.
	ErrHostSetupAlreadyCompleted = errors.New("domain: host setup already completed")
)

// HostSetup is the pre-project wizard record projectHostSetup.* manages.
// ProjectID is empty until SetupExistingFolder finalizes it into a real
// Project. DevServerID is a logical FK -> infra-fleet-service, ID-only,
// validated via gRPC (05-data-architecture.md), never joined in SQL.
type HostSetup struct {
	ID          string
	TenantID    string
	DevServerID string
	FolderPath  string
	DisplayName string
	Status      HostSetupStatus
	ProjectID   string
	CreatedBy   string
}

// NewHostSetup constructs a HostSetup, enforcing the invariants a wizard
// record must satisfy — always starts Pending; Status is advanced only by
// usecase.SetupExistingFolder (via repository SetStatus/Complete calls),
// never chosen at construction time.
func NewHostSetup(id, tenantID, devServerID, folderPath, displayName, createdBy string) (HostSetup, error) {
	if tenantID == "" {
		return HostSetup{}, ErrEmptyTenantID
	}
	if devServerID == "" {
		return HostSetup{}, errors.New("domain: dev_server_id is required")
	}
	if folderPath == "" {
		return HostSetup{}, errors.New("domain: folder_path is required")
	}
	if createdBy == "" {
		return HostSetup{}, errors.New("domain: created_by is required")
	}
	return HostSetup{
		ID: id, TenantID: tenantID, DevServerID: devServerID, FolderPath: folderPath,
		DisplayName: displayName, Status: HostSetupPending, CreatedBy: createdBy,
	}, nil
}

// HostSetupPatch carries UpdateHostSetup's field-mask semantics: an empty
// string means "leave unchanged" — same convention as
// domain.ProjectUpdatePatch/CompanySettingsPatch.
type HostSetupPatch struct {
	FolderPath  string
	DisplayName string
}
```

`ErrEmptyTenantID` already exists in this package (used by
`domain.NewProjectGroup` — see `project_group.go`); reuse it rather than
declaring a duplicate sentinel.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./internal/domain/...
go vet ./internal/domain/...

# Apply the migration against a local/test Postgres, per this service's
# standard migration-verification step (see README or 05-data-architecture.md):
# e.g. migrate -path migrations -database "$DATABASE_DSN" up
```
