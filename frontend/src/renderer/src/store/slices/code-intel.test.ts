/**
 * Tests for code-intel.ts store slice (FE-CV-TASK-050-10)
 * Covers: LRU eviction, invalidation, worktree pruning (both paths)
 */

import { describe, it, expect } from 'vitest'
import { createCodeIntelSlice, CODE_INTEL_WORKTREE_KEYED_STATE_KEYS } from './code-intel'

type SliceState = {
  codeIntelSupportState: ReturnType<typeof createCodeIntelSlice>['codeIntelSupportState']
  codeIntelWorktreeState: ReturnType<typeof createCodeIntelSlice>['codeIntelWorktreeState']
  codeIntelResyncCounter: number
}

function makeSlice() {
  let state: SliceState = {
    codeIntelSupportState: { state: 'unknown' },
    codeIntelWorktreeState: {},
    codeIntelResyncCounter: 0
  }
  const set = (fn: (prev: SliceState) => Partial<SliceState>) => {
    state = { ...state, ...fn(state) }
  }
  const get = () => state
  const slice = createCodeIntelSlice(set, get)
  return { slice, getState: () => state }
}

describe('CODE_INTEL_WORKTREE_KEYED_STATE_KEYS', () => {
  it('contains codeIntelWorktreeState', () => {
    expect(CODE_INTEL_WORKTREE_KEYED_STATE_KEYS).toContain('codeIntelWorktreeState')
  })
})

describe('setCodeIntelSupportState', () => {
  it('updates support state', () => {
    const { slice, getState } = makeSlice()
    slice.setCodeIntelSupportState({ state: 'enabled', effective: { codeIntelEnabled: true } })
    expect(getState().codeIntelSupportState.state).toBe('enabled')
  })
})

describe('setCacheResult / getCacheResult', () => {
  it('stores and retrieves a value', () => {
    const { slice } = makeSlice()
    slice.setCacheResult('wt-1', 'key:1', { data: 42 })
    const result = slice.getCacheResult('wt-1', 'key:1')
    expect(result).toEqual({ data: 42 })
  })

  it('returns undefined for missing key', () => {
    const { slice } = makeSlice()
    expect(slice.getCacheResult('wt-1', 'nonexistent')).toBeUndefined()
  })

  it('evicts oldest entry at LRU capacity (16)', () => {
    const { slice } = makeSlice()
    for (let i = 0; i < 17; i++) {
      slice.setCacheResult('wt-1', `key:${i}`, i)
    }
    // key:0 should be evicted (oldest)
    expect(slice.getCacheResult('wt-1', 'key:0')).toBeUndefined()
    expect(slice.getCacheResult('wt-1', 'key:16')).toBe(16)
  })
})

describe('invalidateCodeIntelWorktree', () => {
  it('clears cache and sets stale=true', () => {
    const { slice, getState } = makeSlice()
    slice.setCacheResult('wt-1', 'key:1', 'value')
    slice.invalidateCodeIntelWorktree('wt-1')
    expect(slice.getCacheResult('wt-1', 'key:1')).toBeUndefined()
    expect(getState().codeIntelWorktreeState['wt-1']?.stale).toBe(true)
  })
})

describe('pruneCodeIntelWorktrees', () => {
  it('removes state for unlisted worktrees (removeWorktree path)', () => {
    const { slice, getState } = makeSlice()
    slice.setCacheResult('wt-1', 'k', 1)
    slice.setCacheResult('wt-2', 'k', 2)
    slice.pruneCodeIntelWorktrees(['wt-2'])
    expect(getState().codeIntelWorktreeState['wt-1']).toBeUndefined()
    expect(getState().codeIntelWorktreeState['wt-2']).toBeDefined()
  })

  it('removes all when live list is empty (buildWorktreePurgeState path)', () => {
    const { slice, getState } = makeSlice()
    slice.setCacheResult('wt-1', 'k', 1)
    slice.pruneCodeIntelWorktrees([])
    expect(getState().codeIntelWorktreeState['wt-1']).toBeUndefined()
  })

  it('keys defined in CODE_INTEL_WORKTREE_KEYED_STATE_KEYS are all pruned', () => {
    const { slice, getState } = makeSlice()
    slice.setCacheResult('wt-1', 'k', 1)
    slice.pruneCodeIntelWorktrees([])
    for (const key of CODE_INTEL_WORKTREE_KEYED_STATE_KEYS) {
      const map = getState()[key as keyof SliceState] as Record<string, unknown>
      expect(Object.keys(map).length).toBe(0)
    }
  })
})

describe('triggerCodeIntelResync', () => {
  it('increments counter', () => {
    const { slice, getState } = makeSlice()
    expect(getState().codeIntelResyncCounter).toBe(0)
    slice.triggerCodeIntelResync()
    expect(getState().codeIntelResyncCounter).toBe(1)
  })
})
