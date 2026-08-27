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
- `internal/adapter/devserveragent/` — **real for all three connection
  modes**: relay-websocket (outbound dial), direct-websocket (inbound
  accept), and relay-ssh (deploy+launch over SSH, via `WithRelaySSH`). A
  `Transport` interface generalizes the WS-specific and SSH-exec-specific
  transports behind one `session`/`Client` — `Exec`/`Health` have no
  relay-ssh-specific code left, see "Known gaps" below.
- `internal/adapter/agentwsserver/` — direct-websocket's inbound WS server
  (`/agent`) and token-issuance HTTP endpoint (`POST/GET /api/agent-token`),
  the Go port of `AgentWebSocketServer`/`agent-token-routes.ts` (Epic A,
  third pass, 2026-08-17).
- `internal/adapter/sshconn/` — a real, tested SSH connection layer
  (Vault-cert-authenticated, via `golang.org/x/crypto/ssh`), the transport
  `internal/adapter/sshrelay/` builds its deploy/launch steps on.
- `internal/adapter/sshrelay/` — relay-ssh's deploy+launch+handshake
  pipeline: SFTP-uploads `agent/out/agent.js` to the resolved `SshTarget`,
  launches it as `node agent.js --stdio` over the SSH exec channel, and
  completes the receiver-side `agent.handshake` exchange — see "Known gaps"
  for the full writeup, including `agent/`'s new third connection mode this
  depends on.
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

- **The Dev Server Agent relay protocol client is real for all three
  connection modes** (Epic A, four passes, 2026-08-17/18 — see
  [`docs/execution-plan.md`](../../docs/execution-plan.md) §18 for the
  relay-ssh writeup; §8/§11/§13 for the earlier three). `internal/adapter/devserveragent/`
  ports Stack B (`relay-protocol.ts`): 13-byte-framed JSON-RPC. All three
  modes now funnel through the same `session`/`Client.Exec`/`Client.Health` —
  a `Transport` interface (`ReadFrame`/`WriteFrame`/`Close`) abstracts *how*
  bytes move: relay-websocket dials out with `Authorization: Bearer
  <ORCA_AGENT_TOKEN>`; direct-websocket accepts an inbound connection via
  `internal/adapter/agentwsserver/` and hands it to
  `Client.AttachInboundSession`; relay-ssh deploys+launches over SSH via
  `internal/adapter/sshrelay/` and hands the result to
  `Client.WithRelaySSH(provisioner)`'s `getOrProvisionSession`. No
  mode-specific branch exists in `Exec`/`Health` — the generic passthrough
  contract ("method/params verbatim, no per-method translation") is uniform
  across all three. Unit- and fake-agent/fake-SSH-server-integration-tested
  throughout (relay-websocket: `frame_test.go`/`jsonrpc_test.go`/`client_test.go`/`session_test.go`;
  direct-websocket: `agentwsserver`'s own suite plus `devserveragent/inbound_test.go`;
  relay-ssh: `devserveragent/ssh_provisioner_test.go` (in-memory transport,
  dispatch/reuse logic) + `sshrelay`'s own suite (a real fake SSH+SFTP
  server, deploy→launch→handshake end to end, including a checksum-mismatch
  case)) but **not yet verified against a real deployed `agent/` binary or
  real SSH host** — no live infrastructure was available in this
  environment; flag this explicitly before relying on it in production.
  `Port`/`Token` are service-level `Config` (env vars: `AGENT_PORT`,
  `ORCA_AGENT_TOKEN`), not per-`DevServer` fields — matches how the real
  system models relay-websocket auth (a deployment-wide shared secret, not
  per-device); direct-websocket's own token model is per-connection and
  ephemeral instead (see `agentwsserver`'s bullet below); relay-ssh has no
  token at all — the SSH connection itself is the trust boundary. Reconnect
  after a relay-websocket drop retries in the background with
  `backoffDelay`-paced attempts; direct-websocket and relay-ssh sessions do
  NOT auto-reconnect this way — direct-websocket because there's nothing to
  dial (the agent must dial in again), relay-ssh because "reconnecting"
  means a fresh deploy+launch, not redialing the same transport, so the next
  `Exec`/`Health` call re-provisions from scratch instead.
- **`internal/adapter/agentwsserver/` (direct-websocket's inbound server)**
  is real: `/agent` WS handler running the receiver-side `agent.handshake`
  exchange, SHA-256-hashed single-use token slots with 60s connect-timeout
  expiry, and `POST/GET /api/agent-token` (fail-secure 401 if
  `ORCA_AGENT_API_SECRET` is unset — never a bypass). Shares this service's
  existing HTTP port, no new listener. Not yet verified against a real
  `agent/` binary, same caveat as above.
- **`relay-ssh` mode is fully real** (Epic A, fourth pass, 2026-08-18 —
  closes the gap the third pass left open). The blocker was never really
  "no build artifact" — it was that `relay-ssh`'s originally-spec'd deploy
  target (`relay.js`, a separately-built Unix-socket-daemon binary) has no
  buildable counterpart in this repo at all. The fix: `agent/` (this repo's
  actual, buildable Dev Server Agent, already used for
  direct-websocket/relay-websocket) grew a third connection mode, `stdio`
  (`agent/src/relay/agent-connection-stdio.ts`) — the exact same
  `agent/out/agent.js` bundle, launched as `node agent.js --stdio` with its
  stdin/stdout wired to an SSH exec channel instead of a WebSocket. No
  token needed — the SSH connection is the trust boundary, matching the
  design doc's relay-ssh auth model exactly.
  `internal/adapter/sshconn/` genuinely dials the resolved `SshTarget`,
  authenticating with a short-lived certificate issued via
  `common/secrets.Client.SSHSignPublicKey` against `ssh_targets.vault_ssh_role`
  (never raw key material, per the design doc §9 invariant). `DevServer`
  carries `SSHTargetID` (proto field 5 / `migrations/0003_dev_server_ssh_target`,
  nullable FK → `infra.ssh_targets`), required by `domain.NewDevServer`
  whenever `mode == relay-ssh` (`ErrMissingSSHTargetForRelaySSH`), resolved
  by `postgres.SshTargetStore.Get` (implements both
  `usecase.SshTargetRepository` and `sshrelay.SshTargetResolver` — a
  structurally-identical interface that package declares for itself, per
  this codebase's "port defined where consumed" convention).
  `internal/adapter/sshrelay/` (new) ties it together: SFTP-uploads
  `agent/out/agent.js` (new `github.com/pkg/sftp` dependency), SHA-256
  checksum-verifies via the same portable `node -e` one-liner the original
  TS reference uses, launches it over a fresh SSH exec session, and runs
  the receiver-side `agent.handshake` exchange — the resulting `Transport`
  and `HandshakeInfo` are what `Client.getOrProvisionSession` attaches,
  after which `Exec`/`Health` work identically to the other two modes: any
  method, not just a hardcoded subset. `cmd/server/main.go` wires it via a
  Vault client constructed for SSH cert issuance only — construction
  failing (malformed `VAULT_ADDR`) logs a warning and leaves relay-ssh
  unavailable rather than crash-looping the whole service over one optional
  mode's dependency; `ORCA_RELAY_BUNDLE_PATH` unset behaves the same way,
  checked lazily at deploy time.
  **Known, deliberate gaps carried forward, not silently fixed:** no
  process detachment/reattach across a dropped SSH connection (TS's
  Unix-socket-daemon model) — one exec channel per session, foreground; a
  dropped connection ends the session, the next call re-provisions from
  scratch. No multi-platform bundle resolution — `ORCA_RELAY_BUNDLE_PATH`
  is one local path, this scaffold runs one platform. `sshconn`'s two
  pre-existing gaps (`sshconn`'s package doc comment): no host-key
  verification (`ssh.InsecureIgnoreHostKey()`, matching the TS reference's
  own posture) and no per-target SSH port (defaults to 22, `SshTarget` has
  no port field). `GetFleetHealth`'s SSH-exec-based poll still does not use
  any of this. **No live SSH host, Vault SSH secrets engine, or deployed
  `agent.js --stdio` process was available to verify against** — every new
  path here is fake-server-tested, not live-verified.
- **`common/secrets` (Vault) is now wired into `main.go`, but only for
  relay-ssh's SSH cert issuance** — `DATABASE_DSN` is still read directly
  from the environment for local dev, same as usage-service's equivalent
  gap. Wire `secrets.DatabaseCredentialsFromFile` before this service is
  deployed anywhere Vault is actually running for real DB-credential
  management too.
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
