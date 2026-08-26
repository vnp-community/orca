# TASK-038: Implement `GetCredentialMetadataByOwner`/`ListCredentialsByCategory` usecases + `ConfigJSON` plumbing

**From Solution:** SOL-007
**Priority:** P0
**Service:** `credential-broker-service`
**File:** `internal/domain/credential_metadata.go`, `internal/usecase/{ports.go,write_credential.go,get_credential_metadata_by_owner.go,list_credentials_by_category.go}`, `internal/adapter/postgres/repository.go`, `internal/adapter/grpc/server.go`
**Depends on:** TASK-037
**Status:** `[x]` DONE — `list_credentials_by_category.go`/`ListCredentialsByCategory` confirmed present and wired end-to-end.

---

## Context

Both new RPCs are metadata-only reads (no Vault call, no plaintext, no
audit row — same "nothing security-sensitive to audit" reasoning
`GetCredentialMetadata`'s doc comment already gives). `GetByOwner` already
exists on `CredentialMetadataRepository` — `GetCredentialMetadataByOwner`
is close to a direct pass-through, distinguished mainly by returning
`metadata` unset (not an error) on not-found, per BUG-007's requirement
that `credentials.status` must distinguish "not configured" from a real
error. `ListCredentialsByCategory` needs one new repository method. This
task also threads `ConfigJSON` (TASK-037's additive proto field) through
the domain type, `WriteCredential`, and the repository.

---

## Changes to make

### Step 1 — `domain/credential_metadata.go`: add `ConfigJSON`

```go
type CredentialMetadata struct {
	ID        string
	TenantID  string
	OwnerID   string
	Category  Category
	Status    Status
	VaultPath string
	ConfigJSON string // non-secret sidecar config (TASK-037) — NEVER a secret value
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

Add `configJSON string` as a new parameter to `NewCredentialMetadata`
(after `status`, before `now`), threaded into the returned struct. Update
every existing call site in this package (`write_credential.go`,
`credential_metadata_test.go`, etc.) to pass `""` where no config applies.

### Step 2 — `usecase/ports.go`: extend `CredentialMetadataRepository`

```go
type CredentialMetadataRepository interface {
	Create(ctx context.Context, metadata domain.CredentialMetadata) error
	Get(ctx context.Context, id string) (domain.CredentialMetadata, error)
	UpdateStatus(ctx context.Context, id string, status domain.Status, now time.Time) error
	GetByOwner(ctx context.Context, tenantID string, category domain.Category, ownerID string) (domain.CredentialMetadata, error)
	// ListByCategory returns every non-revoked row for (tenantID, category)
	// — backs ListCredentialsByCategory ("which owner_ids have a credential
	// in this category").
	ListByCategory(ctx context.Context, tenantID string, category domain.Category) ([]domain.CredentialMetadata, error)
}
```

### Step 3 — `usecase/write_credential.go`: thread `ConfigJSON` through

Add `ConfigJSON string` to `WriteCredentialInput`, and pass it as the new
argument to `domain.NewCredentialMetadata(id, in.TenantID, in.OwnerID,
in.Category, vaultPath, domain.StatusActive, in.ConfigJSON, now)`
(matching Step 1's new parameter position).

### Step 4 — `usecase/get_credential_metadata_by_owner.go` (new)

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

type GetCredentialMetadataByOwnerInput struct {
	TenantID string
	Category domain.Category
	OwnerID  string
}

// GetCredentialMetadataByOwnerResult wraps the metadata plus a Found flag
// so "no credential exists yet" (Found=false, nil error) is distinguished
// from a real fetch error — GetCredentialMetadataByOwnerResponse.metadata
// is `optional` for exactly this reason (TASK-037).
type GetCredentialMetadataByOwnerResult struct {
	Metadata domain.CredentialMetadata
	Found    bool
}

// GetCredentialMetadataByOwner is a pure metadata read, same
// no-Vault-call/no-audit-row shape as GetCredentialMetadata — see that
// usecase's doc comment. Closes the gap BUG-007 flags: previously the only
// by-owner lookup was ResolveCredentialByOwner, which returns the
// plaintext value — a security mismatch for a status check.
type GetCredentialMetadataByOwner struct {
	metadataRepo CredentialMetadataRepository
}

func NewGetCredentialMetadataByOwner(metadataRepo CredentialMetadataRepository) *GetCredentialMetadataByOwner {
	return &GetCredentialMetadataByOwner{metadataRepo: metadataRepo}
}

func (uc *GetCredentialMetadataByOwner) Execute(ctx context.Context, in GetCredentialMetadataByOwnerInput) (GetCredentialMetadataByOwnerResult, error) {
	if in.TenantID == "" || in.OwnerID == "" {
		return GetCredentialMetadataByOwnerResult{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id and owner_id are required", nil)
	}
	metadata, err := uc.metadataRepo.GetByOwner(ctx, in.TenantID, in.Category, in.OwnerID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) {
			// Not found is the normal "not configured yet" case, not an
			// error — see this type's doc comment.
			return GetCredentialMetadataByOwnerResult{Found: false}, nil
		}
		return GetCredentialMetadataByOwnerResult{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_FETCH_FAILED", "failed to fetch credential metadata", err)
	}
	return GetCredentialMetadataByOwnerResult{Metadata: metadata, Found: true}, nil
}
```

### Step 5 — `usecase/list_credentials_by_category.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

type ListCredentialsByCategoryInput struct {
	TenantID string
	Category domain.Category
}

type ListCredentialsByCategory struct {
	metadataRepo CredentialMetadataRepository
}

func NewListCredentialsByCategory(metadataRepo CredentialMetadataRepository) *ListCredentialsByCategory {
	return &ListCredentialsByCategory{metadataRepo: metadataRepo}
}

func (uc *ListCredentialsByCategory) Execute(ctx context.Context, in ListCredentialsByCategoryInput) ([]domain.CredentialMetadata, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id is required", nil)
	}
	rows, err := uc.metadataRepo.ListByCategory(ctx, in.TenantID, in.Category)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "CREDENTIAL_LIST_FAILED", "failed to list credentials", err)
	}
	return rows, nil
}
```

### Step 6 — `adapter/postgres/repository.go`

- Extend `Create`/`Get`/`GetByOwner`'s `INSERT`/`SELECT` column lists to
  include `config_json`, threading it into `domain.NewCredentialMetadata`'s
  new parameter (Step 1).
- Add `ListByCategory`:

```go
func (r *Repository) ListByCategory(ctx context.Context, tenantID string, category domain.Category) ([]domain.CredentialMetadata, error) {
	const q = `
		SELECT id, tenant_id, owner_id, category, status, vault_path, COALESCE(config_json, ''), created_at, updated_at
		FROM credential.metadata
		WHERE tenant_id = $1 AND category = $2 AND status != 'revoked'
		ORDER BY created_at`
	rows, err := r.pool.Query(ctx, q, tenantID, category)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing credentials by category: %w", err)
	}
	defer rows.Close()

	var out []domain.CredentialMetadata
	for rows.Next() {
		var m domain.CredentialMetadata
		if err := rows.Scan(&m.ID, &m.TenantID, &m.OwnerID, &m.Category, &m.Status, &m.VaultPath, &m.ConfigJSON, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scanning credential metadata: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
```

Add a `config_json TEXT` column via a new migration
(`migrations/000X_config_json.up.sql`:
`ALTER TABLE credential.metadata ADD COLUMN config_json TEXT;` — read the
existing `migrations/` directory first to pick the next sequence number).

### Step 7 — `adapter/grpc/server.go`: wire the 2 new RPCs

Add `getCredentialMetadataByOwner *usecase.GetCredentialMetadataByOwner`
and `listCredentialsByCategory *usecase.ListCredentialsByCategory` fields,
thread through `New(...)`, add 2 gRPC methods — `GetCredentialMetadataByOwner`
maps `Found: false` to a response with `metadata` left unset (not an
error), following `GetCredentialMetadata`'s handler in the same file for
the general shape.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/credential-broker-service
go build ./... && go vet ./...
```

Expected: clean build. Usecase-level tests are added in TASK-043.
