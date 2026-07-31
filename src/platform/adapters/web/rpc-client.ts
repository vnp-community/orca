// Why: lightweight JSON-RPC WebSocket client for web mode — replaces Electron
// ipcRenderer with the same invoke/on/once surface web-preload-api depends on.
import type { IRpcClient } from '../../rpc-client-interface'

type PendingInvocation = {
  resolve: (result: unknown) => void
  reject: (error: Error) => void
  timeout: ReturnType<typeof setTimeout>
}

type PushHandlers = Set<(...args: unknown[]) => void>

const INVOKE_TIMEOUT_MS = 30_000

function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
}

function getDefaultWsUrl(): string {
  if (typeof window === 'undefined') return 'ws://localhost:6769/ws'
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host || 'localhost:6769'
  return `${protocol}//${host}/ws`
}

export class WebSocketRpcClient implements IRpcClient {
  private ws: WebSocket | null = null
  private connected = false
  private readonly url: string
  private readonly pending = new Map<string, PendingInvocation>()
  private readonly listeners = new Map<string, PushHandlers>()

  constructor(url?: string) {
    this.url = url ?? getDefaultWsUrl()
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(this.url)
      this.ws = ws

      ws.onopen = () => {
        this.connected = true
        resolve()
      }

      ws.onerror = () => {
        this.connected = false
        reject(new Error(`WebSocket connection failed: ${this.url}`))
      }

      ws.onclose = () => {
        this.connected = false
        this.rejectAllPending(new Error('WebSocket connection closed'))
      }

      ws.onmessage = (event) => {
        this.handleMessage(event.data as string)
      }
    })
  }

  disconnect(): void {
    this.connected = false
    if (this.ws) {
      this.ws.onopen = null
      this.ws.onerror = null
      this.ws.onclose = null
      this.ws.onmessage = null
      this.ws.close()
      this.ws = null
    }
    this.rejectAllPending(new Error('Client disconnected'))
  }

  isConnected(): boolean {
    // Why: use literal 1 (OPEN) — in test environments WebSocket.OPEN may be
    // undefined if WebSocket global is a plain stub function without statics.
    return this.connected && this.ws !== null && this.ws.readyState === 1
  }

  invoke(channel: string, ...args: unknown[]): Promise<unknown> {
    if (!this.isConnected()) {
      return Promise.reject(new Error('Not connected'))
    }
    return new Promise((resolve, reject) => {
      const id = generateId()
      const timeout = setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`invoke timeout: ${channel}`))
      }, INVOKE_TIMEOUT_MS)

      this.pending.set(id, { resolve, reject, timeout })
      this.ws!.send(JSON.stringify({ id, type: 'invoke', channel, args }))
    })
  }

  send(channel: string, data?: unknown): void {
    if (!this.isConnected()) return
    try {
      this.ws!.send(JSON.stringify({ type: 'send', channel, data }))
    } catch {
      // Why: fire-and-forget — silently swallow send errors on degraded connections
    }
  }

  on(channel: string, handler: (...args: unknown[]) => void): () => void {
    if (!this.listeners.has(channel)) {
      this.listeners.set(channel, new Set())
    }
    this.listeners.get(channel)!.add(handler)
    return () => this.off(channel, handler)
  }

  off(channel: string, handler: (...args: unknown[]) => void): void {
    this.listeners.get(channel)?.delete(handler)
  }

  once(channel: string, handler: (...args: unknown[]) => void): void {
    const wrapper = (...args: unknown[]): void => {
      this.off(channel, wrapper)
      handler(...args)
    }
    this.on(channel, wrapper)
  }

  private handleMessage(raw: string): void {
    let msg: Record<string, unknown>
    try {
      msg = JSON.parse(raw) as Record<string, unknown>
    } catch {
      return
    }

    if (msg.type === 'result' || msg.type === 'error') {
      const id = msg.id as string
      const pending = this.pending.get(id)
      if (!pending) return
      this.pending.delete(id)
      clearTimeout(pending.timeout)
      if (msg.type === 'result') {
        pending.resolve(msg.result)
      } else {
        pending.reject(new Error((msg.message as string) ?? 'RPC error'))
      }
      return
    }

    if (msg.type === 'push') {
      const channel = msg.channel as string
      const args = Array.isArray(msg.args) ? msg.args : []
      const handlers = this.listeners.get(channel)
      if (handlers) {
        for (const h of handlers) h(...args)
      }
    }
  }

  private rejectAllPending(error: Error): void {
    for (const [id, pending] of this.pending) {
      clearTimeout(pending.timeout)
      pending.reject(error)
      this.pending.delete(id)
    }
  }
}
