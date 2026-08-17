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

## What's implemented (Phase 3, docs/execution-plan.md §3)

- `internal/domain/` — `Issue`, `PullRequest`, `RateLimitStatus` value
  objects and the `ScmProvider` enum (GitHub/GitLab/Bitbucket/Azure
  DevOps/Gitea), with invariant-enforcing constructors (non-empty
  repo/title), pure unit tests.
- `internal/usecase/` — `ports.go` defines every port this service's
  usecases depend on (`ScmProvider`, `ProviderRegistry`,
  `CredentialResolver`/`CredentialWriter`, `OAuthExchanger`/
  `OAuthExchangerRegistry`, `OAuthStateCodec`, `RateLimitCache`).
  `ListIssues`, `CreatePullRequest`, `ListPullRequests`,
  `GetRateLimitStatus`, `GetAuthStatus`, `StartOAuthFlow`,
  `CompleteOAuthFlow`, `RevokeAuth` each: resolve what they need from a
  port, delegate, translate to a typed `apperrors.AppError`. Unit-tested
  against fakes for every port — dispatch correctness is what this layer
  is responsible for, not the HTTP call itself.
- **Every provider adapter makes real HTTP calls now** — see the table
  below. `internal/adapter/{github,gitlab,bitbucket,azuredevops,gitea}/`
  each implement `usecase.ScmProvider` as a standalone `net/http` client,
  no shared base class, each with its own `httptest.Server`-backed test
  suite.
- `internal/adapter/oauth/` — real OAuth 2.0 authorization-code client
  (`AuthorizationURL` + `ExchangeCode`), one instance per provider
  configured with that provider's real authorize/token URLs (§9.1's
  decision — see below).
- `internal/adapter/oauthstate/` — stateless, HMAC-signed `OAuthStateCodec`
  binding a `StartOAuthFlow` call to its later `CompleteOAuthFlow` callback
  without a database row (§9.1, §5).
- `internal/adapter/credentialbroker/` — real `CredentialResolver` (Epic B)
  **and now real `CredentialWriter`** (`WriteCredential`, exercised for the
  first time by `CompleteOAuthFlow` — see "OAuth account-connect flow" and
  "Known gaps" below for the honest caveat on `encrypted_envelope`).
- `internal/adapter/postgres/` — real `RateLimitCache` backed by
  `scm.rate_limit_cache` (see "Data model" below). This service now opens a
  real Postgres connection at startup.
- `internal/adapter/providerregistry/` — in-memory `ProviderRegistry` and
  `OAuthRegistry` (one entry per provider adapter / configured OAuth app).
- `internal/adapter/grpc/` — implements the generated
  `scmintegrationv1.ScmIntegrationServiceServer` for all eight RPCs
  currently in the proto.
- `cmd/server/main.go` — composition root: config load, provider registry
  wiring, OAuth registry wiring, real Postgres connection
  (`scm.rate_limit_cache`), gRPC server with the shared interceptor chain,
  health/readiness HTTP server (now also pings Postgres), graceful shutdown
  on SIGTERM.

### Provider RPC coverage

| Provider | ListIssues | CreatePullRequest | ListPullRequests | GetRateLimitStatus |
|---|---|---|---|---|
| GitHub | real | real | real | real (`GET /rate_limit`, "core" bucket) |
| GitLab | real | real ("merge requests") | real | real (`RateLimit-*` response headers via `GET /user`) |
| Bitbucket | real | real (nested source/destination branch shape) | real | real (`X-RateLimit-*` headers, best-effort — see adapter doc comment) |
| Azure DevOps | `ErrCapabilityUnsupported` — no native issue concept, see below | real (`refs/heads/`-prefixed branch names) | real | real (`X-RateLimit-*` headers via `GET /_apis/projects`) |
| Gitea | real (GitHub-shaped API) | real | real | `ErrCapabilityUnsupported` — see below |

GitHub is the reference implementation (§3 of the migration scope) —
`internal/adapter/github/` was built first and every other adapter mirrors
its conventions (constructor shape, error wrapping, auth-header
construction immediately before dispatch, `httptest.Server` test style).

**`ErrCapabilityUnsupported` vs `ErrNotImplemented`**: two distinct
sentinels, per `scm-integration-service.md` §4's "a provider that doesn't
support an operation returns typed `ErrCapabilityUnsupported`" precedent.
`ErrNotImplemented` means "not built yet, should be"; `ErrCapabilityUnsupported`
means "this provider genuinely cannot do this, by design" —
`azuredevops.Client.ListIssues` (Azure DevOps has no native issue concept;
work items are a different, more complex, heavily-typed system out of
scope for this pass) and `gitea.Client.GetRateLimitStatus` (Gitea has no
rate-limiting concept in its documented API contract) both return the
latter, deliberately, not as a stub.

Azure DevOps' `repo` string convention is `"org/project/repositoryId"`
(three path segments); every other provider uses the familiar
`"owner/repo"` shape (GitLab additionally accepts a numeric project id).

## OAuth account-connect flow — §9.1 decision: OAuth web flow

**Decision**: this service implements the spec's **recommended default** —
a standard OAuth 2.0 authorization-code web flow
(`StartOAuthFlow`/`CompleteOAuthFlow`/`GetAuthStatus`/`RevokeAuth`), **not**
the TS `gh auth login`/`glab auth login` PTY/CLI mechanism it replaces. The
spec (§9.1) frames this as "lower-friction... but changes the UX from what
TS users see today — confirm with product before it becomes the shipped
default." This scaffold builds the recommended default so the RPC surface
and credential-write path exist and are tested; **product sign-off on
shipping it as the actual replacement for TS's PTY flow is still an open
step**, exactly as §9.1 says it should be, not something this change can
unilaterally settle by writing code.

Why the web flow and not PTY-based CLI login, concretely: no live PTY or
Dev Server Agent connection needed to complete authentication (§9.1's own
stated tradeoff), and it fits this service's architecture directly — no
other RPC in this service touches a PTY, a CLI, or a shared keychain, and
introducing one just for auth would reintroduce exactly the TS Gap 1
mechanism (§10) this whole service exists to avoid, even if scoped
per-user. If PTY-based login is ever kept for a transitional UX reason, the
spec is explicit it must still terminate in the same
`credential-broker-service` write path below — never a shared keychain.

What was built:

- `StartOAuthFlow(tenant_id, user_id, provider, redirect_uri)` → resolves
  that provider's `OAuthExchanger` (configured with real authorize/token
  URLs — see `internal/config`), mints a signed, stateless state token
  (`internal/adapter/oauthstate`, 15-minute TTL) binding
  tenant/user/provider/redirect_uri, returns `{authorization_url, state}`.
- `CompleteOAuthFlow(tenant_id, provider, code, state, redirect_uri)` →
  decodes+verifies the state token (rejects tampering, expiry, and a
  request whose `tenant_id`/`provider` don't match what was signed —
  the state token is the source of truth, not the request body), calls the
  real provider token endpoint (`internal/adapter/oauth`), writes the
  resulting access token via `credential-broker-service.WriteCredential`
  (`CREDENTIAL_CATEGORY_SCM_OAUTH`, `owner_id` = provider name — same
  convention `CredentialResolver` already established for reads).
- `GetAuthStatus(tenant_id, provider)` → attempts `CredentialResolver.Resolve`;
  connected = resolved a non-empty token. A resolve failure reports
  `connected: false`, not an RPC error — "not connected yet" is an expected
  answer for this RPC, not a failure of the check itself.
- `RevokeAuth(tenant_id, provider)` → calls `CredentialRevoker.RevokeByOwner`,
  which reaches `credential-broker-service.RevokeCredentialByOwner`
  (`CREDENTIAL_CATEGORY_SCM_OAUTH`, `owner_id` = provider name — same
  convention `CredentialResolver`/`CredentialWriter` already established).
  **Previously a typed stub error** (`SCM_REVOKE_REQUIRES_BROKER_RPC`) —
  see "Known gaps" below for the now-resolved gap this closes.

No database row is used to correlate `StartOAuthFlow`/`CompleteOAuthFlow` —
the state token itself is self-contained and signed (§5's data model is
deliberately silent on an `oauth_state` table; this service's Postgres
schema still holds only `rate_limit_cache`/`webhook_delivery_log`).

**OAuth app registration is per-provider config, most of it unset by
default.** `GITHUB_OAUTH_CLIENT_ID`/`_SECRET` etc. (see `internal/config`)
default to `""` — a provider with no configured `ClientID` is simply absent
from `cmd/server/main.go`'s `OAuthRegistry`, so `StartOAuthFlow` for that
provider returns `SCM_PROVIDER_UNSUPPORTED` until an operator registers a
real OAuth app and sets its client id/secret. `OAUTH_STATE_SECRET` defaults
to an insecure, well-known dev value — **any real deployment must override
it**, or every minted state token is forgeable.

## `credential-broker-service` is wired for both reads and writes

`internal/adapter/credentialbroker.Resolver` dials `credential-broker-service`
for real (`cfg.CredentialBrokerAddr`) and implements both
`usecase.CredentialResolver` (`ResolveCredentialByOwner`, Epic B) and, as of
Phase 3, `usecase.CredentialWriter` (`WriteCredential`, exercised for the
first time by `CompleteOAuthFlow`). `owner_id` is the provider name itself
(`"github"`, `"gitlab"`, etc.) for both, since `CREDENTIAL_CATEGORY_SCM_OAUTH`
is one category shared by every provider.

**Honest caveat on `encrypted_envelope`** (investigated as part of this
change, not assumed): `WriteCredentialRequest`'s proto doc comment
describes a client-side-encrypted envelope, but **no service in this
codebase implements that half of the design** —
`credential-broker-service`'s own `WriteCredential` usecase treats the
field as opaque bytes end to end and forwards it straight into Vault
Transit (see that service's `write_credential.go` doc comment and its own
README "Known gaps"); `ai-provider-service`'s only non-nil caller is a pure
passthrough of externally-supplied bytes. There is no established
client-side sealing step anywhere in this codebase to reuse, and building
one is net-new infrastructure spanning at least two services — out of
scope for this change (`credential-broker-service` itself is explicitly out
of bounds here). Given that, `credentialbroker.Resolver.Write` sends the
exchanged OAuth access token's bytes directly as `encrypted_envelope`; Vault
Transit at the broker is what protects it at rest, matching this
codebase's actual (not aspirational) contract today.

## Data model — `rate_limit_cache` real, `webhook_delivery_log` schema-only

`migrations/0001_init.up.sql` adds both tables from
`scm-integration-service.md` §5, in the `scm` schema, Row-Level-Security
tenant-isolation policy included (matching every other service's Postgres
migration in this codebase):

- **`rate_limit_cache` — real, consumed by `GetRateLimitStatus`.**
  `internal/usecase.GetRateLimitStatus` reads through the cache first (a
  snapshot fresher than 60s skips the live provider call entirely) and
  writes the live result back after every real provider call
  (`internal/adapter/postgres.RateLimitCacheRepository`, real
  `httptest`-free Postgres integration tests via testcontainers-go, see
  `-tags=integration`). **Scoped to `GetRateLimitStatus` only** — the
  fuller §8 requirement ("every other usecase populates this cache from
  its own response's rate-limit headers, and checks it proactively before
  a burst of calls") is not implemented; `ListIssues`/`CreatePullRequest`/
  `ListPullRequests` still call the provider directly every time, same as
  before this table existed.
- **`webhook_delivery_log` — schema-only, no writer.** No
  webhook-receiving RPC/endpoint exists anywhere in this service's proto or
  usecase layer — matching this doc's own "don't build speculative
  consumers" precedent (Epic G). The table exists so a future webhook
  receiver has somewhere to write idempotency records, per §5's spec; until
  that receiver is built, this table is never queried or written.

This service now opens a real Postgres connection at startup
(`DATABASE_CREDENTIALS_FILE`, falling back to `DATABASE_DSN` — same
Vault-Agent-rendered-credentials convention as `usage-service`'s config).

## Running locally

```sh
# from backend-go/
cd services/scm-integration-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/scm?sslmode=disable \
GITHUB_BASE_URL=https://api.github.com \
GITLAB_BASE_URL=https://gitlab.com/api/v4 \
  go run ./cmd/server
```

A real Postgres connection is now required at startup (see "Data model"
above) — run `migrate -path migrations -database "$DATABASE_DSN" up` first.

## Testing

```sh
go test ./...                        # unit tests only — no external deps, no Docker required
go test -tags=integration ./...      # + internal/adapter/postgres's real-Postgres tests (testcontainers-go, needs Docker + the `migrate` CLI)
```

Every provider adapter's test uses `httptest.Server` to exercise the real
request/response path (method, path, auth header, real provider JSON
response shape) without any network dependency on the provider's actual
API.

## Known gaps / follow-ups (tracked, not silently skipped)

- ~~`RevokeAuth` cannot revoke anything yet — needs a new
  credential-broker-service RPC.`~~ **Closed.** `credential-broker-service`
  now exposes `RevokeCredentialByOwner` (mirroring `ResolveCredentialByOwner`
  on the read side), and `RevokeAuth` calls it for real via
  `usecase.CredentialRevoker` (`internal/adapter/credentialbroker.Resolver.
  RevokeByOwner`). Note the resulting idempotency shape: because
  `RevokeCredentialByOwner`'s `GetByOwner` lookup filters revoked rows out
  at the SQL level (see `credential-broker-service`'s README), revoking an
  already-disconnected provider returns a broker-side not-found error here
  too — `RevokeAuth` does not treat "already revoked" as a silent success.
- **`encrypted_envelope` is not actually client-side-encrypted anywhere in
  this codebase** — see "credential-broker-service is wired" above for the
  full investigation and honest current contract.
- **§9.1's OAuth-web-flow-vs-PTY-CLI-login decision still needs product
  sign-off to ship as the default**, per the spec's own framing — this
  change builds the recommended default, it doesn't unilaterally decide UX.
- **`rate_limit_cache` is only wired into `GetRateLimitStatus`**, not into
  the other three data-fetching usecases' proactive-throttling path (§8's
  fuller requirement) — see "Data model" above.
- **`webhook_delivery_log` is schema-only** — no webhook-receiving
  RPC/endpoint exists to write to it yet.
- **GitLab/Bitbucket/Azure DevOps/Gitea got breadth (all four RPCs each,
  minus the two deliberate `ErrCapabilityUnsupported` cases), not GitHub's
  full depth** — none of the five adapters implement comments, reviewers,
  labels, checks, board views, or hosted-review creation; those RPCs don't
  exist in the proto yet either. `scm-integration-service.md` §3's larger
  API surface (comments/reviewers/labels/checks/board views/hosted review)
  remains unbuilt — extend `scmintegration.proto` and this service's
  usecase/adapter layers together as more of that surface is needed, per
  the design doc's package-layout note (§6).
- **No per-provider circuit breaker.** §8 calls for one open/half-open/
  closed breaker per provider so a GitHub outage doesn't affect GitLab
  traffic. Not implemented — a provider adapter failure today just returns
  an error up through the usecase layer on every call.
- **`common/secrets` (Vault) is not wired into this service's `main.go`
  directly** — `secrets.DatabaseCredentialsFromFile` is used for the
  Postgres DSN (same as `usage-service`), but this service still doesn't
  call Vault Transit/KV/SSH directly for anything else — not applicable, by
  design (`credential-broker-service` is the only service that talks to
  Vault directly for tenant secret material).
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in (see that
  package's doc comment).
- **`api-gateway`'s `grpc-gateway` REST facade** (mentioned in the service
  doc §3), including the `/auth/{provider}/callback` redirect target
  `StartOAuthFlow`'s sequence diagram assumes, isn't wired up in this
  scaffold — this service is gRPC-only today.
