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
import { createTracer } from '../../shared/trace'

const wsRouter = createTracer('wsSession:route')

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

  /**
   * Main entry — called from WebSocket server 'connection' event.
   * - With valid login session: proxy WS ↔ user process Unix socket
   * - Without session: close 4401 (auth required)
   *
   * NOTE: PairCode / deviceToken connections bypass this router entirely.
   * They connect directly to the shared runtime (ORCA_MULTI_USER=0 legacy path).
   */
  async handleConnection(ws: WebSocket, req: IncomingMessage): Promise<void> {
    const span = wsRouter.start()
    const userId = await this.resolveUserFromRequest(req)

    if (!userId) {
      span.fail('auth required', { cookie: req.headers.cookie ? 'present' : 'absent' })
      ws.close(4401, 'Authentication required. Please log in first.')
      return
    }

    span.step('accepted', { userId })
    console.log(`[WsSessionRouter] Connection accepted for userId=${userId}`)
    this.sessionManager.touch(userId)

    let proc
    try {
      proc = await this.sessionManager.getOrSpawnUserProcess(userId)
    } catch (err) {
      span.fail(err, { userId, phase: 'spawn' })
      ws.close(1011, 'Internal error: cannot start user session')
      return
    }

    // Why: use socketPath and authToken directly from UserProcess rather than
    // reading runtime-metadata. The metadata file is only written by the Electron
    // runtime path; user-process-entry sends the values via IPC 'ready' message
    // instead, which SessionManager stores in UserProcess. Reading from proc
    // eliminates the race where the socket exists but the metadata file doesn't.
    const socketPath = proc.socketPath
    const authToken  = proc.authToken

    if (!socketPath) {
      span.fail('no socket path', { userId })
      ws.close(1011, 'Internal error: user session socket unavailable')
      return
    }

    span.step('proxy-start', { userId, socketPath })

    // Proxy WS ↔ Unix socket (binary-safe, bidirectional)
    const upstream = net.createConnection(socketPath)

    upstream.on('error', (err) => {
      span.fail(err, { userId, phase: 'upstream' })
      console.error(`[WsSessionRouter] Upstream error: userId=${userId}`, err)
      if ((ws as unknown as { readyState: number; OPEN: number }).readyState ===
          (ws as unknown as { readyState: number; OPEN: number }).OPEN) {
        ws.close(1011, 'User session unavailable')
      }
    })

    const keepaliveTimer = setInterval(() => {
      if (upstream.writable) {
        upstream.write('\n')
      }
    }, 15000)

    ws.on('message', (data: Buffer | string, isBinary: boolean) => {
      if (!upstream.writable) return
      if (!isBinary) {
        try {
          const raw = (data as string | Buffer).toString('utf8')
          const parsed = JSON.parse(raw)
          if (parsed && typeof parsed === 'object' && parsed.authToken === 'cookie-auth') {
            parsed.authToken = authToken
            upstream.write(JSON.stringify(parsed) + '\n')
            return
          }
        } catch {
          // ignore parse errors, forward raw bytes
        }
      }
      upstream.write(isBinary ? data : Buffer.from(data as string))
    })

    let upstreamBuffer = ''
    upstream.on('data', (chunk: Buffer) => {
      const wsAny = ws as unknown as { readyState: number; OPEN: number; send: (d: string) => void }
      if (wsAny.readyState !== wsAny.OPEN) return

      upstreamBuffer += chunk.toString('utf8')
      let newlineIndex = upstreamBuffer.indexOf('\n')
      while (newlineIndex !== -1) {
        const rawMessage = upstreamBuffer.slice(0, newlineIndex).trim()
        upstreamBuffer = upstreamBuffer.slice(newlineIndex + 1)
        if (rawMessage) {
          wsAny.send(rawMessage)
        }
        newlineIndex = upstreamBuffer.indexOf('\n')
      }
    })

    ws.on('close', () => {
      clearInterval(keepaliveTimer)
      upstream.end()
      this.sessionManager.touch(userId)
    })

    upstream.on('close', () => {
      clearInterval(keepaliveTimer)
      const wsAny = ws as unknown as { readyState: number; OPEN: number; close: (code: number, reason: string) => void }
      if (wsAny.readyState === wsAny.OPEN) wsAny.close(1011, 'User session ended')
    })
  }
}
