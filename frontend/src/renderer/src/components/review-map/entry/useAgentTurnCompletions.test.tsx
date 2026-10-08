// @vitest-environment happy-dom
import { act, render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

type S = {
  agentStatusByPaneKey: Record<string, unknown>
  retainedAgentsByPaneKey: Record<string, unknown>
  tabsByWorktree: Record<string, { id: string }[]>
}
const state: S = {
  agentStatusByPaneKey: {},
  retainedAgentsByPaneKey: {},
  tabsByWorktree: { wt: [{ id: 'tab' }] }
}
const listeners = new Set<() => void>()
vi.mock('@/store', async () => {
  const React = await import('react')
  const useAppStore = (sel: (s: S) => unknown): unknown =>
    React.useSyncExternalStore(
      (cb) => {
        listeners.add(cb)
        return () => listeners.delete(cb)
      },
      () => sel(snapshot)
    )
  return { useAppStore }
})
let snapshot: S = state

import { useAgentTurnCompletions } from './useAgentTurnCompletions'

describe('useAgentTurnCompletions', () => {
  it('does not re-render when unrelated state changes', () => {
    let renders = 0
    function Probe(): null {
      useAgentTurnCompletions('wt')
      renders += 1
      return null
    }
    render(<Probe />)
    const before = renders
    act(() => {
      snapshot = { ...snapshot, agentStatusByPaneKey: { x: { state: 'working', stateStartedAt: 1 } } }
      listeners.forEach((l) => l())
    })
    expect(renders).toBe(before)
    act(() => {
      snapshot = {
        ...snapshot,
        agentStatusByPaneKey: {
          'tab:1': { state: 'done', stateStartedAt: 5, paneKey: 'tab:1', agentType: 'claude' }
        }
      }
      listeners.forEach((l) => l())
    })
    expect(renders).toBe(before + 1)
  })
})
