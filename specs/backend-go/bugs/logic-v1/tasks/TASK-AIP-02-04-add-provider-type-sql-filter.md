# TASK-AIP-02-04: Add `provider_type` filter clause to `Repository.List`'s SQL

**From Solution:** SOL-AIP-02
**Priority:** P0 — correctness bug
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go`
**Depends on:** TASK-AIP-02-03
**Status:** `[x] DONE — provider_type WHERE clause added to List; TestRepository_List_FiltersByProviderType passes against real Postgres.`

---

## Context

`List`'s SQL (`repository.go:76-105`) already filters on `dev_server_id`
via the file's existing `$N = '' OR col = $N` idiom, but has no clause for
`provider_type` — `TASK-AIP-02-03`'s new `ListAccountsFilter.ProviderType`
field is otherwise inert. One filter clause, following the exact idiom
already used for `dev_server_id`, no new query shape.

## Changes to make

In `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go`'s
`List`:

```go
func (r *Repository) List(ctx context.Context, filter usecase.ListAccountsFilter) ([]domain.ProviderAccount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, provider_type, status, credential_ref,
		       scope, user_id, project_id, dev_server_id, rotation_grace_until, created_at, updated_at
		FROM ai_provider.accounts
		WHERE tenant_id = $1
		  AND deleted_at IS NULL
		  AND ($2 = '' OR scope = $2)
		  AND ($3 = '' OR user_id = $3::uuid OR project_id = $3::uuid)
		  AND ($4 = '' OR dev_server_id = $4)
		  AND ($5 = '' OR provider_type = $5)
		ORDER BY created_at
	`, filter.TenantID, string(filter.Scope), filter.ScopeRefID, filter.DevServerID, string(filter.ProviderType))
	// ... rest unchanged
}
```

Note: if `TASK-AIP-01-07` has already landed and extended this query's
`SELECT`/column list, apply this `$5` clause on top of that version rather
than reverting it — the two tasks touch the same query but different
parts of it (column list vs. `WHERE` clause).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/adapter/postgres/... -run TestRepository_List
```

Add `TestRepository_List_FiltersByProviderType` (integration,
`testcontainers-go`): seed one Anthropic and one OpenAI account at the
same scope; `List` with `ProviderType: domain.ProviderTypeAnthropic`
returns only the Anthropic account.
