import type { NodeIpcBridge } from './ipc'
import type { IpcEvent } from '../../ipc-interface'

export type BroadcastFn = (msg: string) => void

/**
 * WebIpcBridge — server-side handler for WebSocket IPC messages.
 *
 * Each WebSocket connection (representing one browser tab/client)
 * sends messages that are routed through this bridge into NodeIpcBridge.
 *
 * Protocol:
 *   Client → Server (invoke):  { id, type: 'invoke', channel, args }
 *   Server → Client (result):  { id, type: 'result', result }
 *   Server → Client (error):   { id, type: 'error', message }
 *   Client → Server (send):    { type: 'send', channel, args }
 *   Server → Client (push):    { type: 'push', channel, args }
 */
export class WebIpcBridge {
  constructor(private readonly ipc: NodeIpcBridge) {}

  /**
   * Handle an incoming WebSocket message from a client.
   *
   * @param data - Raw message string (expected JSON)
   * @param windowId - ID of the NodeWindow/connection this message came from
   * @param reply - Function to send response back to this specific client
   */
  async handleWebSocketMessage(
    data: string,
    windowId: number,
    reply: BroadcastFn
  ): Promise<void> {
    let msg: any
    try {
      msg = JSON.parse(data)
    } catch {
      reply(JSON.stringify({ type: 'error', message: 'Invalid JSON' }))
      return
    }

    if (msg.type === 'invoke') {
      const args: unknown[] = Array.isArray(msg.args) ? msg.args : []
      try {
        const result = await this.ipc.invoke(msg.channel, windowId, ...args)
        reply(JSON.stringify({ id: msg.id, type: 'result', result }))
      } catch (err: any) {
        reply(
          JSON.stringify({
            id: msg.id,
            type: 'error',
            message: err?.message ?? String(err)
          })
        )
      }
    } else if (msg.type === 'send') {
      // Fire-and-forget: route to ipc.emit() without expecting a reply
      const args: unknown[] = Array.isArray(msg.args) ? msg.args : []
      const event: IpcEvent = {
        sender: {
          id: windowId,
          send: (_ch: string, ..._args: any[]) => {} // intentional no-op for send events
        }
      }
      this.ipc.emit(msg.channel, event, ...args)
      // No reply for fire-and-forget
    }
    // Unknown types are silently ignored
  }

  /**
   * Push a server-side event to connected clients.
   * Called when backend code wants to notify the web frontend.
   *
   * @param channel - Event channel name
   * @param args - Event payload args
   * @param broadcast - Function to send to the target client(s)
   */
  pushToClients(channel: string, args: any[], broadcast: BroadcastFn): void {
    broadcast(JSON.stringify({ type: 'push', channel, args }))
  }
}
