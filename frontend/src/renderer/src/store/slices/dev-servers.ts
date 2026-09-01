import type { StateCreator } from 'zustand'
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
// Why: this file deliberately holds ONLY the slice, no useAppStore-reading
// hooks — store/index.ts imports createDevServerSlice from here directly to
// build the aggregate store, and a hook needing a top-level `import {
// useAppStore } from '../index'` living in the SAME file would make that a
// genuine circular module dependency (see dev-servers-selectors.ts's own
// doc comment, where those hooks now live, for the live bug this caused).

export const createDevServerSlice: StateCreator<AppState, [], [], DevServerSlice> = (set) => ({
  devServers: [],
  activeDevServerId: null,

  setDevServers: (servers) => set({ devServers: servers }),

  upsertDevServer: (server) =>
    set((state) => ({
      devServers: state.devServers.some((ds) => ds.id === server.id)
        ? state.devServers.map((ds) => (ds.id === server.id ? { ...ds, ...server } : ds))
        : [...state.devServers, server]
    })),

  removeDevServer: (id) =>
    set((state) => ({
      devServers: state.devServers.filter((ds) => ds.id !== id),
      activeDevServerId: state.activeDevServerId === id ? null : state.activeDevServerId
    })),

  setActiveDevServerId: (id) => set({ activeDevServerId: id }),

  updateDevServerStatus: (id, status, extra = {}) =>
    set((state) => ({
      devServers: state.devServers.map((ds) => (ds.id === id ? { ...ds, status, ...extra } : ds))
    }))
})
