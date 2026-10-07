/**
 * review-decision-tracker.test.ts — FE-CV-TASK-095-04
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  registerCompletion,
  noteReviewOpened,
  decide,
  _resetTrackerForTest,
} from './review-decision-tracker'

vi.mock('./telemetry', () => ({ track: vi.fn() }))
vi.mock('./review-telemetry', () => ({
  bucketDwellMs: vi.fn((ms: number) => (ms < 5000 ? '<5s' : '>=5s')),
  bucketLarge: vi.fn((n: number) => String(n)),
  trackReviewQualityGateViewed: vi.fn(),
}))

import { track } from './telemetry'
import { trackReviewQualityGateViewed } from './review-telemetry'

const mockTrack = track as ReturnType<typeof vi.fn>
const mockGateViewed = trackReviewQualityGateViewed as ReturnType<typeof vi.fn>

beforeEach(() => {
  _resetTrackerForTest()
  mockTrack.mockClear()
  mockGateViewed.mockClear()
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('registerCompletion', () => {
  it('registers a new entry without emitting', () => {
    registerCompletion('fp1')
    expect(mockTrack).not.toHaveBeenCalled()
  })

  it('abandon-emits prior undecided entry when re-registered', async () => {
    registerCompletion('fp1')
    registerCompletion('fp1') // second call: abandons first
    await Promise.resolve()
    const calls = mockTrack.mock.calls
    expect(calls.some(([event]: [string]) => event === 'review_decision')).toBe(true)
    const [, payload] = calls.find(([event]: [string]) => event === 'review_decision')!
    expect(payload.outcome).toBe('abandon')
  })

  it('does not abandon if previous entry was already decided', async () => {
    registerCompletion('fp1')
    decide('fp1', 'commit')
    await Promise.resolve()
    mockTrack.mockClear()
    registerCompletion('fp1') // second call: prior already decided
    await Promise.resolve()
    // No new abandon event
    expect(mockTrack.mock.calls.filter(([e]: [string]) => e === 'review_decision').length).toBe(0)
  })
})

describe('noteReviewOpened', () => {
  it('does nothing for unknown fingerprint', () => {
    expect(() => noteReviewOpened('unknown-fp')).not.toThrow()
  })

  it('records reviewOpenedAt once', () => {
    registerCompletion('fp1')
    vi.advanceTimersByTime(1000)
    noteReviewOpened('fp1')
    vi.advanceTimersByTime(1000)
    noteReviewOpened('fp1') // second call ignored
    // Should not throw; verify decide still works
    expect(() => decide('fp1', 'commit')).not.toThrow()
  })
})

describe('decide', () => {
  it('emits decision event with correct outcome', async () => {
    registerCompletion('fp1')
    decide('fp1', 'commit')
    await Promise.resolve()
    const [, payload] = mockTrack.mock.calls.find(([e]: [string]) => e === 'review_decision')!
    expect(payload.outcome).toBe('commit')
    expect(payload.worktree_fingerprint).toBe('fp1')
  })

  it('used_review=false when review not opened', async () => {
    registerCompletion('fp1')
    decide('fp1', 'no_action')
    await Promise.resolve()
    const [, payload] = mockTrack.mock.calls.find(([e]: [string]) => e === 'review_decision')!
    expect(payload.used_review).toBe(false)
  })

  it('used_review=true when review opened', async () => {
    registerCompletion('fp1')
    noteReviewOpened('fp1')
    decide('fp1', 'commit')
    await Promise.resolve()
    const [, payload] = mockTrack.mock.calls.find(([e]: [string]) => e === 'review_decision')!
    expect(payload.used_review).toBe(true)
  })

  it('second decide is a no-op', async () => {
    registerCompletion('fp1')
    decide('fp1', 'commit')
    await Promise.resolve()
    mockTrack.mockClear()
    decide('fp1', 'pr') // should be ignored
    await Promise.resolve()
    expect(mockTrack).not.toHaveBeenCalled()
  })

  it('emits gate viewed when gate result provided', async () => {
    registerCompletion('fp1', { overallRisk: 'HIGH', openFindings: 3 })
    decide('fp1', 'commit')
    await Promise.resolve()
    expect(mockGateViewed).toHaveBeenCalledOnce()
    expect(mockGateViewed.mock.calls[0][0].overallRisk).toBe('HIGH')
  })

  it('gate=none when no gate result', async () => {
    registerCompletion('fp1')
    decide('fp1', 'no_action')
    await Promise.resolve()
    const [, payload] = mockTrack.mock.calls.find(([e]: [string]) => e === 'review_decision')!
    expect(payload.gate).toBe('none')
  })
})
