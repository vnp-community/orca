import { beforeEach, describe, expect, it, vi } from 'vitest'

const trackDecision = vi.hoisted(() => vi.fn())
vi.mock('./review-telemetry', () => ({ trackReviewDecisionMade: trackDecision }))

import {
  decide,
  hasPendingTurn,
  noteReviewOpened,
  registerCompletion,
  resetReviewDecisionTracker
} from './review-decision-tracker'

const ctx = { gate: 'warn', openFindings: 3 } as const

beforeEach(() => {
  resetReviewDecisionTracker()
  trackDecision.mockClear()
})

describe('review decision tracker', () => {
  it('registering a turn emits nothing', () => {
    registerCompletion('w1', 0)
    expect(trackDecision).not.toHaveBeenCalled()
    expect(hasPendingTurn('w1')).toBe(true)
  })

  it('emits exactly one decision with latency, gate and findings', () => {
    registerCompletion('w1', 1000)
    decide('w1', 'commit', ctx, 1000 + 90_000)
    expect(trackDecision).toHaveBeenCalledTimes(1)
    expect(trackDecision).toHaveBeenCalledWith({
      decision: 'commit',
      latencyMs: 90_000,
      usedReview: false,
      gate: 'warn',
      openFindings: 3
    })
    expect(hasPendingTurn('w1')).toBe(false)
  })

  it('ignores a second decision for the same turn', () => {
    registerCompletion('w1', 0)
    decide('w1', 'commit', ctx, 10)
    decide('w1', 'create_review', ctx, 20)
    expect(trackDecision).toHaveBeenCalledTimes(1)
  })

  it('ignores decisions when no agent turn is pending (no event for manual commits)', () => {
    decide('w1', 'commit', ctx, 10)
    expect(trackDecision).not.toHaveBeenCalled()
  })

  it('records used_review once review was opened after the turn', () => {
    registerCompletion('w1', 0)
    noteReviewOpened('w1', 5)
    noteReviewOpened('w1', 6_000_000) // second open does not change anything
    decide('w1', 'mark_reviewed', ctx, 100)
    expect(trackDecision.mock.calls[0][0]).toMatchObject({ usedReview: true, decision: 'mark_reviewed' })
  })

  it('ignores review opens for worktrees without a pending turn', () => {
    noteReviewOpened('w9')
    registerCompletion('w9', 0)
    decide('w9', 'commit', ctx, 1)
    expect(trackDecision.mock.calls[0][0].usedReview).toBe(false)
  })

  it('a new turn replaces an undecided one and reports abandon for the old one', () => {
    registerCompletion('w1', 0)
    noteReviewOpened('w1', 1)
    registerCompletion('w1', 50_000)
    expect(trackDecision).toHaveBeenCalledTimes(1)
    expect(trackDecision).toHaveBeenCalledWith({
      decision: 'abandon', latencyMs: 50_000, usedReview: true, gate: 'none', openFindings: 0
    })
    decide('w1', 'commit', ctx, 60_000)
    expect(trackDecision).toHaveBeenCalledTimes(2)
    expect(trackDecision.mock.calls[1][0]).toMatchObject({ decision: 'commit', usedReview: false, latencyMs: 10_000 })
  })

  it('keeps worktrees independent', () => {
    registerCompletion('a', 0)
    registerCompletion('b', 0)
    decide('a', 'commit', ctx, 1)
    expect(hasPendingTurn('b')).toBe(true)
    expect(trackDecision).toHaveBeenCalledTimes(1)
  })
})
