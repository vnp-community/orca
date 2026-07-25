import { EventEmitter } from 'node:events'
import type {
  IWindow,
  IWindowManager,
  WindowCreationOptions,
  WindowEvent
} from '../../window-interface'

/**
 * NodeWindow — IWindow implementation for Node.js server mode.
 *
 * Represents a "virtual window" — no actual GUI, but provides:
 * - Event emission (closed, focus, etc.)
 * - send() routing to WebSocket subscribers
 */
export class NodeWindow extends EventEmitter implements IWindow {
  readonly id: number
  private _destroyed = false
  private readonly _sendSubscribers = new Map<string, Set<(args: any[]) => void>>()

  constructor(id: number) {
    super()
    this.id = id
  }

  // ── State queries ──────────────────────────────────────────────────────────
  isDestroyed(): boolean {
    return this._destroyed
  }
  isMinimized(): boolean {
    return false
  }
  isMaximized(): boolean {
    return false
  }
  isFullScreen(): boolean {
    return false
  }
  isVisible(): boolean {
    return true
  }
  isFocused(): boolean {
    return true
  }

  // ── Actions ────────────────────────────────────────────────────────────────
  show(): void {}
  hide(): void {}
  focus(): void {}
  restore(): void {}

  close(): void {
    this.destroy()
  }

  destroy(): void {
    if (this._destroyed) return
    this._destroyed = true
    this._sendSubscribers.clear()
    this.emit('closed')
    this.removeAllListeners()
  }

  // ── Messaging ──────────────────────────────────────────────────────────────

  /**
   * Send a message to all subscribers on this channel.
   * In Node mode, this routes to WebSocket clients via WebIpcBridge.
   */
  send(channel: string, ...args: any[]): void {
    if (this._destroyed) return
    const subs = this._sendSubscribers.get(channel)
    if (subs) {
      for (const cb of subs) cb(args)
    }
  }

  /**
   * Subscribe to messages sent via send() on a specific channel.
   * Returns an unsubscribe function.
   * Used by WebIpcBridge to forward window messages to WebSocket clients.
   */
  onSend(channel: string, callback: (args: any[]) => void): () => void {
    let set = this._sendSubscribers.get(channel)
    if (!set) {
      set = new Set()
      this._sendSubscribers.set(channel, set)
    }
    set.add(callback)
    return () => set!.delete(callback)
  }

  // ── EventEmitter overrides for type compatibility ──────────────────────────
  on(event: WindowEvent, listener: (...args: any[]) => void): this {
    return super.on(event, listener)
  }
  once(event: WindowEvent, listener: (...args: any[]) => void): this {
    return super.once(event, listener)
  }
  off(event: WindowEvent, listener: (...args: any[]) => void): this {
    return super.off(event, listener)
  }
}

/**
 * NodeWindowManager — IWindowManager for Node.js server mode.
 *
 * Creates and tracks NodeWindow instances.
 * Each connected WebSocket client gets its own NodeWindow (virtual).
 */
export class NodeWindowManager implements IWindowManager {
  private readonly _windows = new Map<number, NodeWindow>()
  private _mainWindow: NodeWindow | null = null
  private _nextId = 1

  createWindow(_options: WindowCreationOptions = {}): NodeWindow {
    const win = new NodeWindow(this._nextId++)
    this._windows.set(win.id, win)
    win.once('closed', () => this._windows.delete(win.id))
    return win
  }

  getAllWindows(): IWindow[] {
    return [...this._windows.values()]
  }

  getFocusedWindow(): IWindow | null {
    return this._mainWindow
  }

  getMainWindow(): IWindow | null {
    return this._mainWindow
  }

  setMainWindow(window: IWindow | null): void {
    this._mainWindow = window as NodeWindow | null
  }

  /** Look up a window by id */
  getWindowById(id: number): NodeWindow | undefined {
    return this._windows.get(id)
  }
}
