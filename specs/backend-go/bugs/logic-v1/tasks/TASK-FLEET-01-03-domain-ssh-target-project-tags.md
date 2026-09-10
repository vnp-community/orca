# TASK-FLEET-01-03: Extend `domain.SshTarget` with `Project`/`Tags`

**From Solution:** SOL-FLEET-01
**Priority:** P0
**Service:** `infra-fleet-service` (domain)
**File:** `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go`
**Depends on:** none
**Status:** [x] DONE — added Project/Tags fields, extended NewSshTarget signature, updated all call sites; `go test ./internal/domain/... -run TestNewSshTarget` passes.

---

## Context

`SshTarget` needs `Project`/`Tags` to carry BL-FLEET-01's grouping metadata.
Both are optional — single-server registration via `ssh.*`/`CreateSshTarget`
predates grouping and must keep working with empty values, so no new
invariant is added.

## Changes to make

```go
// internal/domain/ssh_target.go
type SshTarget struct {
    ID           string
    TenantID     string
    Host         string
    UserName     string
    VaultSSHRole string
    Project      string   // "" = ungrouped; matches YAML's servers[].project
    Tags         []string // matches YAML's servers[].tags
}
```

Extend `NewSshTarget`'s signature to accept `project string, tags []string`
(append as new trailing params to keep the diff minimal), and set them on
the constructed value. Do not add any validation/invariant on either field —
both may be zero-valued. Update every existing call site of `NewSshTarget`
in this package/service to pass `"", nil` where the solution doesn't apply
(single-server registration paths), keeping current behavior identical.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/domain/... -run TestNewSshTarget -v
```

Expected: `NewSshTarget` accepts empty `project`/`tags` (backward compat) and
a populated pair; existing `ErrEmptyVaultSSHRole` invariant unchanged.
