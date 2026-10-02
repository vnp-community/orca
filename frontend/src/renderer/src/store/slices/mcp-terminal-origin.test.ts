import { describe, expect, it } from 'vitest'
import { create } from 'zustand'
import type { AppState } from '../types'
import {
  createMcpTerminalOriginSlice,
  selectMcpOriginForTab,
  type McpTerminalOriginSlice
} from './mcp-terminal-origin'

const o = { type: 'mcp', clientName: 'Claude', mcpSessionId: 's', userId: 'u' } as const
const make = () =>
  create<McpTerminalOriginSlice>()((...a) =>
    createMcpTerminalOriginSlice(
      ...(a as unknown as Parameters<typeof createMcpTerminalOriginSlice>)
    )
  )

describe('mcp terminal origin slice', () => {
  it('does not replace the map when the next one is shallow-equal', () => {
    const store = make()
    store.getState().setMcpTerminalOrigins({ h1: o })
    const first = store.getState().mcpOriginByHandle
    store.getState().setMcpTerminalOrigins({ h1: { ...o } })
    expect(store.getState().mcpOriginByHandle).toBe(first)
    store.getState().setMcpTerminalOrigins({ h1: { ...o, clientName: 'Other' } })
    expect(store.getState().mcpOriginByHandle).not.toBe(first)
  })

  it('clears', () => {
    const store = make()
    store.getState().setMcpTerminalOrigins({ h1: o })
    store.getState().clearMcpTerminalOrigins()
    expect(store.getState().mcpOriginByHandle).toEqual({})
    expect(store.getState().mcpOriginsRefreshedAt).toBeNull()
  })
})

describe('selectMcpOriginForTab', () => {
  const state = (
    ptyIdsByTabId: Record<string, string[]>,
    mcpOriginByHandle: Record<string, typeof o>
  ): Pick<AppState, 'ptyIdsByTabId' | 'mcpOriginByHandle'> =>
    ({ ptyIdsByTabId, mcpOriginByHandle }) as never

  it('matches plain and remote pty ids and returns the same object each time', () => {
    const s = state({ t1: ['remote:env@@h1'], t2: ['local'] }, { h1: o })
    expect(selectMcpOriginForTab(s, 't1')).toBe(o)
    expect(selectMcpOriginForTab(s, 't1')).toBe(selectMcpOriginForTab(s, 't1'))
    expect(selectMcpOriginForTab(s, 't2')).toBeNull()
  })

  it('returns null for UI-created tabs and tolerates missing state', () => {
    expect(selectMcpOriginForTab(state({ t: ['p'] }, {}), 't')).toBeNull()
    expect(selectMcpOriginForTab(state({}, { h: o }), 'missing')).toBeNull()
    expect(selectMcpOriginForTab({}, 't')).toBeNull()
  })
})
