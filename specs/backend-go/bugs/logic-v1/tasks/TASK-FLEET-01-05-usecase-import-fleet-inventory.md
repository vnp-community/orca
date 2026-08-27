# TASK-FLEET-01-05: `usecase.ImportFleetInventory`

**From Solution:** SOL-FLEET-01
**Priority:** P1
**Service:** `infra-fleet-service` (usecase)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/import_fleet_inventory.go` (new)
**Depends on:** TASK-FLEET-01-03, TASK-FLEET-01-04
**Status:** `[ ]` TODO

---

## Context

Per-record error handling (skip-and-continue, not fail-fast) matches
BL-FLEET-01's `{imported, updated, skipped, errors}` contract — one
malformed row must not abort an otherwise-valid batch.

## Changes to make

```go
// internal/usecase/import_fleet_inventory.go
package usecase

type FleetServerInput struct {
    Host, UserName, VaultSSHRole, Project string
    Tags                                  []string
}

type ImportFleetInventoryInput struct {
    Servers []FleetServerInput
    DryRun  bool
}

type ImportFleetInventoryError struct {
    Host, UserName, Reason string
}

type ImportFleetInventoryResult struct {
    Imported, Updated, Skipped int
    Errors                     []ImportFleetInventoryError
}

type ImportFleetInventory struct {
    repo SshTargetRepository
}

func NewImportFleetInventory(repo SshTargetRepository) *ImportFleetInventory {
    return &ImportFleetInventory{repo: repo}
}

func (uc *ImportFleetInventory) Execute(ctx context.Context, in ImportFleetInventoryInput) (ImportFleetInventoryResult, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return ImportFleetInventoryResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
    }
    var result ImportFleetInventoryResult
    for _, s := range in.Servers {
        target, err := domain.NewSshTarget(uuid.NewString(), tenantID, s.Host, s.UserName, s.VaultSSHRole, s.Project, s.Tags)
        if err != nil {
            result.Skipped++
            result.Errors = append(result.Errors, ImportFleetInventoryError{Host: s.Host, UserName: s.UserName, Reason: err.Error()})
            continue
        }
        if in.DryRun {
            _, found, _ := uc.repo.GetByHostUser(ctx, tenantID, s.Host, s.UserName)
            if found {
                result.Updated++
            } else {
                result.Imported++
            }
            continue
        }
        _, updated, err := uc.repo.Upsert(ctx, target)
        if err != nil {
            result.Skipped++
            result.Errors = append(result.Errors, ImportFleetInventoryError{Host: s.Host, UserName: s.UserName, Reason: err.Error()})
            continue
        }
        if updated {
            result.Updated++
        } else {
            result.Imported++
        }
    }
    return result, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestImportFleetInventory -v
```

Expected (against a fake `SshTargetRepository`): all-new batch →
`imported=N, updated=0`; batch with one pre-existing `(tenant,host,user)` →
that row counted `updated`; one row with empty `VaultSSHRole` → `skipped` +
populated `Errors`, valid rows in the same batch still commit; `DryRun=true`
→ `repo.Upsert` never called, only `GetByHostUser`.
