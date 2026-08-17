# Execution Plan — From Scaffold to Production

**Status of this document:** grounded in the actual code in this repository
as of the scaffold build — every task below comes from a real gap found
while implementing a service (recorded in that service's own README's
"Known gaps" section), not a generic template. Cross-reference
[`specs/backend-go/migration/ts-to-go-migration-strategy.md`](../../specs/backend-go/migration/ts-to-go-migration-strategy.md)
for the TS-cutover framing this plan's phase numbers reuse; this document
is the scaffold-aware continuation of that plan, not a replacement for it.

## How to use this document

1. **Cross-cutting epics** (§2) block multiple services at once — do these
   first, in the order listed, or a service-by-service push will keep
   re-discovering the same missing piece (the Dev Server Agent relay client
   alone blocks 3 services).
2. **Per-service task lists** (§3) are grouped by the same 5 migration
   phases the strategy doc defined, since the dependency reasoning ("do
   `auth`/`tenant` last, everything depends on them") still holds for
   finishing the scaffold, not just for a TS cutover.
3. Every task lists which service's README it came from, so you can go
   read the exact stub/comment in the code before starting.
4. A service isn't done until it clears
   [`standards/production-readiness-checklist.md`](../../specs/backend-go/standards/production-readiness-checklist.md)
   — §3 tracks feature-completeness, not that checklist's operational bar
   (observability, chaos testing, etc.), which every service still needs
   regardless of which phase it's in.

---

## 1. Current state

19/19 modules (`common`, `proto`, 17 services) build, vet, and test clean
under one `go.work` workspace. See [`../README.md`](../README.md)'s "What's
genuinely real" / "What's intentionally stubbed" sections for the full
current-state summary — not repeated here.

---

## 2. Cross-cutting epics (do these first — they unblock multiple services)

### Epic A — Dev Server Agent relay client (`infra-fleet-service`)

**Why first:** the single most-repeated stub in this codebase. Blocks:
`git-gateway-service` (relay-path Push/Pull/Commit when `connected=true`),
`workflow-service` (Agent/Shell/Notification step executors), and indirectly
`task-service`'s `SimpleExecutor`/`ComplexExecutor` and `project-service`'s
future `agentSpawn`-equivalent path.

- [ ] Decide Option A vs. B from
      [`architecture/08-inter-service-communication.md`](../../specs/backend-go/architecture/08-inter-service-communication.md)
      for real (this scaffold assumed Option A — preserve the TS wire
      protocol — but that decision was never confirmed against `agent/`'s
      actual maintainers).
- [ ] Port the 3-mode connection logic (relay-ssh / relay-websocket /
      direct-websocket) and the 13-byte-framed JSON-RPC wire format into
      `services/infra-fleet-service/internal/adapter/devserveragent/`,
      replacing the stub `Client.Exec`/`Client.Health`.
- [ ] Wire `git-gateway-service`'s `grpcclient.RelayExecutor` to call the
      now-real `infra-fleet-service.ResolveConnection` + relay through it,
      replacing `ErrRelayNotImplemented`.
- [ ] Wire `workflow-service`'s `AgentStub`/`ShellStub`/`NotificationStub`
      (`internal/adapter/stepexecutors/stub.go`) to the same client.
- [ ] Add the `infra.connections` / `infra.port_forwards` /
      `infra.provider_registry_entries` tables the design doc specifies but
      this scaffold's narrower schema skipped (see
      `infra-fleet-service/README.md`'s deviations section) — currently
      `ResolveConnection` resolves directly against `dev_servers.id`, which
      won't hold once real multi-mode connections exist.

### Epic B — Wire `credential-broker-service` into its 4 consumers

**Why:** `credential-broker-service` itself is real (Vault-backed, audited,
tested). What's stubbed is everyone else's *client* to it.

- [ ] `ai-provider-service`: replace `internal/adapter/grpcclient/credential_broker_client.go`'s
      local-only `CredentialRef` synthesis with a real
      `credentialbrokerv1.NewCredentialBrokerServiceClient` call. This is
      where TS Gap 2's actual fix lands — do not let this slip.
- [ ] `scm-integration-service`: replace `internal/adapter/credentialbroker/stub.go`.
- [ ] `issue-tracking-service`: replace `internal/adapter/credential/stub_resolver.go`.
- [ ] `notification-service`: per its own README, it currently calls
      `common/secrets` **directly** for VAPID Transit signing instead of
      going through `credential-broker-service` as
      [`architecture/06-secrets-vault-architecture.md`](../../specs/backend-go/architecture/06-secrets-vault-architecture.md)
      §"credential-broker-service's role" specifies. This is a real
      deviation from the design, not just an unwired stub — fix by routing
      VAPID signing through the broker too, or explicitly amend the
      architecture doc if direct-Vault access for this one case is judged
      acceptable (infrastructure secrets vs. tenant secrets — make the call
      explicitly, don't leave it silently inconsistent).

### Epic C — Proto surface gaps found while implementing (not just "add more RPCs later")

These are cases where a service's real logic needed a cross-service RPC
that doesn't exist yet — discovered by actually writing the code, not
guessed in advance:

- [ ] **`workflow-service` + `task-service`: add `HasActiveExecutions`.**
      `project-service.RebindDevServer`'s active-execution guard (the fix
      for the TS `PROJECT_HAS_ACTIVE_WORKFLOWS` gap) is currently a no-op
      because neither service's proto exposes a way to ask "does this
      project/task have an active execution." Add the RPC to both protos,
      implement it for real, then delete the stub in
      `project-service/internal/adapter/grpcclient/`.
- [ ] **`orchestration-service`: extend the proto.** `CreateDispatchContextRequest`
      has no `orchestration_task_id`, gates have no `question`/`options`,
      and there's no `handle` field for the `KeyedSerializer` key — the
      current implementation bridges this pragmatically but
      `CreateGate` returns `FailedPrecondition` until this is fixed. See
      `orchestration-service/README.md`'s deviations section for the exact
      gap.
- [ ] **`auth-service`: decide the RPC surface, then build it.** The
      generated proto only has 8 RPCs; there's no refresh-token flow, no
      access-policy CRUD, no first-run-setup RPC. Either these were never
      needed (revisit `specs/backend-go/services/auth-service.md` and
      confirm) or the proto needs a real extension pass.
- [ ] **`workflow-service`: `ListTemplates`/`ResolveTemplate`/`CancelExecution`**
      are in the design doc but not the generated proto — add once template
      inheritance (company/team/personal scope chain) is actually being
      built, not before.

### Epic D — JWT/JWKS chain (auth-service ↔ api-gateway)

Two ends of the same gap, should land together:

- [ ] `auth-service`: implement `IssueServiceToken` for real — Vault
      Transit-backed signing (per `credential-broker-service`'s Transit
      adapter as the reference pattern), publish a JWKS endpoint.
- [ ] `api-gateway`: replace `AuthValidator`'s unverified-claims parsing
      (`internal/usecase/validate_identity.go`) with a real JWKS client
      that verifies the signature against `auth-service`'s published keys.
      **Do not deploy `api-gateway` publicly before this lands** — every
      README involved flags this loudly, repeating it here because it's
      the one gap with direct security exposure to the public internet.

### Epic E — OPA policy bundle

No service calls OPA yet. Every service that needs authorization beyond
"is this the right tenant" currently has either a placeholder inline check
(`auth-service`'s `requireAdminActor`) or nothing (`task-service`'s BFS
grant resolution computes an effective permission level but nothing feeds
it into a policy decision yet).

- [ ] Stand up an OPA instance (sidecar or embedded via the Go SDK, per
      [`architecture/07-security-architecture.md`](../../specs/backend-go/architecture/07-security-architecture.md)).
- [ ] Write the `orca-authz` Rego bundle's first policies: admin-action
      checks (replacing `auth-service`'s placeholder), task-grant final
      decision (consuming `task-service`'s BFS output as OPA input, per
      that service's design doc's explicit domain-computes/OPA-decides
      split), annotation author-only edit/delete (`annotation-service`'s
      README flags this as unenforced today).
- [ ] Add `opa test` to CI once the bundle exists.

### Epic F — Horizontal-scaling blockers

Two services currently only work correctly at 1 replica:

- [ ] `tenant-service`'s `GetResolvedProfile` cache is in-process LRU+TTL —
      fine at 1 replica, silently stale/inconsistent at >1. Decide: shared
      cache (Redis) vs. accept eventual consistency vs. no cache (measure
      first).
- [ ] `notification-service`'s `Broadcaster` is in-process channel fan-out —
      a `StreamNotifications` subscriber on replica A never sees an event
      broadcast on replica B. Fix via either sticky routing at
      `api-gateway` or republishing every broadcast to a shared NATS
      subject every replica also consumes (the service's own README
      recommends evaluating both).

### Epic G — Transactional outbox (once event volume justifies it)

Every event-publishing service in this scaffold (`usage-service`,
`issue-tracking-service`'s `LinkIssue`) publishes directly after a DB write
commits, not via the outbox pattern
[`architecture/05-data-architecture.md`](../../specs/backend-go/architecture/05-data-architecture.md)
specifies. Each service's README flags this as an accepted scaffold
simplification. Build a shared `common/outbox` helper (poll-based relay,
per that doc) once a second real consumer exists beyond
`notification-service` and a missed-publish actually has a cost worth the
added complexity — don't build it speculatively.

---

## 3. Per-service tasks, by migration phase

### Phase 1 — Leaf services (lowest risk, per migration strategy doc)

| Service | Remaining tasks |
|---|---|
| `usage-service` | Wire `common/secrets.DatabaseCredentialsFromFile` into `main.go` (currently reads `DATABASE_DSN` directly — fine for dev, not for a Vault-managed deployment). Add OTLP exporter config once a collector endpoint exists. Migrate hand-written SQL to `sqlc`. |
| `annotation-service` | Add `request_id` column + idempotent write (currently the only service missing this despite the proto carrying the field). Wire OPA author-only check (Epic E). |
| `notification-service` | Epic B (route VAPID through the broker or explicitly decide not to), Epic F (broadcaster), add a `processed_events` dedup table (currently not idempotent against NATS redelivery — a real correctness gap, not just a nice-to-have). |
| `issue-tracking-service` | Epic B (credential broker). Jira `CreateIssue` hardcodes issue type `"Task"` — add a real `ListIssueTypes` lookup. |

### Phase 2 — Mid-tier domain services

| Service | Remaining tasks |
|---|---|
| `ai-provider-service` + `credential-broker-service` | Epic B (already covers the client wiring). `credential-broker-service` itself: implement a real Vault delete/revoke path for `RevokeCredential` (currently an empty-KV-write approximation — add a `Ping`/`Delete` method to `common/secrets.Client` to support it properly), make the metadata+audit write genuinely atomic (one SQL transaction, not two calls), replace the `x-orca-service-id` metadata-header service-identity check with real mTLS/SPIFFE identity once the service mesh exists. |
| `automation-service` | Add the scheduler/ticker loop (`RunNow` is currently only callable on-demand — no automatic dispatch on the `rrule` schedule yet, which is the actual point of this service). Add `HandleExternalTrigger`. Move `step_type` from a JSON-blob field to a first-class proto field. |
| `workflow-service` | Epic A (Agent/Shell/Notification executors). Build real DAG wave-dispatch (currently only validates + records — an execution never progresses past `running`). Epic C's proto extension for template listing. |
| `infra-fleet-service` | Epic A (the whole point of this service). Real SSH credential issuance via Vault's SSH secrets engine (currently only DB-credential Vault usage is wired). |
| `project-service` | Epic C's `HasActiveExecutions` addition, then delete the stub checkers. Add `UpdateProject`/`DeleteProject`/repo/worktree/project-group RPCs — current proto only has the 5-RPC subset this scaffold implemented against. |

### Phase 3 — Gateway-facing services

| Service | Remaining tasks |
|---|---|
| `git-gateway-service` | Epic A (relay path). Wire `GenerateCommitMessage` once there's a real AI-inference call to make (currently `codes.Unimplemented`). Wire `ConnectionResolver` to the now-real `infra-fleet-service`. |
| `scm-integration-service` | Implement GitLab/Bitbucket/Azure DevOps/Gitea for real (only GitHub `ListIssues` is a genuine HTTP call today) — do GitHub's remaining 3 RPCs first, it's the reference implementation. Epic B. Decide and build the OAuth web-flow vs. PTY-CLI-login question from the design doc §9.1 — this is the actual fix for TS Gap 1, don't skip it once the HTTP clients exist. Add the `rate_limit_cache`/`webhook_delivery_log` tables. |

### Phase 4 — Identity/tenancy (do last — everything above already depends on stubs pointing here)

| Service | Remaining tasks |
|---|---|
| `tenant-service` | Epic F (cache). Wire `CreateTeamRequest` to actually accept team-layer settings (currently no RPC sets `Team.Settings`, even though the 4-layer merge algorithm already supports it). |
| `auth-service` | Epic D (JWT/JWKS). Epic E (OPA, replacing `requireAdminActor`). Decide SSO for real — this is explicitly a product decision per the design doc, not a mechanical port; don't build an OIDC flow speculatively before that decision is made. Decide whether the missing refresh-token/access-policy-CRUD/first-run-setup surface (Epic C) is actually needed before extending the proto. |

### Phase 5 — Edge

| Service | Remaining tasks |
|---|---|
| `api-gateway` | Epic D (real JWKS verification — blocking for any public deployment). Wire the remaining 14 services' REST routes as each backend matures past its own Phase — don't front-load all 14 proxies before the services behind them are real, that just moves the stub without adding value. |

---

## 4. Definition of done

A service is genuinely done — not just "scaffolded" — when it clears every
item in
[`specs/backend-go/standards/production-readiness-checklist.md`](../../specs/backend-go/standards/production-readiness-checklist.md).
That checklist's bar (own database ✅ already true for all 17, migrations
✅, Vault-sourced secrets ⬜ still env-var-based in every service's `main.go`,
mTLS ⬜, OPA coverage ⬜, load-tested ⬜, chaos-tested ⬜, runbook ⬜, …) is
the actual finish line — the tasks in §2/§3 above get a service to
"feature-complete," not to "production-ready." Don't conflate the two when
planning a release.

## 5. Infrastructure/process work not tied to any one service

- [ ] CI pipeline (`golangci-lint`, `govulncheck`, `buf lint`/`breaking`,
      per-module `go test`, Trivy image scan) — none of this is wired yet,
      only the Makefile targets that a real CI would call.
- [ ] Helm charts (one per service + the umbrella chart), per
      [`architecture/10-deployment-infrastructure.md`](../../specs/backend-go/architecture/10-deployment-infrastructure.md)
      — nothing beyond each service's standalone `Dockerfile` exists yet.
- [ ] A real `staging` environment with real Postgres instances, a real
      Vault cluster (HA, auto-unseal), and real NATS — everything in this
      repository has only been run against `docker-compose.yml`'s
      single-instance dev stack.
- [ ] `sqlc` migration for every service's hand-written `pgx` repository
      layer (flagged as a deferred gap in all 17 service READMEs
      identically) — worth doing as one focused pass across all services
      once the query set in each has stabilized, not per-service ad hoc.
