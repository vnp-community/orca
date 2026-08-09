// src/main/ipc/agent-orchestration.ts
// BUG-FE-ORCH-001: Main-process IPC handlers for remote agent orchestration
// Handles start/stop/resume of agents on remote runtime environments via
// ipcMain.handle, and broadcasts status push events back to renderer windows.

import { ipcMain, BrowserWindow } from 'electron'
import type { OrcaRuntimeService } from '../runtime/orca-runtime'

type AgentType = 'claude' | 'codex' | 'custom'
type TrustPreset = 'standard' | 'permissive' | 'strict'

interface StartOpts {
  worktreeId: string
  agentType: AgentType
  trustPreset?: TrustPreset
}

interface StopOpts {
  sessionId: string
}

interface ResumeOpts {
  sessionId: string
}

/** Push an agent status event to all active renderer windows */
function broadcastAgentStatusEvent(event: {
  worktreeId: string
  sessionId?: string
  status: 'starting' | 'running' | 'stopped' | 'error'
  errorMessage?: string
}): void {
  for (const win of BrowserWindow.getAllWindows()) {
    if (!win.isDestroyed()) {
      win.webContents.send('agentOrchestration:statusChanged', event)
    }
  }
}

export function registerAgentOrchestrationHandlers(runtime: OrcaRuntimeService): void {
  // ─── agentOrchestration:start ──────────────────────────────────────────────
  ipcMain.handle(
    'agentOrchestration:start',
    async (_event, opts: StartOpts): Promise<{ sessionId: string; status: 'started' | 'already-running' }> => {
      const { worktreeId, agentType, trustPreset = 'standard' } = opts

      try {
        // Delegate to runtime service which knows how to spawn the agent on the
        // appropriate environment (local or remote via SSH runtime).
        const result = await runtime.startAgent({
          worktreeId,
          agentType,
          trustPreset,
        })

        broadcastAgentStatusEvent({
          worktreeId,
          sessionId: result.sessionId,
          status: result.alreadyRunning ? 'running' : 'starting',
        })

        return {
          sessionId: result.sessionId,
          status: result.alreadyRunning ? 'already-running' : 'started',
        }
      } catch (err: any) {
        broadcastAgentStatusEvent({
          worktreeId,
          status: 'error',
          errorMessage: err?.message ?? 'Unknown error',
        })
        throw err
      }
    }
  )

  // ─── agentOrchestration:stop ───────────────────────────────────────────────
  ipcMain.handle(
    'agentOrchestration:stop',
    async (_event, opts: StopOpts): Promise<void> => {
      const { sessionId } = opts

      // Look up which worktreeId owns this sessionId
      const session = runtime.getAgentSession(sessionId)
      const worktreeId = session?.worktreeId ?? 'unknown'

      try {
        await runtime.stopAgent({ sessionId })
        broadcastAgentStatusEvent({ worktreeId, sessionId, status: 'stopped' })
      } catch (err: any) {
        broadcastAgentStatusEvent({
          worktreeId,
          sessionId,
          status: 'error',
          errorMessage: err?.message ?? 'Failed to stop',
        })
        throw err
      }
    }
  )

  // ─── agentOrchestration:resume ─────────────────────────────────────────────
  ipcMain.handle(
    'agentOrchestration:resume',
    async (_event, opts: ResumeOpts): Promise<{ resumed: boolean }> => {
      const { sessionId } = opts

      const session = runtime.getAgentSession(sessionId)
      const worktreeId = session?.worktreeId ?? 'unknown'

      try {
        const resumed = await runtime.resumeAgent({ sessionId })
        if (resumed) {
          broadcastAgentStatusEvent({ worktreeId, sessionId, status: 'starting' })
        }
        return { resumed }
      } catch (err: any) {
        broadcastAgentStatusEvent({
          worktreeId,
          sessionId,
          status: 'error',
          errorMessage: err?.message ?? 'Failed to resume',
        })
        return { resumed: false }
      }
    }
  )
}

export function unregisterAgentOrchestrationHandlers(): void {
  ipcMain.removeAllListeners('agentOrchestration:start')
  ipcMain.removeAllListeners('agentOrchestration:stop')
  ipcMain.removeAllListeners('agentOrchestration:resume')
}
