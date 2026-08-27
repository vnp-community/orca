# SOL-SSH-01: Extend `SshTarget` to the TDD's already-specified schema; recommend updating BL-SSH-01 to describe the Vault-cert auth model instead of `~/.ssh/config` parsing

**Resolves:** [BUG-SSH-01](../BUG-SSH-01-ket-noi-ssh-partial.md)
**Service:** `infra-fleet-service`
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go`
- `backend-go/services/infra-fleet-service/migrations/000X_ssh_targets_port_knownhosts_jumphost.up.sql` (+ `.down.sql`)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/ssh_target_repository.go` (or equivalent — wherever `SshTargetStore` lives)
- `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go`
- `backend-go/services/infra-fleet-service/internal/adapter/sshconn/pool.go` (new — concurrent-connection cap)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`SshTarget`, `CreateSshTargetRequest`/`Response`)
- `backend-go/services/infra-fleet-service/internal/usecase/create_ssh_target.go`
- `docs/logic/remote-development/BL-SSH-01-ket-noi-ssh.md` (recommended spec update — see below)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD): this is a spec-update case, not a "build config parsing" case

BUG-SSH-01 explicitly asks whoever picks it up to decide whether the fix is
"add `~/.ssh/config` parsing/password/2FA" or "the spec needs updating." The
TDD settles this decisively, on two independent grounds:

**1. The auth model is dictated architecture-wide, not a per-BL choice.**
`06-secrets-vault-architecture.md`'s "What goes in Vault vs. Postgres" table
states the rule for "SSH credentials for dev servers" in one line: *Vault SSH
secrets engine (signed short-lived SSH certificates) where the target
supports certificate auth; static per-target key material otherwise, stored
in KV v2* — specifically to "remove long-lived SSH private keys from
`infra-fleet-service`'s filesystem/database entirely" (`06-secrets-vault-architecture.md:26`).
There is no row for "user's own password" or "keyboard-interactive/2FA" in
that table, and `infra-fleet-service.md` §9 repeats the invariant as a hard
constraint: *"No long-lived SSH private keys on this service's filesystem or
database, ever"* (`infra-fleet-service.md:487-488`). A per-connection
password/2FA prompt would mean either (a) the browser sends a password to
`api-gateway` on every connect (a live secret in a request body, the exact
anti-pattern `06-secrets-vault-architecture.md`'s "What does NOT change at
the edge" section says only ever happens for the one-shot credential-write
flow, never for routine connection use), or (b) the server holds a
long-lived password/key to retry with — both violate the "no other service
talks to Vault directly for tenant secret material... but `infra-fleet-service`
IS one of the few permitted direct Vault callers for infrastructure-adjacent
secret material" carve-out (`infra-fleet-service.md:378-386`), which exists
*because* the intended model is Vault-mediated certs, not passthrough
credentials.

**2. `~/.ssh/config` is a per-desktop-user concept; this is now a multi-tenant server.**
BL-SSH-01 was written for the Electron desktop app, where "the user's own
`~/.ssh/config`" unambiguously means the file on the machine the user is
sitting at. `infra-fleet-service` is a horizontally-scaled server-side
Go service (`infra-fleet-service.md` §8: "this service is horizontally
scaled (multiple pods)") — there is no single meaningful `~/.ssh/config` to
parse; a server pod's home directory is not a per-user desktop. The
`CreateSshTarget`/`RegisterSshTarget` RPC shape the TDD already specifies
(`infra-fleet-service.md:94`, current proto's `CreateSshTargetRequest` at
`infrafleet.proto`) takes `host, user, vault_ssh_role` directly — an
operator/admin *registers* a target (out-of-band Vault role provisioning is
assumed), which is the correct shape for a multi-tenant fleet-management
system, not a "point Orca at any host you personally SSH into" tool. This is
a product-model change (admin-registered fleet targets, not
ad-hoc-per-user hosts), not an incidental implementation detail.

**Conclusion: recommend updating `BL-SSH-01-ket-noi-ssh.md`** to describe
the actual backend-go model — an admin/operator registers an `SshTarget`
(host, port, user, Vault SSH role or KV v2 static-key path), and every
connection attempt is a fresh Vault-cert-issuance-then-dial, with no
user-supplied key/password/2FA step at connect time. `~/.ssh/config`
parsing, password auth, and keyboard-interactive/2FA are **not** ported
forward — flag this explicitly to product/spec owners rather than silently
dropping the requirement.

### What *does* still need building — genuine gaps independent of the auth-model question

Not everything BUG-SSH-01 lists is contingent on the config-parsing
question. Several items are gaps against the TDD's **own** already-specified
data model (`infra-fleet-service.md` §5), which the current scaffold
explicitly documents as *not yet implemented* — `ssh_target.go:22-24`'s own
doc comment: *"Port, known-hosts fingerprint, and jump-host chaining from
the fuller design-doc entity are not modeled in this scaffold."* The TDD's
`ssh_targets` table (`infra-fleet-service.md:205-216`) already has `port`,
`known_hosts_fingerprint`, and `jump_host_target_id` columns — this is a
straightforward "build what the design doc already specifies" gap, not a new
design decision:

- **Port** (BUG-SSH-01's "every target dials port 22" finding) — add
  `SshTarget.Port`, default 22, migration adds the column.
- **`ProxyJump` chaining (A3)** — the TDD's schema already has
  `jump_host_target_id UUID REFERENCES ssh_targets(id)` (`infra-fleet-service.md:213`);
  `Connector.Connect` needs to walk the chain.
- **Known-hosts verification** — the TDD schema has
  `known_hosts_fingerprint TEXT` (`infra-fleet-service.md:212`); wiring it
  closes the currently-acknowledged `ssh.InsecureIgnoreHostKey()` gap
  (`connector.go:178`).
- **10s unreachable-host timeout with the spec's UX (A2)** — orthogonal to
  auth model; `DialTimeout` already defaults to 10s, only the error-message
  shape needs work.
- **Keepalive (BR-SSH-03)** — orthogonal to auth model; the raw
  `*ssh.Client` in `sshconn.Connection` has none today (`devserveragent`'s
  session-level keepalive doesn't apply here — this is the plain SSH
  transport `sshrelay` builds *on top of*).
- **Max-10-concurrent-connections-per-host (BR-SSH-04)** — orthogonal to
  auth model; no pool/counter exists in `sshconn` today.

---

## Design — domain model extension

```go
// internal/domain/ssh_target.go
type SshTarget struct {
    ID                    string
    TenantID              string
    Host                  string
    Port                  int    // NEW — defaults to 22 at construction
    UserName              string
    VaultSSHRole          string
    KnownHostsFingerprint string // NEW — SHA256 host key fingerprint, "" = unverified (still a
                                 // gap, but now representable/settable instead of impossible)
    JumpHostTargetID      string // NEW — "" = no jump host; FK to another SshTarget.ID
}

func NewSshTarget(id, tenantID, host string, port int, userName, vaultSSHRole, knownHostsFingerprint, jumpHostTargetID string) (SshTarget, error) {
    if port == 0 {
        port = 22 // mirrors sshconn.defaultSSHPort's existing default, now on the domain object
    }
    // ... existing tenantID/host/userName/vaultSSHRole checks unchanged ...
    return SshTarget{ID: id, TenantID: tenantID, Host: host, Port: port, UserName: userName,
        VaultSSHRole: vaultSSHRole, KnownHostsFingerprint: knownHostsFingerprint, JumpHostTargetID: jumpHostTargetID}, nil
}
```

Migration (`0003_ssh_targets_port_knownhosts_jumphost.up.sql`, additive per
`05-data-architecture.md`'s expand/contract policy):

```sql
ALTER TABLE ssh_targets ADD COLUMN port INTEGER NOT NULL DEFAULT 22;
ALTER TABLE ssh_targets ADD COLUMN known_hosts_fingerprint TEXT;
ALTER TABLE ssh_targets ADD COLUMN jump_host_target_id UUID REFERENCES ssh_targets(id);
```

`CreateSshTargetRequest`/`SshTarget` proto messages gain `port`,
`known_hosts_fingerprint`, `jump_host_target_id` fields (all optional,
defaulting the same way the domain constructor does).

## Design — `sshconn.Connector`: port, `ProxyJump`, known-hosts, timeout UX, keepalive

```go
// Connect walks target's jump-host chain (if any), then dials target itself
// through the last hop — mirrors ssh.Dial-through-a-bastion's standard Go
// pattern (ssh.Client.Dial("tcp", addr, ...) from an already-connected
// client), not a new protocol.
func (c *Connector) Connect(ctx context.Context, target domain.SshTarget) (*Connection, error) {
    hops, err := c.resolveJumpChain(ctx, target) // target itself is always the last hop
    if err != nil {
        return nil, err
    }
    var current *ssh.Client
    for i, hop := range hops {
        clientConfig, err := c.buildClientConfig(ctx, hop) // signs a fresh Vault cert per hop
        if err != nil {
            return nil, err
        }
        addr := net.JoinHostPort(hop.Host, strconv.Itoa(hop.Port))
        if current == nil {
            current, err = c.dialDirect(ctx, addr, clientConfig)
        } else {
            current, err = c.dialThroughHop(ctx, current, addr, clientConfig) // client.Dial + ssh.NewClientConn
        }
        if err != nil {
            return nil, &ErrUnreachableHost{Host: hop.Host, Port: hop.Port, HopIndex: i, Cause: err} // A2's exact-shaped error
        }
    }
    return &Connection{client: current}, nil
}
```

- **Timeout UX (A2)**: `ErrUnreachableHost.Error()` renders
  `"Connection refused: <host>:<port>"` (or `"... timed out"` for a
  `context.DeadlineExceeded` cause) — a typed error the `usecase` layer maps
  to `apperrors.KindFailedPrecondition` with a stable `INFRA_SSH_UNREACHABLE`
  code, replacing `establish_connection.go`'s current generic
  `INFRA_SSH_CONNECT_FAILED` for this specific cause.
- **Known-hosts (real host-key verification)**: `HostKeyCallback` becomes
  `ssh.FixedHostKey(...)` when `target.KnownHostsFingerprint != ""`, falling
  back to `ssh.InsecureIgnoreHostKey()` (still logged as a warning) only when
  unset — an explicit, visible degrade rather than a silent blanket
  bypass.
- **Keepalive (BR-SSH-03)**: `Connection` gains a `keepAlive(ctx)` loop
  sending an SSH `keepalive@openssh.com` global request every 30s (matches
  the spec's `ServerAliveInterval`), started by `sshrelay.Provisioner` right
  after `Connect` succeeds — the same "who starts it" placement as
  `devserveragent/session.go`'s `keepAliveLoop`, just one layer lower since
  this is the raw transport, not the JSON-RPC session on top of it. A missed
  keepalive write feeds directly into BUG-SSH-03's drop-detection (see
  `SOL-SSH-03`).

## Design — concurrent-connection cap (BR-SSH-04)

```go
// internal/adapter/sshconn/pool.go (new)
// Cap tracks in-flight connection attempts + live connections per
// (tenantID, target.Host) pair, rejecting the 11th with a typed error
// before ever dialing — a coordination-layer guard, not a TCP-level limit,
// consistent with infra-fleet-service.md §8's "cap concurrent
// TerminalSessions per connectionId" backpressure precedent (same
// in-process-counter shape, applied to raw SSH connections instead of PTY
// sessions).
type Cap struct {
    mu     sync.Mutex
    counts map[string]int // key: tenantID + "/" + host
}

func (c *Cap) Acquire(tenantID, host string) (release func(), err error) {
    // increments, error if already at 10, returns a release closure —
    // Connector.Connect calls Acquire before dialing, defers release() on
    // every return path (success and failure alike, since a failed dial
    // still occupied a slot briefly).
}
```

`NewConnector` takes an optional `*Cap` (nil = no cap, e.g. in tests) —
production wiring in `main.go` constructs one shared `Cap` and passes it to
`sshconn.NewConnector`.

## Test plan

- `domain/ssh_target_test.go` — `NewSshTarget` defaults `Port` to 22 when 0;
  rejects nothing new (jump-host/known-hosts remain optional).
- `sshconn/connector_test.go` — jump-chain dial test against two local fake
  SSH servers (bastion + target), asserting the final connection's traffic
  actually flows through the bastion; known-hosts mismatch rejects the dial;
  `ErrUnreachableHost` renders the A2-exact message shape.
- `sshconn/pool_test.go` — 11th concurrent `Acquire` for the same
  `(tenant, host)` fails immediately without dialing; `release()` frees the
  slot; two different hosts/tenants don't share a counter.
- `usecase/create_ssh_target_test.go` — accepts/persists the three new
  optional fields; omitting them still defaults exactly as before (no
  breaking change for existing callers).
- `postgres/ssh_target_repository_test.go` — migration round-trips
  (`golang-migrate` up/down/up per `05-data-architecture.md`'s CI rule);
  `jump_host_target_id` FK enforced.

## References

- `specs/backend-go/bugs/logic-v1/BUG-SSH-01-ket-noi-ssh-partial.md` — problem statement and the explicit "decide auth-model vs. wiring-gap" framing this solution answers
- `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md:17-29` ("SSH credentials for dev servers" row), `:31-45` (`credential-broker-service`'s mediation rule and why `infra-fleet-service` is a narrow exception), `:121-129` ("What does NOT change at the edge" — plaintext never leaves one request)
- `specs/backend-go/tdd/services/infra-fleet-service.md:142-148` (§4 `SshTarget` domain model — port, known-hosts, jump-host chain already specified), `:205-216` (§5 `ssh_targets` DDL — the schema this solution builds out), `:485-524` (§9 security notes, "no long-lived SSH private keys ... ever")
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:12-18` (design principle 1 — logical FKs / no invented cross-service coupling, applies to `jump_host_target_id`'s self-referential FK)
- `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go:19-31` — current scaffold, doc comment explicitly flagging port/known-hosts/jump-host as unmodeled
- `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go:1-26,46-52,107-196` — current single-hop, no-known-hosts, port-22-only `Connect`
- `docs/logic/remote-development/BL-SSH-01-ket-noi-ssh.md` — spec this solution recommends updating
