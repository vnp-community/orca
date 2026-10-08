import { describe, expect, it } from 'vitest'
import { NO_ACCEPTANCES, acceptedIdsForApprove, getApprovalRequirements } from './risk-approval-rules'
import type { ImpactFinding, ImpactSummary } from '../../../../../shared/request-artifact-types'

const summary = (over: Partial<ImpactSummary> = {}): ImpactSummary => ({
  assessmentId: 'a', digest: 'd1', level: 'high', score: null, topReasons: [], confidence: null, assessedAt: null, tool: null,
  stale: false, mode: 'enforce', status: 'ready', hardRules: [], ...over
})
const finding = (id: string, level: ImpactFinding['level']): ImpactFinding => ({ id, dimension: 'data', level, title: id })
const state = (ids: string[], digest: string | null = 'd1', viewed = false) => ({ viewedImpact: viewed, acceptedFindingIds: new Set(ids), digestAtAcceptance: digest })

describe('getApprovalRequirements', () => {
  it('never blocks without an assessment, for unknown, low or shadow', () => {
    expect(getApprovalRequirements(null, [], NO_ACCEPTANCES)).toMatchObject({ canApprove: true, blockedReasonKey: null, level: 'unknown' })
    expect(getApprovalRequirements(summary({ level: 'unknown' }), [finding('f', 'critical')], NO_ACCEPTANCES).canApprove).toBe(true)
    expect(getApprovalRequirements(summary({ level: 'low' }), [], NO_ACCEPTANCES).canApprove).toBe(true)
    expect(getApprovalRequirements(summary({ level: 'critical', mode: 'shadow' }), [finding('f', 'critical')], NO_ACCEPTANCES).canApprove).toBe(true)
  })

  it('medium needs the impact section opened first', () => {
    const r = getApprovalRequirements(summary({ level: 'medium' }), [], NO_ACCEPTANCES)
    expect(r).toMatchObject({ requiresView: true, canApprove: false })
    expect(r.blockedReasonKey).toContain('viewImpactFirst')
    expect(getApprovalRequirements(summary({ level: 'medium' }), [], state([], null, true)).canApprove).toBe(true)
  })

  it('high needs every high-or-above finding accepted against the current digest', () => {
    const fs = [finding('h', 'high'), finding('c', 'critical'), finding('m', 'medium')]
    const none = getApprovalRequirements(summary(), fs, NO_ACCEPTANCES)
    expect(none.mustAccept.map((f) => f.id)).toEqual(['h', 'c'])
    expect(none.canApprove).toBe(false)
    expect(getApprovalRequirements(summary(), fs, state(['h'])).unaccepted.map((f) => f.id)).toEqual(['c'])
    expect(getApprovalRequirements(summary(), fs, state(['h', 'c'])).canApprove).toBe(true)
  })

  it('a changed digest voids earlier acceptances', () => {
    const fs = [finding('h', 'high')]
    expect(getApprovalRequirements(summary({ digest: 'd2' }), fs, state(['h'], 'd1')).canApprove).toBe(false)
    expect(acceptedIdsForApprove(summary({ digest: 'd2' }), state(['h'], 'd1'))).toEqual([])
    expect(acceptedIdsForApprove(summary(), state(['h']))).toEqual(['h'])
  })

  it('critical adds the second-approver note without blocking the first approver once accepted', () => {
    const r = getApprovalRequirements(summary({ level: 'critical' }), [finding('c', 'critical')], state(['c']))
    expect(r).toMatchObject({ needsSecondApprover: true, canApprove: true })
  })
})
