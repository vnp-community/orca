# SOL-AIP-02: Filter provider resolution by provider/model, add per-dev-server scoping

**Resolves:** [BUG-AIP-02](../BUG-AIP-02-provider-resolution-partial.md)
**Service:** `ai-provider-service` (+ `api-gateway` for the missing `aiProvider.resolve` WS channel)
**Affected files (proposed):**
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`
- `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go`
- `backend-go/services/ai-provider-service/internal/usecase/model_provider_map.go` (new)
- `backend-go/services/ai-provider-service/internal/usecase/ports.go`
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go`
- `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go`
- `backend-go/services/ai-provider-service/internal/usecase/resolve_provider_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Priority note

This is the P0/Critical item across all three `logic-v1` ai-providers bugs:
`ResolveProvider` can hand a caller a *different provider's* account today —
not a missing feature, a correctness bug with immediate blast radius on
every agent spawn and workflow step that resolves a provider. The design
below is deliberately the smallest change that closes the filtering hole
completely, before addressing the bug's lower-severity gaps (explicit
`accountId`/scoped-ref resolution, WS channel registration).

## Design rationale (grounded in TDD)

`ai-provider-service.md` §3 already sketches the request shape this fix
needs, field-for-field — the current backend-go proto is a strict subset of
it, not a divergent design:

```protobuf
message ResolveRequest {
  string tenant_id     = 1;
  string user_id       = 2;
  string project_id    = 3;
  string dev_server_id = 4;   // target execution host — used for the ciphertext-push check (§9)
  optional string model_hint = 5;
}
```
(`ai-provider-service.md:84-90`)

Today's `ResolveProviderRequest` only carries `tenant_id`/`user_id`/
`project_id` (`aiprovider.proto:73-77`) — `dev_server_id` and `model_hint`
are simply not wired through yet. Adding them, and having the usecase
actually use them, is not a scope addition beyond the TDD; it is finishing
what §3 already specifies.

§4 states the cascade must be "filtered to `status='healthy'` accounts of
*that* provider" and is explicit that the ordering is "two-pass:
model-hint-filtered first, then unfiltered" (`ai-provider-service.md:113-116`).
Read literally, this means: **`model_hint` is the TDD's chosen mechanism for
provider filtering** — there is no separate `provider_type` field in the
TDD's own `ResolveRequest` sketch. The Go fix therefore detects
`ProviderType` from `model_hint` via a `MODEL_PROVIDER_MAP`-equivalent
(BUG-AIP-02's own finding: "no such table or function exists... anywhere in
backend-go") and filters every cascade tier's candidate list to that
detected type. This closes "hands back an Anthropic account when an OpenAI
one was requested" completely for the case backend-go's actual callers hit
today (`task-service`/`workflow-service` resolving *before* a spawn, which
always know the target model — `ai-provider-service.md` §7's "Called by"
table).

§7 states plainly: `Resolve` "does **not** call `infra-fleet-service` or
`credential-broker-service` synchronously... reads only this service's own
`accounts`/`usage_daily` tables" and targets **p99 < 20ms** because of that
(`ai-provider-service.md:224-235`). The fix below adds zero cross-service
calls — `MODEL_PROVIDER_MAP` is an in-process static table, matching that
budget exactly.

**A capability that already exists and is simply never used**: `ports.go`'s
`ListAccountsFilter` already has a `DevServerID` field
(`backend-go/services/ai-provider-service/internal/usecase/ports.go:28`),
and `repository.go`'s `List` SQL already filters on it
(`backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go:85`,
`AND ($4 = '' OR dev_server_id = $4)`) — `resolve_provider.go` never sets it
on any of its three `ListAccountsFilter{...}` calls
(`resolve_provider.go:52,64,75`). This is the concrete cause of the
service's own README-documented gap, "`ResolveProvider`'s server-scope
fallback is tenant-wide rather than per-dev-server"
(`ai-provider-service/README.md`, "Known gaps") — closing it is a
one-line-per-tier change, not new plumbing.

**Extensions beyond what `ai-provider-service.md` §3 sketches** — flagged
explicitly, same posture SOL-009 took for `git-gateway-service`'s file I/O
extension:

- **Explicit-`accountId` resolution (BL-AIP-02's Case 1)** and
  **scope-qualified ref-string resolution (Case 2, `"server:<id>"` /
  `"project:<id>:<provider>"` / `"user:<provider>"`)** have no message field
  in the TDD's `ResolveRequest` sketch at all. Proposed as an addition to
  `ResolveProviderRequest` below, needed for BL-AIP-02 acceptance criteria
  but not literally present in `ai-provider-service.md` §3 — flag this to
  whoever owns that doc as a sketch gap, don't treat its absence as "this
  fix is optional."
- **`status='healthy'` filtering** cannot be implemented until
  [BUG-AIP-03](../BUG-AIP-03-provider-health-quota-partial.md)'s health-state
  enum values exist (`domain.AccountStatus` today has no
  `healthy`/`degraded`/`quota_exceeded`/`invalid_key`/`unreachable`, only
  lifecycle states — see [SOL-AIP-03](./SOL-AIP-03-provider-health-quota.md)).
  This solution's `Resolvable()` continues to gate on `status == active`
  only; SOL-AIP-03 is responsible for widening it.
- **Model-in-`account.models` validation** cannot be implemented until
  [BUG-AIP-01](../BUG-AIP-01-register-provider-account-partial.md)'s
  `Models` field exists on `ProviderAccount` (see
  [SOL-AIP-01](./SOL-AIP-01-register-provider-account.md)). This solution's
  `detectProviderFromModel` only narrows the *provider* candidate set; it
  does not yet check that the winning account actually serves the
  requested model — cross-referenced, not silently dropped.

## Design — proto (`aiprovider.proto`)

```protobuf
message ResolveProviderRequest {
  string tenant_id     = 1;
  string user_id       = 2;
  string project_id    = 3;
  string dev_server_id = 4;  // NEW — matches ai-provider-service.md §3's ResolveRequest field-for-field
  optional string model_hint = 5;  // NEW — same

  // NEW — extensions beyond §3's sketch, flagged above. Zero value on both
  // means "run the normal cascade"; setting account_id short-circuits it
  // entirely (Case 1); setting scoped_ref parses and resolves directly
  // (Case 2). Mutually exclusive with each other and with the cascade
  // fields above — the usecase validates precedence, proto doesn't enforce
  // a oneof here to keep wire compatibility trivial for existing callers
  // that only ever set tenant_id/user_id/project_id today.
  string account_id  = 6;
  string scoped_ref   = 7;
}
```

## Design — `usecase/model_provider_map.go` (new)

```go
// modelProviderMap maps known model-name prefixes to the ProviderType that
// serves them — the Go equivalent of the TS resolver's MODEL_PROVIDER_MAP
// (ai-provider-service.md §4). Longest-prefix-wins so e.g. "claude-3-5"
// doesn't need its own entry once "claude-" is present.
var modelProviderMap = []struct {
	prefix   string
	provider domain.ProviderType
}{
	{"claude-", domain.ProviderTypeAnthropic},
	{"gpt-", domain.ProviderTypeOpenAI},
	{"o1-", domain.ProviderTypeOpenAI},
	{"o3-", domain.ProviderTypeOpenAI},
	{"gemini-", domain.ProviderTypeGoogle},
	// azure/aws_bedrock/ollama/vllm models don't have a stable global
	// prefix (they're deployment-name-shaped) — callers targeting those
	// providers must set dev_server_id + rely on scope, or (once
	// implemented) scoped_ref, not model_hint detection.
}

// detectProviderFromModel returns the ProviderType a model name belongs to,
// and false if no known prefix matches — mirrors the TS resolver's
// detectProviderFromModel, used only to narrow ResolveProvider's cascade,
// never to reject a request outright (a caller supplying dev_server_id
// without model_hint still resolves normally, unfiltered by provider).
func detectProviderFromModel(model string) (domain.ProviderType, bool) {
	for _, e := range modelProviderMap {
		if strings.HasPrefix(model, e.prefix) {
			return e.provider, true
		}
	}
	return "", false
}
```

## Design — `usecase/resolve_provider.go`

```go
type ResolveProviderInput struct {
	UserID      string
	ProjectID   string
	DevServerID string  // NEW — threads into every ListAccountsFilter tier
	ModelHint   string  // NEW — detected to a ProviderType filter, per §4's two-pass note
	AccountID   string  // NEW — Case 1, short-circuits the cascade entirely
	ScopedRef   string  // NEW — Case 2, parsed then resolved directly
}

func (uc *ResolveProvider) Resolve(ctx context.Context, in ResolveProviderInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}

	// Case 1 — explicit accountId short-circuits everything else.
	if in.AccountID != "" {
		account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
		if err != nil {
			return domain.ProviderAccount{}, err
		}
		if !account.Resolvable() {
			return domain.ProviderAccount{}, &domain.ErrNoProviderAvailable{Reason: domain.ReasonQuotaOrInactive}
		}
		return account, nil
	}

	// Case 2 — scoped ref string, parsed then resolved directly (see
	// resolve_scoped_ref.go — new file, same package).
	if in.ScopedRef != "" {
		return uc.resolveScopedRef(ctx, tenantID, in.ScopedRef)
	}

	// Case 3 — cascade, now filtered by provider (detected from ModelHint)
	// and scoped to DevServerID at every tier.
	var providerFilter domain.ProviderType
	if in.ModelHint != "" {
		if p, ok := detectProviderFromModel(in.ModelHint); ok {
			providerFilter = p
		}
	}

	sawAnyCandidate := false

	if in.UserID != "" {
		accounts, err := uc.repo.List(ctx, ListAccountsFilter{
			TenantID: tenantID, Scope: domain.ScopeUser, ScopeRefID: in.UserID,
			DevServerID: in.DevServerID, ProviderType: providerFilter,
		})
		if err != nil {
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_RESOLVE_FAILED", "failed to list user-scope accounts", err)
		}
		if acc, ok := firstResolvable(accounts); ok {
			return acc, nil
		}
		sawAnyCandidate = sawAnyCandidate || len(accounts) > 0
	}

	if in.ProjectID != "" {
		accounts, err := uc.repo.List(ctx, ListAccountsFilter{
			TenantID: tenantID, Scope: domain.ScopeProject, ScopeRefID: in.ProjectID,
			DevServerID: in.DevServerID, ProviderType: providerFilter,
		})
		// ... same shape, sawAnyCandidate accumulation unchanged
	}

	accounts, err := uc.repo.List(ctx, ListAccountsFilter{
		TenantID: tenantID, Scope: domain.ScopeServer,
		DevServerID: in.DevServerID, ProviderType: providerFilter,
	})
	// ... unchanged tail
}
```

`firstResolvable`/error-reason logic is untouched — the fix is entirely in
what gets passed to `ListAccountsFilter`, not the cascade's control flow.

### `resolve_scoped_ref.go` (new — Case 2)

```go
// resolveScopedRef parses "server:<provider>", "project:<id>:<provider>",
// or "user:<provider>" and resolves directly against that scope — BL-AIP-02
// Case 2, not sketched in ai-provider-service.md §3 (flagged in this
// solution's rationale section as an extension beyond the TDD).
func (uc *ResolveProvider) resolveScopedRef(ctx context.Context, tenantID, ref string) (domain.ProviderAccount, error) {
	parts := strings.SplitN(ref, ":", 3)
	var scope domain.AccountScope
	var scopeRefID string
	var providerStr string
	switch {
	case len(parts) == 2 && parts[0] == "server":
		scope, providerStr = domain.ScopeServer, parts[1]
	case len(parts) == 2 && parts[0] == "user":
		scope, providerStr = domain.ScopeUser, parts[1]
		// scopeRefID left empty on purpose: "user:<provider>" resolves
		// against the CALLING user, taken from ctx, not embedded in the ref
		// string — mirrors how tenant_id is never trusted from a client body.
	case len(parts) == 3 && parts[0] == "project":
		scope, scopeRefID, providerStr = domain.ScopeProject, parts[1], parts[2]
	default:
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_INVALID_SCOPED_REF", "unrecognized scoped_ref format: "+ref, nil)
	}
	provider := domain.ProviderType(providerStr)
	if !provider.Valid() {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_INVALID_SCOPED_REF", "unrecognized provider in scoped_ref: "+providerStr, nil)
	}
	if scope == domain.ScopeUser {
		userID, ok := tenant.UserID(ctx)
		if !ok {
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_USER", "no user in request context for user:-scoped ref", nil)
		}
		scopeRefID = userID
	}
	accounts, err := uc.repo.List(ctx, ListAccountsFilter{TenantID: tenantID, Scope: scope, ScopeRefID: scopeRefID, ProviderType: provider})
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_RESOLVE_FAILED", "failed to list accounts for scoped_ref", err)
	}
	if acc, ok := firstResolvable(accounts); ok {
		return acc, nil
	}
	return domain.ProviderAccount{}, &domain.ErrNoProviderAvailable{Reason: domain.ReasonQuotaOrInactive}
}
```

## Design — `usecase/ports.go`

```go
type ListAccountsFilter struct {
	TenantID     string
	Scope        domain.AccountScope
	ScopeRefID   string
	DevServerID  string
	ProviderType domain.ProviderType // NEW — zero value = any provider, matches DevServerID's "empty = no filter" convention
}
```

## Design — `adapter/postgres/repository.go`

One filter clause added, following the SQL file's existing `$N = '' OR
col = $N` idiom exactly (no new query shape):

```sql
SELECT id, tenant_id, provider_type, status, credential_ref,
       scope, user_id, project_id, dev_server_id, rotation_grace_until, created_at, updated_at
FROM ai_provider.accounts
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND ($2 = '' OR scope = $2)
  AND ($3 = '' OR user_id = $3::uuid OR project_id = $3::uuid)
  AND ($4 = '' OR dev_server_id = $4)
  AND ($5 = '' OR provider_type = $5)   -- NEW
ORDER BY created_at
```
`List(ctx, filter)` passes `string(filter.ProviderType)` as `$5`.

## Design — wiring (`grpc`/REST/`wscompat`)

- `adapter/grpc/server.go`'s `ResolveProvider` handler threads
  `req.GetDevServerId()`, `req.GetModelHint()`, `req.GetAccountId()`,
  `req.GetScopedRef()` into `ResolveProviderInput` alongside the existing
  `UserId`/`ProjectId`.
- `httpgateway/ai_provider_routes.go`'s `handleResolveProvider` reads
  `dev_server_id`, `model_hint`, `account_id`, `scoped_ref` query params in
  addition to the existing `user_id`/`project_id`.
- **New**: `wscompat/channels_ai_provider.go` gains `aiProvider.resolve`,
  registered alongside the existing 6 channels in
  `registerAiProviderChannels` (`channels_ai_provider.go:23-30`), following
  `handleAiProviderCreate`'s exact shape — `decodeArg` a JSON args struct,
  `attachAiProviderIdentity`, call the client, return `resp.GetAccount()`.
  This closes the bug's "no WS channel" finding, the one gap in this bug
  that's pure wiring, no usecase logic.

## Test plan

- `resolve_provider_test.go` — **regression guard for the exact symptom**:
  seed one Anthropic account and one OpenAI account both at user scope for
  the same user; resolve with `model_hint="claude-3-5-sonnet"` must return
  the Anthropic account, never the OpenAI one, regardless of `created_at`
  ordering (construct the OpenAI account with an earlier `created_at` to
  prove the old bug — "whichever comes back depends on `ORDER BY
  created_at`" — is actually fixed, not coincidentally passing).
- Same test shape at project- and server-scope tiers.
- `TestResolveProvider_DevServerScoping` — two server-scope accounts for
  the same tenant/provider, bound to different `dev_server_id`s; resolving
  with a specific `dev_server_id` must never return the other server's
  account — closes the README's tenant-wide-fallback gap directly.
- `TestDetectProviderFromModel` — table-driven over the map; unknown model
  name returns `false`, cascade proceeds unfiltered (existing behavior
  preserved for callers with no model hint).
- `TestResolveProvider_ExplicitAccountID` — bypasses scope entirely;
  inactive/quota-excluded account returns `ErrNoProviderAvailable`, not the
  account.
- `TestResolveScopedRef` — table-driven over `"server:openai"`,
  `"project:<id>:anthropic"`, `"user:google"` (using `tenant.WithUserID` on
  the test context), and a malformed-string rejection case.
- `wscompat/channels_ai_provider_test.go` — new test for `aiProvider.resolve`
  using a fake `AiProviderServiceClient`, matching the existing 6 tests'
  shape in that file.
- Regression: existing `TestResolveProvider_UserScopeWinsOverProjectScope`
  (cited in the service's README) must still pass unmodified — the ordering
  logic itself is untouched by this fix.

## References

- `specs/backend-go/tdd/services/ai-provider-service.md:61-97` (§3 API
  surface, `ResolveRequest`'s exact field list this solution extends
  toward), `:99-123` (§4 domain model, cascade ordering + two-pass
  model-hint-filtered note), `:213-227` (§7, no cross-service call from
  `Resolve`), `:229-236` (§8, p99 < 20ms budget)
- `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:1-100`
- `backend-go/services/ai-provider-service/internal/usecase/ports.go:20-29`
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go:76-105`
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto:69-81`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go:23-58`
  (pattern `aiProvider.resolve`'s new handler follows)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:66-93`
- `backend-go/services/ai-provider-service/README.md` — "Known gaps"
  (tenant-wide server-scope fallback, this solution's `DevServerID` fix)
- [BUG-AIP-01](../BUG-AIP-01-register-provider-account-partial.md) /
  [SOL-AIP-01](./SOL-AIP-01-register-provider-account.md) — `Models` field
  dependency for model-in-account validation
- [BUG-AIP-03](../BUG-AIP-03-provider-health-quota-partial.md) /
  [SOL-AIP-03](./SOL-AIP-03-provider-health-quota.md) — health-state enum
  dependency for `status='healthy'` filtering
