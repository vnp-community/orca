// ─── DevServer IPC Handlers ──────────────────────────────────────────────────
// Registers all devServer.* IPC channels following the Electron ipcMain pattern
// used throughout src/main/ipc/.

import { ipcMain, BrowserWindow } from 'electron'
import type { Store } from '../persistence'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type {
  DevServerInput,
  DevServerStatus,
  AgentTokenInfo
} from '../../shared/dev-server-types'

// IPC channel names for the devServer namespace
const DEV_SERVER_IPC_CHANNELS = [
  'devServer.list',
  'devServer.add',
  'devServer.remove',
  'devServer.testConnection',
  'devServer.connect',
  'devServer.disconnect',
  'devServer.get',
  'devServer.getPlatform',
  'settings.setActiveDevServer'
] as const

/**
 * Broadcast an event to all renderer windows.
 * Mirrors the pattern used in src/main/ipc/ssh.ts.
 */
function broadcastToAllWindows(channel: string, payload: unknown): void {
  for (const win of BrowserWindow.getAllWindows()) {
    if (!win.isDestroyed()) {
      win.webContents.send(channel, payload)
    }
  }
}

export function registerDevServerIpcHandlers(manager: DevServerManager, store?: Store): void {
  // Remove any existing handlers first (idempotent registration)
  for (const channel of DEV_SERVER_IPC_CHANNELS) {
    ipcMain.removeHandler(channel)
  }

  // List all dev servers
  ipcMain.handle('devServer.list', () => manager.list())

  // Add a new dev server
  ipcMain.handle('devServer.add', async (_event, input: DevServerInput) => {
    return manager.add(input)
  })

  // Remove a dev server
  ipcMain.handle('devServer.remove', async (_event, id: string) => {
    await manager.remove(id)
  })

  // Test connection (ephemeral — does not save)
  ipcMain.handle('devServer.testConnection', async (_event, input: DevServerInput) => {
    return manager.testConnection(input)
  })

  // Connect to a dev server and return its updated state
  ipcMain.handle('devServer.connect', async (_event, id: string) => {
    await manager.connect(id)
    return manager.get(id)
  })

  // Disconnect from a dev server
  ipcMain.handle('devServer.disconnect', async (_event, id: string) => {
    await manager.disconnect(id)
  })

  // Get a single dev server by id
  ipcMain.handle('devServer.get', (_event, id: string) => manager.get(id))

  // Get the platform of a connected dev server
  // Why: renderer needs platform to conditionally render OS-specific UI
  // without requiring a full getPreflightStatus call (TASK-017).
  ipcMain.handle(
    'devServer.getPlatform',
    (_event, devServerId: string): NodeJS.Platform | null =>
      manager.get(devServerId)?.platform ?? null
  )

  // Set the active dev server (persists to GlobalSettings)
  // Why: "active" is a session-level concept stored in settings, not in
  // DevServerStore, so the user's last-used server survives app restarts.
  ipcMain.handle(
    'settings.setActiveDevServer',
    async (_event, devServerId: string | null): Promise<void> => {
      if (store) {
        store.updateSettings({ activeDevServerId: devServerId ?? null }, { notifyListeners: true })
      }
      manager.emit('activeDevServerChanged', devServerId)
    }
  )

  // ── Push events to renderer ───────────────────────────────────────

  manager.on('devServer:statusChanged', (id: string, status: DevServerStatus) => {
    broadcastToAllWindows('devServer:statusChanged', { id, status })
  })

  manager.on('devServer:added', (id: string) => {
    broadcastToAllWindows('devServer:added', { id })
  })

  manager.on('devServer:removed', (id: string) => {
    broadcastToAllWindows('devServer:removed', { id })
  })

  // Forward agentTokenGenerated from direct-websocket bridge to renderer.
  // Why: when direct-websocket mode runs, the bridge generates a one-time
  // agent token that the user must copy to start their agent. The renderer
  // displays it in AgentTokenPanel via window.api.devServer.onAgentToken().
  manager.on('devServer:agentToken', (info: AgentTokenInfo) => {
    broadcastToAllWindows('devServer:agentToken', info)
  })
}
