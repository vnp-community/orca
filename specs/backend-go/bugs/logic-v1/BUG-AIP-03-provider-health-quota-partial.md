# BUG-AIP-03: No background health-check job and no quota-threshold write path — only a read-only usage rollup exists

**Business Logic:** [BL-AIP-03](../../../../docs/logic/ai-providers/BL-AIP-03-provider-health-quota.md) — Provider Health Check & Quota Management
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** A provider account whose API key has been revoked, or whose Dev Server has gone offline, stays `active` forever in backend-go — nothing ever pings it or flips its status, so BL-AIP-02's resolution cascade keeps handing it out and every spawn against it fails at execution time instead of being filtered out ahead of time. Separately, an account that blows through its daily token budget is never flagged: there is no code path anywhere that increments `ai_provider.usage` or checks it against a limit, so no 80%-warning or quota-exceeded alert/status-change ever fires (there is also no limit column to check against — see BUG-AIP-01).

---

## Spec summary

A background job runs every 15 minutes, pings every `healthy`/`degraded` account over its Dev Server's already-open WebSocket (`ai.ping`), and updates `status`/`latencyMs`/`lastCheckedAt` — classifying failures into `quota_exceeded`/`invalid_key`/`unreachable` and alerting (webhook + WS push) on the first two. Separately, every completed agent/workflow run calls `recordTokenUsage(accountId, tokensUsed)`, upserting into `orca_provider_usage` and checking the running total against `quotaLimitPerDay`, sending an 80%-warning alert and flipping the account to `quota_exceeded` (with an alert) once the limit is hit.

## What backend-go has

- **Schema for the daily rollup exists**: `ai_provider.usage(account_id, tenant_id, date, cost_usd, request_count)` — `backend-go/services/ai-provider-service/migrations/0001_init.up.sql:53-67`.
- **Read path exists**: `GetUsageToday` usecase reads today's rollup row via `UsageRepository.GetToday` — `backend-go/services/ai-provider-service/internal/usecase/get_usage_today.go:22-46`, backed by `repository.go:173-189`, reachable over gRPC (`aiprovider.proto:17,83-90`) and REST `GET /v1/ai-providers/usage-today` (`ai_provider_routes.go:112-127`). No WS channel for it.
- **Lifecycle status enum exists** but only covers account-management states, not health states: `pending|active|rotating|revoked|error` — `backend-go/services/ai-provider-service/internal/domain/provider_account.go:49-55`. `RotateKey` is the only usecase that ever transitions status, and only between `active`/`rotating` (`backend-go/services/ai-provider-service/internal/usecase/rotate_key.go`).
- **A one-shot `TestConnection` RPC exists** (see BUG-AIP-01) that *could* be a building block for a periodic health check, but nothing calls it on a schedule — it's caller-invoked only (`test_connection.go:27-51`).
- The service's own README explicitly documents this as an acknowledged, tracked gap, not an oversight: *"No health-check reconciliation job — the design doc's §8 'every 15 minutes, call TestConnection per active account' cron job isn't implemented; status/last_health_check_at only change via RotateKey today. last_health_check_at itself isn't modeled on ProviderAccount yet, since nothing writes it."* (`backend-go/services/ai-provider-service/README.md`, "Known gaps" section).

## What's missing

- **No background health-check job at all** — no ticker/cron/scheduler anywhere in `ai-provider-service` (confirmed via grep for `ticker`/`cron`/`scheduler` under the service — zero hits besides the README's own prose mention of the gap). `cmd/server/main.go` starts only the gRPC server and health/readiness HTTP endpoints, no periodic worker.
- **No `ai.ping` (or equivalent) relay call on a schedule** — `InfraFleetClient.Relay` (`ports.go:162-169`) is only invoked from `TestConnection`, which is itself only invoked on explicit user action (`aiProvider.testConnection`), never in a loop.
- **No health-state enum values** — `AccountStatus` has no `healthy`/`degraded`/`quota_exceeded`/`invalid_key`/`unreachable` values (`domain/provider_account.go:49-55`); the DB `CHECK` constraint enumerates the same 5 lifecycle-only values (`migrations/0001_init.up.sql:17-18`). There is nowhere to *store* a health-check classification even if the job existed.
- **No `latency_ms` / `last_checked_at` columns** on `ai_provider.accounts` (confirmed by the migration's full column list, `migrations/0001_init.up.sql:12-32`) — matches the README's own "not modeled yet" admission.
- **No alerting path** — no webhook-dispatch or WS-push-on-status-change code exists in `ai-provider-service` for either health transitions or quota events.
- **No `recordTokenUsage`-equivalent write path** — `UsageRepository` (`ports.go:76-83`) exposes only `GetToday`; there is no `Increment`/`RecordUsage` method on the interface, no implementation in `repository.go` (which only has `GetToday`, `repository.go:173-189`), and no usecase anywhere that upserts `ai_provider.usage` after a spawn/workflow step completes. Nothing in `usage-service` fills this role either — `usage-service`'s `RecordUsageSession` (`backend-go/services/usage-service/internal/usecase/record_usage_session.go`) is a different bounded context (per-session AI-CLI usage tracking), explicitly *not* this table, per `ai-provider-service.md`'s bounded-context distinction cited in both services' code comments.
- **No quota-threshold logic** — no 80%-warning check, no quota-exceeded status flip, and (per BUG-AIP-01) no `quota_limit_per_day` column to compare against in the first place.
- **`ai_provider.usage` has no `cost_usd`/`request_count` writer**, so the read-only `GetUsageToday` RPC will only ever return zeros in practice — there is no code path in the entire repo that inserts a non-zero row into this table.

## See also

None — no prior missing-v1/api-v1 bug documents this gap; the service's own README ("Known gaps" section) is the only existing acknowledgment.

## References

- `backend-go/services/ai-provider-service/internal/usecase/get_usage_today.go:1-46`
- `backend-go/services/ai-provider-service/internal/usecase/ports.go:76-83`
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go:170-189`
- `backend-go/services/ai-provider-service/internal/domain/provider_account.go:43-65`
- `backend-go/services/ai-provider-service/migrations/0001_init.up.sql:12-67`
- `backend-go/services/ai-provider-service/internal/usecase/test_connection.go:1-65`
- `backend-go/services/ai-provider-service/internal/usecase/rotate_key.go`
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto:17,83-90`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:112-127`
- `backend-go/services/usage-service/internal/usecase/record_usage_session.go` — confirmed different bounded context, not a substitute
- `backend-go/services/ai-provider-service/README.md` — "Known gaps / follow-ups" section, explicit "No health-check reconciliation job" admission
