// src/relay/cdp-client.ts
// Minimal raw CDP-over-WebSocket client — split out of
// browser-screencast-handler.ts (its only consumer so far) since it's a
// genuinely generic primitive, not screencast-specific: CDP's plain-WS
// protocol is commands ({id, method, params}, correlated by id to a
// {id, result} or {id, error} response) plus events ({method, params}, no
// id). This is the same wire protocol
// backend/src/main/browser/cdp-ws-proxy.ts speaks from the proxying
// direction — this class just consumes the target endpoint directly,
// the same approach the OLD Electron desktop bridge's CdpWsProxy/
// AgentBrowserBridge used (backend/src/main/browser/agent-browser-bridge.ts's
// `--cdp <port>` flag).

import { WebSocket } from 'ws'

const CDP_CONNECT_TIMEOUT_MS = 10_000
const CDP_COMMAND_TIMEOUT_MS = 8_000

export class CdpClient {
  private nextId = 1
  private readonly pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>()
  private readonly eventHandlers = new Map<string, (params: Record<string, unknown>) => void>()
  private closeHandler: ((reason: string) => void) | null = null

  private constructor(private readonly ws: WebSocket) {
    ws.on('message', (data: Buffer | ArrayBuffer | Buffer[]) => this.handleMessage(data.toString()))
    ws.on('close', () => this.closeHandler?.('CDP connection closed'))
    ws.on('error', (err: Error) => this.closeHandler?.(err.message))
  }

  static connect(url: string): Promise<CdpClient> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(url)
      const timer = setTimeout(() => {
        ws.terminate()
        reject(new Error(`Timed out connecting to CDP endpoint ${url}`))
      }, CDP_CONNECT_TIMEOUT_MS)
      ws.once('open', () => {
        clearTimeout(timer)
        resolve(new CdpClient(ws))
      })
      ws.once('error', (err: Error) => {
        clearTimeout(timer)
        reject(err)
      })
    })
  }

  private handleMessage(raw: string): void {
    let msg: Record<string, unknown>
    try {
      msg = JSON.parse(raw) as Record<string, unknown>
    } catch {
      return
    }
    if (typeof msg.id === 'number') {
      const pending = this.pending.get(msg.id)
      if (!pending) {
        return
      }
      this.pending.delete(msg.id)
      if (msg.error) {
        const err = msg.error as { message?: string }
        pending.reject(new Error(err.message ?? 'CDP command failed'))
      } else {
        pending.resolve(msg.result)
      }
      return
    }
    if (typeof msg.method === 'string') {
      const handler = this.eventHandlers.get(msg.method)
      handler?.((msg.params as Record<string, unknown>) ?? {})
    }
  }

  send(method: string, params: Record<string, unknown> = {}): Promise<unknown> {
    const id = this.nextId++
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`Timed out waiting for CDP ${method}`))
      }, CDP_COMMAND_TIMEOUT_MS)
      this.pending.set(id, {
        resolve: (v) => {
          clearTimeout(timer)
          resolve(v)
        },
        reject: (e) => {
          clearTimeout(timer)
          reject(e)
        }
      })
      this.ws.send(JSON.stringify({ id, method, params }))
    })
  }

  on(method: string, handler: (params: Record<string, unknown>) => void): void {
    this.eventHandlers.set(method, handler)
  }

  onClose(handler: (reason: string) => void): void {
    this.closeHandler = handler
  }

  close(): void {
    this.closeHandler = null
    try {
      this.ws.close()
    } catch {
      // already closing/closed
    }
  }
}
