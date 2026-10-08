import { beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => ({
  decide: vi.fn(),
  support: { state: 'unknown' } as Record<string, unknown>
}))
vi.mock('./review-decision-tracker', () => ({ decide: h.decide }))
vi.mock('@/store', () => ({
  useAppStore: { getState: () => ({ codeIntelSupportState: h.support }) }
}))

import {
  countOpenGateFindings,
  recordReviewSurfaceDecision,
  reviewDecisionContextFromGate
} from './review-surface-decision'
import { setReviewOpenSource, takeReviewOpenSource } from './review-open-source'

beforeEach(() => {
  h.decide.mockReset()
})

describe('recordReviewSurfaceDecision', () => {
  it('does nothing until code intelligence is enabled', () => {
    h.support = { state: 'disabled' }
    recordReviewSurfaceDecision('wt', 'send_to_agent')
    expect(h.decide).not.toHaveBeenCalled()
  })

  it('reports gate none without the quality gate and unknown with it', () => {
    h.support = { state: 'enabled', effective: { qualityGateEnabled: false } }
    recordReviewSurfaceDecision('wt', 'mark_reviewed')
    expect(h.decide).toHaveBeenLastCalledWith('wt', 'mark_reviewed', {
      gate: 'none',
      openFindings: 0
    })
    h.support = { state: 'enabled', effective: { qualityGateEnabled: true } }
    recordReviewSurfaceDecision('wt', 'send_to_agent')
    expect(h.decide).toHaveBeenLastCalledWith('wt', 'send_to_agent', {
      gate: 'unknown',
      openFindings: 0
    })
  })

  it('also carries the create_review decision from the Checks panel', () => {
    h.support = { state: 'enabled' }
    recordReviewSurfaceDecision('wt', 'create_review')
    expect(h.decide).toHaveBeenCalledWith('wt', 'create_review', expect.anything())
  })

  it('never throws into the caller', () => {
    h.support = { state: 'enabled' }
    h.decide.mockImplementation(() => {
      throw new Error('boom')
    })
    expect(() => recordReviewSurfaceDecision('wt', 'mark_reviewed')).not.toThrow()
  })
})

describe('open_findings from the cached gate', () => {
  it('counts failing and warning reasons only', () => {
    expect(
      countOpenGateFindings([
        { result: 'fail' },
        { result: 'warn' },
        { result: 'pass' },
        { result: 'x' }
      ])
    ).toBe(2)
    expect(countOpenGateFindings(undefined)).toBe(0)
  })

  it('reads verdict (contract) or result (legacy) and ignores unknown verdicts', () => {
    expect(
      reviewDecisionContextFromGate(true, { verdict: 'warn', reasons: [{ result: 'warn' }] })
    ).toEqual({
      gate: 'warn',
      openFindings: 1
    })
    expect(reviewDecisionContextFromGate(true, { result: 'pass', reasons: [] })).toEqual({
      gate: 'pass',
      openFindings: 0
    })
    expect(reviewDecisionContextFromGate(true, { verdict: 'weird' })).toEqual({
      gate: 'unknown',
      openFindings: 0
    })
    expect(
      reviewDecisionContextFromGate(false, { verdict: 'fail', reasons: [{ result: 'fail' }] })
    ).toEqual({
      gate: 'none',
      openFindings: 0
    })
  })
})

describe('review open source', () => {
  it('is taken once and defaults to a restore', () => {
    setReviewOpenSource('wt', { source: 'agent_row', afterAgentTurn: true })
    expect(takeReviewOpenSource('wt')).toEqual({ source: 'agent_row', afterAgentTurn: true })
    expect(takeReviewOpenSource('wt')).toEqual({ source: 'restore', afterAgentTurn: false })
  })
})
