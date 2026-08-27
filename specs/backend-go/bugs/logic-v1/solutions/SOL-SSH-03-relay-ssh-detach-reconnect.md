# SOL-SSH-03: Detached-socket relay-ssh launch mode (agent/ change) + backend-go background reconnect/reattach for relay-ssh

**Resolves:** [BUG-SSH-03](../BUG-SSH-03-auto-reconnect-partial.md)
**Service:** `infra-fleet-service` **and** the Dev Server Agent (`agent/`) — this bug cannot be fully closed in `backend-go` alone; see "Needs `agent/` changes" below.
**Affected files (proposed):**
- `agent/src/relay/agent-connection-stdio.ts` (extend: `--detach --sock-path` mode)
- `agent/src/relay/agent-entry.ts` (CLI flag plumbing)
- `agent/src/relay/relay-detach-bridge.ts` (new — the tiny bridge process)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/reattach.go` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go` (relay-ssh background reconnect)
- `backend-go/services/infra-fleet-service/internal/domain/connection.go` (add `"reconnecting"` status)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (add `TeardownConnection`, per §3's existing sketch)
- `backend-go/services/infra-fleet-service/internal/usecase/teardown_connection.go` (new)
**Status:** 📋 Proposed — not yet implemented

---

## Needs `agent/` changes — explicit flag

Everything else in this solution set (SOL-SSH-01, -02, -04) is a
`backend-go`-only fix. **This one is not.** BR-SSH-10 ("agent process trên
remote PHẢI tiếp tục chạy khi local disconnect") is structurally impossible
to satisfy without an `agent/` change, for a reason specific to how
`agent-connection-stdio.ts` is built today: its own header comment states
plainly, *"There is no WebSocket dial or listen at all — the SSH connection
itself is both the transport and the trust boundary... no reconnect loop"*
(`agent-connection-stdio.ts:6-8`). In stdio mode, the agent's entire JSON-RPC
wire protocol travels over the SSH exec channel's own stdin/stdout pipes.
When the SSH connection drops, those pipes close — there is no way for a Go
service to "reconnect" to a process whose only communication channel just
died, no matter what retry loop backend-go builds. A backend-go-only fix can
retry *deploying a new instance*, but that is not agent continuity (BR-SSH-10
explicitly rules out "just restart it," which is what `Provisioner.Provision`
already does on the next call — `client.go:179-200`'s doc comment already
names this as the current, insufficient behavior).

This is exactly the gap `sshrelay`'s package doc comment already names as
the road not taken: the TS reference's originally-spec'd relay-ssh model was
"a separate `relay.js` binary, launched via `--detached --connect
--sock-path`" (`provisioner.go`'s sibling package doc comment, cited
verbatim in BUG-SSH-03's own "See also" section), abandoned only because "no
buildable artifact anywhere in this repo" existed for it at the time
`agent-connection-stdio.ts` was built. **Closing BR-SSH-10 for real means
building that detach mode now** — it is not a redesign, it is finishing the
mode the architecture already anticipated.

## Design — `agent/`: detached-socket stdio launch mode

Add a second launch mode to `agent-connection-stdio.ts`, selected by a new
`--detach --sock-path <path>` CLI pair (additive — plain `--stdio` keeps
working exactly as today for anything not opting in):

```ts
// agent/src/relay/agent-connection-stdio.ts (extended)
//
// --detach mode: instead of wiring the wire protocol to this process's own
// stdin/stdout (which die with the SSH exec channel), this process:
//   1. double-forks / setsid()s itself into a session leader detached from
//      the controlling SSH exec channel (so SIGHUP on channel close does
//      not propagate to it) — Node's child_process.spawn with
//      {detached: true, stdio: 'ignore'} re-execing itself achieves this
//      without a native addon.
//   2. opens a Unix domain socket listener at sockPath instead of using
//      stdin/stdout for the wire protocol — StdioWebSocketAdapter's
//      duck-typed interface (readyState/send/close/on) is satisfied by a
//      thin net.Socket wrapper instead of process.stdin/stdout, so
//      agent-session.ts (already explicitly "NOT modified" per its header)
//      needs zero changes — only the transport this file hands it changes.
//   3. writes a pidfile next to sockPath.
//   4. the ORIGINAL (parent) process, still attached to the SSH exec
//      channel, waits for the child to report "listening" over a pipe,
//      then exits 0 — letting that first SSH exec session close cleanly
//      without killing the now-detached child.
export async function runDetachedStdioMode(sockPath: string, config: AgentConfig, ...): Promise<void> { ... }

// --connect mode: a SEPARATE, much smaller entry point launch() now uses
// for every SUBSEQUENT (re)attach — connects to sockPath as a client and
// bridges it 1:1 onto ITS OWN stdin/stdout, i.e. this is the "socat over
// SSH exec" bridge, implemented in Node (already required on the remote
// host, so no new binary dependency) rather than assuming socat/nc exist.
export async function runConnectBridgeMode(sockPath: string): Promise<void> { ... }
```

`relay-detach-bridge.ts` is the `--connect` mode's implementation: opens
`net.createConnection(sockPath)`, pipes `process.stdin` into it and its data
back onto `process.stdout`, exits when either side closes — genuinely small
(bidirectional pipe), no new protocol, no changes to `agent-wire.ts` or
`agent-session.ts`'s framing.

**Liveness without a connection**: a fresh SSH session can check whether the
detached process is still alive with a plain `kill -0 $(cat pidfile)` before
attempting `--connect` — needed because a crashed detached process leaves a
stale socket file that would otherwise hang a connect attempt.

## Design — `backend-go`: launch, reattach, background reconnect

```go
// sshrelay/launch.go — extended
// launch now always uses --detach on FIRST provision, and returns the
// sockPath alongside the transport (obtained by bridging through --connect
// immediately after detaching) — every subsequent call is reattach(), not
// launch(), for the lifetime of the detached process.
func launch(conn *sshconn.Connection, remoteDir, devServerID string) (transport *sshExecTransport, sockPath string, err error) {
    sockPath = remoteDir + "/relay.sock"
    if _, _, err := conn.RunCommand(ctx, fmt.Sprintf(
        "cd %s && DEV_SERVER_ID=%s node %s --detach --sock-path %s",
        shellQuote(remoteDir), shellQuote(devServerID), shellQuote(remoteAgentFile), shellQuote(sockPath),
    )); err != nil { // blocks only until the parent reports "listening" and exits — see agent/ design above
        return nil, "", err
    }
    return reattach(conn, sockPath)
}

// reattach opens a FRESH SSH exec session running `node agent.js --connect
// --sock-path <path>` — the cheap path every reconnect after the first
// takes: no SFTP, no checksum, no new node process, just a new bridge over
// the SSH exec channel onto the SAME already-running detached agent. This
// is what makes BUG-SSH-02's version-check (SOL-SSH-02) matter for more
// than just first-connect: a reconnect that hits this path never redeploys
// at all.
func reattach(conn *sshconn.Connection, sockPath string) (*sshExecTransport, string, error) {
    alive, _, _ := conn.RunCommand(ctx, fmt.Sprintf("test -S %s && echo alive", shellQuote(sockPath)))
    if strings.TrimSpace(alive) != "alive" {
        return nil, "", ErrDetachedProcessGone // caller's cue: fall back to a full Provision (redeploy+relaunch), not just reattach
    }
    session, err := conn.NewSession()
    // ... StdinPipe/StdoutPipe, session.Start("node agent.js --connect --sock-path " + sockPath) ...
    return newSSHExecTransport(conn, session, stdin, stdout), sockPath, nil
}
```

```go
// provisioner.go — Provision now records sockPath on success (threaded
// into devserveragent.HandshakeInfo's existing free-form fields, or a new
// small typed field — either way, Client needs it to call reattach()
// later without re-resolving the SshTarget from scratch).
```

### Background reconnect loop for relay-ssh (closes the `managedExternally` no-op)

`session.go`'s `backgroundReconnect` currently no-ops entirely for relay-ssh
(`session.go:420-429`, `managedExternally`'s doc comment). This solution
gives relay-ssh sessions a **real** background loop — structurally the same
shape as relay-websocket's (`backoffDelay`, same jitter formula) but calling
`reattach()` instead of `connect()`:

```go
// devserveragent/session.go — extended
// relaySSHReconnect mirrors backgroundReconnect's loop structure exactly
// (same backoffDelay call, same closed/superseded checks) but is installed
// only for relay-ssh sessions (a new managedMode enum replaces the bool
// managedExternally: none | inboundOnly | relaySSHReattach — direct-websocket
// keeps the true no-op, relay-ssh gets this loop instead of a no-op).
func (s *session) relaySSHReconnect(reattacher SshReattacher, target domain.SshTarget) {
    for {
        // ... same wait/closed/alreadyLive checks as backgroundReconnect ...
        conn, err := reattacher.Reconnect(ctx, target) // new SSH dial + reattach() over it;
                                                          // on ErrDetachedProcessGone, falls back to
                                                          // a full Provision instead — see below
        // ... same attempt bookkeeping ...
        if err == nil {
            info, herr := reattacher.Handshake(ctx, conn) // detached process needs no NEW handshake
                                                             // (it already handshaked once at first
                                                             // launch) — this just confirms the bridge
                                                             // is live; see "no re-handshake" note below
            if herr == nil {
                s.attachTransport(conn, info)
                return
            }
        }
    }
}
```

**Why the reattached bridge needs no fresh `agent.handshake`**: the detached
agent process's in-memory state (its `AgentSession`, its pty-daemon
children) survived the SSH drop untouched — only the *bridge* died. Treating
every reattach as a full handshake would be wrong (the agent already told
Orca its platform/capabilities once; nothing changed). `reattach()` reuses
the `HandshakeInfo` captured at first `Provision`, cached on `*session`.

**Fallback to full re-provision**: `ErrDetachedProcessGone` (the detached
process itself crashed, host rebooted, etc.) is the one case where
`relaySSHReconnect` gives up on reattach and calls the full
`Client.getOrProvisionSession` path instead (redeploy+relaunch) — the
existing behavior, now reached deliberately instead of being the *only*
behavior.

## Design — buffered output while offline (BR-SSH-11) — mostly free, not reinvented

`session.go`/`client.go` already reference a `pty.replay` notification
(`session.go:294`, `client.go:293`'s comment identifying it as "the
unrelated pty.replay PTY-scrollback notification" — unrelated *to the
notification demux this pass adds*, not evidence the mechanism itself
doesn't exist). Once the detached agent process genuinely survives a drop
(this solution's core fix), its own pty-daemon — a further-detached child of
the agent process per `pty-daemon-server.ts`/`pty-daemon-client.ts`, already
unaffected by the agent process's own lifecycle — keeps buffering PTY output
the way it already does for any brief disconnect today. A reattaching
`StreamPty` subscriber re-requests replay the same way it would after any
other transient gap; **no new 10MB ring buffer needs inventing** for the PTY
case. Flag as a verification item, not a build item: confirm
`pty-daemon-server.ts`'s existing replay buffer's cap during implementation
and raise it to 10MB if it's smaller — do not build a second, parallel
buffer in `infra-fleet-service`.

## Design — "reconnecting" state + cancel (BR-SSH-13, "Reconnecting..." UX)

```go
// domain/connection.go — Status gains "reconnecting" alongside the existing
// "established" | "degraded" | "closed" (connection.go:29-31's comment).
```

`get_ssh_state.go`'s `SshState` gains a `Status string` field (not just
`Connected bool`) so `ssh.getState` can render the spec's overlay state;
`relaySSHReconnect` transitions the `connections` row to `"reconnecting"` on
entry and `"established"`/back on success, via a new
`ConnectionRepository.UpdateStatus` method.

**Cancel (BR-SSH-13)**: add `TeardownConnection`, which `infra-fleet-service.md`
§3 already specifies (`infra-fleet-service.md:102`) but the current proto
doesn't yet implement — this is completing an already-designed RPC, not
inventing one:

```go
// usecase/teardown_connection.go (new)
type TeardownConnection struct { conns ConnectionRepository; agent DevServerAgentClient }
func (uc *TeardownConnection) Execute(ctx context.Context, connectionID string) error {
    // marks connections row "closed", and calls a new Client.CancelReconnect(devServerID)
    // which closes s.closeCh on the *session — relaySSHReconnect's select on
    // s.closeCh (mirroring backgroundReconnect's existing pattern) returns
    // immediately, same shape as session.close() already uses.
}
```

## Test plan

- `agent/src/relay/agent-connection-stdio.test.ts` — `--detach` mode:
  parent process exits 0 once the child reports listening; child survives
  parent's controlling-session SIGHUP (spawn a real detached child in a
  test harness, close its controlling terminal, assert the socket is still
  accept()-able); `--connect` mode bridges bytes bidirectionally against a
  fake Unix socket peer.
- `sshrelay/launch_test.go` — first `launch()` uses `--detach`; a second
  `reattach()` call against the same still-alive `sockPath` opens a plain
  `--connect` session with no SFTP/checksum calls (assert the fake
  `Connection` records zero `SFTPClient()` calls).
- `sshrelay/reattach_test.go` — `test -S` reporting the socket gone returns
  `ErrDetachedProcessGone` without attempting a session.
- `devserveragent/session_test.go` — new
  `TestSession_RelaySSHReconnect_ReattachesWithoutRedeploy` (mirrors the
  existing `TestSession_BackgroundReconnect_RecoversAfterDropWithoutCallerRetry`
  at `session_test.go:181-233`, asserting `Reconnect` was called, `Provision`
  was not, for a live-detached-process scenario) and a second test for the
  `ErrDetachedProcessGone` → full-reprovision fallback path.
- `usecase/teardown_connection_test.go` — cancels an in-flight
  `relaySSHReconnect` loop (fake `closeCh`-equivalent signal observed);
  idempotent on an already-closed connection.
- `get_ssh_state_test.go` — reports `"reconnecting"` mid-backoff,
  `"established"` after a successful reattach.

## References

- `specs/backend-go/bugs/logic-v1/BUG-SSH-03-auto-reconnect-partial.md` — problem statement; explicitly names the abandoned `--detached --connect --sock-path` model this solution builds
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:79-85` ("What's deliberately not a separate service" — Dev Server Agent execution plane stays in `agent/`; confirms this fix's `agent/`-side half belongs there, not ported into `infra-fleet-service`)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:84-108` (Option A — keep the existing wire protocol; this solution's bridge is a new *launch mode*, not a new wire protocol, so it stays inside Option A's scope)
- `specs/backend-go/tdd/services/infra-fleet-service.md:100-110` (§3 `BootstrapFleetTarget`/streaming progress), `:518-523` (§9 relay-ssh trust model, unaffected by this change — the bridge session still authenticates via the same Vault-cert SSH connection)
- `agent/src/relay/agent-connection-stdio.ts:1-13` — current stdio-only mode, doc comment confirming no reconnect/detach exists
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go:60-88,414-473` — `managedExternally`'s current no-op, `backgroundReconnect`'s loop shape this solution's `relaySSHReconnect` mirrors
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:14-21` — package doc comment naming the abandoned detach model
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go:1-45`
- `docs/logic/remote-development/BL-SSH-03-auto-reconnect.md`
