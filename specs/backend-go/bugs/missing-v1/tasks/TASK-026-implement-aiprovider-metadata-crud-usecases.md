# TASK-026: Implement `ListAccounts`/`UpdateAccount`/`DeleteAccount` usecases + `dev_server_id` domain field

**From Solution:** SOL-005 (Group B)
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `internal/domain/provider_account.go`, `internal/usecase/{ports.go,list_accounts.go,update_account.go,delete_account.go}` (3 new), `internal/adapter/postgres/repository.go`, `migrations/0002_dev_server_id.up.sql` (new), `internal/adapter/grpc/server.go`
**Depends on:** TASK-024
**Status:** `[ ]` TODO

---

## Context

Group B (`aiProvider.list`/`update`/`delete`) is metadata-only CRUD — no
credential-broker involvement, same shape as `CreateAccount`'s
non-credential half. This task also closes the `dev_server_id`
implementation-vs-schema drift SOL-005 flags: `ai-provider-service.md` §5
already specifies a `dev_server_id` column the domain type doesn't carry.

`DeleteAccount` is a **soft delete** (`status='revoked'` + `deleted_at`),
not a hard `DELETE` — preserves `usage_daily`'s FK and the account's row in
the audit trail, mirroring `credential-broker-service`'s revoke-not-erase
convention.

---

## Changes to make

### Step 1 — migration: `migrations/0002_dev_server_id.up.sql` (new)

```sql
ALTER TABLE ai_provider.accounts
  ADD COLUMN dev_server_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN deleted_at TIMESTAMPTZ;
```

`migrations/0002_dev_server_id.down.sql` (new):

```sql
ALTER TABLE ai_provider.accounts
  DROP COLUMN dev_server_id,
  DROP COLUMN deleted_at;
```

### Step 2 — `domain/provider_account.go`: add `DevServerID`

Add a field to `ProviderAccount` (after `ProjectID`):

```go
type ProviderAccount struct {
	ID                 string
	TenantID           string
	ProviderType       ProviderType
	Status             AccountStatus
	CredentialRef      string
	Scope              AccountScope
	UserID             string
	ProjectID          string
	DevServerID        string // which dev server holds this account's pushed ciphertext; empty until first push (§9)
	RotationGraceUntil *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
```

Add `DevServerID string` as a new parameter to `NewProviderAccount` (insert
after `projectID`, before `rotationGraceUntil`) and thread it through to
the returned struct literal. Update every existing call site of
`NewProviderAccount` in this package (`create_account.go`,
`provider_account_test.go`, `rotate_key.go` if any) to pass `""` for the
new parameter — no behavior change for existing callers.

### Step 3 — `usecase/ports.go`: extend `ProviderAccountRepository`

```go
type ProviderAccountRepository interface {
	Create(ctx context.Context, account domain.ProviderAccount) error
	Get(ctx context.Context, tenantID, id string) (domain.ProviderAccount, error)
	List(ctx context.Context, filter ListAccountsFilter) ([]domain.ProviderAccount, error)
	UpdateStatus(ctx context.Context, in UpdateStatusInput) (domain.ProviderAccount, error)
	// Update mutates the user-editable metadata fields — Label/ModelHint/
	// BaseURL — distinct from UpdateStatus, which only mutates lifecycle
	// fields (status/credential_ref/rotation grace). Never touches Status
	// or CredentialRef.
	Update(ctx context.Context, in UpdateFields) (domain.ProviderAccount, error)
	// Delete soft-deletes: sets status='revoked' and deleted_at=now(), never
	// a hard DELETE — preserves the row for audit and usage_daily's FK.
	Delete(ctx context.Context, tenantID, accountID string) error
}
```

Add `ListAccountsFilter.DevServerID` (optional filter) and the new
`UpdateFields` input type:

```go
type ListAccountsFilter struct {
	TenantID    string
	Scope       domain.AccountScope
	ScopeRefID  string
	DevServerID string // optional — empty means no filter
}

// UpdateFields is UpdateAccount's usecase input — only the 3 fields
// ai-provider-service.md documents as user-editable outside the status
// machine.
type UpdateFields struct {
	TenantID  string
	AccountID string
	Label     string
	ModelHint string
	BaseURL   string
}
```

### Step 4 — `usecase/list_accounts.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

type ListAccountsInput struct {
	DevServerID string
}

// ListAccounts is a thin translation layer over
// ProviderAccountRepository.List — the repo method and filter already
// existed (ports.go), this usecase just adds tenant enforcement.
type ListAccounts struct {
	repo ProviderAccountRepository
}

func NewListAccounts(repo ProviderAccountRepository) *ListAccounts {
	return &ListAccounts{repo: repo}
}

func (uc *ListAccounts) Execute(ctx context.Context, in ListAccountsInput) ([]domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	return uc.repo.List(ctx, ListAccountsFilter{TenantID: tenantID, DevServerID: in.DevServerID})
}
```

### Step 5 — `usecase/update_account.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// UpdateAccount mutates Label/ModelHint/BaseURL only — guards against
// becoming a second path to mutate lifecycle state, which must only ever
// go through UpdateStatus (see ports.go's UpdateStatusInput doc comment).
type UpdateAccount struct {
	repo ProviderAccountRepository
}

func NewUpdateAccount(repo ProviderAccountRepository) *UpdateAccount {
	return &UpdateAccount{repo: repo}
}

func (uc *UpdateAccount) Execute(ctx context.Context, in UpdateFields) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	if in.AccountID == "" {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_NO_ACCOUNT_ID", "account_id is required", nil)
	}
	in.TenantID = tenantID
	account, err := uc.repo.Update(ctx, in)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_UPDATE_FAILED", "failed to update provider account", err)
	}
	return account, nil
}
```

### Step 6 — `usecase/delete_account.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type DeleteAccount struct {
	repo ProviderAccountRepository
}

func NewDeleteAccount(repo ProviderAccountRepository) *DeleteAccount {
	return &DeleteAccount{repo: repo}
}

func (uc *DeleteAccount) Execute(ctx context.Context, accountID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	if accountID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_NO_ACCOUNT_ID", "account_id is required", nil)
	}
	if err := uc.repo.Delete(ctx, tenantID, accountID); err != nil {
		return apperrors.New(apperrors.KindInternal, "AIPROVIDER_DELETE_FAILED", "failed to delete provider account", err)
	}
	return nil
}
```

### Step 7 — `adapter/postgres/repository.go`: implement `Update`/`Delete`, extend `List`/`Get`/`Create`

- `Create`/`Get`/`List`/`UpdateStatus`'s existing `SELECT`/`INSERT` column
  lists must add `dev_server_id` and (`Get`/`List` only) filter out rows
  where `deleted_at IS NOT NULL`.
- `List`: when `filter.DevServerID != ""`, add `AND dev_server_id = $N` to
  the `WHERE` clause.

```go
// Update implements usecase.ProviderAccountRepository.Update.
func (r *Repository) Update(ctx context.Context, in usecase.UpdateFields) (domain.ProviderAccount, error) {
	const q = `
		UPDATE ai_provider.accounts
		SET label = $3, model_hint = $4, base_url = $5, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
		RETURNING id, tenant_id, type, status, credential_ref, scope, user_id, project_id, dev_server_id, created_at, updated_at`
	row := r.pool.QueryRow(ctx, q, in.TenantID, in.AccountID, in.Label, in.ModelHint, in.BaseURL)
	return scanProviderAccount(row) // reuse the existing row-scan helper Get/UpdateStatus already use
}

// Delete implements usecase.ProviderAccountRepository.Delete — soft-delete
// only (status='revoked' + deleted_at), never a hard DELETE. See ports.go's
// doc comment.
func (r *Repository) Delete(ctx context.Context, tenantID, accountID string) error {
	const q = `
		UPDATE ai_provider.accounts
		SET status = 'revoked', deleted_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2`
	tag, err := r.pool.Exec(ctx, q, tenantID, accountID)
	if err != nil {
		return fmt.Errorf("postgres: deleting provider account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAccountNotFound
	}
	return nil
}
```

If `Get`/`UpdateStatus` don't already share a `scanProviderAccount(row
pgx.Row) (domain.ProviderAccount, error)` helper, extract one now (both
methods' row-scan logic is otherwise duplicated 3 ways after this change) —
read `repository.go`'s current `Get`/`UpdateStatus` bodies first to match
their exact column order before extracting.

### Step 8 — `adapter/grpc/server.go`: wire the 3 new RPC handlers

Add `ListAccounts`/`UpdateAccount`/`DeleteAccount` gRPC methods on the
server struct, each translating the proto request into the usecase input
above and back — follow `CreateAccount`'s existing handler in the same file
for the exact translation-and-error-mapping shape (`apperrors.ToGRPCStatus`
on error).

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/ai-provider-service
go build ./... && go vet ./...
```

Expected: clean build. Full usecase-level tests are added in TASK-030.
