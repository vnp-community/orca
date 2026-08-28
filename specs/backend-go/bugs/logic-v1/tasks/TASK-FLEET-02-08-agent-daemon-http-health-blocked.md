# TASK-FLEET-02-08: `agent/` daemon mode + HTTP `/health` listener + PID file (BLOCKED)

**From Solution:** SOL-FLEET-02
**Priority:** P1
**Service:** `agent/` (not backend-go)
**File:** `agent/src/relay/agent-connection-relay.ts`, `agent/src/relay/relay.ts`, and a new CLI entrypoint (TBD — likely `agent/src/relay/relay-daemon.ts` or similar)
**Depends on:** none (independent of the backend-go tasks in this set; those substitute an interim handshake-based liveness check instead of waiting on this)
**Status:** `[ ]` BLOCKED — requires `agent/` engineering; the TypeScript daemon/HTTP-health surface this task needs does not exist anywhere in `agent/src` today

---

## Context

BL-FLEET-02's "Start Relay" step (`~/.local/bin/orca-relay --daemon`, PID
file) and "Health Check" step (`curl http://localhost:<relayPort>/health`)
assume a **daemon binary with an HTTP listener** that is not a buildable
artifact anywhere in `agent/` today. Confirmed by direct inspection of
`agent/src`:
- no `--daemon` CLI flag on any relay entrypoint
- no PID-file writer for the relay/agent process
- no `/health` HTTP route on `agent-connection-relay.ts` or any relay
  transport
- `agent/src/relay/relay.ts` (the standalone SSH-relay-deploy script the
  *original* TS design targeted) parses `--grace-time`/`--connect`/
  `--detached`/`--sock-path` flags and reconnects over a **Unix domain
  socket**, never TCP/HTTP
- backend-go's relay-ssh mode deploys `agent/out/agent.js --stdio` (the Part
  A dispatcher), which has no daemon/HTTP-health surface either — it's a WS
  server on `relay-websocket` mode, not an HTTP server

This is genuine `agent/` engineering work, out of scope for a
backend-go-only change. SOL-FLEET-02's `bulkProvisionOne`
(TASK-FLEET-02-05) uses the **already-real** launch+handshake exchange
(`sshrelay.launch`/`Provisioner.receiveHandshake`) as an interim liveness
substitute — a genuine liveness proof, just not an HTTP `/health` probe
against an independent daemon, and not resilient to this service restarting
or the SSH connection dropping the way a true `--daemon`-launched,
PID-file-tracked process would be.

## What closing this gap for real requires (scope for a future `agent/` task)

1. **An HTTP listener on a negotiated `relayPort`** — mirroring
   `agent-connection-relay.ts`'s existing WS listener pattern
   (`agent/src/relay/agent-connection-relay.ts:45`), but HTTP instead of WS.
   Must expose at minimum `GET /health` returning process liveness.
2. **A `--daemon`/detach mode with a PID file** — analogous to what
   `relay.js`'s CLI flags (`--detached`, `--sock-path`) and its
   `relay.status` JSON-RPC response shape (`{pid, uptimeMs, detached,
   stdoutAlive, memory, ptys, socket, grace}`,
   `specs/agent/api/agent-rpc-catalog-runtime.md:247`) suggest were
   *intended* for `relay.js` specifically, not `agent.js`. The daemon must
   survive the SSH exec channel/session that launched it, unlike today's
   `launch.go`-launched process ("a dropped SSH connection just ends the
   session").
3. A decision on which binary owns this: extending `agent.js`'s Part A
   dispatcher with an HTTP listener + daemonization, or reviving/adapting
   `relay.js`'s `--detached` mode to be the actual deploy target instead of
   `agent.js --stdio`. Either choice has knock-on effects for
   `sshrelay.Provisioner`'s deploy step in backend-go and should be
   coordinated with whoever owns that adapter.

## Changes to make

None in this pass — do not implement. This task exists to track the gap
explicitly so it isn't silently dropped. When picked up, it should start
with a design doc under `specs/agent/` covering the listener/daemon
contract, then implement in `agent/src/relay/`, then update
`backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`
and `launch.go` to use the new daemon/health surface once it exists,
replacing the interim handshake-based check TASK-FLEET-02-05 uses.

## Verify

Not applicable — no code change in this task. Verification criteria for the
eventual `agent/` implementation: a `GET http://127.0.0.1:<relayPort>/health`
call succeeds against a daemonized relay process; the process survives the
launching SSH session ending; a PID file exists at a documented path and
matches the running process.
