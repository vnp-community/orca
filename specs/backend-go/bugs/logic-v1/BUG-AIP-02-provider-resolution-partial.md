# BUG-AIP-02: Provider resolution has no provider/model filtering — it can hand an agent the wrong provider's account

**Business Logic:** [BL-AIP-02](../../../../docs/logic/ai-providers/BL-AIP-02-provider-resolution.md) — Provider Account Resolution cho Agent/Workflow
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Critical
**Symptom:** A workflow step that needs, say, an OpenAI account can be silently handed back an Anthropic (or any other) account's `credential_ref`, if that's whatever the narrowest matching scope happens to contain — `ResolveProvider` never filters candidates by which provider was actually requested. There is also no way to resolve by explicit `accountId`, by a scoped-ref string (`"server:anthropic-default"`, `"project:<id>:<provider>"`, etc.), or by model name (`"claude-opus-4-5"` → `anthropic`); every one of the spec's three resolution "cases" except a stripped-down version of Case 3 is entirely absent.

---

## Spec summary

`resolveProviderAccount(ctx)` handles three input shapes — explicit `accountId`, a scope-qualified ref string, or a bare model name — and for the model case must detect the provider from the model name (`MODEL_PROVIDER_MAP`), then run a priority cascade (user-scope → project-scope → server-default → any server-scope) filtered to `status='healthy'` accounts of *that* provider, then validate the requested model is in the winning account's `models` list, with a clear error when nothing matches.

## What backend-go has

- `ResolveProvider.Resolve` implements the **priority-cascade ordering** correctly: user-scope first, then project-scope, then server-scope (tenant-wide) — `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:42-89`, each tier calling `repo.List` with `ListAccountsFilter{TenantID, Scope, ScopeRefID}` and returning the first `Resolvable()` (status==`active`) hit (`resolve_provider.go:50-82`, `firstResolvable` at `:93-100`).
- Distinguishes "no candidates at any scope" (`ReasonNoScopeMatch`) from "candidates existed but none usable" (`ReasonQuotaOrInactive`) for a clearer error — `backend-go/services/ai-provider-service/internal/domain/provider_account.go:112-134`, used at `resolve_provider.go:84-88`.
- Reachable via gRPC `ResolveProvider` (`aiprovider.proto:14,72-81`), REST `GET /v1/ai-providers/resolve` (`backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:66-93`) — WS channel `aiProvider.resolve` is not registered at all in `channels_ai_provider.go` (only create/list/update/delete/writeCredential/testConnection are).
- Ordering itself is test-covered: `TestResolveProvider_UserScopeWinsOverProjectScope` etc. per the service README (`backend-go/services/ai-provider-service/README.md`'s "What's implemented" section).

## What's missing

- **No provider-type filtering at all.** `ResolveProviderInput` (`resolve_provider.go:14-17`) and `ResolveProviderRequest` proto (`aiprovider.proto:70-75`) carry only `UserID`/`ProjectID` — no `provider`/`model` field. `ListAccountsFilter` (`backend-go/services/ai-provider-service/internal/usecase/ports.go:24-29`) has no `ProviderType` field either, and the SQL in `repository.go:76-105` never filters on `provider_type`. This means the cascade returns the first *resolvable account of any provider* at the narrowest matching scope — if a user has both an Anthropic and an OpenAI account, which one comes back depends on `ORDER BY created_at` (`repository.go:86`), not on what the caller asked for.
- **No explicit-`accountId` resolution path (spec's Case 1)** — no `Get`-by-id short-circuit exists in `ResolveProvider`; the usecase has no `AccountID` input field at all.
- **No scoped-ref-string resolution (spec's Case 2)** — none of `"server:<name-or-id>"`, `"project:<id>:<provider>"`, `"user:<provider>"`, or `"fleet:tag:<tag>:<provider>"` parsing exists anywhere in the codebase (confirmed via grep for `scopedRef`/`ScopeRef` outside the unrelated `ScopeRefID` filter param — zero hits).
- **No model → provider detection (spec's `MODEL_PROVIDER_MAP`/`detectProviderFromModel`)** — no such table or function exists in `ai-provider-service` or elsewhere in backend-go.
- **No model-in-account.models validation** — moot today since `ProviderAccount` has no `models` field at all (see BUG-AIP-01), but the spec's acceptance criterion "Validate model in account.models list" has no implementation path even in principle yet.
- **No `status='healthy'` filtering** — the spec explicitly excludes `quota_exceeded`/`invalid_key` accounts from candidates; backend-go's `AccountStatus` enum has no `healthy`/`quota_exceeded`/`invalid_key`/`unreachable`/`degraded` values at all (only `pending`/`active`/`rotating`/`revoked`/`error` — `domain/provider_account.go:49-55`), so quota/health state can never gate resolution (see BUG-AIP-03).
- **Server-scope fallback is tenant-wide, not per-dev-server** — the service's own README documents this as a known gap ("`ResolveProvider`'s server-scope fallback is tenant-wide rather than per-dev-server" — `ai-provider-service/README.md`'s "Known gaps" section), meaning `devServerId` from `ExecutionContext` is never used to scope the resolution, unlike the spec's `db.aiProviderAccounts.findAll({ devServerId, provider, ... })`.
- **`aiProvider.resolve` has no WS channel** — only reachable via REST/gRPC, not `wscompat` (confirmed: `channels_ai_provider.go:24-30` registers 6 methods, none named `resolve`).

## See also

- [missing-v1/BUG-005](../missing-v1/BUG-005-aiprovider-channels-not-implemented.md) — flags the broader `aiProvider.*` proto-thinness pattern this bug is a specific instance of (`ResolveProviderRequest` too thin), though BUG-005's own channel-wiring claims for create/list/update/delete/writeCredential/testConnection are now stale (see BUG-AIP-01's "See also").

## References

- `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:1-100`
- `backend-go/services/ai-provider-service/internal/usecase/ports.go:20-29`
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go:76-105`
- `backend-go/services/ai-provider-service/internal/domain/provider_account.go:43-65,112-134,215-221`
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto:14,70-81`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:66-93`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go:23-30` — no `aiProvider.resolve` channel
- `backend-go/services/ai-provider-service/README.md` — "Known gaps" (tenant-wide server-scope fallback) and "Deviations" sections
