# TASK-AIP-01-07: Transactional `Create` (demotion + outbox), extend `scanAccount`, wire outbox relay in `main.go`

**From Solution:** SOL-AIP-01
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go`
**Depends on:** TASK-AIP-01-01, TASK-AIP-01-02, TASK-AIP-01-03
**Status:** `[ ]` TODO

---

## Context

`Create` today is a single non-transactional `INSERT` with no default
demotion and no outbox write (`repository.go:41-56`). `scanAccount`
(`repository.go:198-218`) never selects `label`/`model_hint`/`base_url`/
`quota_limit_day`/`models`/`is_default`/`last_health_check_at`/
`created_by` — confirmed bug: `Update`'s `SET` clause already writes
`label`/`model_hint`/`base_url` but its `RETURNING` list omits them
(`repository.go:132-149`), so a write is immediately discarded on
read-back. This task fixes both, matching `usage-service`'s outbox
implementation precedent exactly (`FetchUnpublished`/`MarkPublished`
against `common/outbox.Store`).

## Changes to make

Replace `Create` in `repository.go`:

```go
func (r *Repository) Create(ctx context.Context, account domain.ProviderAccount) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin create-account tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	if account.IsDefault {
		// Demote any prior default for this dev_server+provider pair BEFORE
		// inserting — the partial unique index
		// (uq_accounts_one_default_per_dev_server_provider) would otherwise
		// reject the insert outright rather than performing the demotion.
		if _, err := tx.Exec(ctx, `
			UPDATE ai_provider.accounts SET is_default = false, updated_at = now()
			WHERE tenant_id = $1 AND dev_server_id = $2 AND provider_type = $3 AND is_default AND deleted_at IS NULL
		`, account.TenantID, account.DevServerID, string(account.ProviderType)); err != nil {
			return fmt.Errorf("postgres: demote prior default account: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_provider.accounts (
			id, tenant_id, provider_type, status, credential_ref, scope, user_id, project_id,
			dev_server_id, label, model_hint, base_url, quota_limit_day, models, is_default,
			created_by, rotation_grace_until, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`, account.ID, account.TenantID, string(account.ProviderType), string(account.Status), account.CredentialRef,
		string(account.Scope), nullableString(account.UserID), nullableString(account.ProjectID), account.DevServerID,
		account.Label, nullableString(account.ModelHint), nullableString(account.BaseURL), account.QuotaLimitDay,
		account.Models, account.IsDefault, nullableString(account.CreatedBy), account.RotationGraceUntil,
		account.CreatedAt, account.UpdatedAt); err != nil {
		return fmt.Errorf("postgres: insert account: %w", err)
	}

	if err := insertOutboxEvent(ctx, tx, account); err != nil {
		return err // same tx — a failed outbox write rolls back the account insert too
	}

	return tx.Commit(ctx)
}

// insertOutboxEvent builds and writes the ai_provider.account.registered
// event — same table shape usage-service/issue-tracking-service already use.
func insertOutboxEvent(ctx context.Context, tx pgx.Tx, account domain.ProviderAccount) error {
	payload, err := json.Marshal(map[string]any{
		"account_id":    account.ID,
		"provider_type": string(account.ProviderType),
		"dev_server_id": account.DevServerID,
		"scope":         string(account.Scope),
		"created_by":    account.CreatedBy,
	})
	if err != nil {
		return fmt.Errorf("postgres: marshal outbox payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO ai_provider.outbox (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1,$2,'ai_provider.account.registered',$3,1,$4)
	`, uuid.NewString(), account.TenantID, account.CreatedAt, payload)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}
	return nil
}
```

Add `FetchUnpublished`/`MarkPublished` (copy-adapt from
`backend-go/services/usage-service/internal/adapter/postgres/repository.go`'s
implementations against `usage.outbox_events`, targeting
`ai_provider.outbox` instead):

```go
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM ai_provider.outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		var ev eventbus.Event
		if err := rows.Scan(&rec.ID, &ev.TenantID, &rec.Subject, &ev.OccurredAt, &ev.Version, &ev.Payload); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox row: %w", err)
		}
		ev.ID = rec.ID
		rec.Event = ev
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE ai_provider.outbox SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}

var _ outbox.Store = (*Repository)(nil)
```

Add imports: `"encoding/json"`, `"github.com/google/uuid"`,
`"github.com/stablyai/orca-go/common/eventbus"`,
`"github.com/stablyai/orca-go/common/outbox"`.

Extend `scanAccount`'s `SELECT`/`Scan` column list (used by `Get`, `List`,
`UpdateStatus`, `Update`) to include the 8 new columns:

```go
func scanAccount(row rowScanner) (domain.ProviderAccount, error) {
	var a domain.ProviderAccount
	var providerType, status, scope string
	var userID, projectID, modelHint, baseURL, createdBy *string
	if err := row.Scan(
		&a.ID, &a.TenantID, &providerType, &status, &a.CredentialRef,
		&scope, &userID, &projectID, &a.DevServerID, &a.Label, &modelHint, &baseURL,
		&a.QuotaLimitDay, &a.Models, &a.IsDefault, &a.LastHealthCheckAt, &createdBy,
		&a.RotationGraceUntil, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domain.ProviderAccount{}, err
	}
	a.ProviderType = domain.ProviderType(providerType)
	a.Status = domain.AccountStatus(status)
	a.Scope = domain.AccountScope(scope)
	if userID != nil {
		a.UserID = *userID
	}
	if projectID != nil {
		a.ProjectID = *projectID
	}
	if modelHint != nil {
		a.ModelHint = *modelHint
	}
	if baseURL != nil {
		a.BaseURL = *baseURL
	}
	if createdBy != nil {
		a.CreatedBy = *createdBy
	}
	return a, nil
}
```

Update every `SELECT ... FROM ai_provider.accounts` in `Get`/`List`/
`UpdateStatus`/`Update` to select the matching column list in the same
order: `id, tenant_id, provider_type, status, credential_ref, scope,
user_id, project_id, dev_server_id, label, model_hint, base_url,
quota_limit_day, models, is_default, last_health_check_at, created_by,
rotation_grace_until, created_at, updated_at`.

In `cmd/server/main.go`, add the outbox relay (copy `usage-service`'s
`cmd/server/main.go:100-114` wiring, publishing under a
`"orca.ai_provider.>"` subject and `"AI_PROVIDER"` stream name), and pass
`infraFleet` into `usecase.NewCreateAccount`:

```go
createAccountUC := usecase.NewCreateAccount(repo, broker, infraFleet, uuid.NewString, nil)
// ... after healthSrv registration, mirroring usage-service's main.go ...
var relay *outbox.Relay
pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
if err != nil {
	logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
} else {
	defer func() { _ = closeBus() }()
	if err := pub.EnsureStream(ctx, "AI_PROVIDER", []string{"orca.ai_provider.>"}); err != nil {
		logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
	} else {
		relay = outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
	}
}
if relay != nil {
	go relay.Run(ctx)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go test ./services/ai-provider-service/... -run TestRepository
```

Add to `repository_test.go` (integration, `testcontainers-go`):
- `uq_accounts_one_default_per_dev_server_provider` rejects a raw
  double-default insert bypassing the demotion path.
- `Create`'s transaction rolls back the account insert if the outbox
  insert fails (inject a constraint violation on `ai_provider.outbox`).
- `Update` round-trips `label`/`model_hint`/`base_url` (the specific bug
  this task fixes).
- Outbox contract: `FetchUnpublished` returns the row with
  `published_at IS NULL` right after `Create`; `MarkPublished` clears it
  from a subsequent `FetchUnpublished` call.
