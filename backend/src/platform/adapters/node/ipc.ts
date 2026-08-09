import { EventEmitter } from 'node:events'
import type { IIpcBridge, IpcHandler, IpcListener, IpcEvent } from '../../ipc-interface'
import type { IWindowManager } from '../../window-interface'

/**
 * NodeIpcBridge — IIpcBridge implementation for Node.js server mode.
 *
 * Replaces Electron's ipcMain in the server context:
 * - handle(channel, handler): registers async IPC handler
 * - invoke(channel, windowId, ...args): dispatches a call (used by WebIpcBridge)
 * - on/off/emit: fire-and-forget events
 * - sendToWindow/sendToAll: push events to connected clients
 *
 * When WebIpcBridge receives a WebSocket message, it calls invoke() here.
 * When backend code calls sendToWindow(), it routes to NodeWindow.send()
 * which then propagates to WebSocket clients.
 */
export class NodeIpcBridge extends EventEmitter implements IIpcBridge {
  private readonly _handlers = new Map<string, IpcHandler>()
  private readonly _listeners = new Map<string, Set<IpcListener>>()
  private readonly _windowManager: IWindowManager

  constructor(windowManager: IWindowManager) {
    super()
    this._windowManager = windowManager
  }

  // ── Handler registration ───────────────────────────────────────────────────

  handle(channel: string, listener: IpcHandler): void {
    if (this._handlers.has(channel)) {
      console.warn(
        `[NodeIpcBridge] Overwriting existing handler for channel: "${channel}"`
      )
    }
    this._handlers.set(channel, listener)
  }

  removeHandler(channel: string): void {
    this._handlers.delete(channel)
  }

  // ── Event subscription ─────────────────────────────────────────────────────

  on(channel: string, listener: IpcListener): this {
    let set = this._listeners.get(channel)
    if (!set) {
      set = new Set()
      this._listeners.set(channel, set)
    }
    set.add(listener)
    return this
  }

  off(channel: string, listener: IpcListener): this {
    this._listeners.get(channel)?.delete(listener)
    return this
  }

  // ── Dispatch (called by WebIpcBridge) ──────────────────────────────────────

  /**
   * Invoke a registered handler.
   * Called by WebIpcBridge when a WebSocket 'invoke' message arrives.
   *
   * @param channel - The IPC channel name
   * @param windowId - The ID of the window/connection that sent the request
   * @param args - Arguments to pass to the handler
   */
  async invoke(channel: string, windowId: number, ...args: any[]): Promise<any> {
    const handler = this._handlers.get(channel)
    if (!handler) {
      throw new Error(
        `[NodeIpcBridge] No IPC handler registered for channel: "${channel}"`
      )
    }

    const event: IpcEvent = {
      sender: {
        id: windowId,
        send: (replyChannel: string, ...replyArgs: any[]) => {
          this.sendToWindow(windowId, replyChannel, ...replyArgs)
        }
      }
    }

    return handler(event, ...args)
  }

  /**
   * Emit a fire-and-forget event.
   * Notifies all listeners registered via on().
   * Called by WebIpcBridge when a WebSocket 'send' message arrives.
   */
  emit(channel: string, event: IpcEvent | string, ...args: any[]): boolean {
    if (typeof event === 'string') {
      // EventEmitter.emit() called with string — delegate to super
      return super.emit(channel, event, ...args)
    }

    const set = this._listeners.get(channel)
    if (set) {
      for (const listener of set) {
        try {
          listener(event, ...args)
        } catch (err) {
          console.error(`[NodeIpcBridge] Listener error on "${channel}":`, err)
        }
      }
    }
    return super.emit(channel, event, ...args)
  }

  // ── Push notifications ─────────────────────────────────────────────────────

  sendToWindow(windowId: number, channel: string, ...args: any[]): void {
    const windows = this._windowManager.getAllWindows()
    const win = windows.find((w) => w.id === windowId)
    if (win) {
      win.send(channel, ...args)
    }
  }

  sendToAll(channel: string, ...args: any[]): void {
    for (const win of this._windowManager.getAllWindows()) {
      win.send(channel, ...args)
    }
  }
}
