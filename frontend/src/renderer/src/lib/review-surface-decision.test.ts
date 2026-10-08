import { beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => ({ decide: vi.fn(), support: { state: 'unknown' } as Record<string, unknown> }))
vi.mock('./review-decision-tracker', () => ({ decide: h.decide }))
vi.mock('@/store', () => ({ useAppStore: { getState: () => ({ codeIntelSupportState: h.support }) } }))

import { recordReviewSurfaceDecision } from './review-surface-decision'
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
    expect(h.decide).toHaveBeenLastCalledWith('wt', 'mark_reviewed', { gate: 'none', openFindings: 0 })
    h.support = { state: 'enabled', effective: { qualityGateEnabled: true } }
    recordReviewSurfaceDecision('wt', 'send_to_agent')
    expect(h.decide).toHaveBeenLastCalledWith('wt', 'send_to_agent', { gate: 'unknown', openFindings: 0 })
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

describe('review open source', () => {
  it('is taken once and defaults to a restore', () => {
    setReviewOpenSource('wt', { source: 'agent_row', afterAgentTurn: true })
    expect(takeReviewOpenSource('wt')).toEqual({ source: 'agent_row', afterAgentTurn: true })
    expect(takeReviewOpenSource('wt')).toEqual({ source: 'restore', afterAgentTurn: false })
  })
})
