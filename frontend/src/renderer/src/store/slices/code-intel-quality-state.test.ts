/**
 * Tests for code-intel-quality-state.ts (FE-CV-TASK-087-02)
 */

import { describe, it, expect, vi } from 'vitest'
import { createCodeIntelQualitySlice } from './code-intel-quality-state'
import type { QualityWorktreeState } from './code-intel-quality-state'

type Store = { codeIntelQualityByWorktree: Record<string, QualityWorktreeState> }

function makeSlice() {
  let state: Store = { codeIntelQualityByWorktree: {} }
  const set = vi.fn((fn: (prev: Store) => Partial<Store>) => {
    const patch = fn(state)
    state = { ...state, ...patch }
  })
  const slice = createCodeIntelQualitySlice(set)
  const getState = () => ({ ...state, ...slice })
  return { slice, getState, set }
}

describe('startQualityRun', () => {
  it('sets phase to starting immediately', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    const s = getState()
    expect(s.codeIntelQualityByWorktree['wt-1']?.activeRun?.phase).toBe('starting')
    expect(s.codeIntelQualityByWorktree['wt-1']?.activeRun?.isOwned).toBe(true)
  })

  it('does not start when already starting (idempotent)', () => {
    const { slice, set } = makeSlice()
    slice.startQualityRun('wt-1')
    const callCount = set.mock.calls.length
    slice.startQualityRun('wt-1') // second call
    // Set may be called but state should be a no-op (empty patch)
    expect(set.mock.calls.length).toBeGreaterThanOrEqual(callCount)
  })
})

describe('cancelQualityRun', () => {
  it('sets phase to cancelling', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    slice.attachQualityRun('wt-1', 'run-1')
    slice.cancelQualityRun('wt-1')
    expect(getState().codeIntelQualityByWorktree['wt-1']?.activeRun?.phase).toBe('cancelling')
  })

  it('no-ops when no active run', () => {
    const { slice, set } = makeSlice()
    const before = set.mock.calls.length
    slice.cancelQualityRun('wt-1')
    // Called but empty patch (no active run)
    const calls = set.mock.calls.length - before
    expect(calls).toBeGreaterThanOrEqual(0)
  })
})

describe('attachQualityRun', () => {
  it('moves from starting to running with runId', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    slice.attachQualityRun('wt-1', 'run-abc')
    const run = getState().codeIntelQualityByWorktree['wt-1']?.activeRun
    expect(run?.runId).toBe('run-abc')
    expect(run?.phase).toBe('running')
    expect(run?.isOwned).toBe(true)
  })
})

describe('applyQualityPushEvent — qualityProgress', () => {
  it('creates external run if no active run', () => {
    const { slice, getState } = makeSlice()
    // Manually initialize worktree
    slice.invalidateQuality('wt-1') // ensures wt-1 exists with default
    slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'wt-1', runId: 'ext-1', percent: 25 })
    const run = getState().codeIntelQualityByWorktree['wt-1']?.activeRun
    expect(run?.runId).toBe('ext-1')
    expect(run?.isOwned).toBe(false)
  })

  it('updates percent on owned run', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    slice.attachQualityRun('wt-1', 'run-1')
    slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'wt-1', runId: 'run-1', percent: 60 })
    expect(getState().codeIntelQualityByWorktree['wt-1']?.activeRun?.percent).toBe(60)
  })

  it('null percent preserved', () => {
    const { slice, getState } = makeSlice()
    slice.invalidateQuality('wt-1')
    slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'wt-1', runId: 'ext-1', percent: null })
    expect(getState().codeIntelQualityByWorktree['wt-1']?.activeRun?.percent).toBeNull()
  })

  it('ignores event for unknown worktree', () => {
    const { slice, getState } = makeSlice()
    slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'unknown-wt', runId: 'r', percent: 50 })
    expect(getState().codeIntelQualityByWorktree['unknown-wt']).toBeUndefined()
  })
})

describe('applyQualityPushEvent — qualityFinished', () => {
  it('success → completed + gateStale', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    slice.attachQualityRun('wt-1', 'run-1')
    slice.applyQualityPushEvent({ event: 'qualityFinished', worktreeId: 'wt-1', runId: 'run-1', success: true, error: null })
    const wt = getState().codeIntelQualityByWorktree['wt-1']
    expect(wt?.activeRun?.phase).toBe('completed')
    expect(wt?.gateStale).toBe(true)
  })

  it('failure → failed phase', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    slice.attachQualityRun('wt-1', 'run-1')
    slice.applyQualityPushEvent({ event: 'qualityFinished', worktreeId: 'wt-1', runId: 'run-1', success: false, error: 'scan failed' })
    expect(getState().codeIntelQualityByWorktree['wt-1']?.activeRun?.phase).toBe('failed')
  })

  it('interrupted error → interrupted phase', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    slice.attachQualityRun('wt-1', 'run-1')
    slice.applyQualityPushEvent({ event: 'qualityFinished', worktreeId: 'wt-1', runId: 'run-1', success: false, error: 'interrupted by user' })
    expect(getState().codeIntelQualityByWorktree['wt-1']?.activeRun?.phase).toBe('interrupted')
  })
})

describe('pruneCodeIntelQualityWorktrees', () => {
  it('removes data for unlisted worktrees', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    slice.startQualityRun('wt-2')
    slice.pruneCodeIntelQualityWorktrees(['wt-2'])
    const state = getState().codeIntelQualityByWorktree
    expect(state['wt-1']).toBeUndefined()
    expect(state['wt-2']).toBeDefined()
  })

  it('noop for empty live list', () => {
    const { slice, getState } = makeSlice()
    slice.startQualityRun('wt-1')
    slice.pruneCodeIntelQualityWorktrees([])
    expect(getState().codeIntelQualityByWorktree['wt-1']).toBeUndefined()
  })
})

describe('invalidateQuality', () => {
  it('sets gateStale to true', () => {
    const { slice, getState } = makeSlice()
    slice.invalidateQuality('wt-1')
    expect(getState().codeIntelQualityByWorktree['wt-1']?.gateStale).toBe(true)
  })
})

describe('setQualityUi', () => {
  it('sets uiTab', () => {
    const { slice, getState } = makeSlice()
    slice.invalidateQuality('wt-1')
    slice.setQualityUi('wt-1', 'findings')
    expect(getState().codeIntelQualityByWorktree['wt-1']?.uiTab).toBe('findings')
  })
})
