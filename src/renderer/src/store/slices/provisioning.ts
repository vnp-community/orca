// Provisioning slice — bulk SSH fleet relay deployment (CR-003)
import type { StateCreator } from 'zustand'
import type { AppState } from '../types'

// ─── Types ────────────────────────────────────────────────────────────────────

export type ProvisioningServerStatus =
  | 'pending'
  | 'connecting'
  | 'deploying-relay'
  | 'done'
  | 'error'
  | 'skipped'

export type ProvisioningServerEntry = {
  serverId: string
  label: string
  host: string
  status: ProvisioningServerStatus
  error: string | null
  startedAt: number | null
  completedAt: number | null
  relayVersion: string | null
}

export type ProvisioningSessionPhase = 'running' | 'done' | 'cancelled'

export type ProvisioningSession = {
  sessionId: string
  startedAt: number
  phase: ProvisioningSessionPhase
  servers: ProvisioningServerEntry[]
  concurrency: number
}

export type ProvisioningSlice = {
  provisioningSession: ProvisioningSession | null

  startProvisioningSession: (serverIds: string[]) => void
  updateProvisioningServerStatus: (
    serverId: string,
    update: Partial<ProvisioningServerEntry>
  ) => void
  finishProvisioningSession: () => void
  cancelProvisioningSession: () => void
}

// ─── Slice Factory ────────────────────────────────────────────────────────────

export const createProvisioningSlice: StateCreator<
  AppState,
  [],
  [],
  ProvisioningSlice
> = (set, get) => ({
  provisioningSession: null,

  startProvisioningSession: (serverIds) => {
    const targets = get().sshTargets
    set({
      provisioningSession: {
        sessionId: crypto.randomUUID(),
        startedAt: Date.now(),
        phase: 'running',
        concurrency: 3,
        servers: serverIds.map((id) => {
          const target = targets.find((t) => t.id === id)
          return {
            serverId: id,
            label: target?.label ?? id,
            host: target?.host ?? '',
            status: 'pending' satisfies ProvisioningServerStatus,
            error: null,
            startedAt: null,
            completedAt: null,
            relayVersion: null
          }
        })
      }
    })
  },

  updateProvisioningServerStatus: (serverId, update) =>
    set((s) => {
      const session = s.provisioningSession
      if (!session) return s
      const idx = session.servers.findIndex((e) => e.serverId === serverId)
      if (idx === -1) return s
      // Replace the specific entry immutably
      const updatedServers = session.servers.slice()
      updatedServers[idx] = { ...updatedServers[idx], ...update }
      return {
        provisioningSession: { ...session, servers: updatedServers }
      }
    }),

  finishProvisioningSession: () =>
    set((s) => {
      if (!s.provisioningSession) return s
      return {
        provisioningSession: { ...s.provisioningSession, phase: 'done' }
      }
    }),

  cancelProvisioningSession: () => set({ provisioningSession: null })
})
