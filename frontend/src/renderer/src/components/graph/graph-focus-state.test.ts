import { describe, expect, it } from 'vitest'
import { focusNeighborhood, focusReducer, isDimmed } from './graph-focus-state'
import { gEdge } from './graph-test-fixtures'

describe('graph focus', () => {
  const edges = [gEdge('a', 'b'), gEdge('c', 'a'), gEdge('d', 'e')]
  it('returns the node and its one-step neighbors, undirected', () => {
    expect([...focusNeighborhood(edges, 'a')].sort()).toEqual(['a', 'b', 'c'])
    expect([...focusNeighborhood(edges, 'zzz')]).toEqual(['zzz'])
  })
  it('toggles and clears focus', () => {
    let s = focusReducer({ focusId: null }, { type: 'focus', id: 'a' })
    expect(s.focusId).toBe('a')
    s = focusReducer(s, { type: 'focus', id: 'b' })
    expect(s.focusId).toBe('b')
    s = focusReducer(s, { type: 'focus', id: 'b' })
    expect(s.focusId).toBeNull()
    expect(focusReducer(s, { type: 'clear' })).toBe(s)
  })
  it('dims only outside the focus set', () => {
    expect(isDimmed('x', null)).toBe(false)
    expect(isDimmed('x', new Set(['a']))).toBe(true)
    expect(isDimmed('a', new Set(['a']))).toBe(false)
  })
})
