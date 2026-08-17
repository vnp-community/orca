# scm-integration-service

Go rewrite of the TS backend's `github.ts`/`gitlab.ts`/`hosted-review.ts` SCM
integration surface — see
[`specs/backend-go/services/scm-integration-service.md`](../../../specs/backend-go/services/scm-integration-service.md)
for the full design. This service exists mainly to close **TS Gap 1**: the
current production system shells out to the `gh`/`glab` CLI through a
**shared OS keychain with no per-user isolation** — this service is built
from day one as direct HTTP REST clients instead, per-tenant OAuth tokens
resolved through `credential-broker-service`, no CLI, no shell-out, at any
point in its history (§10).

Package layout follows
[`usage-service`](../usage-service/README.md), this repository's reference
implementation.

## What's implemented

- `internal/domain/` — `Issue`, `PullRequest`, `RateLimitStatus` value
  objects and the `ScmProvider` enum (GitHub/GitLab/Bitbucket/Azure
  DevOps/Gitea), with invariant-enforcing constructors (non-empty
  repo/title), pure unit tests.
- `internal/usecase/` — `ports.go` defines the `ScmProvider` port (one
  interface, implemented once per concrete provider adapter),
  `ProviderRegistry` (resolves which concrete adapter to use for a given
  provider enum value), and `CredentialResolver` (resolves this tenant's
  OAuth token — **stubbed**, see below). `ListIssues`, `CreatePullRequest`,
  `ListPullRequests`, `GetRateLimitStatus` each: resolve the credential,
  resolve the provider adapter from the registry, delegate. Unit-tested
  against a fake `ScmProvider` to verify the dispatch logic itself — the
  right provider gets called, the resolved credential reaches it, an
  unregistered provider or a credential-resolution failure surfaces as an
  error. This is the valuable, real-logic part of this layer even though the
  underlying HTTP calls are stubbed for everything but GitHub's `ListIssues`.
- `internal/adapter/github/` — **real** `net/http` client against GitHub's
  REST API (`https://api.github.com`). `ListIssues` is a genuine
  `GET /repos/{repo}/issues` call with the resolved token as a `Bearer`
  Authorization header, parsing GitHub's actual response shape (and
  filtering out the pull requests GitHub's issues endpoint also returns).
  `CreatePullRequest`/`ListPullRequests`/`GetRateLimitStatus` return
  `ErrNotImplemented` — see the TODO comments in `client.go`.
- `internal/adapter/gitlab/` — package/`Client` structure satisfies
  `usecase.ScmProvider`; every method is a stub returning
  `ErrNotImplemented`. No real HTTP call yet.
- `internal/adapter/bitbucket/`, `internal/adapter/azuredevops/`,
  `internal/adapter/gitea/` — stub packages satisfying the interface, every
  method returns `ErrNotImplemented`.
- `internal/adapter/credentialbroker/` — **stub** `CredentialResolver`. See
  "What's stubbed" below.
- `internal/adapter/providerregistry/` — in-memory `ProviderRegistry`,
  populated once by `cmd/server/main.go` with one entry per provider
  adapter.
- `internal/adapter/grpc/` — implements the generated
  `scmintegrationv1.ScmIntegrationServiceServer`, pure wire<->usecase
  translation for the four RPCs currently in the proto
  (`ListIssues`/`CreatePullRequest`/`ListPullRequests`/`GetRateLimitStatus`).
- `cmd/server/main.go` — composition root: config load, provider registry
  wiring, gRPC server with the shared interceptor chain, health/readiness
  HTTP server, graceful shutdown on SIGTERM. **No database connection** —
  see "Known gaps".

## What's stubbed (read before relying on this service for anything real)

- **`credential-broker-service` integration is a stub.**
  `internal/adapter/credentialbroker.StubResolver` returns a fake,
  obviously-not-real token string (`stub-credential-broker-token:...`) — it
  never contacts any external system. `credential-broker-service` doesn't
  exist as a running service in this scaffold. Replace `StubResolver` with a
  real gRPC client scoped to `(tenant_id, provider, user_id)` before this
  service is deployed anywhere real tenant credentials matter — see the
  service doc §7/§9. Until then, **do not point this service's GitHub
  adapter at a real tenant repository with a real token** outside of
  controlled testing; the token would come from this stub, not a genuine
  per-user OAuth grant.
- **GitLab, Bitbucket, Azure DevOps, and Gitea clients are fully stubbed.**
  Only GitHub's `ListIssues` makes a real HTTP call. Every other
  provider/method returns a typed `ErrNotImplemented` with a `TODO` comment
  pointing at the REST endpoint that needs wiring.
- **No `rate_limit_cache` or `webhook_delivery_log` table.** The service
  doc's §5 data model calls for thin operational bookkeeping tables (last-
  known rate-limit snapshot per `(tenant_id, provider, bucket)`; append-only
  webhook delivery log). This scaffold has no database connection at all —
  `cmd/server/main.go` never opens a Postgres pool. `GetRateLimitStatus`
  always calls straight through to the (stubbed) provider adapter; there is
  no local cache to check before a burst of calls, and the §8 proactive-
  throttling requirement isn't implemented. Add `internal/adapter/postgres/`
  (directory already scaffolded, currently empty) plus a migration once this
  service needs to persist anything.
- **Only 4 of the proto's larger intended RPC surface are implemented.**
  The generated `scmintegrationv1` package (from
  `proto/orca/scmintegration/v1/scmintegration.proto`) currently defines
  `ListIssues`, `CreatePullRequest`, `ListPullRequests`,
  `GetRateLimitStatus` — this scaffold implements exactly those four. The
  service doc's §3 API surface (comments, reviewers, labels, checks, board
  views, hosted review, OAuth flow endpoints) is not yet reflected in the
  proto or this service; extend `scmintegration.proto` and this service's
  usecase/adapter layers together as more of the surface is needed, per the
  design doc's package-layout note (§6).
- **No per-provider circuit breaker.** §8 calls for one open/half-open/
  closed breaker per provider so a GitHub outage doesn't affect GitLab
  traffic. Not implemented — a provider adapter failure today just returns
  an error up through the usecase layer on every call.

## Running locally

```sh
# from backend-go/
cd services/scm-integration-service
GITHUB_BASE_URL=https://api.github.com \
GITLAB_BASE_URL=https://gitlab.com/api/v4 \
  go run ./cmd/server
```

No `DATABASE_DSN` is required — this service opens no database connection
(see "Known gaps").

## Testing

```sh
go test ./...   # unit tests only — no external deps, no Docker required
```

`internal/adapter/github`'s test uses `httptest.Server` to exercise the real
request/response path (method, path, Authorization header, real GitHub JSON
response shape) without any network dependency on `api.github.com`.

## Known gaps / follow-ups (tracked, not silently skipped)

- **`credential-broker-service` is stubbed**, per "What's stubbed" above —
  the single highest-priority follow-up, since per-tenant credential
  isolation is this service's entire reason for existing (§9, TS Gap 1).
- **GitLab/Bitbucket/Azure DevOps/Gitea adapters are stubbed** — only
  GitHub's `ListIssues` is real.
- **No `rate_limit_cache`/`webhook_delivery_log` tables, no database
  connection at all.** §5's data model isn't implemented.
- **No per-provider circuit breaking or proactive rate-limit throttling**
  (§8).
- **`common/secrets` (Vault) is not wired into this service's `main.go`** —
  not applicable yet since there's no database credential to render, but
  will matter once a real `credential-broker-service` gRPC client needs its
  own mTLS/service credentials.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in (see that
  package's doc comment).
- **`api-gateway`'s `grpc-gateway` REST facade** (mentioned in the service
  doc §3) isn't wired up in this scaffold — this service is gRPC-only today.
