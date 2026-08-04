import { ipcMain } from 'electron'
import type { Store } from '../persistence'
import type { WorkspaceSessionPatch, WorkspaceSessionState } from '../../shared/types'
import { Tracers } from '../../shared/trace/tracers'

export function registerSessionHandlers(store: Store): void {
  // Why: hostId is an optional second arg so an older renderer that invokes
  // these channels without it keeps reading/writing the 'local' partition
  // exactly as before. Channel names stay stable.
  ipcMain.handle('session:get', (_event, hostId?: string | null) => {
    return store.getWorkspaceSession(hostId)
  })

  ipcMain.handle('session:set', (_event, args: WorkspaceSessionState, hostId?: string | null) => {
    store.setWorkspaceSession(args, hostId)
  })

  ipcMain.handle('session:patch', (_event, args: WorkspaceSessionPatch, hostId?: string | null) => {
    store.patchWorkspaceSession(args, hostId)
  })

  // Synchronous variant for the renderer's beforeunload handler.
  // sendSync blocks the renderer until this returns, guaranteeing the
  // data (including terminal scrollback buffers) is persisted to disk
  // before the window closes — regardless of before-quit ordering.
  ipcMain.on('session:set-sync', (event, args: WorkspaceSessionState, hostId?: string | null) => {
    store.setWorkspaceSession(args, hostId)
    store.flush()
    event.returnValue = true
  })

  ipcMain.on(
    'session:read-terminal-scrollback-sync',
    (event, args: { ref?: unknown } | undefined) => {
      const ref = typeof args?.ref === 'string' ? args.ref : null
      if (!ref) {
        event.returnValue = null
        return
      }
      // Why: this is an in-process Electron IPC round-trip (renderer ↔ main,
      // same process) — not one of CR-TRACE-000 §3.3's cross-boundary
      // transports, so no traceId wire field is threaded through. Reuses
      // Tracers.terminalReattach (flow terminal:reattach) — the closest
      // existing "reconnect to a terminal's prior state" tracer already
      // registered by the agent-domain pty-agent-bridge.ts work; no
      // terminal:reconnect tracer exists (additive-only rule: don't mint a
      // near-duplicate entry for the same concept).
      const span = Tracers.terminalReattach.start({ ref })
      try {
        const buffer = store.readTerminalScrollbackSnapshot(ref)
        span.ok({ ref, restoredBytes: buffer?.length ?? 0 })
        event.returnValue = buffer
      } catch (err) {
        span.fail(err, { ref })
        event.returnValue = null
      }
    }
  )
}
