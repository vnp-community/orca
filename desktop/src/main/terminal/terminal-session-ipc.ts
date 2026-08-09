/**
 * Terminal Session IPC Handlers
 *
 * Main-process IPC handlers for terminal session snapshot persistence (TM-003).
 * Exposed to renderer via preload `window.api.terminalSessions.*`.
 *
 * @module main/terminal/terminal-session-ipc
 */

import { ipcMain } from 'electron'
import type { TerminalSessionService, TerminalSessionKey, UpsertSnapshotInput } from './terminal-session-service'

export function registerTerminalSessionHandlers(service: TerminalSessionService): void {

  // ── terminal.session.save ────────────────────────────────────────────────────
  // Called by renderer just before terminal disconnect / tab close.
  ipcMain.handle(
    'terminal.session.save',
    async (_event, input: UpsertSnapshotInput) => {
      return service.saveSnapshot(input)
    }
  )

  // ── terminal.session.get ─────────────────────────────────────────────────────
  // Called by renderer on reconnect to check if a snapshot exists for restoration.
  ipcMain.handle(
    'terminal.session.get',
    async (_event, key: TerminalSessionKey) => {
      return service.getSnapshot(key)
    }
  )

  // ── terminal.session.list ────────────────────────────────────────────────────
  // Returns all active snapshots for a worktree (used by session restore UI).
  ipcMain.handle(
    'terminal.session.list',
    async (_event, worktreeId: string) => {
      return service.listByWorktree(worktreeId)
    }
  )

  // ── terminal.session.archive ─────────────────────────────────────────────────
  // Called when a terminal is intentionally closed / session no longer needed.
  ipcMain.handle(
    'terminal.session.archive',
    async (_event, key: TerminalSessionKey) => {
      await service.archiveSnapshot(key)
      return { ok: true }
    }
  )
}

export function unregisterTerminalSessionHandlers(): void {
  ipcMain.removeAllListeners('terminal.session.save')
  ipcMain.removeAllListeners('terminal.session.get')
  ipcMain.removeAllListeners('terminal.session.list')
  ipcMain.removeAllListeners('terminal.session.archive')
}
