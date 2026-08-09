/**
 * IIpcBridge — abstraction over Electron's ipcMain.
 *
 * NodeAdapter provides an in-process implementation that
 * dispatches via WebSocket (via WebIpcBridge in SOL-BE-003).
 */
export type IIpcBridge = {
  /** Register an async handler for a channel (like ipcMain.handle) */
  handle(channel: string, listener: IpcHandler): void

  /** Remove a registered handler */
  removeHandler(channel: string): void

  /** Subscribe to fire-and-forget events (like ipcMain.on) */
  on(channel: string, listener: IpcListener): this

  /** Unsubscribe */
  off(channel: string, listener: IpcListener): this

  /** Push a message to a specific window/client */
  sendToWindow(windowId: number, channel: string, ...args: any[]): void

  /** Broadcast to all connected windows/clients */
  sendToAll(channel: string, ...args: any[]): void
}

export type IpcHandler = (event: IpcEvent, ...args: any[]) => Promise<any> | any

export type IpcListener = (event: IpcEvent, ...args: any[]) => void

export type IpcEvent = {
  readonly sender: {
    readonly id: number
    send(channel: string, ...args: any[]): void
  }
}
