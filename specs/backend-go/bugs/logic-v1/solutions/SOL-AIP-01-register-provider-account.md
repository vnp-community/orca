# SOL-AIP-01: Finish account-metadata fields, add default-demotion, quota limit, test-before-save gate, and a registration audit event

**Resolves:** [BUG-AIP-01](../BUG-AIP-01-register-provider-account-partial.md)
**Service:** `ai-provider-service` (+ `api-gateway` REST/WS wiring). The
test-before-save gate additionally needs a Dev Server Agent change — see
"Dev Server Agent dependency" below.
**Affected files (proposed):**
- `backend-go/services/ai-provider-service/internal/domain/provider_account.go`
- `backend-go/services/ai-provider-service/migrations/0003_account_registration_fields.up.sql` / `.down.sql` (new)
- `backend-go/services/ai-provider-service/migrations/0004_outbox.up.sql` / `.down.sql` (new)
- `backend-go/services/ai-provider-service/internal/usecase/ports.go`
- `backend-go/services/ai-provider-service/internal/usecase/create_account.go`
- `backend-go/services/ai-provider-service/internal/usecase/verify_connection.go` (new — shared by `CreateAccount` and `TestConnection`)
- `backend-go/services/ai-provider-service/internal/domain/outbox.go` (new)
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go`
- `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`
- `backend-go/services/ai-provider-service/cmd/server/main.go`
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go`
- `agent/src/relay/agent-rpc-dispatch.ts` (or equivalent — see "Dev Server Agent dependency")
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

This bug's gaps split cleanly into three tiers by how firmly they're
already grounded in `ai-provider-service.md`:

### Tier 1 — fields the TDD already specifies, just not yet implemented

§4's domain model sketch and §5's schema already list `Label`, `ModelHint`,
`BaseURL`, `Status`, `QuotaLimitDay`, `RotationGraceUntil`,
`LastHealthCheckAt`, `CredentialRefID`, `CreatedBy`
(`ai-provider-service.md:101-106`, `:130-157`). The Go migration
`0002_dev_server_id.up.sql` already added `label`/`model_hint`/`base_url`
columns to `ai_provider.accounts`
(`backend-go/services/ai-provider-service/migrations/0002_dev_server_id.up.sql:10-15`,
its own comment citing "`ai-provider-service.md` §5 already documented all
three") — but `domain.ProviderAccount` never gained the matching Go fields,
and `repository.go`'s `scanAccount`/`Create`/`Update` SQL never
selects/returns them (confirmed: `Update`'s `RETURNING` clause,
`repository.go:137-138`, omits `label`/`model_hint`/`base_url` even though
its own `SET` clause just wrote them — a write that's immediately
discarded on read-back). `quota_limit_day`, `last_health_check_at`, and
`created_by` have no column at all yet. This tier is "finish what's already
speced," not new design.

### Tier 2 — fields BL-AIP-01 requires that §4's sketch doesn't have

BL-AIP-01 (the business-logic spec this bug traces to) requires an
allowed-**models list** and an **`isDefault`** flag with auto-demotion.
Neither appears in `ai-provider-service.md` §4's `ProviderAccount` sketch
or §5's `CREATE TABLE ai_provider.accounts`. This is a genuine gap between
the TDD and the business-logic spec it's supposed to realize, not an
oversight in this bug report — flagged explicitly, the same way SOL-009
flagged `git-gateway-service`'s file-I/O RPC group as an extension beyond
that service's own `.md`. Both fields are needed for reasons the TDD
itself depends on elsewhere:

- **`Models []string`**: without it, [BUG-AIP-02](../BUG-AIP-02-provider-resolution-partial.md)'s
  "validate model in account.models list" step
  (`ai-provider-service.md`'s own resolution cascade in §4 implicitly
  assumes an account "serves" some model set once model-hint filtering
  exists) has nothing to validate against — see
  [SOL-AIP-02](./SOL-AIP-02-provider-resolution-filtering.md)'s explicit
  dependency note.
- **`IsDefault bool`**: BL-AIP-02's "server-default" resolution tier — a
  narrower priority than plain server-scope but broader than
  project/user-scope — can never be populated without it (BUG-AIP-02's own
  finding: "this also means BL-AIP-02's 'server-default' resolution tier
  can never actually be populated in backend-go").

Both are added in the same schema style §5 already uses elsewhere
(`TEXT[]` for a list, `BOOLEAN` for a flag, a partial `UNIQUE` index for
the demotion invariant — matching `credential-broker-service.md`'s
`unique_vault_path` constraint pattern, `credential-broker-service.md:259`).

### Tier 3 — behavioral gates the TDD achieves differently than BL-AIP-01 literally describes

BL-AIP-01 describes a **synchronous "Test Connection before save" gate**:
reject the whole create if a live provider API call fails. §9's actual
design achieves a related but architecturally different guarantee: an
account is **never `active`, and therefore never returned by `Resolve`**
(`Resolvable()`, `provider_account.go:219-221`), **until ciphertext push to
the target dev server is confirmed** — "`Resolve` must not return an
account whose ciphertext isn't yet confirmed pushed to its target dev
server, to avoid recreating Gap 2 transiently mid-migration... via the
existing `status` state machine (`pending` until push-confirmed, then
`active`)" (`ai-provider-service.md:357-362`). This is a *stronger* version
of "don't hand out an unproven account" — it can't be defeated by a
provider outage transiently making the account look fine at test time — but
it does not by itself give the *user* a synchronous "your key is wrong,
fix it before saving" signal at create time, which is real UX value
BL-AIP-01 asks for and §9's push-confirmation alone doesn't provide.

This solution keeps §9's `pending`-until-push-confirmed invariant
(untouched) and **adds** a synchronous pre-persist connection check as a
second, complementary gate — reusing the existing `TestConnection`
usecase's relay call rather than inventing a parallel mechanism. See
"Design — test-before-save gate" below.

### Audit trail — the TDD's mechanism is the outbox, not a shared `audit_log` table

§5's schema already includes `ai_provider.outbox` — "Transactional outbox
for lifecycle events (create/rotate/revoke) consumed by
`infra-fleet-service`/`credential-broker-service`"
(`ai-provider-service.md:175-184`) — and
[`05-data-architecture.md`](../../../tdd/architecture/05-data-architecture.md)'s
"Transactional outbox + async events (default)" section is the system-wide
mechanism for "service A does something, other services need to eventually
know." `auth-service` separately owns a literal `audit_log` table
(`02-microservices-decomposition.md`'s service catalog, row 1: "audit log"
folded into `auth-service`), with a working `Append`/`Query` port
(`backend-go/services/auth-service/internal/adapter/postgres/audit_repository.go:15-28`)
— but **no RPC exists for another service to write into it**; `auth.proto`
only exposes `QueryAuditLog` (read-only,
`backend-go/proto/orca/auth/v1/auth.proto:28`). Routing this bug's
`audit_log('ai_provider.registered', ...)` requirement through a new
cross-service write RPC on `auth-service` would be a second, larger design
decision this bug doesn't need to force — the TDD already gives
`ai-provider-service` its own mechanism for exactly this (an outbox event,
consumed asynchronously by whatever eventually needs to know, including a
future audit-trail consumer). This solution implements the outbox route and
flags the "should this also land in `auth-service`'s queryable
`audit_log`" question as an open design decision for whoever owns that
cross-cutting concern, rather than silently picking one.

`common/outbox` (`backend-go/common/outbox/outbox.go`) is a ready-made,
generic relay already used by two services end-to-end —
`usage-service` (`usage.outbox_events`,
`backend-go/services/usage-service/internal/adapter/postgres/repository.go:40-99`)
and `issue-tracking-service`
(`issuetracking.outbox_events`,
`backend-go/services/issue-tracking-service/internal/adapter/postgres/repository.go:40-85`)
— this solution follows that exact precedent for `ai_provider.outbox`
rather than inventing a fourth shape.

## Design — schema

```sql
-- 0003_account_registration_fields.up.sql
ALTER TABLE ai_provider.accounts
  ADD COLUMN quota_limit_day      INTEGER NOT NULL DEFAULT 0,  -- 0 = unlimited, per §5
  ADD COLUMN last_health_check_at TIMESTAMPTZ,
  ADD COLUMN created_by           UUID,
  -- Tier 2 — not in §5's sketch, added here per this doc's rationale.
  ADD COLUMN models               TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN is_default           BOOLEAN NOT NULL DEFAULT false;

-- At most one default per (tenant, dev_server, provider_type) — the
-- demotion invariant enforced at the DB level, not just app code, mirroring
-- credential-broker-service's unique_vault_path posture (defense in depth,
-- not "trust the usecase layer alone").
CREATE UNIQUE INDEX uq_accounts_one_default_per_dev_server_provider
  ON ai_provider.accounts (tenant_id, dev_server_id, provider_type)
  WHERE is_default AND deleted_at IS NULL;

-- quotaLimitDay >= 1000 (BL-AIP-01's field-level validation) is enforced in
-- the domain constructor, not a CHECK constraint — 0 (unlimited) must stay
-- legal, and "no lower than 1000 unless 0" isn't expressible as a single
-- clean CHECK without duplicating the domain's own decision about what
-- counts as a valid non-zero quota.
```

```sql
-- 0004_outbox.up.sql — identical shape to usage.outbox_events /
-- issuetracking.outbox_events, see this doc's rationale section.
CREATE TABLE ai_provider.outbox (
    id           UUID PRIMARY KEY,
    tenant_id    UUID NOT NULL,
    subject      TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    version      INT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX idx_ai_provider_outbox_unpublished
    ON ai_provider.outbox (created_at) WHERE published_at IS NULL;
```

(§5's sketch names the table `ai_provider.outbox` with `event_type`/
`payload`/`created_at`/`published_at` columns and no `tenant_id`/`subject`/
`occurred_at`/`version` — this solution's shape instead matches
`common/outbox.Store`'s actual Go interface, which every existing
implementer satisfies with the richer shape above; flagged as a minor,
mechanical divergence from §5's column list, not a design disagreement.)

## Design — domain (`provider_account.go`)

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
	DevServerID        string
	Label              string     // NEW — Tier 1, "name" in BL-AIP-01's terms
	ModelHint          string     // NEW — Tier 1 (existing single-model-hint field, distinct from Models below)
	BaseURL            string     // NEW — Tier 1
	QuotaLimitDay      int        // NEW — Tier 1; 0 = unlimited
	Models             []string   // NEW — Tier 2, allowed-model allow-list (BL-AIP-01/BUG-AIP-02 dependency)
	IsDefault          bool       // NEW — Tier 2
	LastHealthCheckAt  *time.Time // NEW — Tier 1 (written by SOL-AIP-03's health-check job, not this solution)
	CreatedBy          string     // NEW — Tier 1
	RotationGraceUntil *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
```

`NewProviderAccount` gains two validations beyond the existing
scope/ref-consistency check:

```go
// ErrQuotaLimitTooLow — BL-AIP-01's field-level rule: quotaLimitPerDay must
// be either 0 (unlimited) or >= 1000; anything in between is almost always
// a units mistake (per-request vs. per-day) worth catching at write time.
var ErrQuotaLimitTooLow = errors.New("domain: quota_limit_day must be 0 (unlimited) or >= 1000")

func (a ProviderAccount) validateQuotaLimit() error {
	if a.QuotaLimitDay != 0 && a.QuotaLimitDay < 1000 {
		return ErrQuotaLimitTooLow
	}
	return nil
}
```

Label uniqueness (BL-AIP-01: "name uniqueness per (devServer, provider)")
is **not** a domain-constructor check — it needs a database round-trip, so
it lives in `CreateAccount.Execute` (below) as an explicit existence check
before insert, same pattern the codebase already uses for tenant-mismatch
checks (`create_account.go:55-57`), not a unique index — a `label` isn't
guaranteed non-empty for every legacy row this migration doesn't backfill,
so a straight `UNIQUE(dev_server_id, provider_type, label)` index would
reject two intentionally-unlabeled accounts; the app-layer check can treat
`label == ""` as "no uniqueness constraint applies."

## Design — `usecase/create_account.go`

```go
type CreateAccountInput struct {
	TenantID      string
	ProviderType  domain.ProviderType
	Scope         domain.AccountScope
	UserID        string
	ProjectID     string
	DevServerID   string   // NEW — required now; see "test-before-save gate" below for why
	Label         string   // NEW
	ModelHint     string   // NEW
	BaseURL       string   // NEW
	QuotaLimitDay int      // NEW
	Models        []string // NEW
	IsDefault     bool     // NEW
	EncryptedBlob []byte
}

func (uc *CreateAccount) Execute(ctx context.Context, in CreateAccountInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	// ... unchanged tenant-mismatch check ...

	if in.DevServerID == "" {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_NO_DEV_SERVER", "dev_server_id is required to register a provider account", nil)
	}

	// Label uniqueness per (dev_server, provider) — BL-AIP-01's field
	// validation; app-layer, not a unique index (see domain design note).
	if in.Label != "" {
		existing, err := uc.repo.List(ctx, ListAccountsFilter{TenantID: tenantID, DevServerID: in.DevServerID})
		if err != nil {
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_LABEL_CHECK_FAILED", "failed to check label uniqueness", err)
		}
		for _, acc := range existing {
			if acc.ProviderType == in.ProviderType && acc.Label == in.Label {
				return domain.ProviderAccount{}, apperrors.New(apperrors.KindAlreadyExists, "AIPROVIDER_LABEL_TAKEN", "an account with this name already exists for this provider on this dev server", nil)
			}
		}
	}

	ref, err := uc.broker.WriteCredential(ctx, WriteCredentialInput{...}) // unchanged

	// NEW — test-before-save gate. See verify_connection.go. A failed test
	// means the credential is provably bad; roll back the just-written
	// broker credential rather than persisting an account nobody can use.
	result, testErr := uc.verify.VerifyConnection(ctx, in.DevServerID, ref.ID, in.ProviderType)
	if testErr != nil || !result.Success {
		if revokeErr := uc.broker.RevokeCredential(ctx, ref.ID); revokeErr != nil {
			// Both failures are surfaced — a revoke failure here means an
			// orphaned pending credential in credential-broker-service,
			// worth knowing about even though the create still correctly
			// fails.
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_TEST_CONNECTION_FAILED",
				fmt.Sprintf("connection test failed and credential cleanup also failed: %v / %v", testErr, revokeErr), testErr)
		}
		msg := result.Message
		if testErr != nil {
			msg = testErr.Error()
		}
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_TEST_CONNECTION_FAILED", "connection test failed: "+msg, testErr)
	}

	// Still "pending", not "active" — §9/§10's push-confirmation invariant
	// is untouched; a passed live test is a necessary but not sufficient
	// condition for "active" (see rationale section).
	now := uc.now()
	account, err := domain.NewProviderAccount(
		uc.newID(), tenantID, in.ProviderType, domain.AccountStatusPending, ref.ID,
		scope, in.UserID, in.ProjectID, in.DevServerID,
		in.Label, in.ModelHint, in.BaseURL, in.QuotaLimitDay, in.Models, in.IsDefault,
		nil /* LastHealthCheckAt */, createdBy, nil, now, now,
	)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_INVALID_ACCOUNT", err.Error(), err)
	}

	if in.IsDefault {
		// Auto-demote: same dev_server+provider pair, existing defaults
		// cleared in the SAME transaction as the insert — repo.Create takes
		// an isDefault flag and does both under one DB transaction (see
		// repository.go design below), never two round trips that could
		// race a concurrent create.
	}

	if err := uc.repo.Create(ctx, account); err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_CREATE_FAILED", "failed to persist provider account", err)
	}

	// Outbox event — same transaction as the account insert, inside
	// repo.Create (see repository.go design below), not a separate call
	// here; a separate post-insert call would reopen the exact dual-write
	// race the outbox pattern exists to close.

	return account, nil
}
```

## Design — `usecase/verify_connection.go` (new)

Extracted from `TestConnection.Execute`'s existing relay logic so
`CreateAccount` and the standalone `aiProvider.testConnection` RPC share one
implementation instead of two copies of the same
`InfraFleetClient.Relay(...)` call:

```go
// verifyConnection is shared by CreateAccount (test-before-save gate) and
// TestConnection (on-demand check) — one relay call, one place that knows
// the agent method name and result-parsing shape.
func verifyConnection(ctx context.Context, infra InfraFleetClient, devServerID, credentialRef string, providerType domain.ProviderType) (ConnectionTestResult, error) {
	result, err := infra.Relay(ctx, devServerID, "ai.testProviderConnection", map[string]any{
		"credentialRef": credentialRef,
		"providerType":  string(providerType),
	})
	if err != nil {
		return ConnectionTestResult{}, err
	}
	return parseConnectionTestResult(result), nil
}
```

`TestConnection.Execute` and `CreateAccount.Execute` both call this;
`TestConnection`'s own `internal/usecase/test_connection.go` shrinks to a
thin `repo.Get` + `verifyConnection` call, `CreateAccount` calls it right
after obtaining `ref` from the broker, using the credential ref it just
wrote rather than one already on an existing account.

## Design — `CredentialBrokerClient` port addition

```go
type CredentialBrokerClient interface {
	WriteCredential(ctx context.Context, in WriteCredentialInput) (CredentialRef, error)
	RotateCredential(ctx context.Context, credentialRef string) (CredentialRef, error)
	ResolveCredential(ctx context.Context, credentialRef string) (CredentialRef, error)
	// RevokeCredential — NEW, needed for the test-before-save rollback
	// path. Mirrors credential-broker-service.md §3's RevokeCredential RPC
	// exactly (RevokeCredentialRequest -> Empty); this port's stub
	// implementation (internal/adapter/grpcclient) marks the ref revoked
	// without ever touching a secret value, same posture as every other
	// method here.
	RevokeCredential(ctx context.Context, credentialRef string) error
}
```

## Design — `adapter/postgres/repository.go`

`Create` becomes transactional (single `pgx.Tx`) to cover three writes
atomically: the account insert, the default-demotion `UPDATE` (if
`IsDefault`), and the outbox insert:

```go
func (r *Repository) Create(ctx context.Context, account domain.ProviderAccount) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin create-account tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	if account.IsDefault {
		// Demote any prior default for this dev_server+provider pair BEFORE
		// inserting the new row — the partial unique index
		// (uq_accounts_one_default_per_dev_server_provider) would otherwise
		// reject the insert outright rather than performing the demotion
		// BL-AIP-01 asks for.
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
// event — see domain/outbox.go for the event payload shape, matching
// usage-service's domain.OutboxEvent precedent.
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

`FetchUnpublished`/`MarkPublished` implementing `common/outbox.Store` are
copy-adapted from `usage-service`'s
(`backend-go/services/usage-service/internal/adapter/postgres/repository.go:103-137`)
against `ai_provider.outbox` — mechanical, no new design.

`cmd/server/main.go` gains the same `outbox.NewRelay(repo, pub,
outbox.DefaultConfig, logger)` + `go relay.Run(ctx)` wiring
`usage-service`'s `main.go` already has
(`backend-go/services/usage-service/cmd/server/main.go:100-109`).

`scanAccount` extends its `SELECT`/`Scan` list to include
`label, model_hint, base_url, quota_limit_day, models, is_default,
last_health_check_at, created_by` everywhere it's called (`Get`, `List`,
`UpdateStatus`, `Update` — the last of which finally reads back what it
just wrote, closing the "written then discarded" bug this doc's rationale
section identified).

## Dev Server Agent dependency

The test-before-save gate calls `ai.testProviderConnection` via
`InfraFleetClient.Relay` — the **same** agent method
`TestConnection.Execute` already targets today, which per that usecase's
own doc comment "doesn't exist yet"
(`backend-go/services/ai-provider-service/internal/usecase/test_connection.go:40-42`).
**This solution's backend-go changes are correct and ready, but the gate is
inert until the Dev Server Agent implements `ai.testProviderConnection`** —
same limitation BUG-AIP-01 itself already flagged for the pre-existing
`TestConnection` RPC. Whoever picks up this solution's implementation must
coordinate with `agent/`'s owner to add the JSON-RPC handler (likely in
`agent/src/relay/agent-rpc-dispatch.ts`'s `ai.*` case group, alongside
wherever `ai.credential.write`/`ai.ping` land) before this gate does
anything beyond "always fails" or "always succeeds," depending on how the
relay's error path is exercised without a real handler on the other end.
Land the backend-go half first (behind the existing inert-relay behavior,
matching how `TestConnection` already ships today) rather than blocking on
the agent-side change.

## Design — wiring (proto/REST/WS)

- `CreateAccountRequest` gains `dev_server_id`, `label`, `model_hint`,
  `base_url`, `quota_limit_day`, `models` (repeated string), `is_default`.
- `ProviderAccount` (the wire message) gains the same fields as
  `toProtoAccount` outputs — `Label`, `ModelHint` (already a field name
  collision risk with the request's `model_hint` singular hint vs. this
  bug's separate `models` list; kept as two distinct wire fields,
  `model_hint` and `models`, matching the two distinct domain fields).
- `httpgateway/ai_provider_routes.go`'s `createAccountRequestBody` gains
  the same JSON fields; `handleCreateAccount` threads them through.
- `wscompat/channels_ai_provider.go`'s `handleAiProviderCreate`'s
  `createArgs` struct gains the same fields, following
  `handleAiProviderWriteCredential`'s existing base64-decode pattern for
  any binary field (none needed here — `models` is a plain JSON string
  array).

## Test plan

- `provider_account_test.go` — `TestNewProviderAccount_QuotaLimitTooLow`
  (1–999 rejected, 0 and >=1000 accepted); existing
  `TestProviderAccount_HasNoSecretField`-style test extended to assert none
  of the 8 new fields can ever hold secret material (string/bool/int/slice
  types only, by construction).
- `create_account_test.go`:
  - `TestCreateAccount_RequiresDevServerID`
  - `TestCreateAccount_LabelUniquenessPerDevServerProvider` — same label,
    same dev server, same provider → `AlreadyExists`; same label, different
    provider → succeeds.
  - `TestCreateAccount_DefaultDemotion` — create account A with
    `IsDefault=true`, then account B with `IsDefault=true` for the same
    dev-server+provider; fetch A afterward, assert `IsDefault=false`.
  - `TestCreateAccount_TestConnectionGate` — fake `InfraFleetClient`
    returns `{success: false}`; assert `Create` fails, the fake broker's
    `RevokeCredential` was called with the ref just written, and
    `repo.Create` was never called (fake repo records zero calls) —
    regression guard against silently persisting a proven-bad account.
  - `TestCreateAccount_TestConnectionGate_RevokeFailureSurfaced` — both the
    test and the revoke fail; error message includes both.
- `adapter/postgres/repository_test.go` (integration,
  `testcontainers-go`) — `uq_accounts_one_default_per_dev_server_provider`
  actually rejects a raw double-default insert bypassing the demotion path
  (constraint exists independent of app-layer correctness); `Create`'s
  transaction rolls back the account insert if the outbox insert fails
  (inject a constraint violation on the outbox table in the test).
- `adapter/postgres/repository_test.go` — `Update` round-trips
  `label`/`model_hint`/`base_url` (the specific bug this bug report found:
  written then discarded).
- Outbox contract test, mirroring `usage-service`'s: `FetchUnpublished`
  returns the row with `published_at IS NULL` immediately after `Create`;
  `MarkPublished` clears it from a subsequent `FetchUnpublished` call.

## References

- `specs/backend-go/tdd/services/ai-provider-service.md:99-123` (§4 domain
  model), `:124-184` (§5 schema, `ai_provider.outbox`), `:250-336` (§9
  ciphertext-push/pending-until-confirmed invariant), `:357-362` (§10
  cutover-ordering — `Resolve` must not return a pending account)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:82-98`
  (transactional outbox + async events, the default cross-service pattern)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:44`
  (auth-service owns "audit log" — cited for why this solution routes
  through the outbox instead of inventing a cross-service audit write)
- `specs/backend-go/tdd/services/credential-broker-service.md:118-138` (§3
  `RevokeCredential` RPC this solution's port addition mirrors), `:259`
  (`unique_vault_path` constraint precedent for this solution's partial
  unique index)
- `backend-go/services/ai-provider-service/internal/domain/provider_account.go:136-221`
- `backend-go/services/ai-provider-service/internal/usecase/create_account.go:1-98`
- `backend-go/services/ai-provider-service/internal/usecase/test_connection.go:1-65`
  (relay logic this solution extracts into `verify_connection.go`)
- `backend-go/services/ai-provider-service/internal/usecase/ports.go:114-144`
  (`CredentialBrokerClient` port this solution extends with `RevokeCredential`)
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go:41-56,132-149`
  (`Create`/`Update`'s current column lists — the write-then-discard bug)
- `backend-go/services/ai-provider-service/migrations/0001_init.up.sql`,
  `0002_dev_server_id.up.sql:10-15` (existing `label`/`model_hint`/`base_url`
  columns this solution's domain fields finally consume)
- `backend-go/common/outbox/outbox.go:1-156` (generic relay this solution
  reuses)
- `backend-go/services/usage-service/internal/adapter/postgres/repository.go:40-137`,
  `backend-go/services/usage-service/migrations/0002_outbox.up.sql:1-23`,
  `backend-go/services/usage-service/cmd/server/main.go:100-109` (outbox
  implementation precedent this solution copy-adapts)
- `backend-go/services/auth-service/internal/adapter/postgres/audit_repository.go:15-28`,
  `backend-go/proto/orca/auth/v1/auth.proto:28` (existing audit_log —
  read-only from other services today, cited for the open question this
  doc flags rather than resolves)
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto:60-67`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:35-64`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go:38-58`
- `backend-go/services/ai-provider-service/README.md` — "Deviations from
  the design doc" (confirms `Label`/`ModelHint`/`BaseURL`/`QuotaLimitDay`
  omission was a known, scoped-out gap, not an oversight) and "Known gaps"
  (`PushCiphertext` absence, cited for why this solution doesn't attempt to
  also close §9's push-confirmation gap)
- [BUG-AIP-02](../BUG-AIP-02-provider-resolution-partial.md) /
  [SOL-AIP-02](./SOL-AIP-02-provider-resolution-filtering.md) — `Models`
  field consumer
