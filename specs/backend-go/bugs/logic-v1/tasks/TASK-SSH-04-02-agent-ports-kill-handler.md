# TASK-SSH-04-02: `agent/` — `ports.kill` handler

**From Solution:** SOL-SSH-04
**Priority:** P0 — `KillWorkspacePort` already relays to `ports.kill`; today it always fails
**Service:** agent/ (Dev Server Agent)
**File:** `agent/src/relay/port-kill-handler.ts` (new)
**Depends on:** none
**Status:** `[x] DONE — port-kill-handler.ts's PortKillHandler + handlePortsKill added; detectListeningPorts extracted from PortScanHandler as a shared free function; wired into BOTH the class-based RelayDispatcher (relay.ts, per spec) AND the actually-live switch dispatcher (agent-rpc-dispatch.ts's 'ports.detect'/'ports.kill' cases — the class-based relay.ts/dispatcher.ts registration was found to be unreachable dead code from agent-entry.ts's real bundle, so the switch-case wiring is what makes this work end-to-end); tests pass, verified present in the built out/agent.js bundle`

---

## Context

`KillWorkspacePort` (`kill_workspace_port.go:49`) already relays to
`"ports.kill"`, but no such handler exists anywhere in `agent/src/relay/` —
only `ports.detect` (scan) does, in `port-scan-handler.ts`. This is a real
gap with no existing right-name to rename to (unlike TASK-SSH-04-01's
`ports.scan`→`ports.detect` fix) — it needs a genuinely new handler.

## Changes to make

Create `agent/src/relay/port-kill-handler.ts`, mirroring
`PortScanHandler`'s class shape and registration pattern:

```ts
import type { RelayDispatcher, RequestContext } from './dispatcher'

type KillPortParams = {
  worktreeId?: string
  pid?: number
  port?: number
}

type KillPortResult = {
  ok: boolean
  reason?: string
}

export class PortKillHandler {
  constructor(dispatcher: RelayDispatcher) {
    dispatcher.onRequest('ports.kill', async (params, _context: RequestContext): Promise<KillPortResult> => {
      const { pid, port } = params as KillPortParams
      if (typeof pid !== 'number' || !Number.isSafeInteger(pid) || pid <= 0) {
        return { ok: false, reason: 'invalid pid' }
      }
      if (typeof port !== 'number' || !Number.isSafeInteger(port) || port <= 0) {
        return { ok: false, reason: 'invalid port' }
      }

      // Validate the pid is actually one ports.detect would report before
      // killing anything — mirrors the old TS system's
      // workspace-port-ownership.ts killWorkspacePort validation shape,
      // ported to the agent side since the agent (not backend-go) is what
      // can see the remote process table.
      const owns = await this.pidOwnsPort(pid, port)
      if (!owns) {
        return { ok: false, reason: `pid ${pid} is not listening on port ${port}` }
      }

      try {
        process.kill(pid, 'SIGTERM')
        return { ok: true }
      } catch (error) {
        return { ok: false, reason: error instanceof Error ? error.message : String(error) }
      }
    })
  }

  private async pidOwnsPort(pid: number, port: number): Promise<boolean> {
    const { PortScanHandler } = await import('./port-scan-handler')
    // Reuse the scan handler's own detection logic rather than duplicating
    // /proc parsing — see PortScanHandler's private scan methods; expose a
    // small static/exported helper there if it isn't already callable
    // standalone (scanLinuxListeningPorts today is a private instance
    // method — widen its visibility or extract a free function shared by
    // both handlers, whichever this codebase's existing handler-sharing
    // convention prefers).
    void PortScanHandler
    return true // placeholder — implementer wires the real detect-then-match check
  }
}
```

Wire it into `agent/src/relay/relay.ts` alongside `PortScanHandler`:

```ts
import { PortKillHandler } from './port-kill-handler'
// ...
const _portKillHandler = new PortKillHandler(dispatcher)
```

## Verify

```bash
cd /opt/repos/orca/agent
pnpm vitest run src/relay/port-kill-handler.test.ts
pnpm tsc --noEmit
```

Expected new test (`port-kill-handler.test.ts`, mirroring
`port-scan-handler.test.ts`'s harness): killing a pid that IS listening on
the given port succeeds (`{ok: true}`); a pid/port mismatch returns
`{ok: false, reason: ...}` without calling `process.kill`; an invalid
pid/port returns `{ok: false}` without touching the process table.
