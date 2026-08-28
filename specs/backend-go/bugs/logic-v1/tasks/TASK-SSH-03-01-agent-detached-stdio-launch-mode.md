# TASK-SSH-03-01: `agent/` — `--detach --sock-path` launch mode

**From Solution:** SOL-SSH-03
**Priority:** P0 — everything else in this solution's reconnect story depends on the detached process existing
**Service:** agent/ (Dev Server Agent)
**File:** `agent/src/relay/agent-connection-stdio.ts`
**Depends on:** none
**Status:** `[x] DONE — runDetachedStdioMode (parent spawn+wait + child net.Server/pidfile) added to agent-connection-stdio.ts; new tests cover both branches`

---

## Context

`agent-connection-stdio.ts`'s own header comment states plainly there is
"no reconnect loop" because the SSH exec channel's stdin/stdout pipes ARE
the transport — when SSH drops, the agent process's only communication
channel dies with it. BR-SSH-10 ("agent process trên remote PHẢI tiếp tục
chạy khi local disconnect") is structurally impossible without a detached
process whose lifecycle survives the SSH exec channel closing. This task
adds that launch mode, additive to the existing `--stdio` mode (which keeps
working unchanged for anything not opting in).

## Changes to make

In `agent/src/relay/agent-connection-stdio.ts`, add a detached-mode entry
point alongside the existing `connectStdio`:

```ts
import { spawn } from 'node:child_process'
import { existsSync, unlinkSync, writeFileSync } from 'node:fs'
import * as net from 'node:net'

/**
 * runDetachedStdioMode implements BR-SSH-10's "agent survives local
 * disconnect": instead of wiring the wire protocol to THIS process's own
 * stdin/stdout (which die with the SSH exec channel that launched it), it:
 *
 *   1. re-execs itself with { detached: true, stdio: 'ignore' } — Node's
 *      standard technique for a session-leader child not tied to the
 *      parent's controlling terminal, so SIGHUP on the SSH exec channel's
 *      close does not propagate to it (no native addon/setsid binary
 *      dependency needed on the remote host).
 *   2. the CHILD opens a Unix domain socket listener at sockPath instead of
 *      using stdin/stdout for the wire protocol — StdioWebSocketAdapter's
 *      duck-typed interface (readyState/send/close/on) is satisfied by a
 *      thin net.Socket wrapper instead of process.stdin/stdout, so
 *      agent-session.ts needs zero changes; only the transport this file
 *      hands it changes.
 *   3. the child writes a pidfile at `${sockPath}.pid`.
 *   4. the ORIGINAL (parent) process, still attached to the SSH exec
 *      channel, waits for the child to report "listening" over a pipe, then
 *      exits 0 — letting that first SSH exec session close cleanly without
 *      killing the now-detached child.
 */
export async function runDetachedStdioMode(
  sockPath: string,
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<void> {
  if (process.env['ORCA_RELAY_DETACHED_CHILD'] === '1') {
    await runDetachedChildProcess(sockPath, config, tools, log)
    return
  }

  const child = spawn(process.execPath, [process.argv[1]!, '--detach', '--sock-path', sockPath], {
    detached: true,
    stdio: ['ignore', 'ignore', 'ignore', 'ipc'],
    env: { ...process.env, ORCA_RELAY_DETACHED_CHILD: '1' }
  })

  await new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('detached child did not report listening within 10s')), 10_000)
    child.once('message', (msg) => {
      if (msg === 'listening') {
        clearTimeout(timeout)
        child.unref()
        child.disconnect()
        resolve()
      }
    })
    child.once('error', (err) => {
      clearTimeout(timeout)
      reject(err)
    })
  })

  log.info(`detached relay process started (pid ${child.pid}), listening at ${sockPath}`)
}

async function runDetachedChildProcess(
  sockPath: string,
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<void> {
  if (existsSync(sockPath)) {
    unlinkSync(sockPath) // stale socket from a prior crashed run
  }

  const server = net.createServer((socket) => {
    const adapter = new StdioWebSocketAdapter(log, socket, socket)
    const session = createSession(config, tools, log)
    session.start(adapter as unknown as WebSocket)
  })

  await new Promise<void>((resolve, reject) => {
    server.once('error', reject)
    server.listen(sockPath, () => resolve())
  })

  writeFileSync(`${sockPath}.pid`, String(process.pid))
  process.send?.('listening')

  // Runs forever — the detached process's whole point is outliving any one
  // SSH exec channel bridging into it. Only killed by the remote host
  // itself (reboot, explicit teardown via TeardownConnection — see
  // backend-go's usecase/teardown_connection.go).
  await new Promise<void>(() => {})
}
```

`StdioWebSocketAdapter`'s constructor already accepts `input`/`output`
params separately from `process.stdin`/`process.stdout` (see its existing
signature) — passing the same `net.Socket` for both is exactly what the
child's per-connection handler above does, no change needed to that class
itself.

## Verify

```bash
cd /opt/repos/orca/agent
pnpm vitest run src/relay/agent-connection-stdio.test.ts
pnpm tsc --noEmit
```

Expected new tests in `agent-connection-stdio.test.ts`: `--detach` mode's
parent process resolves (exits cleanly) once the child reports listening;
a real detached child (spawned in the test harness) survives its own
`disconnect()`/parent exit and the socket remains `net.connect`-able
afterward.
