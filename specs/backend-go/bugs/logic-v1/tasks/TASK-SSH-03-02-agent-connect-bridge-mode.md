# TASK-SSH-03-02: `agent/` — `relay-detach-bridge.ts` (`--connect` mode)

**From Solution:** SOL-SSH-03
**Priority:** P0
**Service:** agent/ (Dev Server Agent)
**File:** `agent/src/relay/relay-detach-bridge.ts` (new)
**Depends on:** TASK-SSH-03-01
**Status:** `[x] DONE — relay-detach-bridge.ts's runConnectBridgeMode + --detach/--connect CLI wiring in agent-entry.ts (readSockPathArg helper); new bidirectional-pipe tests pass`

---

## Context

Once the detached process (TASK-SSH-03-01) is running and listening on a
Unix socket, every SUBSEQUENT SSH connection needs a way to bridge that
socket back onto the fresh exec channel's stdin/stdout — this is the
"socat over SSH exec" bridge, implemented in Node (already required on the
remote host — no new binary dependency) since `socat`/`nc` availability
can't be assumed.

## Changes to make

Create `agent/src/relay/relay-detach-bridge.ts`:

```ts
// src/relay/relay-detach-bridge.ts
// --connect mode's implementation: bridges a fresh SSH exec channel's own
// stdin/stdout 1:1 onto an already-running detached agent process's Unix
// socket (opened by runDetachedStdioMode's child — see
// agent-connection-stdio.ts). No new protocol, no changes to agent-wire.ts
// or agent-session.ts's framing — this is a plain bidirectional byte pipe.
import * as net from 'node:net'

/**
 * runConnectBridgeMode connects to sockPath and pipes process.stdin into it
 * and its data back onto process.stdout, resolving when either side closes
 * — the entry point backend-go's sshrelay.reattach() launches via
 * `node agent.js --connect --sock-path <path>` for every reattach after the
 * first.
 */
export async function runConnectBridgeMode(sockPath: string): Promise<void> {
  const socket = net.createConnection(sockPath)

  await new Promise<void>((resolve, reject) => {
    socket.once('connect', () => resolve())
    socket.once('error', reject)
  })

  process.stdin.pipe(socket)
  socket.pipe(process.stdout)

  await new Promise<void>((resolve) => {
    const done = (): void => resolve()
    socket.once('close', done)
    socket.once('error', done)
    process.stdin.once('end', done) // SSH exec channel's peer went away
  })
}
```

Wire the CLI flag in `agent/src/relay/agent-entry.ts` (alongside the
existing `--stdio` branch):

```ts
if (process.argv.includes('--detach')) {
  const sockPath = readSockPathArg(process.argv)
  const config = loadAgentConfig({ stdio: true })
  const log = createAgentLogger(config.logLevel)
  const tools = await discoverTools(config)
  const { runDetachedStdioMode } = await import('./agent-connection-stdio')
  await runDetachedStdioMode(sockPath, config, tools, log)
  return
}
if (process.argv.includes('--connect')) {
  const sockPath = readSockPathArg(process.argv)
  const { runConnectBridgeMode } = await import('./relay-detach-bridge')
  await runConnectBridgeMode(sockPath)
  return
}
```

Add a small `readSockPathArg` helper in `agent-entry.ts` reading the
`--sock-path <path>` pair from `process.argv` (throw a clear error if
absent — both new modes require it).

## Verify

```bash
cd /opt/repos/orca/agent
pnpm vitest run src/relay/agent-connection-stdio.test.ts
pnpm tsc --noEmit
```

Expected new test: `--connect` mode bridges bytes bidirectionally against a
fake Unix socket peer (write on one side observed on the other, both
directions); resolves cleanly when the peer closes.
