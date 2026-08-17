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

- **`internal/adapter/grpcclient.ConnectionResolver` is a stub.** It always
  answers `Connected=false`, so every request routes to the local git
  executor. A real implementation needs a gRPC client dialing
  infra-fleet-service's `ResolveConnection` RPC (design doc §7). It also
  currently uses the client-supplied `worktree_id` verbatim as the local
  filesystem path — safe only for local/dev use of this scaffold; the
  design doc's §3 explicitly requires resolving `worktree_id` via
  `project-service` instead of trusting a client-supplied path. Neither
  `project-service` nor `infra-fleet-service` exist yet in this workspace
  (both are empty scaffold directories), so there was nothing real to wire
  against — this mirrors the cross-service-stub pattern used elsewhere in
  this repo for dependencies that don't exist yet.
- **`internal/adapter/grpcclient.RelayExecutor` is a stub.** Every method
  returns `ErrRelayNotImplemented` rather than silently no-op'ing or
  falling back to local execution — per §8's explicit requirement that a
  relay failure return a typed error, not a silent fallback to the wrong
  worktree. Because `ConnectionResolver` always reports `Connected=false`
  today, `RelayExecutor` is currently unreachable via the gRPC surface, but
  the routing logic that would dispatch to it is implemented and tested
  (`dispatch_test.go`'s `*_Connected_RoutesToRelayExecutor` /
  `*_RoutesByConnectionState` tests construct `ConnectionResolver` fakes
  directly to exercise that branch).
- **`GenerateCommitMessage` always returns `codes.Unimplemented`.** Per
  design doc §3.1, this RPC is meant to relay diff/status context to the
  Dev Server Agent's `ai.complete` — this service must never call an LLM
  API directly. That relay isn't implemented in this scaffold; the usecase
  returns a clear sentinel error (`usecase.ErrGenerateCommitMessageNotImplemented`)
  that the gRPC adapter maps to `Unimplemented`, rather than returning an
  empty message that looks like a successful (but useless) response.
- **No connection-resolution caching.** §8 recommends caching the resolved
  `(worktree_id) -> (repo_path, connectionId)` tuple with a short in-process
  TTL to keep `GetStatus`/`GetDiff` inside their latency budget. Not added
  here — the current `ConnectionResolver` is a stub with no real round-trip
  cost to amortize; add the cache alongside the real client.
- **No audit trail** for mutating operations (`Commit`, `Push`, `Pull`).
  §5 explicitly recommends starting without one and adding it only if an
  operational need materializes — not added here, per that guidance.
