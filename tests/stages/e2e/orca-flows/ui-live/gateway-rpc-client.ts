/**
 * Cookie-authenticated RPC client for the api-gateway `/ws`, in the same newline-delimited `{id, authToken, method, params}`
 * dialect the browser SPA speaks. The live spec uses it only to cross-check what the UI shows and to clean up; every
 * product step goes through the UI.
 */
import WebSocket from 'ws'
import type { LiveStackConfig } from './live-stack-config'

/** The fields of a gateway row (request, approval, dev server ...) this suite reads; anything else stays unknown. */
export type RpcRow = {
  id: string
  status: string
  version: number
  title: string
  subjectType: string
  type: string
  name: string
  approvalStatus: string
  [key: string]: unknown
}

export class GatewayRpcError extends Error {
  constructor(
    readonly code: string,
    message: string
  ) {
    super(`${code}: ${message}`)
  }
}

type Pending = { resolve: (v: unknown) => void; reject: (e: Error) => void }

/** A push frame of a stream channel (the SPA dialect sends them as `streaming` replies with the subscribe call's id). */
export type StreamFrame = { receivedAt: number; result: unknown }

export class GatewayRpcClient {
  private ws: WebSocket | null = null
  private nextId = 0
  private readonly pending = new Map<string, Pending>()
  private buffer = ''
  private readonly streams = new Map<string, (frame: StreamFrame) => void>()
  /** How many calls the request-service throttled and we retried, so a too-tight limit shows in the run output. */
  rateLimitRetries = 0
  /** Times the gateway dropped the socket and we logged in again (a restarted gateway looks exactly like this). */
  reconnects = 0
  private closedByUs = false

  constructor(private readonly cfg: LiveStackConfig) {}

  /** POST /auth/local, then open /ws with the session cookie. Never logs the credentials. */
  async connect(): Promise<void> {
    const login = await fetch(`${this.cfg.gatewayUrl}/auth/local`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: this.cfg.adminEmail, password: this.cfg.adminPassword })
    })
    if (login.status !== 200) {
      throw new Error(`gateway login failed with HTTP ${login.status}`)
    }
    const cookie = login.headers
      .getSetCookie()
      .map((c) => c.split(';')[0])
      .join('; ')
    const wsUrl = `${this.cfg.gatewayUrl.replace(/^http/, 'ws')}/ws`
    await new Promise<void>((resolveOpen, rejectOpen) => {
      const ws = new WebSocket(wsUrl, { headers: { Cookie: cookie, Origin: this.cfg.frontendUrl } })
      ws.on('open', () => resolveOpen())
      ws.on('error', (e) => rejectOpen(e))
      ws.on('message', (data) => this.onData(String(data)))
      ws.on('close', () => {
        if (this.ws === ws) {
          this.ws = null
        }
        for (const p of this.pending.values()) {
          p.reject(new Error('CONNECTION_CLOSED: the gateway closed the WebSocket'))
        }
        this.pending.clear()
      })
      this.ws = ws
    })
  }

  close(): void {
    this.closedByUs = true
    this.ws?.close()
  }

  private get isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  /** The gateway may be restarting: keep trying to log in again for up to a minute. */
  private async reconnect(): Promise<void> {
    const deadline = Date.now() + 60_000
    for (;;) {
      try {
        await this.connect()
        this.reconnects++
        return
      } catch (e) {
        if (Date.now() > deadline) {
          throw e
        }
        await new Promise((r) => setTimeout(r, 2_000))
      }
    }
  }

  private onData(chunk: string): void {
    this.buffer += chunk
    let nl = this.buffer.indexOf('\n')
    while (nl >= 0) {
      const line = this.buffer.slice(0, nl).trim()
      this.buffer = this.buffer.slice(nl + 1)
      nl = this.buffer.indexOf('\n')
      if (!line) {
        continue
      }
      let msg: {
        id?: string
        ok?: boolean
        result?: unknown
        streaming?: boolean
        error?: { code?: string; message?: string }
      }
      try {
        msg = JSON.parse(line)
      } catch {
        continue
      }
      if (msg.id && msg.streaming) {
        this.streams.get(msg.id)?.({ receivedAt: Date.now(), result: msg.result })
        continue
      }
      const pending = msg.id ? this.pending.get(msg.id) : undefined
      if (!pending || !msg.id) {
        continue
      }
      this.pending.delete(msg.id)
      if (msg.ok) {
        pending.resolve(msg.result)
      } else {
        pending.reject(new GatewayRpcError(msg.error?.code ?? 'internal', msg.error?.message ?? ''))
      }
    }
  }

  private async callOnce<T>(
    method: string,
    params: Record<string, unknown>,
    timeoutMs: number
  ): Promise<T> {
    if (!this.isOpen && !this.closedByUs) {
      await this.reconnect()
    }
    const id = `live-e2e-${++this.nextId}`
    return await new Promise<T>((resolveCall, rejectCall) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        rejectCall(new Error(`no reply to ${method} within ${timeoutMs}ms`))
      }, timeoutMs)
      this.pending.set(id, {
        resolve: (v) => {
          clearTimeout(timer)
          resolveCall(v as T)
        },
        reject: (e) => {
          clearTimeout(timer)
          rejectCall(e)
        }
      })
      this.ws?.send(`${JSON.stringify({ id, authToken: 'cookie-auth', method, params })}\n`)
    })
  }

  /**
   * Opens a push subscription (e.g. `task.activity.subscribe`): resolves on the ack, then `onFrame` gets every pushed
   * frame until `close()` or the end of the socket. The spec uses it to see what the backend broadcasts while the UI shows
   * (or fails to show) the same thing.
   */
  async subscribe(
    method: string,
    params: Record<string, unknown>,
    onFrame: (frame: StreamFrame) => void
  ): Promise<{ close: () => void }> {
    if (!this.isOpen && !this.closedByUs) {
      await this.reconnect()
    }
    const id = `live-e2e-sub-${++this.nextId}`
    this.streams.set(id, onFrame)
    try {
      await new Promise<void>((resolveAck, rejectAck) => {
        const timer = setTimeout(
          () => rejectAck(new Error(`no ack to ${method} within 15000ms`)),
          15_000
        )
        this.pending.set(id, {
          resolve: () => {
            clearTimeout(timer)
            resolveAck()
          },
          reject: (e) => {
            clearTimeout(timer)
            rejectAck(e)
          }
        })
        this.ws?.send(`${JSON.stringify({ id, authToken: 'cookie-auth', method, params })}\n`)
      })
    } catch (e) {
      this.streams.delete(id)
      this.pending.delete(id)
      throw e
    }
    return { close: () => void this.streams.delete(id) }
  }

  /** A REQUEST_RATE_LIMITED reply is pacing, not failure: wait what the service asks and retry. */
  async call<T = RpcRow>(
    method: string,
    params: Record<string, unknown> = {},
    timeoutMs = 30_000
  ): Promise<T> {
    for (let attempt = 0; ; attempt++) {
      try {
        return await this.callOnce<T>(method, params, timeoutMs)
      } catch (e) {
        const limited = e instanceof GatewayRpcError && e.message.includes('REQUEST_RATE_LIMITED')
        const dropped = e instanceof Error && e.message.startsWith('CONNECTION_CLOSED')
        if (dropped && attempt < 3) {
          continue // reads and the idempotent writes this spec issues are safe to repeat after a reconnect
        }
        if (!limited || attempt >= 9) {
          throw e
        }
        this.rateLimitRetries++
        const hint = /"retryAfterMs":\s*(\d+)/.exec(e.message)
        await new Promise((r) => setTimeout(r, hint ? Number(hint[1]) + 50 : 600 + 200 * attempt))
      }
    }
  }
}
