// ─── DevServerStore ──────────────────────────────────────────────────────────
// CRUD operations on PersistedState.devServers via the Store.
// Handles persistence layer only — no runtime state.

import type { Store } from '../persistence'
import type { DevServerInput, PersistedDevServer } from '../../shared/dev-server-types'
import { randomUUID } from 'node:crypto'

export class DevServerStore {
  constructor(private store: Store) {}

  list(): PersistedDevServer[] {
    return this.store.getState().devServers ?? []
  }

  add(input: DevServerInput): PersistedDevServer {
    const record: PersistedDevServer = {
      id: `ds-${randomUUID()}`,
      name: input.name,
      connectionType: input.connectionType,
      sshTargetId: input.sshTargetId,
      wsUrl: input.wsUrl,
      workspaceDir: null,
      addedAt: Date.now()
    }
    this.store.mutate((state) => {
      state.devServers = [...(state.devServers ?? []), record]
    })
    return record
  }

  update(id: string, updates: Partial<PersistedDevServer>): void {
    this.store.mutate((state) => {
      const servers = state.devServers ?? []
      const idx = servers.findIndex((ds) => ds.id === id)
      if (idx >= 0) {
        servers[idx] = { ...servers[idx], ...updates }
        state.devServers = servers
      }
    })
  }

  remove(id: string): void {
    this.store.mutate((state) => {
      state.devServers = (state.devServers ?? []).filter((ds) => ds.id !== id)
    })
  }
}
