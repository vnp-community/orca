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
