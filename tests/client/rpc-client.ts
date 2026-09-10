/**
 * Minimal RPC-over-WebSocket client for tests/client specs.
 *
 * Mirrors the wire protocol `frontend/src/renderer/src/web/web-session-client.ts`
 * uses for the browser SPA's cookie-authenticated RPC connection: open a
 * WebSocket to `${wsBaseUrl}/ws` with the session cookie as an HTTP header on
 * the handshake (`WsSessionRouter`, `backend/src/main/session/ws-session-router.ts`,
 * resolves the user from that cookie at upgrade time and proxies the socket to
 * the per-user `OrcaRuntimeRpcServer`), then exchange newline-delimited JSON
 * `{id, authToken: 'cookie-auth', method, params}` requests for
 * `{id, ok, result}` / `{id, ok: false, error}` responses.
 *
 * This is the SAME method registry `specs/frontend/api/rpc-catalog.md` (desktop/
 * web frontend) and `specs/frontend/api/mobile-rpc-catalog.md` (mobile) document —
 * those files are the source of truth for which methods exist and who calls them.
 */
import WebSocket from 'ws'

export type RpcSuccess<T = unknown> = { id: string; ok: true; result: T }
export type RpcFailure = {
  id: string
  ok: false
  error: { code: string; message: string; data?: unknown }
}
export type RpcResponse<T = unknown> = RpcSuccess<T> | RpcFailure

/** Thrown when the WS handshake itself is rejected (e.g. WsSessionRouter's 4401). */
export class RpcConnectError extends Error {
  constructor(
    public readonly code: number,
    public readonly reason: string
  ) {
    super(`RPC connection rejected: ${code} ${reason}`)
  }
}

export class RpcSession {
  private requestCounter = 0
  private readonly pending = new Map<
    string,
    { resolve: (r: RpcResponse) => void; reject: (e: Error) => void }
  >()

  private constructor(private readonly ws: WebSocket) {
    ws.on('message', (data) => this.handleMessage(data.toString('utf8')))
    ws.once('close', () => this.rejectAllPending(new Error('RPC connection closed')))
  }

  // Why: WsSessionRouter (backend/src/main/session/ws-session-router.ts)
  // completes the WebSocket handshake (client 'open' fires) BEFORE it has
  // resolved the session cookie — an unauthenticated/invalid cookie closes
  // the socket with code 4401 a moment *after* 'open', not instead of it.
  // Resolving connect() on 'open' alone would hand back a session that's
  // already doomed. Give an already-open socket a brief grace window for
  // that immediate auth-close before declaring the connection good.
  private static readonly AUTH_CLOSE_GRACE_MS = 500

  /** Opens a cookie-authenticated RPC connection. Rejects with RpcConnectError on auth failure. */
  static connect(wsUrl: string, cookie: string, timeoutMs = 10_000): Promise<RpcSession> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(wsUrl, { headers: { Cookie: cookie } })
      let settled = false
      const timer = setTimeout(() => {
        if (settled) {
          return
        }
        settled = true
        ws.terminate()
        reject(new Error(`RPC connect timed out: ${wsUrl}`))
      }, timeoutMs)
      ws.once('open', () => {
        const graceTimer = setTimeout(() => {
          if (settled) {
            return
          }
          settled = true
          clearTimeout(timer)
          resolve(new RpcSession(ws))
        }, RpcSession.AUTH_CLOSE_GRACE_MS)
        ws.once('close', (code: number, reasonBuf: Buffer) => {
          if (settled) {
            return
          }
          settled = true
          clearTimeout(timer)
          clearTimeout(graceTimer)
          reject(new RpcConnectError(code, reasonBuf.toString('utf8')))
        })
      })
      ws.once('error', (err: Error) => {
        if (settled) {
          return
        }
        settled = true
        clearTimeout(timer)
        reject(err)
      })
    })
  }

  private rejectAllPending(error: Error): void {
    for (const [id, pending] of this.pending) {
      this.pending.delete(id)
      pending.reject(error)
    }
  }

  private handleMessage(raw: string): void {
    // Why: ws-session-router.ts forwards one line per WS text frame, but stay
    // defensive in case multiple frames get coalesced by the transport.
    for (const line of raw.split('\n')) {
      const trimmed = line.trim()
      if (!trimmed) {
        continue
      }
      let msg: Record<string, unknown>
      try {
        msg = JSON.parse(trimmed) as Record<string, unknown>
      } catch {
        continue
      }
      if (msg['_keepalive'] === true || typeof msg['id'] !== 'string') {
        continue
      }
      const pending = this.pending.get(msg['id'])
      if (pending) {
        this.pending.delete(msg['id'])
        pending.resolve(msg as unknown as RpcResponse)
      }
    }
  }

  /** Sends one RPC request and resolves with its response envelope (ok:true or ok:false). */
  call<T = unknown>(method: string, params?: unknown, timeoutMs = 15_000): Promise<RpcResponse<T>> {
    const id = `test-rpc-${++this.requestCounter}-${Math.random().toString(36).slice(2)}`
    // Why: the live deployment's wscompat shim (backend-go/services/api-gateway/
    // internal/adapter/wscompat/) decodes every handler's args as args[0] — an
    // omitted `params` key serializes to no `args` at all and fails with
    // "missing arg[0]", even for methods whose real (TS-backend) schema is
    // `params: null`. Always send an object so that decode has something to
    // unmarshal.
    const wireParams = params ?? {}
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`RPC call timed out: ${method}`))
      }, timeoutMs)
      this.pending.set(id, {
        resolve: (r) => {
          clearTimeout(timer)
          resolve(r as RpcResponse<T>)
        },
        reject: (e) => {
          clearTimeout(timer)
          reject(e)
        }
      })
      this.ws.send(`${JSON.stringify({ id, authToken: 'cookie-auth', method, params: wireParams })}\n`)
    })
  }

  /** Calls and throws if the backend returned ok:false — for specs that only assert success. */
  async callOk<T = unknown>(method: string, params?: unknown, timeoutMs?: number): Promise<T> {
    const res = await this.call<T>(method, params, timeoutMs)
    if (!res.ok) {
      throw new Error(`${method} failed: ${res.error.code} — ${res.error.message}`)
    }
    return res.result
  }

  close(): void {
    this.ws.close()
  }
}
