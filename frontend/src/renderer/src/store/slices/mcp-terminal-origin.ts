import type { StateCreator } from 'zustand'
import type { McpOrigin } from '../../../../shared/mcp-types'
import { ptyIdToOriginKey, type OriginByHandle } from '../../lib/mcp-terminal-origin'
import type { AppState } from '../types'

// Runtime-only (never persisted): origins describe live PTYs, not saved layout.
export type McpTerminalOriginSlice = {
  mcpOriginByHandle: OriginByHandle
  mcpOriginsRefreshedAt: number | null
  setMcpTerminalOrigins: (next: OriginByHandle) => void
  clearMcpTerminalOrigins: () => void
}

function shallowEqualOrigins(a: OriginByHandle, b: OriginByHandle): boolean {
  const keys = Object.keys(a)
  if (keys.length !== Object.keys(b).length) {
    return false
  }
  return keys.every((k) => {
    const x = a[k]
    const y = b[k]
    return (
      y !== undefined &&
      x.clientName === y.clientName &&
      x.mcpSessionId === y.mcpSessionId &&
      x.userId === y.userId
    )
  })
}

export const createMcpTerminalOriginSlice: StateCreator<
  AppState,
  [],
  [],
  McpTerminalOriginSlice
> = (set, get) => ({
  mcpOriginByHandle: {},
  mcpOriginsRefreshedAt: null,
  setMcpTerminalOrigins: (next) => {
    // Keep the old map when unchanged so subscribers do not re-render every poll.
    if (shallowEqualOrigins(get().mcpOriginByHandle, next)) {
      set({ mcpOriginsRefreshedAt: Date.now() })
      return
    }
    set({ mcpOriginByHandle: next, mcpOriginsRefreshedAt: Date.now() })
  },
  clearMcpTerminalOrigins: () => set({ mcpOriginByHandle: {}, mcpOriginsRefreshedAt: null })
})

type OriginSelectorState = Pick<Partial<AppState>, 'mcpOriginByHandle' | 'ptyIdsByTabId'>

/** Returns a map-owned object (stable reference) or null when the tab has no MCP-created PTY. */
export function selectMcpOriginForTab(state: OriginSelectorState, tabId: string): McpOrigin | null {
  const origins = state.mcpOriginByHandle
  if (!origins) {
    return null
  }
  for (const ptyId of state.ptyIdsByTabId?.[tabId] ?? []) {
    const origin = origins[ptyIdToOriginKey(ptyId)]
    if (origin) {
      return origin
    }
  }
  return null
}
