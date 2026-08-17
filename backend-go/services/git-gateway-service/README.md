# git-gateway-service

A stateless dispatcher for git operations — see
[`specs/backend-go/services/git-gateway-service.md`](../../../specs/backend-go/services/git-gateway-service.md)
for the full design. This service follows the package layout and
conventions demonstrated by
[`usage-service`](../usage-service/README.md), **with one deliberate
deviation**: **git-gateway-service owns no database.** No `DATABASE_DSN`,
no `migrations/` directory, no `internal/adapter/postgres/` package — see
the design doc's §5 ("Data model: None.") and §6 ("Package layout notes")
for why. Its `internal/domain/` and `internal/usecase/` layers are
correspondingly thin: value objects with no invariant-enforcing
constructors, and usecases whose only owned logic is "resolve host ->
dispatch -> translate" (§2), not business rules.

## What's implemented

- `internal/domain/` — `FileStatus`, `GitStatus`, `DiffResult`,
  `CommitResult`, `PushResult`, `PullResult`: plain value objects mirrored
  from the wire protocol, plus a `FileState` string-enum with a `Valid()`
  check (the one meaningfully pure validation this package has). No
  constructor enforces a git invariant — this service never constructs a
  commit, it only relays and reflects what `git` or the Dev Server Agent
  already produced.
- `internal/usecase/` — one usecase per implemented RPC (`GetStatus`,
  `GetDiff`, `Commit`, `Push`, `Pull`, `GenerateCommitMessage`), each doing
  exactly the resolve -> dispatch -> translate flow from the design doc's
  §2/§7. `ports.go` defines `ConnectionResolver` and `GitExecutor`;
  `dispatchExecutor` centralizes the "connected=false -> local,
  connected=true -> relay" routing decision so it's implemented and tested
  exactly once. Tested with fakes (`dispatch_test.go`) — no real
  infra-fleet-service or `git` binary needed for these tests, per
  `standards/testing-strategy.md`'s unit-test section.
- `internal/adapter/localgit/` — a **real**, working `GitExecutor`
  implementation, shelling out to the host's `git` binary via `os/exec` for
  `GetStatus` (`git status --porcelain=v1 -b`, parsed), `GetDiff` (`git
  diff`), `Commit` (`git add` + `git commit` + `git rev-parse HEAD`), `Push`,
  and `Pull`. Tested against a real temp git repository
  (`executor_test.go`) — no mocking of the `git` binary itself. Commands
  used are all available since Git 2.5, well under the Git 2.25 baseline in
  `docs/reference/git-compatibility.md`.
- `internal/adapter/grpc/` — implements
  `gitgatewayv1.UnimplementedGitGatewayServiceServer`; pure wire<->usecase
  translation, no business logic.
- `internal/config/` — no database config; adds
  `INFRA_FLEET_SERVICE_ADDR`/`PROJECT_SERVICE_ADDR` env vars for the real
  clients a follow-up should wire into `internal/adapter/grpcclient` (unused
  by the stubs currently there).
- `cmd/server/main.go` — composition root: config load, gRPC server with
  the shared interceptor chain, health server, graceful shutdown. No
  Postgres pool, no NATS connection, no eventbus wiring (§6: "git
  operations are synchronous request/response by nature — a `git.commit`
  isn't a fact other services react to asynchronously").

## Running locally

```sh
# from backend-go/
cd services/git-gateway-service
go run ./cmd/server
```

No external dependencies (database, NATS) need to be running — the stub
adapters described below mean this binary starts and serves `GetStatus`/
`GetDiff`/etc. against the local git binary with zero setup.

## Testing

```sh
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./...   # includes localgit's real-git-repo integration tests; no Docker/testcontainers needed
```

## Stubbed / not implemented — flagged explicitly, not silently skipped

This scaffold implements 6 of the ~26 RPCs sketched in the design doc's §3
(`GetStatus`, `GetDiff`, `Commit`, `Push`, `Pull`,
`GenerateCommitMessage` — the current generated proto's actual surface;
branch/history/conflict-resolution RPCs from the doc's fuller sketch aren't
in `gitgateway.proto` yet). Within that surface:

- **`internal/adapter/grpcclient.ConnectionResolver` and `.RelayExecutor` are
  now real** (Epic A's second pass, see
  [`docs/execution-plan.md`](../../docs/execution-plan.md) §2 Epic A).
  `ConnectionResolver` dials infra-fleet-service's `ResolveConnection` RPC
  (`INFRA_FLEET_SERVICE_ADDR`) and passes `worktree_id` through verbatim as
  infra-fleet-service's `connectionId`. `RelayExecutor` calls
  infra-fleet-service's generic `Relay` RPC with methods `git.status`/
  `git.diff`/`git.commit`/`git.push`/`git.pull`. Both attach tenant identity
  onto the outbound gRPC context explicitly (`withTenantMetadata` in
  `tenant_forwarding.go`) — no prior backend-to-backend call in this service
  did this, so without it every infra-fleet-service call would fail with
  `INFRA_NO_TENANT`. **Known gaps, not silently papered over:**
  - `RelayExecutor`'s `git.*` method param/result JSON field names are
    best-effort — matched 1:1 to this service's own `domain.GitStatus`/
    `DiffResult`/etc. field names, not verified against a real Dev Server
    Agent. `specs/agent/api/agent-rpc-catalog-git-fs.md` documents a
    *different* contract (`worktreePath`, `filePath`, `pushTarget`, etc.) for
    the SSH Relay Daemon's `git.*` handlers — reconcile before production use
    (see `relay_executor.go`'s doc comment).
  - `usecase.GitExecutor`'s methods only receive `repoPath`, not the
    `connectionId` `ConnectionResolver` actually resolved — `RelayExecutor`
    passes `repoPath` through as the relay's `connectionId` too, which is
    only correct because those two values happen to coincide today (see
    `relay_executor.go`'s `relay` doc comment for the exact reasoning).
    Threading `ConnectionID` through `GitExecutor`'s signature is the real
    fix; not done here since `ports.go`/`dispatchExecutor` were out of scope
    for this pass.
  - It also still uses the client-supplied `worktree_id` verbatim (now as
    infra-fleet-service's `connectionId` lookup key, not a raw filesystem
    path) rather than resolving it via `project-service` per the design
    doc's §3 — `project-service` doesn't expose that RPC yet.
  - **No live Dev Server Agent or infra-fleet-service deployment was
    available to verify this against** — unit- and fake-client-tested only
    (`grpcclient_test.go`), same honest caveat as infra-fleet-service's own
    relay-websocket pass.
- **`GenerateCommitMessage` is now real.** It resolves the worktree's
  connection (same `ConnectionResolver` as every other RPC), fetches the
  staged diff via the existing `GetDiff` usecase (no duplicated
  diff-fetching logic), and relays a prompt built from that diff to the Dev
  Server Agent's `ai.complete` through `RelayExecutor.Complete` — the same
  infra-fleet-service `Relay` RPC/tenant-forwarding path `git.*` uses, just
  with method `"ai.complete"`. That method name and its
  `prompt(required), format?, taskId?, model?, accountId?, resolvedApiKey?`
  → `{content, model?}` shape come from
  `specs/agent/api/agent-rpc-catalog-runtime.md`'s confirmed real handler
  (`ai-complete-handler.ts:47`), not a guess — this call only ever sends
  `prompt`; model/account resolution is deliberately out of scope here
  (`git-gateway-service.md` §3.1's "context assembler and relay point"
  framing), so the agent falls back to its own configured default model.
  If the worktree has no relay connection (`Connected=false`, i.e. no dev
  server), there is no host-local AI-inference fallback to degrade to —
  `GenerateCommitMessage` returns a `FailedPrecondition` error rather than
  a silently empty message. See `usecase/generate_commit_message.go` and
  `adapter/grpcclient/relay_executor.go`'s `Complete` method.
- **No connection-resolution caching.** §8 recommends caching the resolved
  `(worktree_id) -> (repo_path, connectionId)` tuple with a short in-process
  TTL to keep `GetStatus`/`GetDiff` inside their latency budget. Not added
  here — the current `ConnectionResolver` is a stub with no real round-trip
  cost to amortize; add the cache alongside the real client.
- **No audit trail** for mutating operations (`Commit`, `Push`, `Pull`).
  §5 explicitly recommends starting without one and adding it only if an
  operational need materializes — not added here, per that guidance.
