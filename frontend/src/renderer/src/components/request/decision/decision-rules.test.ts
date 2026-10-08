import { describe, expect, it } from 'vitest'
import {
  DECISION_RATIONALE_MIN_LENGTH, getDecisionGate, isRationaleValid, matchesConfirmation, normalizeTitle, requiresRationale
} from './decision-rules'
import type { Decision } from '../../../../../shared/request-artifact-types'

const dec = (over: Partial<Decision> = {}): Decision => ({
  id: 'd', displayId: 'D', subjectKind: 'solution_option', subjectId: 's', subjectDigest: 'x', rationale: '',
  riskLevel: 'normal', riskReasons: [], status: 'chosen', version: 1, ...over
})

describe('confirmation matching', () => {
  it('ignores case, outer whitespace and Unicode normalization form', () => {
    expect(matchesConfirmation('  DROP LEGACY table ', 'Drop legacy table')).toBe(true)
    expect(matchesConfirmation('Café', 'Café')).toBe(true)
    expect(matchesConfirmation('Drop legacy', 'Drop legacy table')).toBe(false)
    expect(matchesConfirmation('', '')).toBe(false)
    expect(normalizeTitle(' É ')).toBe('é')
  })
})

describe('rationale rules', () => {
  it('is required only for a different option than the recommendation', () => {
    expect(requiresRationale('b', 'a')).toBe(true)
    expect(requiresRationale('a', 'a')).toBe(false)
    expect(requiresRationale('a', undefined)).toBe(false)
    expect(requiresRationale(undefined, 'a')).toBe(false)
  })
  it('needs 10 characters after trim when required', () => {
    expect(DECISION_RATIONALE_MIN_LENGTH).toBe(10)
    expect(isRationaleValid('123456789', true)).toBe(false)
    expect(isRationaleValid('  1234567890  ', true)).toBe(true)
    expect(isRationaleValid('', false)).toBe(true)
  })
})

describe('getDecisionGate', () => {
  it('maps decision state to a gate', () => {
    expect(getDecisionGate(null, null, 'x')).toBe('noDecision')
    expect(getDecisionGate(dec({ status: 'superseded' }), null, 'x')).toBe('superseded')
    expect(getDecisionGate(dec(), null, 'other')).toBe('digestChanged')
    expect(getDecisionGate(dec({ riskLevel: 'high' }), { status: 'pending' }, 'x')).toBe('needsConfirmation')
    expect(getDecisionGate(dec({ riskLevel: 'high', status: 'effective' }), { status: 'pending' }, 'x')).toBe('ok')
    expect(getDecisionGate(dec(), { status: 'pending' }, 'x')).toBe('ok')
    expect(getDecisionGate(dec(), null, undefined, { selfChoiceForbidden: true })).toBe('selfChoiceForbidden')
  })
})

import { planBlockedByDecision } from './decision-rules'

describe('planBlockedByDecision', () => {
  it('blocks only when solution decisions exist and none is effective', () => {
    expect(planBlockedByDecision([])).toBe(false)
    expect(planBlockedByDecision([dec({ status: 'chosen' })])).toBe(true)
    expect(planBlockedByDecision([dec({ status: 'superseded' }), dec({ status: 'effective' })])).toBe(false)
    expect(planBlockedByDecision([dec({ subjectKind: 'plan', status: 'chosen' })])).toBe(false)
  })
})
