/**
 * WsSessionRouter — Route WebSocket connections to per-user processes
 *
 * Intercepts WS connections, resolves userId from session cookie via AuthManager,
 * then proxies WS traffic ↔ user process Unix socket.
 *
 * @module main/session/ws-session-router
 */

import * as net from 'node:net'
import type { IncomingMessage } from 'node:http'
import type { WebSocket } from 'ws'
import type { SessionManager } from './session-manager'
import type { AuthManager } from '../auth/auth-manager'

export class WsSessionRouter {
  private readonly sessionManager: SessionManager
  private readonly authManager:    AuthManager

  constructor(opts: { sessionManager: SessionManager; authManager: AuthManager }) {
    this.sessionManager = opts.sessionManager
    this.authManager    = opts.authManager
  }

  /**
   * Synchronously resolve userId from cookie header.
   * NOTE: AuthManager.validateRequest is async in the real implementation,
   * but the cookie-parse + DB lookup happens inside validateRequest.
   * For WS routing we use a fast path via extractSessionCookie (sync).
   */
  resolveUserFromRequest(req: IncomingMessage): Promise<string | null> {
    return this.authManager.validateRequest(req.headers.cookie)
      .then(session => session?.userId ?? null)
  }

  async getOrCreateUserSocket(userId: string): Promise<string> {
    const proc = await this.sessionManager.getOrSpawnUserProcess(userId)
    return proc.socketPath
  }

  /**
   * Main entry — called from WebSocket server 'connection' event.
   * - With valid login session: proxy WS ↔ user process Unix socket
   * - Without session: close 4401 (auth required)
   *
   * NOTE: PairCode / deviceToken connections bypass this router entirely.
   * They connect directly to the shared runtime (ORCA_MULTI_USER=0 legacy path).
   */
  async handleConnection(ws: WebSocket, req: IncomingMessage): Promise<void> {
    const userId = await this.resolveUserFromRequest(req)

    if (!userId) {
      ws.close(4401, 'Authentication required. Please log in first.')
      return
    }

    this.sessionManager.touch(userId)

    let socketPath: string
    try {
      socketPath = await this.getOrCreateUserSocket(userId)
    } catch (err) {
      console.error(`[WsSessionRouter] Failed to spawn process: userId=${userId}`, err)
      ws.close(1011, 'Internal error: cannot start user session')
      return
    }

    // Proxy WS ↔ Unix socket (binary-safe, bidirectional)
    const upstream = net.createConnection(socketPath)

    upstream.on('error', (err) => {
      console.error(`[WsSessionRouter] Upstream error: userId=${userId}`, err)
      if ((ws as unknown as { readyState: number; OPEN: number }).readyState ===
          (ws as unknown as { readyState: number; OPEN: number }).OPEN) {
        ws.close(1011, 'User session unavailable')
      }
    })

    ws.on('message', (data: Buffer | string, isBinary: boolean) => {
      if (upstream.writable) {
        upstream.write(isBinary ? data : Buffer.from(data as string))
      }
    })

    upstream.on('data', (chunk: Buffer) => {
      const wsAny = ws as unknown as { readyState: number; OPEN: number; send: (d: Buffer) => void }
      if (wsAny.readyState === wsAny.OPEN) wsAny.send(chunk)
    })

    ws.on('close', () => {
      upstream.end()
      this.sessionManager.touch(userId)
    })

    upstream.on('close', () => {
      const wsAny = ws as unknown as { readyState: number; OPEN: number; close: (code: number, reason: string) => void }
      if (wsAny.readyState === wsAny.OPEN) wsAny.close(1011, 'User session ended')
    })
  }
}
