/**
 * PtyDaemon — Unix socket IPC daemon for headless CLI mode (TASK-CLI-001)
 *
 * Listens on a Unix domain socket and handles JSON-RPC-style commands from CLI clients.
 * Allows the CLI tool to forward PTY commands to OrcaRuntime without Electron IPC.
 *
 * Protocol: newline-delimited JSON
 *   Request:  { id: string, method: string, params: unknown } + '\n'
 *   Response: { id: string, ok: true, result: unknown } + '\n'
 *           | { id: string, ok: false, error: string } + '\n'
 *
 * Usage:
 *   const daemon = new PtyDaemon('/tmp/orca-daemon.sock', async (cmd) => handler(cmd))
 *   await daemon.start()
 *   // ... on shutdown:
 *   daemon.stop()
 *
 * @module main/cli/PtyDaemon
 */

import * as net from 'node:net'
import { existsSync, unlinkSync, chmodSync } from 'node:fs'

// ── Types ─────────────────────────────────────────────────────────────────────

export type DaemonCommand = {
  id:      string
  method:  string
  params?: unknown
}

export type DaemonResponse = {
  id:      string
  ok:      boolean
  result?: unknown
  error?:  string
}

export type CommandHandler = (cmd: DaemonCommand) => Promise<unknown>

// ── PtyDaemon ─────────────────────────────────────────────────────────────────

export class PtyDaemon {
  private server: net.Server | null = null

  constructor(
    /** Unix socket path, e.g. /tmp/orca-daemon.sock */
    private readonly socketPath: string,
    /** Handler called for each incoming command */
    private readonly handler:    CommandHandler
  ) {}

  /** Start listening on the socket. Removes stale socket file if present. */
  start(): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      // FIX TASK-CLI-001: Clean up stale socket from previous run
      if (existsSync(this.socketPath)) {
        try {
          unlinkSync(this.socketPath)
        } catch {
          // Another process may have removed it between check and unlink — ignore
        }
      }

      this.server = net.createServer((client) => {
        this.handleClient(client)
      })

      this.server.listen(this.socketPath, () => {
        // Restrict access to owner only (mode 0600)
        try { chmodSync(this.socketPath, 0o600) } catch { /* non-fatal */ }
        console.log(`[PtyDaemon] Listening on ${this.socketPath}`)
        resolve()
      })

      this.server.on('error', (err) => {
        if (!this.server) {return}  // already stopped
        console.error('[PtyDaemon] Server error:', err)
        reject(err)
      })
    })
  }

  /** Stop the server and remove the socket file. */
  stop(): void {
    this.server?.close()
    this.server = null
    try {
      if (existsSync(this.socketPath)) {unlinkSync(this.socketPath)}
    } catch {
      // Ignore cleanup errors
    }
  }

  // ── Private: per-client handler ─────────────────────────────────────────────

  private handleClient(client: net.Socket): void {
    let buffer = ''

    client.on('data', (chunk) => {
      buffer += chunk.toString('utf8')

      // Process all complete newline-delimited messages in buffer
      let newline: number
      while ((newline = buffer.indexOf('\n')) !== -1) {
        const line = buffer.slice(0, newline).trim()
        buffer = buffer.slice(newline + 1)

        if (!line) {continue}

        let cmd: DaemonCommand
        try {
          cmd = JSON.parse(line) as DaemonCommand
        } catch {
          client.write(`${JSON.stringify({ id: 'unknown', ok: false, error: 'Invalid JSON' })  }\n`)
          continue
        }

        this.handler(cmd)
          .then((result) => {
            const resp: DaemonResponse = { id: cmd.id, ok: true, result }
            client.write(`${JSON.stringify(resp)  }\n`)
          })
          .catch((err: unknown) => {
            const resp: DaemonResponse = {
              id:    cmd.id,
              ok:    false,
              error: err instanceof Error ? err.message : String(err),
            }
            client.write(`${JSON.stringify(resp)  }\n`)
          })
      }
    })

    client.on('error', (err) => {
      console.warn('[PtyDaemon] Client error:', err.message)
    })

    client.on('close', () => {
      // Client disconnected — nothing to clean up (no persistent state per connection)
    })
  }
}
