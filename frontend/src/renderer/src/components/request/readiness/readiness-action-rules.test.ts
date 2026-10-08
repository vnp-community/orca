import { describe, expect, it } from 'vitest'
import { getReadinessAction, getReadinessPresentation, groupFindingsByTier, isRunBlockedByReadiness } from './readiness-action-rules'
import type { TaskReadinessOutcome, TaskReadinessReport } from '../../../../../shared/request-artifact-types'

const report = (outcome: TaskReadinessOutcome): TaskReadinessReport => ({ taskId: 't', outcome, findings: [] })

describe('readiness presentation and blocking', () => {
  it('has a badge for the four outcomes and none for unknown', () => {
    for (const o of ['ready', 'needs_info', 'spec_defect', 'env_defect'] as const) {expect(getReadinessPresentation(o)?.Icon).toBeTruthy()}
    expect(getReadinessPresentation('unknown')).toBeNull()
  })
  it('blocks Run only for a checked, non-ready report on a supporting runtime', () => {
    expect(isRunBlockedByReadiness(null, true)).toBe(false)
    expect(isRunBlockedByReadiness(report('needs_info'), false)).toBe(false)
    expect(isRunBlockedByReadiness(report('ready'), true)).toBe(false)
    expect(isRunBlockedByReadiness(report('unknown'), true)).toBe(false)
    for (const o of ['needs_info', 'spec_defect', 'env_defect'] as const) {expect(isRunBlockedByReadiness(report(o), true)).toBe(true)}
  })
})

describe('getReadinessAction', () => {
  const ctx = { canWrite: true, hasDevServer: false }
  it('maps each outcome to its action', () => {
    expect(getReadinessAction(report('ready'), ctx).kind).toBe('run')
    expect(getReadinessAction(report('needs_info'), ctx).kind).toBe('answer')
    expect(getReadinessAction(report('spec_defect'), ctx).kind).toBe('regenerate')
    expect(getReadinessAction(report('env_defect'), ctx)).toMatchObject({ kind: 'connect', noteKey: expect.stringContaining('notCounted') })
    expect(getReadinessAction(report('env_defect'), { ...ctx, hasDevServer: true }).kind).toBe('notify')
  })
  it('offers nothing without a report or write access', () => {
    expect(getReadinessAction(null, ctx).kind).toBe('none')
    expect(getReadinessAction(report('ready'), { ...ctx, canWrite: false }).kind).toBe('none')
    expect(getReadinessAction(report('unknown'), ctx).kind).toBe('none')
  })
})

describe('groupFindingsByTier', () => {
  it('orders structure, semantic, environment, then others', () => {
    const f = (tier: string) => ({ code: tier, tier, message: '' })
    expect(groupFindingsByTier([f('environment'), f('zzz'), f('structure'), f('semantic')]).map((g) => g.tier)).toEqual(['structure', 'semantic', 'environment', 'zzz'])
  })
})
