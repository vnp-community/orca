# infra-fleet-service

The system of record for "which host owns this `connectionId`, and how do I
reach it" — see
[`specs/backend-go/services/infra-fleet-service.md`](../../../specs/backend-go/services/infra-fleet-service.md)
for the full design and
[`usage-service`](../usage-service/) for the reference package layout this
service follows.

`ResolveConnection` is the core call: `git-gateway-service` invokes it on
every `git.*` dispatch to decide local-exec vs. relay, `project-service`
invokes it to validate a dev-server binding, and every other
`connectionId`-bound feature in the system (terminal sessions, filesystem
ops, port scans) reduces to the same call.

## What's implemented

- `internal/domain/` — `DevServer` (host + `ConnectionMode` enum:
  `relay-ssh` / `relay-websocket` / `direct-websocket` + `SSHTargetID`,
  required when `mode == relay-ssh`), `SshTarget` (host/user + a Vault
  SSH-role pointer, never raw key material), `DevServerHealth` (CPU/RAM/disk/
  latency snapshot) — invariant-enforcing constructors, pure unit tests.
- `internal/usecase/` — `RegisterDevServer`, `ResolveConnection` (the
  important one — tested for both its found and not-found branches),
  `CreateSshTarget`, `GetFleetHealth`, `ScanWorkspacePorts`, each tested
  against in-memory fakes for `DevServerRepository` / `SshTargetRepository`
  / `ConnectionResolver` / `FleetHealthPort` / `DevServerAgentClient`, no
  real Postgres or agent connection needed.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  all four usecase ports off one `Repository` type (mirrors usage-service's
  single-`Repository` shape). Hand-written SQL, same rationale as
  usage-service (`sqlc` codegen is the eventual target, not wired yet).
- `internal/adapter/devserveragent/` — **real for relay-websocket
  (outbound dial) and direct-websocket (inbound accept) modes**; `relay-ssh`
  still returns `ErrConnectionModeNotImplemented`. See "Known gaps" below.
- `internal/adapter/agentwsserver/` — direct-websocket's inbound WS server
  (`/agent`) and token-issuance HTTP endpoint (`POST/GET /api/agent-token`),
  the Go port of `AgentWebSocketServer`/`agent-token-routes.ts` (Epic A,
  third pass, 2026-08-17).
- `internal/adapter/sshconn/` — a real, tested SSH connection layer
  (Vault-cert-authenticated, via `golang.org/x/crypto/ssh`) for `relay-ssh`
  mode's eventual use — **not wired into anything yet**, see "Known gaps".
- `internal/adapter/grpc/` — implements the generated
  `infrafleetv1.InfraFleetServiceServer`, pure wire<->usecase translation.
- `migrations/0001_init.{up,down}.sql` — real DDL: `infra.dev_servers`,
  `infra.ssh_targets`, `infra.fleet_health`, RLS policies on the two
  tenant-scoped tables.
- `migrations/0002_connections.{up,down}.sql` — real DDL: `infra.connections`
  (the routing model `ResolveConnection`/`CreateConnection` use), plus
  schema-only `infra.port_forwards`/`infra.provider_registry_entries` (no
  consumer yet).
- `migrations/0003_dev_server_ssh_target.{up,down}.sql` — adds the nullable
  `infra.dev_servers.ssh_target_id` FK → `infra.ssh_targets(id)`.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, gRPC server with the shared interceptor chain,
  health/readiness HTTP server, graceful shutdown on SIGTERM. Wires the
  devserveragent client in cleanly, real for relay-websocket-mode dev
  servers (see below).

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/infra-fleet-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/infra-fleet-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/infra?sslmode=disable \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **The Dev Server Agent relay protocol client is real for relay-websocket
  and direct-websocket modes** (Epic A, third pass, 2026-08-17 — see
  [`docs/execution-plan.md`](../../docs/execution-plan.md) §13 for the full
  writeup; §8/§11 for the earlier two passes). `internal/adapter/devserveragent/`
  ports Stack B (`relay-protocol.ts`): 13-byte-framed JSON-RPC, shared by
  both modes — relay-websocket dials out with `Authorization: Bearer
  <ORCA_AGENT_TOKEN>`; direct-websocket accepts an inbound connection via
  `internal/adapter/agentwsserver/` (below) and hands it to
  `Client.AttachInboundSession`, after which both modes share identical
  read/keepalive/call machinery. `relay-ssh` is the one mode still returning
  `ErrConnectionModeNotImplemented` — see the next bullet for why, and what
  IS real underneath it. Unit- and fake-agent-integration-tested throughout
  (relay-websocket: `frame_test.go`/`jsonrpc_test.go`/`client_test.go`/`session_test.go`;
  direct-websocket: `agentwsserver`'s own suite plus `devserveragent/inbound_test.go`)
  but **not yet verified against a real `agent/` binary** — no live agent
  was available in this environment, unlike every other fix verified live
  earlier in this session; flag this explicitly before relying on it in
  production. `Port`/`Token` are service-level `Config` (env vars:
  `AGENT_PORT`, `ORCA_AGENT_TOKEN`), not per-`DevServer` fields — matches how
  the real system models relay-websocket auth (a deployment-wide shared
  secret, not per-device), see `config.go`'s doc comment; direct-websocket's
  own token model is per-connection and ephemeral instead (see
  `agentwsserver`'s bullet below). Reconnect after a relay-websocket drop
  retries in the background with `backoffDelay`-paced attempts; a dropped
  direct-websocket session does NOT auto-reconnect this way (there's nothing
  to dial — the agent must dial in again on its own, matching how the TS
  reference's own agent-side reconnect loop works for this mode). Because
  `relay-ssh` is still not implemented, `ScanWorkspacePorts` correctly
  *routes* (always relays when a `connectionId` resolves — closing TS Gap 7)
  but the relay call itself only succeeds for relay-websocket/direct-websocket
  dev servers today.
- **`internal/adapter/agentwsserver/` (direct-websocket's inbound server)**
  is real: `/agent` WS handler running the receiver-side `agent.handshake`
  exchange, SHA-256-hashed single-use token slots with 60s connect-timeout
  expiry, and `POST/GET /api/agent-token` (fail-secure 401 if
  `ORCA_AGENT_API_SECRET` is unset — never a bypass). Shares this service's
  existing HTTP port, no new listener. Not yet verified against a real
  `agent/` binary, same caveat as above.
- **SSH credential handling via Vault's SSH secrets engine — connection
  layer real, now wired into `devserveragent.Client` for a probe + one exec
  method (Epic A, fourth pass, 2026-08-17).** `internal/adapter/sshconn/`
  genuinely dials an SSH target, authenticating with a short-lived
  certificate issued via `common/secrets.Client.SSHSignPublicKey` against
  `ssh_targets.vault_ssh_role` (never raw key material, per the design doc
  §9 invariant). `DevServer` now carries `SSHTargetID` (proto field 5 /
  `migrations/0003_dev_server_ssh_target`, nullable FK →
  `infra.ssh_targets`), required by `domain.NewDevServer` whenever
  `mode == relay-ssh` (`ErrMissingSSHTargetForRelaySSH`), and
  `postgres.SshTargetStore.Get` resolves it into a full `domain.SshTarget`
  (implements both `usecase.SshTargetRepository` and the new
  `usecase.SshTargetResolver` — a separate Go type from `Repository`
  because Go doesn't allow two differently-typed `Get` methods on one
  receiver, see `repository.go`'s doc comment).
  `devserveragent.Client.WithRelaySSH(connector, resolver)` turns this on:
  **`Health`** dials the resolved target and runs a trivial command as a
  point-in-time liveness probe, reporting `(false, nil)` on any failure —
  closes the connection after (no session reuse, unlike the WS modes).
  **`Exec`** supports ONLY `method == "shell.exec"` — params are decoded
  into the same `{"script", "env"}` shape `infrafleetclient.shellExecParams`
  (workflow-service's real Relay caller) actually sends, env vars are
  exported as quoted POSIX `export` lines ahead of the script, and the
  result is shaped as `{"exitCode","stdout","stderr","error"}` — the same
  keys `infrafleetclient.execResult` decodes regardless of transport. Every
  OTHER relay-ssh method returns `ErrRelaySSHMethodNotSupported`, a clear
  typed error, never a silent success or empty result — there is still no
  JSON-RPC agent listening on a relay-ssh connection.
  Fake-SSH-server-tested (`devserveragent/relay_ssh_test.go`, reusing
  `sshconn/connector_test.go`'s fake-CA/fake-server philosophy): success,
  wrong-method-returns-typed-error, and connection-failure-returns-false
  cases all covered.
  **Still not implemented, exactly as before:** `relay-ssh` mode's actual
  point — SFTP-deploy + launch `relay.js`, then JSON-RPC over the exec
  channel — needs a `relay.js` build artifact with no path reachable from
  backend-go's build at all (it's `agent/`'s Electron-packaged output); see
  `sshconn`'s package doc comment. **Not wired into `cmd/server/main.go`
  either** — `WithRelaySSH` needs a `sshconn.SSHCertIssuer` (production:
  `common/secrets.Client`), and `common/secrets`/Vault has no config or
  client construction in `main.go` at all yet (see the next bullet — an
  already-tracked, separate gap). `GetFleetHealth`'s SSH-exec-based poll
  also does not use this. Two further gaps `sshconn` itself still carries,
  documented not silently matched: no host-key verification
  (`ssh.InsecureIgnoreHostKey()`, matching the TS reference's own posture)
  and no per-target SSH port (defaults to 22, `SshTarget` has no port
  field).
- **`common/secrets` (Vault) is not wired into `main.go` at all** —
  `DATABASE_DSN` is read directly from the environment for local dev, same
  as usage-service's equivalent gap. Wire
  `secrets.DatabaseCredentialsFromFile` before this service is deployed
  anywhere Vault is actually running.
- **`infra.connections` is now real (Epic A's second pass,
  `migrations/0002_connections`)** — `ResolveConnection` joins
  `connections`→`dev_servers` within the caller's tenant scope instead of
  equating `connectionId == dev_server_id`, and `CreateConnection` is the
  write path (dev_server_id + repo_path + worktree_id → connectionId). No
  `terminal_sessions` table yet, and `port_forwards`/`provider_registry_entries`
  got schema only in the same migration — no usecase or RPC reads/writes
  them, tracked as a follow-up once a real caller needs port-forward or
  provider-registry-audit behavior. The full design doc's connection
  lifecycle (`EstablishConnection`/`TeardownConnection`/`CheckConnectionHealth`)
  beyond create+resolve is still not implemented — only what
  `infrafleet.proto` exposes today: `RegisterDevServer`, `ResolveConnection`,
  `CreateSshTarget`, `GetFleetHealth`, `ScanWorkspacePorts`,
  `ListDevServers`, `CreateConnection`, `Relay`.
- **No fleet health polling job.** `infra.fleet_health` stores the latest
  sample per dev server and `GetFleetHealth` reads it, but nothing writes
  to it — the 30s-cadence poller described in the design doc §8
  (leader-election-per-target via Postgres advisory locks) isn't
  implemented.
- **No NATS eventbus wiring.** The design doc §7 calls for
  `connection.established` / `connection.lost` /
  `dev_server.health_degraded` events via NATS JetStream; this scaffold's
  `internal/config` and `cmd/server/main.go` have no NATS dependency at all
  (unlike usage-service, which does publish events) since none of the 5
  implemented usecases need to publish anything yet.
- **`ScanWorkspacePorts` never performs a local port scan.** Per the design
  doc §10, this closes TS Gap 7 for the *relay* path — a bound
  `connectionId` always relays to the agent, never silently short-circuits
  to an empty result (see `usecase.ScanWorkspacePorts`'s tests for the
  regression coverage). The *local* path (no `connectionId`, or one that
  doesn't resolve) intentionally returns an empty slice rather than actually
  scanning localhost's open ports — this service's contract is routing the
  scan, not executing it, and a real local-scan implementation was out of
  scope here.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in, same as
  usage-service.
