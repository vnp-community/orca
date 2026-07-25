import type { StateCreator } from 'zustand'
import { useShallow } from 'zustand/react/shallow'
import type { DevServer } from '../../../../shared/dev-server-types'
import type { AppState } from '../types'

// ─── Slice Type ───────────────────────────────────────────────────────────────

export type DevServerSlice = {
  devServers: DevServer[]
  activeDevServerId: string | null

  setDevServers: (servers: DevServer[]) => void
  upsertDevServer: (server: DevServer) => void
  removeDevServer: (id: string) => void
  setActiveDevServerId: (id: string | null) => void
  updateDevServerStatus: (
    id: string,
    status: DevServer['status'],
    extra?: Partial<
      Pick<DevServer, 'platform' | 'arch' | 'nodeVersion' | 'lastConnectedAt' | 'lastError'>
    >
  ) => void
}

// ─── Slice Implementation ─────────────────────────────────────────────────────

export const createDevServerSlice: StateCreator<AppState, [], [], DevServerSlice> = (set) => ({
  devServers: [],
  activeDevServerId: null,

  setDevServers: (servers) => set({ devServers: servers }),

  upsertDevServer: (server) =>
    set((state) => ({
      devServers: state.devServers.some((ds) => ds.id === server.id)
        ? state.devServers.map((ds) => (ds.id === server.id ? { ...ds, ...server } : ds))
        : [...state.devServers, server],
    })),

  removeDevServer: (id) =>
    set((state) => ({
      devServers: state.devServers.filter((ds) => ds.id !== id),
      activeDevServerId: state.activeDevServerId === id ? null : state.activeDevServerId,
    })),

  setActiveDevServerId: (id) => set({ activeDevServerId: id }),

  updateDevServerStatus: (id, status, extra = {}) =>
    set((state) => ({
      devServers: state.devServers.map((ds) =>
        ds.id === id ? { ...ds, status, ...extra } : ds
      ),
    })),
})

// ─── Selectors ────────────────────────────────────────────────────────────────
// Import useAppStore lazily to avoid circular dependency at module evaluation time.
// These hooks are only called at React render time, so the import is always resolved.

import { useAppStore } from '../index'

/** All dev servers (stable reference via shallow equality) */
export function useDevServers() {
  return useAppStore(useShallow((s) => s.devServers))
}

/** Currently active dev server, or null */
export function useActiveDevServer() {
  return useAppStore(
    useShallow((s) => {
      const id = s.activeDevServerId
      return id ? (s.devServers.find((ds) => ds.id === id) ?? null) : null
    })
  )
}

/** Only servers that are actively connected */
export function useConnectedDevServers() {
  return useAppStore(useShallow((s) => s.devServers.filter((ds) => ds.status === 'connected')))
}

/** Look up a single server by id */
export function useDevServerById(id: string | null) {
  return useAppStore((s) => (id ? (s.devServers.find((ds) => ds.id === id) ?? null) : null))
}
