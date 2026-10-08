// src/renderer/src/store/slices/remote-agent-sessions.ts
// BUG-FE-ORCH-001: Track remote agent sessions started via agentOrchestration IPC
// Separate from the main agent-status slice (which tracks PTY-based agents)
// because remote agents are long-running services on runtime environments.

import type { StateCreator } from 'zustand'
import type { AppState } from '../types'

export type RemoteAgentStatus = 'starting' | 'running' | 'stopped' | 'error'

export type RemoteAgentSession = {
  sessionId: string
  worktreeId: string
  agentType: 'claude' | 'codex' | 'custom'
  trustPreset: 'standard' | 'permissive' | 'strict'
  status: RemoteAgentStatus
  startedAt: number
  stoppedAt?: number
  errorMessage?: string
}

export type AgentOrchestrationStatusEvent = {
  worktreeId: string
  sessionId?: string
  status: RemoteAgentStatus
  errorMessage?: string
}

export type RemoteAgentSessionSlice = {
  /** Map from worktreeId → session (one session per worktree at a time) */
  remoteAgentSessions: Record<string, RemoteAgentSession>

  setRemoteAgentSession: (worktreeId: string, session: RemoteAgentSession) => void
  updateAgentStatus: (event: AgentOrchestrationStatusEvent) => void
  clearRemoteAgentSession: (worktreeId: string) => void
}

export const createRemoteAgentSessionSlice: StateCreator<
  AppState,
  [],
  [],
  RemoteAgentSessionSlice
> = (set) => ({
  remoteAgentSessions: {},

  setRemoteAgentSession: (worktreeId, session) =>
    set(s => ({
      remoteAgentSessions: { ...s.remoteAgentSessions, [worktreeId]: session }
    })),

  updateAgentStatus: (event) =>
    set(s => {
      const existing = s.remoteAgentSessions[event.worktreeId]
      if (!existing && !event.sessionId) {return s}

      const updated: RemoteAgentSession = {
        ...(existing ?? {
          worktreeId: event.worktreeId,
          agentType: 'claude',
          trustPreset: 'standard',
          startedAt: Date.now(),
        }),
        sessionId: event.sessionId ?? existing?.sessionId ?? '',
        status: event.status,
        ...(event.errorMessage ? { errorMessage: event.errorMessage } : {}),
        ...(event.status === 'stopped' ? { stoppedAt: Date.now() } : {}),
      }

      return {
        remoteAgentSessions: {
          ...s.remoteAgentSessions,
          [event.worktreeId]: updated,
        },
      }
    }),

  clearRemoteAgentSession: (worktreeId) =>
    set(s => {
      const next = { ...s.remoteAgentSessions }
      delete next[worktreeId]
      return { remoteAgentSessions: next }
    }),
})
