import { randomUUID } from 'node:crypto'
import type { DevServer, DevServerInput, ConnectionTestResult } from '../../shared/dev-server-types'

type IpcRequestMessage = {
  type: 'devServer:proxyRequest'
  requestId: string
  method: string
  args: any[]
}

type IpcResponseMessage = {
  type: 'devServer:proxyResponse'
  requestId: string
  error?: string
  result?: any
}

type IpcNotificationMessage = {
  type: 'devServer:proxyNotification'
  devServerId: string
  method: string
  params: Record<string, unknown>
}

/**
 * Proxy for DevServerManager to be used inside the User Process.
 * Routes all method calls over IPC to the Main Process.
 */
export class GatewayDevServerManagerProxy {
  private pendingRequests = new Map<string, { resolve: (val: any) => void; reject: (err: Error) => void }>()
  private listeners = new Map<string, Set<Function>>()
  private notificationHandlers = new Map<
    string,
    Set<(method: string, params: Record<string, unknown>) => void>
  >()

  constructor() {
    process.on('message', (message: any) => {
      if (message && message.type === 'devServer:proxyResponse') {
        this.handleResponse(message as IpcResponseMessage)
      } else if (message && message.type === 'devServer:event') {
        this.emitLocal(message.event, ...message.args)
      } else if (message && message.type === 'devServer:proxyNotification') {
        this.handleNotification(message as IpcNotificationMessage)
      }
    })
  }

  private handleNotification(message: IpcNotificationMessage): void {
    const handlers = this.notificationHandlers.get(message.devServerId)
    if (!handlers) {return}
    for (const handler of handlers) {
      handler(message.method, message.params)
    }
  }

  private handleResponse(message: IpcResponseMessage) {
    const pending = this.pendingRequests.get(message.requestId)
    if (!pending) {return}

    this.pendingRequests.delete(message.requestId)
    if (message.error) {
      pending.reject(new Error(message.error))
    } else {
      pending.resolve(message.result)
    }
  }

  private sendRequest<T>(method: string, ...args: any[]): Promise<T> {
    return new Promise((resolve, reject) => {
      if (!process.send) {
        return reject(new Error('IPC channel not available'))
      }

      const requestId = randomUUID()
      this.pendingRequests.set(requestId, { resolve, reject })

      const msg: IpcRequestMessage = {
        type: 'devServer:proxyRequest',
        requestId,
        method,
        args
      }
      process.send(msg)

      // Timeout just in case
      setTimeout(() => {
        if (this.pendingRequests.has(requestId)) {
          this.pendingRequests.delete(requestId)
          reject(new Error(`IPC request timeout for method: ${method}`))
        }
      }, 30000)
    })
  }

  async list(): Promise<DevServer[]> {
    return this.sendRequest('list')
  }

  async add(input: DevServerInput): Promise<DevServer> {
    return this.sendRequest('add', input)
  }

  async remove(id: string): Promise<void> {
    return this.sendRequest('remove', id)
  }

  get(_id: string): DevServer | null {
    // Synchronous get is trickier via IPC, but typically 'list' or 'connect' are what matter.
    // For now we'll throw, or return a stub. The RPC layer uses `list` and `connect`.
    // getRelay uses getRelay, which we must mock below.
    throw new Error('Synchronous get() not supported in User Process')
  }

  async connect(id: string): Promise<DevServer> {
    return this.sendRequest('connect', id)
  }

  async disconnect(id: string): Promise<void> {
    return this.sendRequest('disconnect', id)
  }

  async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
    return this.sendRequest('testConnection', input)
  }

  async generateAgentToken(id: string): Promise<{ token: string }> {
    return this.sendRequest('generateAgentToken', id)
  }

  getRelay(id: string): any {
    // Returns a mock relay object that forwards `call` over IPC, and dispatches
    // `onNotification` from devServer:proxyNotification broadcasts (see
    // handleNotification above) — the Main process holds the actual relay/mux
    // connected to the dev server; this process only sees what gets forwarded.
    return {
      call: async <T>(method: string, params: any, timeoutMs?: number): Promise<T> => {
        return this.sendRequest('relayCall', id, method, params, timeoutMs)
      },
      onNotification: (handler: (method: string, params: Record<string, unknown>) => void): (() => void) => {
        let handlers = this.notificationHandlers.get(id)
        if (!handlers) {
          handlers = new Set()
          this.notificationHandlers.set(id, handlers)
        }
        handlers.add(handler)
        return () => handlers!.delete(handler)
      },
    }
  }

  on(event: string, listener: Function): this {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set())
    }
    this.listeners.get(event)!.add(listener)
    return this
  }

  off(event: string, listener: Function): this {
    const set = this.listeners.get(event)
    if (set) {
      set.delete(listener)
    }
    return this
  }

  emit(event: string, ...args: any[]): boolean {
    // emit from User Process is likely an error or we just broadcast locally.
    this.emitLocal(event, ...args)
    return true
  }

  private emitLocal(event: string, ...args: any[]): void {
    const set = this.listeners.get(event)
    if (set) {
      for (const listener of set) {
        listener(...args)
      }
    }
  }
}
