import { beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => ({ decide: vi.fn(), state: {} as Record<string, unknown> }))
vi.mock('@/lib/review-decision-tracker', () => ({ decide: h.decide }))
vi.mock('@/store', () => ({ useAppStore: { getState: () => h.state } }))

import { recordChecksPanelReviewCreated } from './checks-panel-review-decision'

const gate = (verdict: string, results: string[]) => ({
  gate: {
    data: { gate: { verdict, reasons: results.map((result, i) => ({ check: `c${i}`, result })) } }
  }
})

beforeEach(() => {
  h.decide.mockReset()
  h.state = {
    codeIntelSupportState: { state: 'enabled', effective: { qualityGateEnabled: true } },
    codeIntelQualityByWorktree: { wt: gate('fail', ['fail', 'warn', 'pass']) }
  }
})

describe('recordChecksPanelReviewCreated', () => {
  it('records create_review with the cached gate verdict and open findings after a successful create', () => {
    recordChecksPanelReviewCreated('wt', { ok: true })
    expect(h.decide).toHaveBeenCalledWith('wt', 'create_review', { gate: 'fail', openFindings: 2 })
  })

  it('records nothing for a failed create or without a worktree', () => {
    recordChecksPanelReviewCreated('wt', { ok: false })
    recordChecksPanelReviewCreated(null, { ok: true })
    expect(h.decide).not.toHaveBeenCalled()
  })

  it('reports unknown without a cached gate and none when the gate is off', () => {
    h.state.codeIntelQualityByWorktree = {}
    recordChecksPanelReviewCreated('wt', { ok: true })
    expect(h.decide).toHaveBeenLastCalledWith('wt', 'create_review', {
      gate: 'unknown',
      openFindings: 0
    })
    h.state.codeIntelSupportState = { state: 'enabled', effective: { qualityGateEnabled: false } }
    recordChecksPanelReviewCreated('wt', { ok: true })
    expect(h.decide).toHaveBeenLastCalledWith('wt', 'create_review', {
      gate: 'none',
      openFindings: 0
    })
  })
})
