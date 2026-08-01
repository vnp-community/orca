// src/renderer/src/hooks/use-terminal-session.ts
// BUG-FE-TM-003-D: Hook for terminal session snapshot save/restore
// Called from terminal-pane lifecycle to persist state on disconnect
// and restore on reconnect.

import { useCallback, useRef } from 'react'
import type { PtyConnectResult } from '../components/terminal-pane/pty-transport-types'

interface UseTerminalSessionOptions {
  worktreeId: string
  tabId: string
  leafId?: string
  runtimeEnvId?: string
}

interface TerminalSessionSnapshot {
  snapshotData: string | null
  snapshotCols: number
  snapshotRows: number
  remoteHandle: string | null
}

/**
 * Provides save/restore helpers for terminal scrollback snapshot persistence.
 *
 * Usage:
 * ```ts
 * const { saveSnapshot, restoreSnapshot, archiveSession } = useTerminalSession({
 *   worktreeId, tabId, leafId, runtimeEnvId,
 * })
 *
 * // On disconnect: serialize xterm and save
 * await saveSnapshot({ snapshotData, snapshotCols, snapshotRows, remoteHandle })
 *
 * // On reconnect: check for snapshot
 * const snap = await restoreSnapshot()
 * if (snap?.snapshotData) { terminal.write(atob(snap.snapshotData)) }
 *
 * // On intentional close: archive (soft delete)
 * await archiveSession()
 * ```
 */
export function useTerminalSession({
  worktreeId,
  tabId,
  leafId = '',
  runtimeEnvId = '',
}: UseTerminalSessionOptions) {
  // Track last saved snapshot in-memory to avoid redundant IPC calls
  const lastSnapshotRef = useRef<string | null>(null)

  /** Save serialized xterm scrollback to DB via IPC. */
  const saveSnapshot = useCallback(async (opts: {
    snapshotData: string
    snapshotCols: number
    snapshotRows: number
    remoteHandle?: string
  }): Promise<void> => {
    // Skip if snapshot hasn't changed (avoid DB writes on every heartbeat)
    if (opts.snapshotData === lastSnapshotRef.current) return
    lastSnapshotRef.current = opts.snapshotData

    try {
      await window.api.terminalSessions.save({
        worktreeId,
        tabId,
        leafId:        leafId    || undefined,
        runtimeEnvId:  runtimeEnvId || undefined,
        snapshotData:  opts.snapshotData,
        snapshotCols:  opts.snapshotCols,
        snapshotRows:  opts.snapshotRows,
        remoteHandle:  opts.remoteHandle,
      })
    } catch {
      // Non-fatal — snapshot save failure should not break terminal
    }
  }, [worktreeId, tabId, leafId, runtimeEnvId])

  /** Retrieve the latest snapshot for this session. Returns null if none. */
  const restoreSnapshot = useCallback(async (): Promise<TerminalSessionSnapshot | null> => {
    try {
      const snap = await window.api.terminalSessions.get({
        worktreeId,
        tabId,
        leafId:       leafId       || undefined,
        runtimeEnvId: runtimeEnvId || undefined,
      })
      return snap ?? null
    } catch {
      return null
    }
  }, [worktreeId, tabId, leafId, runtimeEnvId])

  /** Mark session as archived (soft delete) when terminal is intentionally closed. */
  const archiveSession = useCallback(async (): Promise<void> => {
    lastSnapshotRef.current = null
    try {
      await window.api.terminalSessions.archive({
        worktreeId,
        tabId,
        leafId:       leafId       || undefined,
        runtimeEnvId: runtimeEnvId || undefined,
      })
    } catch {
      // Non-fatal
    }
  }, [worktreeId, tabId, leafId, runtimeEnvId])

  return { saveSnapshot, restoreSnapshot, archiveSession }
}
