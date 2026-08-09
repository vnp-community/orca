import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'
import { isKeepaliveFrame } from '../../../shared/runtime-rpc-envelope'

type WebRuntimeConnectionState =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'auth-failed'

type PendingRequest = {
  method: string
  resolve: (response: RuntimeRpcResponse<unknown>) => void
  reject: (error: Error) => void
  timeout: number
}

type SubscriptionCallbacks = {
  onResponse: (response: RuntimeRpcResponse<unknown>) => void
  onBinary?: (bytes: Uint8Array<ArrayBufferLike>) => void
  onError?: (error: { code: string; message: string }) => void
  onClose?: () => void
}

type RuntimeSubscription = {
  method: string
  params: unknown
  callbacks: SubscriptionCallbacks
}

export type WebRuntimeSubscriptionHandle = {
  unsubscribe: () => void
  sendBinary: (bytes: Uint8Array<ArrayBufferLike>) => void
}

export type SubscribeOptions = {
  timeoutMs?: number
  buildUnsubscribe?: (params: unknown) => { method: string; params: unknown } | null
}

const REQUEST_TIMEOUT_MS = 30_000
const CONNECT_TIMEOUT_MS = 12_000
const RECONNECT_DELAYS_MS = [500, 1000, 2000, 4000, 8000, 15_000]

export class WebSessionClient {
  private ws: WebSocket | null = null
  private state: WebRuntimeConnectionState = 'disconnected'
  private requestCounter = 0
  private reconnectAttempt = 0
  private intentionallyClosed = false
  private connectTimer: number | null = null
  private reconnectTimer: number | null = null
  private readonly pending = new Map<string, PendingRequest>()
  private readonly subscriptions = new Map<string, RuntimeSubscription>()
  private readonly waiters: { resolve: () => void; reject: (error: Error) => void }[] = []

  constructor(private readonly endpoint: string) {
    this.openConnection()
  }

  async call(
    method: string,
    params?: unknown,
    options?: { timeoutMs?: number }
  ): Promise<RuntimeRpcResponse<unknown>> {
    await this.waitForConnected(options?.timeoutMs)
    return new Promise((resolve, reject) => {
      const id = this.nextId()
      const timeoutMs = options?.timeoutMs ?? REQUEST_TIMEOUT_MS
      const timeout = window.setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`Request timed out: ${method}`))
      }, timeoutMs)
      this.pending.set(id, { method, resolve, reject, timeout })
      if (!this.send({ id, authToken: 'cookie-auth', method, params })) {
        this.pending.delete(id)
        window.clearTimeout(timeout)
        reject(new Error('Remote Orca runtime is not connected.'))
      }
    })
  }

  async subscribe(
    method: string,
    params: unknown,
    callbacks: SubscriptionCallbacks,
    options?: SubscribeOptions
  ): Promise<WebRuntimeSubscriptionHandle> {
    await this.waitForConnected(options?.timeoutMs)
    const id = this.nextId()
    this.subscriptions.set(id, { method, params, callbacks })
    if (!this.send({ id, authToken: 'cookie-auth', method, params })) {
      this.subscriptions.delete(id)
      throw new Error('Remote Orca runtime is not connected.')
    }
    return {
      unsubscribe: () => {
        this.subscriptions.delete(id)
        const teardown = options?.buildUnsubscribe?.(params)
        if (teardown) {
          this.send({
            id: this.nextId(),
            authToken: 'cookie-auth',
            method: teardown.method,
            params: teardown.params
          })
        }
      },
      sendBinary: (bytes) => {
        throw new Error('Binary frames not supported in session mode over this channel')
      }
    }
  }

  close(options: { notifySubscriptions?: boolean } = {}): void {
    const shouldNotifySubscriptions = options.notifySubscriptions ?? true
    this.intentionallyClosed = true
    this.clearTimers()
    this.rejectAllPending('Remote Orca runtime connection closed.')
    this.rejectAllWaiters(new Error('Remote Orca runtime connection closed.'))
    if (shouldNotifySubscriptions) {
      this.notifySubscriptionsClosed()
    } else {
      this.subscriptions.clear()
    }
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.setState('disconnected')
  }

  private openConnection(): void {
    if (this.intentionallyClosed) {
      return
    }
    let ws: WebSocket
    try {
      ws = new WebSocket(this.endpoint)
    } catch (error) {
      this.rejectAllPending(error instanceof Error ? error.message : String(error))
      this.scheduleReconnect()
      return
    }

    this.ws = ws
    this.setState('connecting')

    this.connectTimer = window.setTimeout(() => {
      if (this.ws === ws && ws.readyState === WebSocket.CONNECTING) {
        ws.close()
        this.handleSocketClosed(ws)
      }
    }, CONNECT_TIMEOUT_MS)

    ws.onopen = () => {
      if (this.ws !== ws) {
        return
      }
      this.clearConnectTimer()
      this.reconnectAttempt = 0
      this.setState('connected')
    }

    ws.onmessage = (event) => {
      if (this.ws !== ws) {
        return
      }
      void this.handleSocketMessage(event.data)
    }

    ws.onclose = (event) => this.handleSocketClosed(ws, event)
    ws.onerror = () => {
      if (this.state === 'connecting') {
        this.rejectAllWaiters(new Error('Could not connect to the remote Orca runtime.'))
      }
    }
  }

  private handleSocketMessage(rawData: unknown): void {
    if (this.state !== 'connected') {
      return
    }

    if (typeof rawData !== 'string') {
      return
    }

    let response: RuntimeRpcResponse<unknown> | Record<string, unknown>
    try {
      response = JSON.parse(rawData) as RuntimeRpcResponse<unknown> | Record<string, unknown>
    } catch {
      return
    }
    if (isKeepaliveFrame(response)) {
      return
    }
    if (!('id' in response) || typeof response.id !== 'string') {
      return
    }
    if (isRuntimeFailureResponse(response) && response.error.code === 'unauthorized') {
      this.intentionallyClosed = true
      this.setState('auth-failed')
      if (typeof window !== 'undefined') {window.dispatchEvent(new CustomEvent('orca:auth-failed'))}
      this.rejectAllPending('Unauthorized. Session cookie may have expired.')
      this.notifySubscriptionsError('unauthorized', 'Unauthorized. Session cookie may have expired.')
      this.ws?.close()
      return
    }

    const subscription = this.subscriptions.get(response.id)
    if (subscription && isSubscriptionResponse(response)) {
      subscription.callbacks.onResponse(response)
      if (response.ok && isEndResult(response.result)) {
        this.subscriptions.delete(response.id)
        subscription.callbacks.onClose?.()
      }
      return
    }

    const pending = this.pending.get(response.id)
    if (!pending) {
      return
    }
    this.pending.delete(response.id)
    window.clearTimeout(pending.timeout)
    pending.resolve(response as RuntimeRpcResponse<unknown>)
  }

  private send(message: unknown): boolean {
    const ws = this.ws
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      return false
    }
    ws.send(`${JSON.stringify(message)  }\n`)
    return true
  }

  private waitForConnected(timeoutMs = REQUEST_TIMEOUT_MS): Promise<void> {
    if (this.state === 'connected') {
      return Promise.resolve()
    }
    if (this.state === 'auth-failed') {
      return Promise.reject(new Error('Unauthorized. Session cookie may have expired.'))
    }
    if (this.intentionallyClosed) {
      return Promise.reject(new Error('Remote Orca runtime connection closed.'))
    }
    return new Promise((resolve, reject) => {
      const timeout = window.setTimeout(() => {
        const index = this.waiters.findIndex((waiter) => waiter.resolve === resolve)
        if (index !== -1) {
          this.waiters.splice(index, 1)
        }
        reject(new Error('Timed out while connecting to the remote Orca runtime.'))
      }, timeoutMs)
      this.waiters.push({
        resolve: () => {
          window.clearTimeout(timeout)
          resolve()
        },
        reject: (error) => {
          window.clearTimeout(timeout)
          reject(error)
        }
      })
    })
  }

  private handleSocketClosed(closedWs: WebSocket, event?: CloseEvent): void {
    if (this.ws !== closedWs) {
      return
    }
    this.ws = null
    this.clearConnectTimer()
    this.clearKeepaliveTimer()

    // Handle session expiry or unauthorized disconnect from backend
    // FIX TASK-TRM-007: Also handle code 4401 (WsSessionRouter sends this for missing/expired session).
    // 1008: WebSocket protocol "Policy Violation"
    // 3000: Legacy Orca session expired
    // 4401: WsSessionRouter unauthenticated (no valid session cookie)
    if (event && (event.code === 1008 || event.code === 3000 || event.code === 4401)) {
      this.intentionallyClosed = true
      this.setState('auth-failed')
      if (typeof window !== 'undefined') {window.dispatchEvent(new CustomEvent('orca:auth-failed'))}
      this.rejectAllPending('Unauthorized. Session cookie may have expired.')
      this.notifySubscriptionsError('unauthorized', 'Unauthorized. Session cookie may have expired.')
      return
    }

    this.rejectAllPending('Remote Orca runtime connection interrupted.')
    this.notifySubscriptionsClosed()
    if (this.intentionallyClosed || this.state === 'auth-failed') {
      this.setState(this.state === 'auth-failed' ? 'auth-failed' : 'disconnected')
      return
    }
    this.setState('disconnected')
    this.scheduleReconnect()
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer || this.intentionallyClosed) {
      return
    }
    const delay = RECONNECT_DELAYS_MS[Math.min(this.reconnectAttempt, RECONNECT_DELAYS_MS.length - 1)]
    this.reconnectAttempt += 1
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null
      this.openConnection()
    }, delay)
  }

  private setState(next: WebRuntimeConnectionState): void {
    this.state = next
    if (next === 'connected') {
      for (const waiter of this.waiters.splice(0)) {
        waiter.resolve()
      }
    } else if (next === 'auth-failed') {
      this.rejectAllWaiters(new Error('Unauthorized. Session cookie may have expired.'))
    }
  }

  private nextId(): string {
    this.requestCounter += 1
    return `web-session-rpc-${this.requestCounter}-${Date.now()}`
  }

  private rejectAllPending(reason: string): void {
    const error = new Error(reason)
    for (const [id, pending] of this.pending) {
      this.pending.delete(id)
      window.clearTimeout(pending.timeout)
      pending.reject(error)
    }
  }

  private rejectAllWaiters(error: Error): void {
    for (const waiter of this.waiters.splice(0)) {
      waiter.reject(error)
    }
  }

  private notifySubscriptionsClosed(): void {
    const subscriptions = Array.from(this.subscriptions.values())
    this.subscriptions.clear()
    for (const subscription of subscriptions) {
      subscription.callbacks.onClose?.()
    }
  }

  private notifySubscriptionsError(code: string, message: string): void {
    const subscriptions = Array.from(this.subscriptions.values())
    this.subscriptions.clear()
    for (const subscription of subscriptions) {
      subscription.callbacks.onError?.({ code, message })
    }
  }

  private clearTimers(): void {
    this.clearConnectTimer()
    if (this.reconnectTimer) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  private clearConnectTimer(): void {
    if (this.connectTimer) {
      window.clearTimeout(this.connectTimer)
      this.connectTimer = null
    }
  }

  // Why: handleSocketClosed calls this to cancel any pending keepalive/heartbeat
  // timer. WebSessionClient uses a simpler connection model without a dedicated
  // keepalive loop (cookie-auth sessions rely on the server-side session TTL
  // instead), so this is a no-op stub kept for symmetry with WebRuntimeClient.
  private clearKeepaliveTimer(): void {
    // no-op: this client has no keepalive timer
  }
}

function isSubscriptionResponse(
  response: RuntimeRpcResponse<unknown> | Record<string, unknown>
): response is RuntimeRpcResponse<unknown> {
  if (!('ok' in response)) {
    return false
  }
  if (response.ok === false) {
    return true
  }
  const success = response as RuntimeRpcResponse<unknown> & { ok: true; streaming?: true }
  return (
    success.streaming === true || isEndResult(success.result) || isScrollbackResult(success.result)
  )
}

function isRuntimeFailureResponse(
  response: RuntimeRpcResponse<unknown> | Record<string, unknown>
): response is RuntimeRpcResponse<unknown> & { ok: false } {
  return (
    'ok' in response &&
    response.ok === false &&
    'error' in response &&
    !!response.error &&
    typeof response.error === 'object' &&
    'code' in response.error
  )
}

function isEndResult(result: unknown): boolean {
  return (
    result !== null &&
    typeof result === 'object' &&
    'type' in result &&
    (result as { type: string }).type === 'end'
  )
}

function isScrollbackResult(result: unknown): boolean {
  return (
    result !== null &&
    typeof result === 'object' &&
    'type' in result &&
    (result as { type: string }).type === 'scrollback'
  )
}
