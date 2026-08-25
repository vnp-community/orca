# TASK-003: Implement access-policy CRUD with append-only versioning

**From Solution:** SOL-001
**Priority:** P0
**Service:** `auth-service`
**File:** `services/auth-service/internal/usecase/{create,get,list,update,delete}_access_policy.go` (new), `internal/adapter/postgres/repository.go`
**Depends on:** TASK-001
**Status:** `[ ]` TODO

---

## Context

Per `auth-service.md:150`, an `UpdateAccessPolicy` call inserts a NEW
version row, never mutates in place — OPA bundle sync and audit both need
a stable version history. The `access_policies` table already exists per
`auth-service.md:172` (`id UUID PK, name TEXT, kind TEXT, document JSONB,
version INT, updated_by UUID, updated_at`) with a unique `(name, version)`
index — confirm this table exists in the actual migrations before writing
queries; add it if missing.

## Changes to make

### Repository methods

```go
func (r *Repository) InsertPolicyVersion(ctx context.Context, p domain.AccessPolicy) error {
    _, err := r.db.ExecContext(ctx, `INSERT INTO auth.access_policies
        (id, name, kind, document, version, updated_by, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, now())`,
        p.ID, p.Name, p.Kind, p.DocumentJSON, p.Version, p.UpdatedBy)
    return err
}

func (r *Repository) GetLatestPolicy(ctx context.Context, id string) (*domain.AccessPolicy, error) {
    // SELECT ... WHERE id = $1 ORDER BY version DESC LIMIT 1
}

func (r *Repository) ListLatestPolicies(ctx context.Context, pageToken string, pageSize int) ([]domain.AccessPolicy, string, error) {
    // SELECT DISTINCT ON (id) ... ORDER BY id, version DESC — one row per id, latest version
}

func (r *Repository) DeletePolicy(ctx context.Context, id string) error {
    // soft-delete or hard-delete all versions for id — confirm which per auth-service.md's audit-retention stance before implementing
}
```

### Usecase — update creates a new version

```go
// internal/usecase/update_access_policy.go
func (uc *PolicyUseCase) UpdateAccessPolicy(ctx context.Context, id, documentJSON string, expectedVersion int32, actorID string) (*domain.AccessPolicy, error) {
    current, err := uc.repo.GetLatestPolicy(ctx, id)
    if err != nil {
        return nil, err
    }
    if current.Version != expectedVersion {
        return nil, apperrors.FailedPrecondition("policy was updated concurrently, refetch and retry")
    }
    next := domain.AccessPolicy{
        ID: current.ID, Name: current.Name, Kind: current.Kind,
        DocumentJSON: documentJSON, Version: current.Version + 1, UpdatedBy: actorID,
    }
    if err := uc.repo.InsertPolicyVersion(ctx, next); err != nil {
        return nil, err
    }
    // Publish to the OPA bundle registry — see auth-service.md:194's
    // PolicyDataPublisher port. If PolicyDataPublisher isn't implemented
    // yet, stub it behind an interface now so this usecase compiles and
    // is swappable later without touching this function again.
    if err := uc.publisher.PublishPolicyChange(ctx, next); err != nil {
        return nil, err
    }
    return &next, nil
}
```

`CreateAccessPolicy`/`GetAccessPolicy`/`ListAccessPolicies`/`DeleteAccessPolicy`
follow the obvious direct-repository-call shape — no versioning nuance for
those, `create` starts at `version=1`.

### gRPC server wiring

Add the 5 RPC handlers in `internal/adapter/grpc/server.go`, admin-only
enforcement via the existing interceptor (confirm `requireAdminActor`'s
OPA check already covers new RPC names by pattern, or add them to its
allow-list if it's an explicit list).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/auth-service
go build ./...
go vet ./...
```
