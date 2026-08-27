# SOL-FLEET-02: Fan-out bulk provisioning — `orca fleet provision`

**Resolves:** [BUG-FLEET-02](../BUG-FLEET-02-bulk-provisioning-not-implemented.md)
**Service:** `infra-fleet-service` (usecase/domain/proto/postgres/sshrelay extensions)
+ `api-gateway` (new REST route) + `fleetctl` (new subcommand, see
[SOL-FLEET-01](./SOL-FLEET-01-fleet-inventory-import.md))
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/domain/dev_server.go` (add `Status`, handshake fields — shared with SOL-FLEET-04)
- `backend-go/services/infra-fleet-service/migrations/0007_dev_server_status.up.sql` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/prereq.go` (new — Node/Git/disk checks)
- `backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend `DevServerRepository`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`BulkProvisionFleet` RPC, `DevServer.status`)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (wire new RPC)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go` (`/v1/infra/fleet/provision`)
- `backend-go/services/infra-fleet-service/cmd/fleetctl/main.go` (`provision` subcommand)
**Status:** 📋 Proposed — not yet implemented; **Step 5's daemon/HTTP-health model requires `agent/` engineering, see below**

---

## Design rationale (grounded in TDD)

`infra-fleet-service.md` §2's bounded-context table already assigns fleet
health/bootstrap coordination — including "SSH exec is a coordination-layer
act — establishing/monitoring the connection itself, not doing dev-work on
it" — to this service
(`specs/backend-go/tdd/services/infra-fleet-service.md:69`), and §3's proto
sketch already anticipates a *streaming* bootstrap RPC for exactly this
reason: `rpc BootstrapFleetTarget(...) returns (stream BootstrapProgress)`,
"streaming so callers can show live progress" (`infra-fleet-service.md:107-111`).
That RPC was never added to the real `infrafleet.proto` (confirmed: no
`Bootstrap*` RPC exists there today) — this solution is the first concrete
implementation of that design-doc intent, scoped as a unary
`BulkProvisionFleet` (see "Why unary, not streaming" below) rather than the
sketch's literal streaming shape.

**Single-server pipeline reused, not rebuilt.** `sshrelay.Provisioner.Provision`
(`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:82-115`)
already does resolve→dial→deploy→launch→handshake for one server — this
solution adds a prerequisite-check step in front of it and a fan-out
usecase around it, per `infra-fleet-service.md` §8's "Fleet health polling
cadence" section's general pattern of coordination logic living in the
usecase layer while the adapter stays protocol-focused.

**A hard boundary this solution runs into and must flag explicitly**: BL-FLEET-02's
"Start Relay" step (`~/.local/bin/orca-relay --daemon`, PID file) and
"Health Check" step (`curl http://localhost:<relayPort>/health`) assume a
**daemon binary with an HTTP listener** that does not exist as a buildable
artifact anywhere in `agent/` — confirmed by direct inspection of `agent/src`:
no `--daemon` CLI flag, no PID-file writer for the relay/agent process, no
`/health` HTTP route on `agent-connection-relay.ts` or any relay transport.
`agent/src/relay/relay.ts` (the standalone SSH-relay-deploy script the
*original* TS design targeted) parses `--grace-time`/`--connect`/`--detached`/
`--sock-path` flags and reconnects over a **Unix domain socket**, never TCP/
HTTP — and per `sshrelay`'s own package doc comment, backend-go's relay-ssh
mode doesn't even deploy `relay.js`: "relay-ssh's originally-spec'd deploy
target — a separate `relay.js` binary ... — has no buildable artifact
anywhere in this repo" (`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:6-9`).
Backend-go deploys `agent/out/agent.js --stdio` instead — the same Part A
dispatcher used by `direct-websocket`/`relay-websocket` mode, which has no
daemon/HTTP-health surface either (it's a WS server on `relay-websocket`,
not an HTTP server). **Building a real daemon+PID-file+HTTP-health model is
`agent/` engineering work, out of scope for a backend-go-only change** — see
"What this solution does instead" below.

---

## Design — domain / migration

```go
// internal/domain/dev_server.go (extended — shared with SOL-FLEET-04's
// platform/arch/nodeVersion persistence, since both need a place to write
// handshake-derived facts onto DevServer)
type DevServerStatus string

const (
    DevServerStatusPending   DevServerStatus = "pending"   // registered, never provisioned
    DevServerStatusHealthy   DevServerStatus = "healthy"
    DevServerStatusDegraded  DevServerStatus = "degraded"  // prerequisites marginal or health degraded
    DevServerStatusUnhealthy DevServerStatus = "unhealthy" // provisioning/deploy failed after retries
)

type DevServer struct {
    ID, TenantID, Host string
    Mode                ConnectionMode
    SSHTargetID         string
    Status              DevServerStatus // new
    Platform, Arch, NodeVersion, AgentVersion string // new — see SOL-FLEET-04
    LastProvisionedAt   *time.Time // new
}
```

`NewDevServer`'s existing invariants are unchanged; `Status` defaults to
`DevServerStatusPending` when unset (registration via `devServer.add`/
`RegisterDevServer` doesn't provision, so `pending` is the honest initial
value — closes part of BUG-FLEET-02's "no `degraded`/`unhealthy` status
field to persist into" gap, `dev_server.go:48-54` in the original report).

```sql
-- migrations/0007_dev_server_status.up.sql
ALTER TABLE infra.dev_servers
  ADD COLUMN status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','healthy','degraded','unhealthy')),
  ADD COLUMN platform TEXT, ADD COLUMN arch TEXT,
  ADD COLUMN node_version TEXT, ADD COLUMN agent_version TEXT,
  ADD COLUMN last_provisioned_at TIMESTAMPTZ;
```

```go
// internal/usecase/ports.go (extended)
type DevServerRepository interface {
    // ... existing methods unchanged ...
    // UpdateProvisionResult persists the outcome of one provisioning
    // attempt — status plus the handshake facts SOL-FLEET-04 needs
    // surfaced. Called once per server at the end of bulkProvisionOne,
    // success or failure.
    UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerStatus, info devserveragent.HandshakeInfo, provisionedAt time.Time) error
}
```

## Design — prerequisite checks (`sshrelay/prereq.go`, new)

Prerequisite checks run over the **raw SSH connection** (`sshconn.Connection`,
already used by `deploy.go`'s checksum step,
`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go:42,71`),
**before** the agent is deployed — there is nothing to relay a JSON-RPC call
to yet, so this cannot go through `devserveragent.Client.Exec`. This matches
BL-FLEET-02's own step ordering (SSH connect → check prerequisites → deploy
→ ..., `docs/logic/fleet/BL-FLEET-02-bulk-provisioning.md:19-27`).

```go
// internal/adapter/sshrelay/prereq.go
type PrereqResult struct {
    NodeVersion, GitVersion string
    NodeOK, GitOK, DiskOK   bool
    FreeDiskGB              float64
}

// checkPrerequisites runs node --version / git --version / df -P against
// conn and parses the results — same conn.RunCommand primitive deploy.go's
// checksum step already uses (sshconn.Connection.RunCommand,
// deploy.go:42,71), no new transport needed.
func checkPrerequisites(ctx context.Context, conn *sshconn.Connection, minNode, minGit semver.Version) (PrereqResult, error) {
    nodeOut, _, err := conn.RunCommand(ctx, "node --version")
    // ... parse "v22.3.0" -> semver, compare >= minNode ...
    gitOut, _, err := conn.RunCommand(ctx, "git --version")
    // ... parse "git version 2.39.2" -> semver, compare >= minGit (2.25 per
    // BL-FLEET-02 and docs/reference/git-compatibility.md's own 2.25
    // core-workflow baseline — same threshold, different concern: this
    // checks the TARGET host's git, not the git-gateway-service executor's) ...
    diskOut, _, err := conn.RunCommand(ctx, "df -P ~ | tail -1 | awk '{print $4}'") // KB free
    // ... parse, convert to GB, compare >= 5GB ...
}
```

A failed prerequisite does not abort the pipeline outright — per BL-FLEET-02's
"Prerequisites không đủ → log specific error, mark server `degraded`"
(`BL-FLEET-02-bulk-provisioning.md:45`), `bulkProvisionOne` (below) continues
to attempt deploy on a `degraded` prereq result but records the specific
shortfall in the per-server error list, letting an operator see *why* a
server is degraded rather than just that it is.

## Design — `usecase/bulk_provision_fleet.go` (new)

```go
type BulkProvisionFleetInput struct {
    Project     string // "" = all of tenant's relay-ssh dev servers
    Concurrency int    // default 5, per BL-FLEET-02
}

type ProvisionOutcome struct {
    DevServerID, Host, Status string
    Error                     string // "" on success
}

type BulkProvisionFleetResult struct {
    Success, Failed, Skipped int
    Outcomes                 []ProvisionOutcome
}

type BulkProvisionFleet struct {
    sshTargets SshTargetRepository
    devServers DevServerRepository
    provisioner Provisioner // sshrelay.Provisioner, narrowed to this usecase's needs
}

func (uc *BulkProvisionFleet) Execute(ctx context.Context, in BulkProvisionFleetInput) (BulkProvisionFleetResult, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
    }
    targets, err := uc.sshTargets.List(ctx, tenantID)
    if err != nil {
        return BulkProvisionFleetResult{}, apperrors.New(apperrors.KindInternal, "INFRA_FLEET_LIST_FAILED", "failed to list ssh targets", err)
    }
    if in.Project != "" {
        targets = filterByProject(targets, in.Project) // SOL-FLEET-01's Project field
    }
    concurrency := in.Concurrency
    if concurrency <= 0 {
        concurrency = 5
    }

    sem := make(chan struct{}, concurrency)
    var wg sync.WaitGroup
    outcomes := make([]ProvisionOutcome, len(targets))
    for i, target := range targets {
        wg.Add(1)
        sem <- struct{}{}
        go func(i int, target domain.SshTarget) {
            defer wg.Done()
            defer func() { <-sem }()
            outcomes[i] = uc.bulkProvisionOne(ctx, tenantID, target)
        }(i, target)
    }
    wg.Wait()

    var result BulkProvisionFleetResult
    for _, o := range outcomes {
        switch o.Status {
        case string(domain.DevServerStatusHealthy):
            result.Success++
        case string(domain.DevServerStatusDegraded):
            result.Skipped++ // "proceeded with warnings" bucket, per BL-FLEET-02/04's proceed-with-warnings posture
        default:
            result.Failed++
        }
    }
    result.Outcomes = outcomes
    return result, nil
}

// bulkProvisionOne is one server's find-or-create-DevServer + prereq-check +
// provision(retry x3) + status-write pipeline — log+continue on any error,
// per BL-FLEET-02's "SSH connect failure → log + continue" error-handling
// rule (BL-FLEET-02-bulk-provisioning.md:44). Idempotent re-runs (BL's
// "Partial provision → idempotent" rule) fall out naturally: a server
// already `healthy` is re-provisioned harmlessly (deploy.go's checksum
// verify is itself idempotent — a matching hash is a no-op re-upload).
func (uc *BulkProvisionFleet) bulkProvisionOne(ctx context.Context, tenantID string, target domain.SshTarget) ProvisionOutcome {
    devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, target.ID)
    if !found {
        devServer, _ = domain.NewDevServer(uuid.NewString(), tenantID, target.Host, domain.ConnectionModeRelaySSH, target.ID)
        devServer, err = uc.devServers.Register(ctx, devServer)
    }
    if err != nil {
        return ProvisionOutcome{Host: target.Host, Status: "unhealthy", Error: err.Error()}
    }

    var lastErr error
    for attempt := 1; attempt <= 3; attempt++ { // BL-FLEET-02's "retry 3 lần" on deploy failure
        var info devserveragent.HandshakeInfo
        _, info, lastErr = uc.provisioner.Provision(ctx, devServer)
        if lastErr == nil {
            status := domain.DevServerStatusHealthy
            _ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, status, info, time.Now())
            return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: string(status)}
        }
    }
    _ = uc.devServers.UpdateProvisionResult(ctx, tenantID, devServer.ID, domain.DevServerStatusUnhealthy, devserveragent.HandshakeInfo{}, time.Now())
    return ProvisionOutcome{DevServerID: devServer.ID, Host: target.Host, Status: "unhealthy", Error: lastErr.Error()}
}
```

Prerequisite checks run inside `Provisioner.Provision` itself (extended, not
duplicated in the usecase) — `Provisioner.Provision` gains a
`checkPrerequisites` call right after `p.connector.Connect` and before
`deploy`, returning a distinguishable `ErrPrerequisitesNotMet` the usecase
maps to `degraded` rather than `unhealthy` (a prereq shortfall is not the
same failure class as a deploy/handshake failure, and per BL-FLEET-02 does
not consume a retry attempt — retries are for transient deploy failures,
not for "this host will never have Node 22").

### What this solution does instead of a true daemon (flagged limitation)

`bulkProvisionOne`'s "Start Relay" + "Health Check" steps are satisfied by
the **existing real** launch+handshake step
(`sshrelay.launch`/`Provisioner.receiveHandshake`,
`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go:18-45`,
`provisioner.go:117-172`) — a live `agent.handshake` exchange is a genuine
liveness proof, just not an HTTP `/health` probe against an independent
daemon. This is **not** the spec's daemon model: the process is still tied
to the one SSH exec channel/session (`launch.go`'s own doc comment: "a
dropped SSH connection just ends the session"), so it doesn't survive this
service restarting or the SSH connection dropping the way a true
`--daemon`-launched, PID-file-tracked process would. Closing this gap for
real requires `agent/` to gain: (1) an HTTP listener on a negotiated
`relayPort` (mirroring `agent-connection-relay.ts`'s existing WS listener
pattern, `agent/src/relay/agent-connection-relay.ts:45`, but HTTP instead of
WS), and (2) a `--daemon`/detach mode with a PID file, analogous to what
`relay.js`'s CLI flags (`--detached`, `--sock-path`) and its `relay.status`
JSON-RPC response shape (`{pid, uptimeMs, detached, stdoutAlive, memory,
ptys, socket, grace}`, `specs/agent/api/agent-rpc-catalog-runtime.md:247`)
suggest were *intended* for `relay.js` specifically, not `agent.js`. **This
is explicit `agent/` engineering work this solution does not attempt** —
flagged per the task's instruction to call out agent-side dependencies
rather than silently working around them.

## Design — proto / wiring

```protobuf
message BulkProvisionFleetRequest {
  string project = 1; // "" = all
  int32 concurrency = 2; // 0 = default 5
}
message ProvisionOutcome { string dev_server_id = 1; string host = 2; string status = 3; string error = 4; }
message BulkProvisionFleetResponse {
  int32 success = 1; int32 failed = 2; int32 skipped = 3;
  repeated ProvisionOutcome outcomes = 4;
}

service InfraFleetService {
  // ...
  rpc BulkProvisionFleet(BulkProvisionFleetRequest) returns (BulkProvisionFleetResponse);
}
```

### Why unary, not the design doc's streaming sketch

`infra-fleet-service.md`'s `BootstrapFleetTarget` sketch is `stream
BootstrapProgress` for a *single* target's multi-step progress
(`infra-fleet-service.md:107-111`). Bulk provisioning is a different shape —
N servers in parallel, not one server's steps in sequence — and
BL-FLEET-02's own contract is a single terminal summary object
(`{success, failed, skipped}`, `BL-FLEET-02-bulk-provisioning.md:28`), not a
progress stream. A unary RPC keeps `fleetctl provision`'s HTTP call (via
`api-gateway`'s REST proxy, following `infra_routes.go`'s existing
request/response pattern, `infra_routes.go:42-65`) simple — no SSE/chunked
transfer needed. Live per-server progress for a future UI (BL-FLEET-04 Step 7's
checklist) is a real future need but out of this solution's scope; flagged
as a natural follow-up (`BulkProvisionFleet` could gain a server-streaming
sibling later without changing this unary RPC's contract).

`infra_routes.go` gains `sub.Post("/fleet/provision", handleBulkProvisionFleet(client))`,
following `handleImportFleetInventory`'s exact shape (SOL-FLEET-01). `fleetctl`
gains `fleetctl provision [--project X] [--concurrency 5]`, POSTing to that
route and printing the returned summary.

---

## Test plan

- `adapter/sshrelay/prereq_test.go` — fake `conn.RunCommand` outputs:
  `v22.3.0`/`git version 2.39.2`/sufficient disk → all-OK; `v18.0.0` →
  `NodeOK=false`; unparseable output → treated as not-OK, not a crash.
- `usecase/bulk_provision_fleet_test.go` — fake `SshTargetRepository`/
  `DevServerRepository`/`Provisioner`: 5 targets, concurrency=2 → asserts no
  more than 2 concurrent `Provisioner.Provision` calls in flight (channel-based
  fake tracks a high-water mark); one target's `Provisioner.Provision` always
  errors → after 3 attempts, `Failed++`, `UpdateProvisionResult` called with
  `unhealthy`; one target's prereq check fails but deploy succeeds →
  `Skipped++` with `degraded` status and no retry consumed; `--project`
  filter excludes non-matching targets (fake `SshTargetRepository.List`
  returns mixed projects).
- `adapter/postgres/dev_server_store_test.go` (integration) —
  `UpdateProvisionResult` persists `status`/`platform`/`node_version`/
  `last_provisioned_at`; a second call updates in place (idempotent), no
  duplicate row.
- `httpgateway/infra_routes_test.go` — `POST /v1/infra/fleet/provision`
  request/response marshaling against a fake gRPC client.
- Idempotency contract test: run `BulkProvisionFleet` twice against the same
  fixture set → second run's `Success` count matches the first (re-running
  a healthy fleet doesn't regress statuses), per BL-FLEET-02's explicit
  idempotent-re-run requirement.

## References

- `docs/logic/fleet/BL-FLEET-02-bulk-provisioning.md` — flow, bootstrap
  steps table, error-handling rules
- `specs/backend-go/bugs/logic-v1/BUG-FLEET-02-bulk-provisioning-not-implemented.md`
- `specs/backend-go/tdd/services/infra-fleet-service.md:69` (§2 bounded
  context — SSH exec as coordination-layer act), `:107-111` (§3
  `BootstrapFleetTarget` streaming sketch this solution's `BulkProvisionFleet`
  is grounded in but deliberately diverges from, see "Why unary" above)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:6-9,82-115,117-172`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/deploy.go:42,71`
  (`conn.RunCommand` precedent this solution's prereq checks reuse)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go:18-45`
  (foreground exec launch — the daemon-model gap)
- `specs/agent/api/agent-rpc-catalog-runtime.md:247` (`relay.status`'s
  daemon-shaped response, evidence the daemon model targeted `relay.js`
  specifically) — confirmed via direct `agent/src` inspection: no
  `--daemon` flag, no PID file, no `/health` HTTP route anywhere in
  `agent/src/relay/`
- `specs/backend-go/bugs/logic-v1/BUG-FLEET-01-fleet-inventory-not-implemented.md`
  and [SOL-FLEET-01](./SOL-FLEET-01-fleet-inventory-import.md) — server list
  + `--project` filter this solution's `Execute` depends on
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go:20-31,42-65`
