# SOL-SSH-04: Periodic scan loop, local-port allocation, SSH tunnel establishment, and push notification for auto port-forwarding

**Resolves:** [BUG-SSH-04](../BUG-SSH-04-port-forwarding-partial.md)
**Service:** `infra-fleet-service` (+ `api-gateway` for the push path)
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/domain/port_forward.go` (new — `PortForward` entity, per §4)
- `backend-go/services/infra-fleet-service/migrations/000X_port_forwards_status_process.up.sql`
- `backend-go/services/infra-fleet-service/internal/usecase/scan_workspace_ports.go` (fix method-name bug + attribution)
- `backend-go/services/infra-fleet-service/internal/usecase/poll_workspace_ports.go` (new — the 2s loop)
- `backend-go/services/infra-fleet-service/internal/usecase/create_port_forward.go` / `delete_port_forward.go` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/sshconn/tunnel.go` (new — `Listen`/forward primitive)
- `backend-go/services/infra-fleet-service/internal/adapter/portalloc/allocator.go` (new — local-port picker)
- `backend-go/services/infra-fleet-service/internal/adapter/eventbus/` (publish `dev_server.port_opened`/`port_closed`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`CreatePortForward`/`ListPortForwards`/`DeletePortForward`, per §3's existing sketch; extend `ScanWorkspacePortsResponse`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go` (subscribe to the new event, push to WS)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

**A real bug independent of the missing-forwarding gap, found while
grounding this solution**: `scan_workspace_ports.go:50` and
`kill_workspace_port.go:49` relay to agent methods named `"ports.scan"` and
`"ports.kill"`. The Dev Server Agent's actual registered RPC is
`"ports.detect"` (`agent/src/relay/port-scan-handler.ts:20`,
`dispatcher.onRequest('ports.detect', ...)`) — there is no `ports.scan` or
`ports.kill` handler anywhere in `agent/src/relay/`. This is exactly the
failure mode `infra-fleet-service.md` §10's migration notes warn about by
name: *"the agent process runs two independently-implemented RPC surfaces
... that frequently diverge in method names ... porting a call site without
checking which Part actually implements the target method ... is a known
source of TS-side bugs"* (`infra-fleet-service.md:560-573`). This solution
fixes the method name (`ports.scan` → `ports.detect`) as a prerequisite —
without it, every relayed scan in this bug's design fails at the transport
level regardless of what's built around it. `ports.kill` has no agent-side
counterpart at all today; see "Needs `agent/` changes" below.

**Where the periodic-scan/tunnel/notify machinery belongs**: §1 already
states `infra-fleet-service` owns "Port-forward CRUD and process lifecycle"
(`infra-fleet-service.md:22`) and §4 already specifies the `PortForward`
domain entity ("local/remote port pair, direction, owning `Connection`,
process/tunnel handle, status" — `infra-fleet-service.md:154-155`) and §5
already has the `port_forwards` table (`infra-fleet-service.md:233-242`).
§3's RPC sketch already lists `CreatePortForward`/`ListPortForwards`/
`DeletePortForward` (`infra-fleet-service.md:113-115`). **None of this is a
new design decision** — the periodic-scan-and-tunnel feature this solution
builds is filling in an already-fully-specified piece of this service that
the current scaffold (§10: "not implemented in this scaffold") just hasn't
been built yet, not something requiring a new architectural call. The
"pull-based scan/kill relay pair" BUG-SSH-04 found is the *routing*
primitive (`ScanWorkspacePorts`/`KillWorkspacePort`, correctly scoped per
§10's "closes TS Gap 7" framing) that this solution's poller calls
underneath — it is not itself the port-forwarding feature and was never
meant to be (BUG-027/SOL-027 in `missing-v1` only ever scoped the wiring,
per BUG-SSH-04's own "See also" section).

## Design — per-worktree attribution needs no remote per-process filtering

BUG-SSH-04 flags "no per-worktree port namespacing (BR-SSH-19)" as missing.
Grounded against the domain model, this needs **less** new machinery than it
sounds: `domain.Connection` already carries `WorktreeID`
(`connection.go:20-30`) and a relay-ssh `DevServer` is provisioned
1:1 for one `Connection` at a time (one detached agent process — or, before
`SOL-SSH-03`, one foreground process — per SSH target, not a shared
multi-tenant host). `ports.detect`'s OS-wide result on that host **is
already this worktree's own view** — there is no other worktree's process
sharing the same remote host to filter out. This solution therefore does
**not** need remote per-process cwd/command attribution (the richer
`WorkspacePortOwner{confidence: cwd|command|none}` heuristic the old TS
system's *local* `backend/src/main/ports/workspace-port-ownership.ts` used
is a different problem — disambiguating many worktrees sharing the *user's
own desktop*, which doesn't apply to a dedicated remote dev server). The
"namespacing" BR-SSH-19 actually asks for reduces to: **the local port
allocator must key by `connectionId`** (below), so two different worktrees'
remote port 3000s don't collide over the same local port — not "attribute
this specific process to this specific worktree on a shared host."

## Design — domain model

```go
// internal/domain/port_forward.go (new)
type PortForwardStatus string
const (
    PortForwardStatusActive PortForwardStatus = "active"
    PortForwardStatusClosed PortForwardStatus = "closed"
)

// PortForward is a live local:remote tunnel, per infra-fleet-service.md §4/§5.
type PortForward struct {
    ID           string
    TenantID     string
    ConnectionID string
    LocalPort    int
    RemotePort   int
    ProcessName  string // NEW beyond the §5 DDL's literal columns — carries
                         // ports.detect's processName through to the
                         // frontend's "Port 3001 → remote:3000 (node)" notification;
                         // additive, not a schema change to what's already committed
    Status       PortForwardStatus
}
```

Migration adds `process_name TEXT` to the existing `port_forwards` table
(§5's DDL already has everything else this needs).

## Design — local-port allocator (BR-SSH-16/17)

```go
// internal/adapter/portalloc/allocator.go (new)
var wellKnownExcluded = map[int]bool{22: true, 25: true, 53: true, 80: true, 443: true} // BR-SSH-16

// Allocator hands out a free local port in [3001, 9999] (BR-SSH-17),
// avoiding both wellKnownExcluded and any port this process already bound
// for a DIFFERENT (connectionId, remotePort) pair — namespacing (BR-SSH-19)
// lives here: two worktrees requesting remote:3000 each get their own
// local port because the allocator's in-use set is keyed globally, not
// per-connection, so a second request for the same remote port from a
// different connection can never collide on the same local port even
// though both remote ports are numerically identical.
type Allocator struct {
    mu     sync.Mutex
    inUse  map[int]string // local port -> portForwardID holding it
}

func (a *Allocator) Allocate(portForwardID string) (int, error) {
    // tries net.Listen("tcp", "127.0.0.1:0") is NOT used here — the OS-chosen
    // ephemeral port would violate BR-SSH-17's fixed [3001,9999] UX range
    // (users bookmark/expect stable-looking local ports); instead: probe
    // sequential/random candidates in range, net.Listen+immediately-Close
    // each candidate to confirm it's free at the OS level too (defense
    // against a non-Orca process already holding it), first success wins.
}
func (a *Allocator) Release(localPort int) { ... }
```

## Design — SSH tunnel primitive (no `agent/` change — plain SSH port-forward over the existing connection)

`sshconn.Connection` exposes only `RunCommand`/`NewSession`/`SFTPClient`
today (`connector.go:198-262`) — none of them a tunnel primitive. Adding one
needs no new agent-side capability: SSH port-forwarding is a standard
capability of the already-authenticated `*ssh.Client` `sshconn.Connection`
wraps (`ssh.Client.Dial("tcp", remoteAddr)` from the Go SSH library) — this
is the client dialing *through* the existing SSH connection, unrelated to
the JSON-RPC relay `SOL-SSH-03` bridges over the same connection's exec
channels.

```go
// internal/adapter/sshconn/tunnel.go (new)

// Forward opens a local TCP listener on 127.0.0.1:localPort and, for every
// accepted connection, dials remotePort on conn's target via the SSH
// connection's own direct-tcpip channel type, then copies bytes both
// directions until either side closes. Returns a handle whose Close() stops
// the listener and every in-flight forwarded connection — the "process/tunnel
// handle" §4's PortForward.domain comment already names.
func (conn *Connection) Forward(ctx context.Context, localPort, remotePort int) (*Tunnel, error) {
    listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
    if err != nil {
        return nil, fmt.Errorf("sshconn: binding local port %d: %w", localPort, err)
    }
    t := &Tunnel{listener: listener, done: make(chan struct{})}
    go t.acceptLoop(conn.client, remotePort)
    return t, nil
}

func (t *Tunnel) acceptLoop(client *ssh.Client, remotePort int) {
    for {
        local, err := t.listener.Accept()
        if err != nil {
            return // listener closed — Close() was called
        }
        go func() {
            remote, err := client.Dial("tcp", fmt.Sprintf("localhost:%d", remotePort))
            if err != nil {
                _ = local.Close()
                return
            }
            go func() { _, _ = io.Copy(remote, local); _ = remote.Close() }()
            _, _ = io.Copy(local, remote)
            _ = local.Close()
        }()
    }
}
```

## Design — periodic scan/diff/tunnel/notify loop

```go
// internal/usecase/poll_workspace_ports.go (new)

// PollWorkspacePorts runs one 2s-interval loop PER established relay-ssh
// Connection (matches BL-SSH-04's "scan every 2 seconds") — started by
// EstablishConnection on success (mirrors devserveragent's own
// per-connection lifecycle: one goroutine per live connection, torn down
// on TeardownConnection/SOL-SSH-03), not one global poller sweeping every
// tenant's every connection on a shared ticker (which would need its own
// leader-election machinery infra-fleet-service.md §8 reserves for fleet
// HEALTH polling specifically — port scanning is much higher-frequency and
// inherently per-connection, so per-connection goroutines are the right
// granularity, not a shared scheduler).
type PollWorkspacePorts struct {
    scan   *ScanWorkspacePorts // reused as-is — SOL-SSH-04 fixes its method
                                 // name bug (ports.scan -> ports.detect) but
                                 // keeps its resolve-then-relay shape
    alloc  *portalloc.Allocator
    tunnel TunnelOpener // port to *sshconn.Connection.Forward, narrowed the
                          // usual consumer-side way
    repo   PortForwardRepository
    events EventPublisher // dev_server.port_opened / port_closed, per infra-fleet-service.md §7
}

func (p *PollWorkspacePorts) Run(ctx context.Context, tenantID, connectionID, worktreeID string) {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    known := map[int]domain.PortForward{} // remotePort -> active forward
    for {
        select {
        case <-ctx.Done():
            for _, pf := range known { p.teardown(pf) } // BR-SSH-18: cleanup on connection close
            return
        case <-ticker.C:
            detected, err := p.scan.Execute(ctx, ScanWorkspacePortsInput{ConnectionID: connectionID, WorktreeID: worktreeID})
            if err != nil {
                continue // transient scan failure — try again next tick, don't tear down existing tunnels on one bad poll
            }
            seen := map[int]bool{}
            for _, d := range detected { // now DetectedPort-shaped {port, processName}, see decoder fix below
                seen[d.Port] = true
                if _, exists := known[d.Port]; !exists {
                    if pf, err := p.openForward(ctx, tenantID, connectionID, d); err == nil {
                        known[d.Port] = pf
                        p.events.Publish(ctx, "dev_server.port_opened", pf) // -> notification-service / wscompat push
                    }
                }
            }
            for remotePort, pf := range known {
                if !seen[remotePort] { // BR-SSH-18: remote port closed
                    p.teardown(pf)
                    delete(known, remotePort)
                    p.events.Publish(ctx, "dev_server.port_closed", pf)
                }
            }
        }
    }
}
```

**`ScanWorkspacePorts`/`decodeOpenPorts` fix**: the current decoder only
extracts an `openPorts []int32` field and discards everything else
(`scan_workspace_ports.go:69-86`), but `ports.detect`'s real response shape
is `{ports: DetectedPort[], platform}` with each entry carrying
`port/host/pid/processName` (`port-scan-handler.ts:7-12`). Replace
`decodeOpenPorts` with a decoder matching the real shape, and change
`ScanWorkspacePorts.Execute`'s return type from `[]int32` to
`[]DetectedPort{Port, ProcessName}` — a **breaking change to the usecase's
return type**, justified because the flat `int32` shape can never carry the
`{port, process}` pair BUG-SSH-04 and BL-SSH-04 both require; propagate the
richer shape through `ScanWorkspacePortsResponse` too (add `process_name` to
each entry, replacing the bare `repeated int32 open_ports`).

## Design — push notification path

`infra-fleet-service.md` §7 already establishes the pattern for
`dev_server.health_degraded`-style events via "NATS JetStream ... publishes
... that `notification-service` and others can subscribe to"
(`infra-fleet-service.md:389-390`). `dev_server.port_opened`/`port_closed`
follow the same transactional-outbox convention
(`05-data-architecture.md`'s "Transactional outbox + async events" pattern —
write the `port_forwards` row and an outbox row in the same transaction).
`api-gateway`'s existing WS-session-push machinery (§ "Manages WebSocket
sessions for real-time surfaces ... `notification-service` for push/WS
events" — `08-inter-service-communication.md:61-63`) subscribes and forwards
to the connected browser as a `workspacePorts.opened`/`workspacePorts.closed`
WS push, giving the frontend the "Port 3001 → remote:3000 [Open Browser]"
notification without polling.

## Needs `agent/` changes (small, one RPC) — `ports.kill`

`KillWorkspacePort` already relays to `"ports.kill"`
(`kill_workspace_port.go:49`), but **no such handler exists anywhere in
`agent/src/relay/`** — only `ports.detect` (scan) does. This is a real,
separate gap from the method-name bug above (that one has a right answer to
rename to; this one has no agent-side implementation at all, of any name).
Flag explicitly: closing "kill the process holding a workspace port" needs a
new `agent/src/relay/port-kill-handler.ts` registering `dispatcher.onRequest('ports.kill', ...)`
(`kill(pid, signal)` after validating the pid is actually one `ports.detect`
would report, mirroring `workspace-port-ownership.ts`'s existing
`killWorkspacePort`'s validation shape, ported to the agent side since the
agent — not backend-go — is what can see the remote process table). Out of
this solution's `backend-go`-only diff; tracked here so `KillWorkspacePort`'s
existing relay isn't mistaken for already-functional.

## Test plan

- `usecase/scan_workspace_ports_test.go` — relays to `"ports.detect"`, not
  `"ports.scan"` (regression guard against the method-name bug); decodes
  `{port, processName}` pairs, not a flat int array.
- `portalloc/allocator_test.go` — excludes 22/25/53/80/443; stays within
  [3001, 9999]; two `Allocate` calls for the same remote port from different
  `portForwardID`s get different local ports; `Release` frees a port for
  reuse.
- `sshconn/tunnel_test.go` — integration test against a local fake SSH
  server with an echo listener on a "remote" port: a client connecting to
  the tunnel's local port round-trips bytes through it; `Close()` stops
  accepting and terminates in-flight copies.
- `usecase/poll_workspace_ports_test.go` — fake scanner: a newly-seen port
  opens a tunnel + publishes `port_opened`; a port that stops appearing
  tears its tunnel down + publishes `port_closed` (BR-SSH-18); a transient
  scan error leaves existing tunnels untouched; `ctx` cancellation tears
  down every open tunnel (worktree-deleted case).
- `eventbus`/`wscompat` — a published `port_opened` event reaches a
  connected WS client as `workspacePorts.opened`, end-to-end through the
  outbox relay.

## References

- `specs/backend-go/bugs/logic-v1/BUG-SSH-04-port-forwarding-partial.md` — problem statement and the BR-SSH-15..19 checklist this solution addresses
- `specs/backend-go/tdd/services/infra-fleet-service.md:22` (§1, "Port-forward CRUD and process lifecycle" ownership), `:154-155` (§4 `PortForward` entity), `:233-242` (§5 `port_forwards` DDL), `:113-115` (§3 `CreatePortForward`/`ListPortForwards`/`DeletePortForward` sketch), `:560-573` (§10 "two independently-implemented RPC surfaces" warning — grounds the `ports.scan`→`ports.detect` fix), `:388-390` (§7 event-publishing precedent)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:82-98` (transactional outbox pattern this solution's event path follows)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:58-63` (`api-gateway`'s WS push pattern for real-time surfaces)
- `agent/src/relay/port-scan-handler.ts:1-20` — the real `ports.detect` RPC (name and response shape) backend-go's current code has wrong
- `backend-go/services/infra-fleet-service/internal/usecase/scan_workspace_ports.go:1-87`, `kill_workspace_port.go:1-73`
- `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go:198-262` (no forwarding primitive today)
- `docs/logic/remote-development/BL-SSH-04-port-forwarding.md`
