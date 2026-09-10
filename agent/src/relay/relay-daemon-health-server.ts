/**
 * relay-daemon-health-server.ts — minimal HTTP `/health` listener for a
 * daemonized relay process. Part of TASK-FLEET-02-08's minimal daemon/
 * health primitive (see relay-daemon.ts's doc comment) — BL-FLEET-02's
 * "Health Check" step is `curl http://localhost:<relayPort>/health`; this
 * is that endpoint, loopback-bound like agent-hook-server.ts's HTTP
 * listener (same shape: bind 127.0.0.1, no external exposure).
 *
 * Deliberately minimal: one route, no auth (matches the "no token/auth
 * beyond..." posture of a loopback-only liveness probe — nothing here
 * exposes PTY data or credentials, only "is this process alive"), no
 * dependency on the rest of the daemon's runtime beyond an optional
 * `extraStatus` callback for future fields (uptime, PTY count, etc. once a
 * real daemon runtime exists to report them from).
 */
import { createServer, type Server, type ServerResponse } from 'node:http'

export type RelayHealthStatus = {
  status: 'ok'
  pid: number
  uptimeMs: number
}

export type RelayHealthServerOptions = {
  /** Port to bind. 0 = OS-assigned ephemeral port (tests use this). */
  port?: number
  /** Called on every /health request to build the response body — lets a
   *  real daemon runtime attach extra fields (ptys, memory, etc.) later
   *  without this module needing to know about daemon internals. Defaults
   *  to `{status: 'ok', pid: process.pid, uptimeMs: process.uptime() * 1000}`. */
  buildStatus?: () => RelayHealthStatus
}

function defaultStatus(): RelayHealthStatus {
  return { status: 'ok', pid: process.pid, uptimeMs: Math.round(process.uptime() * 1000) }
}

export class RelayDaemonHealthServer {
  private server: Server | null = null
  private port = 0
  private readonly buildStatus: () => RelayHealthStatus
  private readonly preferredPort: number

  constructor(options: RelayHealthServerOptions = {}) {
    this.preferredPort = options.port ?? 0
    this.buildStatus = options.buildStatus ?? defaultStatus
  }

  /** Starts listening. Resolves once bound; rejects on bind failure (e.g.
   *  EADDRINUSE) — callers that want fallback-to-ephemeral behavior handle
   *  that themselves, unlike agent-hook-server.ts's built-in fallback,
   *  since a daemon's health port is operator-documented, not
   *  re-coordinated via an endpoint file. */
  start(): Promise<number> {
    if (this.server) {
      return Promise.resolve(this.port)
    }
    return new Promise<number>((resolve, reject) => {
      const server = createServer((req, res) => this.handleRequest(req.method, req.url, res))
      const onError = (err: Error): void => {
        server.off('listening', onListening)
        this.server = null
        reject(err)
      }
      const onListening = (): void => {
        server.off('error', onError)
        server.on('error', (err) => {
          process.stderr.write(`[relay-daemon-health-server] server error: ${err.message}\n`)
        })
        const address = server.address()
        this.port = address && typeof address === 'object' ? address.port : 0
        resolve(this.port)
      }
      server.once('error', onError)
      // Loopback only — mirrors agent-hook-server.ts's binding rationale:
      // this is a same-box liveness probe, never exposed off-host.
      server.listen(this.preferredPort, '127.0.0.1', onListening)
      this.server = server
    })
  }

  stop(): void {
    this.server?.close()
    this.server = null
    this.port = 0
  }

  get boundPort(): number {
    return this.port
  }

  private handleRequest(method: string | undefined, url: string | undefined, res: ServerResponse): void {
    const pathname = new URL(url ?? '/', 'http://127.0.0.1').pathname
    if (method !== 'GET' || pathname !== '/health') {
      res.writeHead(404, { 'content-type': 'application/json' })
      res.end(JSON.stringify({ error: 'not found' }))
      return
    }
    const body = JSON.stringify(this.buildStatus())
    res.writeHead(200, { 'content-type': 'application/json' })
    res.end(body)
  }
}
