# SOL-024: Add `ssh.*` read/connect RPCs to `infra-fleet-service` — 3 already named in the TDD, 1 (`connect`) designed as a remote handshake act

**Resolves:** [BUG-024](../BUG-024-ssh-channels-not-implemented.md)
**Service:** `infra-fleet-service` (new RPCs) + `api-gateway` (new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `backend-go/services/infra-fleet-service/internal/usecase/list_ssh_targets.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/get_ssh_state.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/establish_connection.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend `SshTargetRepository`, `ConnectionRepository`)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/*.go`
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Status:** ✅ Implemented — all 5 task(s) (TASK-162–166) DONE; see each task file's own Status/Verify section for evidence.

---

## The TDD already names 3 of these 4 RPCs — the read surface is a gap-closing task, not a new design

`infra-fleet-service.md` §3's RPC sketch already lists the target contract
for everything `ssh.listTargets`/`ssh.getState`/`ssh.getUserAccount` need,
under two headings the actual `infrafleet.proto` hasn't caught up to yet:

> **SSH target registration** — `RegisterSshTarget`, `GetSshTarget`,
> `ListSshTargets`, `UpdateSshTarget`, `DeleteSshTarget`
> (`infra-fleet-service.md:94-98`)
>
> **Connection lifecycle** — `EstablishConnection`, `TeardownConnection`,
> `CheckConnectionHealth`, `ListConnections`
> (`infra-fleet-service.md:101-104`)

Today's `infrafleet.proto` only implements the *create* half of the first
group (`CreateSshTarget`, `infrafleet.proto:13,118-127` — the TDD's draft
name `RegisterSshTarget` and the shipped `CreateSshTarget` are the same
create-path RPC under two names, not two different RPCs; this solution
keeps the shipped name for consistency with what's already deployed) and
none of the second group — `ResolveConnection`/`CreateConnection`/`Relay`
exist (`infrafleet.proto:10-32`) but they're the coordination-layer
primitives, not the lifecycle CRUD the TDD's connection-lifecycle group
describes. This solution adds `ListSshTargets` (backs `ssh.listTargets` and
`ssh.getUserAccount`, see below) and a scoped-down `EstablishConnection`
(backs `ssh.connect`) plus a light local read for `ssh.getState` — it does
not add `UpdateSshTarget`/`DeleteSshTarget`/`TeardownConnection`/
`CheckConnectionHealth`/`GetSshTarget` since none of the 4 missing channels
need them; flagged as future work if the frontend ever calls
`ssh.updateTarget`/`ssh.removeTarget` (it currently doesn't, per
`rpc-catalog.md:420-427`'s 4-method list).

---

## Dispatch model, per BUG-024's own reading of `backend-agent-execution-boundary.md:112-113`

- `ssh.listTargets`, `ssh.getState`, `ssh.getUserAccount` — 🏠 always-local:
  Postgres reads against `infra-fleet-service`'s own `ssh_targets`/
  `connections` tables (`infra-fleet-service.md` §5), no Dev Server Agent
  hop.
- `ssh.connect` — 🔌 always-remote: it *is* the connection-establishment
  act, via the Dev Server Agent relay protocol (Option A,
  `08-inter-service-communication.md`'s "Talking to the Dev Server Agent").
  Designed below as a synchronous handshake usecase, not a Postgres write
  with a background dial — the same posture `EstablishConnection`'s TDD
  placement under "Connection lifecycle" (not "SSH target registration")
  implies: this call's job is to *make the connection exist*, not just
  record that one was requested.

---

## Design — proto additions

```protobuf
// infrafleet.proto — SSH target reads (infra-fleet-service.md:94-98)
rpc ListSshTargets(ListSshTargetsRequest) returns (ListSshTargetsResponse);

message SshTarget {
  string id = 1;
  string tenant_id = 2;
  string host = 3;
  string user = 4;
  string vault_ssh_role = 5; // a Vault role pointer, never key material — safe to return (infra-fleet-service.md §9)
}

message ListSshTargetsRequest {
  // tenant_id intentionally absent — pulled from context, same convention
  // as ListDevServersRequest (infrafleet.proto:75-78).
}

message ListSshTargetsResponse {
  repeated SshTarget ssh_targets = 1;
}

// infrafleet.proto — connection lifecycle (infra-fleet-service.md:101-104),
// scoped to just the one RPC ssh.connect needs. Establishing IS the SSH/
// agent handshake, not a write-then-poll — Connection.status comes back
// already resolved to "established" or the call errors.
rpc EstablishConnection(EstablishConnectionRequest) returns (Connection);

message EstablishConnectionRequest {
  string ssh_target_id = 1;
}

message Connection {
  string id = 1;               // the connectionId every other RPC keys on
  string dev_server_id = 2;
  string status = 3;            // "established" | "degraded" | "closed" — see domain note below
  int64 established_at_unix_ms = 4;
}
```

`buf breaking` passes cleanly — both additive.

---

## Design — `usecase/` layer

### `ListSshTargets` — backs both `ssh.listTargets` and `ssh.getUserAccount`

Per BUG-024's own note (and `rpc-catalog.md:426`), `ssh.getUserAccount` in
the old backend was never a distinct "Linux account provisioning" concept
— it just read the target's configured username. So `ssh.getUserAccount`
does not need its own RPC; `wscompat` can serve it off the same
`ListSshTargets` read (filtered client-side to the requested target), same
"derive a narrower view from a broader existing read" move BUG-024's
analysis already licenses.

```go
// internal/usecase/list_ssh_targets.go
type ListSshTargets struct {
    repo SshTargetRepository
}

func NewListSshTargets(repo SshTargetRepository) *ListSshTargets {
    return &ListSshTargets{repo: repo}
}

func (uc *ListSshTargets) Execute(ctx context.Context) ([]domain.SshTarget, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
    }
    return uc.repo.List(ctx, tenantID)
}
```

`SshTargetRepository` (`ports.go:29-35`) needs a new method:

```go
type SshTargetRepository interface {
    Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error)
    Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
    List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) // NEW
}
```

### `GetSshState` — 🏠 local read, no dial

```go
// internal/usecase/get_ssh_state.go
//
// 🏠 always-local, per backend-agent-execution-boundary.md:112-113 — reads
// whichever `connections` row (if any) currently binds this SSH target's
// dev server, never dials out. `ssh.connect` (EstablishConnection) is the
// only path that touches the network.
type GetSshState struct {
    sshTargets SshTargetRepository
    devServers DevServerRepository
    conns      ConnectionRepository
}

type SshStateInput struct {
    SshTargetID string
}

type SshState struct {
    Connected     bool
    ConnectionID  string
    LastActivity  *time.Time
}

func (uc *GetSshState) Execute(ctx context.Context, in SshStateInput) (SshState, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return SshState{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
    }
    // No live dev server bound to this SSH target yet -> never connected.
    devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, in.SshTargetID)
    if err != nil || !found {
        return SshState{Connected: false}, err
    }
    conn, found, err := uc.conns.GetActiveByDevServer(ctx, tenantID, devServer.ID)
    if err != nil || !found {
        return SshState{Connected: false}, err
    }
    return SshState{Connected: true, ConnectionID: conn.ID, LastActivity: conn.LastActivityAt}, nil
}
```

`ConnectionRepository` (`ports.go:39-42`) needs a read-side addition
(`GetActiveByDevServer`) alongside its existing write-only
`CreateConnection` — same "two narrow ports over one Repository" split
`ports.go`'s own comment already documents for
`ConnectionRepository`/`ConnectionResolver`.

### `EstablishConnection` — 🔌 the remote act itself

```go
// internal/usecase/establish_connection.go
//
// EstablishConnection performs the actual SSH + Dev Server Agent handshake
// synchronously — it is the connection-establishment act, not a record of
// one requested (backend-agent-execution-boundary.md:112-113's "IS the
// connection-establishment act itself" framing for ssh.connect). Uses
// adapter/devserveragent's relayssh package (SSH exec channel +
// SshChannelMultiplexer, infra-fleet-service.md §6) via the
// DevServerAgentClient port — same port ScanWorkspacePorts/Relay already
// depend on, no new adapter interface needed.
type EstablishConnection struct {
    sshTargets SshTargetRepository
    devServers DevServerRepository
    conns      ConnectionRepository
    agent      DevServerAgentClient
}

func NewEstablishConnection(sshTargets SshTargetRepository, devServers DevServerRepository, conns ConnectionRepository, agent DevServerAgentClient) *EstablishConnection {
    return &EstablishConnection{sshTargets: sshTargets, devServers: devServers, conns: conns, agent: agent}
}

type EstablishConnectionInput struct {
    SshTargetID string
}

func (uc *EstablishConnection) Execute(ctx context.Context, in EstablishConnectionInput) (domain.Connection, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return domain.Connection{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
    }
    target, err := uc.sshTargets.Get(ctx, tenantID, in.SshTargetID)
    if err != nil {
        return domain.Connection{}, err
    }
    // Find-or-create the DevServer row this SSH target backs — an SSH
    // target only becomes routable once it's the ssh_target_id of a
    // relay-ssh-mode DevServer (infrafleet.proto:42-48's comment on
    // DevServer.ssh_target_id).
    devServer, err := uc.devServers.FindOrCreateForSshTarget(ctx, tenantID, target)
    if err != nil {
        return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_RESOLVE_FAILED", "failed to resolve dev server for ssh target", err)
    }
    // The handshake itself — bootstrap/deploy is BootstrapFleetTarget's
    // job (streaming, separate RPC) if the relay binary isn't deployed
    // yet; Health() here confirms an already-bootstrapped target is
    // actually reachable before the Connection is marked established.
    // Per infra-fleet-service.md §8's deadline rule, this call carries an
    // explicit timeout longer than the intra-cluster 5s default — SSH
    // handshakes are not a sub-second operation.
    reachable, err := uc.agent.Health(ctx, devServer)
    if err != nil || !reachable {
        return domain.Connection{}, apperrors.New(apperrors.KindUnavailable, "INFRA_SSH_CONNECT_FAILED", "failed to establish SSH connection to target", err)
    }
    return uc.conns.CreateConnection(ctx, domain.Connection{
        DevServerID: devServer.ID,
        Status:      "established",
    })
}
```

`DevServerRepository` (`ports.go:20-26`) needs `FindOrCreateForSshTarget`
added alongside its existing `Register`/`Get`/`List`.

---

## Design — `wscompat` wiring

New `registerSshChannels`, mirroring `registerDevServerChannels`'s exact
shape (`channels.go:390-433`) — identity attach, per-RPC deadline:

```go
func registerSshChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
    r.Register("ssh.listTargets", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ListSshTargets(rpcCtx, &infrafleetv1.ListSshTargetsRequest{})
        if err != nil {
            return nil, err
        }
        return resp.GetSshTargets(), nil
    })

    r.Register("ssh.getUserAccount", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type getUserAccountArgs struct {
            SshTargetID string `json:"sshTargetId"`
        }
        in, err := decodeArg[getUserAccountArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ListSshTargets(rpcCtx, &infrafleetv1.ListSshTargetsRequest{})
        if err != nil {
            return nil, err
        }
        for _, t := range resp.GetSshTargets() {
            if t.GetId() == in.SshTargetID {
                return map[string]string{"username": t.GetUser()}, nil
            }
        }
        return nil, fmt.Errorf("ssh target %q not found", in.SshTargetID)
    })

    r.Register("ssh.getState", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type getStateArgs struct {
            SshTargetID string `json:"sshTargetId"`
        }
        in, err := decodeArg[getStateArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetSshState(rpcCtx, &infrafleetv1.GetSshStateRequest{SshTargetId: in.SshTargetID})
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    r.Register("ssh.connect", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type connectArgs struct {
            SshTargetID string `json:"sshTargetId"`
        }
        in, err := decodeArg[connectArgs](args, 0)
        if err != nil {
            return nil, err
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        // Longer-than-rpcTimeout deadline: this is an SSH handshake, not a
        // Postgres read — infra-fleet-service.md §8's "explicit timeout
        // distinct from the default 5s intra-cluster gRPC deadline" rule,
        // same reasoning as BootstrapFleetTarget's streaming RPC.
        rpcCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
        defer cancel()
        resp, err := client.EstablishConnection(rpcCtx, &infrafleetv1.EstablishConnectionRequest{SshTargetId: in.SshTargetID})
        if err != nil {
            return nil, err
        }
        return resp, nil
    })
}
```

Registered from `RegisterRealChannels` next to `registerDevServerChannels`
(`channels.go:70`): `registerSshChannels(r, infraFleetClient)` — no new
gRPC client dial needed, `infraFleetClient` is already threaded through.

---

## Test plan

- `services/infra-fleet-service/internal/usecase/list_ssh_targets_test.go`
  — fake `SshTargetRepository`, tenant-scoping assertion (a target from
  another tenant never appears).
- `get_ssh_state_test.go` — three cases: no `DevServer` bound (not
  connected), `DevServer` bound but no active `Connection` (not
  connected), active `Connection` present (connected + `connectionId`
  returned) — table-driven, fake repos, no real network.
- `establish_connection_test.go` — fake `DevServerAgentClient.Health`
  returning `true`/`false`/error, asserting `Connected: true` only on the
  `true` path and `INFRA_SSH_CONNECT_FAILED` on the other two; assert
  `FindOrCreateForSshTarget` is called with a `relay-ssh`-mode
  `DevServer`.
- `services/api-gateway/internal/adapter/wscompat/channels_test.go` — one
  test per new channel, fake `InfraFleetServiceClient`; `ssh.getUserAccount`
  specifically asserts it derives from `ListSshTargets`, not a second RPC.
- Contract test: `POST /v1/infra/ssh-targets` (existing REST create) then
  `ssh.listTargets` over WS returns the same target — round-trip guard
  against the REST and WS surfaces drifting.

## References

- `specs/backend-go/tdd/services/infra-fleet-service.md:94-104` — TDD's already-specified `ListSshTargets`/`EstablishConnection` target RPCs this solution implements
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:84-108` — "Talking to the Dev Server Agent," Option A wire protocol `ssh.connect` relies on
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:10-32,118-127` — current `InfraFleetService` RPC list (only `CreateSshTarget`)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:20-42,80-90` — `SshTargetRepository`/`ConnectionRepository`/`DevServerAgentClient` ports to extend
- `backend-go/services/infra-fleet-service/internal/usecase/create_ssh_target.go`, `scan_workspace_ports.go:17-62`, `relay.go` — existing usecase-layer patterns followed above
- `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go` — `SshTarget` domain type, current scaffold-scope invariants
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:390-433` — `registerDevServerChannels`, wiring pattern mirrored above
- `specs/frontend/api/rpc-catalog.md:420-427,519` — `ssh.*` 4-method table, `ssh.getUserAccount` old-backend semantics note
- `specs/frontend/api/backend-agent-execution-boundary.md:112-113` — 🏠/🔌 dispatch rows this design follows
- `specs/backend-go/bugs/missing-v1/BUG-024-ssh-channels-not-implemented.md` — full method inventory and dispatch-model analysis this solution builds on
