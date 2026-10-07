/**
 * Tests for solution-view-model.ts (CR-REQ-020-01)
 */

import { describe, it, expect } from 'vitest'
import {
  getSolutionPresentation,
  buildComparisonRows,
  canApproveSolution,
  validateRejectReason,
  clampFeedback,
  pickPendingApproval,
  REJECT_REASON_MIN_LENGTH,
  FEEDBACK_MAX_LENGTH
} from './solution-view-model'
import type { Solution, SolutionOption, Approval } from '../../../../../shared/request-types'

function makeSolution(overrides: Partial<Solution> = {}): Solution {
  return {
    id: 'sol-1',
    requestId: 'req-1',
    kind: 'solution',
    status: 'ready',
    ...overrides
  }
}

function makeApproval(overrides: Partial<Approval> = {}): Approval {
  return {
    id: 'apv-1',
    requestId: 'req-1',
    subjectType: 'solution',
    subjectId: 'sol-1',
    status: 'pending',
    createdAt: '',
    updatedAt: '',
    ...overrides
  }
}

describe('getSolutionPresentation', () => {
  it('returns readOnly=true for post-analysis status', () => {
    const result = getSolutionPresentation({
      solution: makeSolution({ status: 'chosen' }),
      requestStatus: 'planning',
      requestType: 'change_request',
      hasPendingApproval: false
    })
    expect(result.readOnly).toBe(true)
    expect(result.actions).toHaveLength(0)
  })

  it('returns readOnly=true for cancelled', () => {
    const result = getSolutionPresentation({
      solution: makeSolution({ status: 'ready' }),
      requestStatus: 'cancelled',
      requestType: 'bug',
      hasPendingApproval: false
    })
    expect(result.readOnly).toBe(true)
  })

  it('returns readOnly=true for hotfix type', () => {
    const result = getSolutionPresentation({
      solution: makeSolution({ status: 'ready' }),
      requestStatus: 'analyzing',
      requestType: 'hotfix',
      hasPendingApproval: false
    })
    expect(result.readOnly).toBe(true)
  })

  it('generating: no actions', () => {
    const result = getSolutionPresentation({
      solution: makeSolution({ status: 'generating' }),
      requestStatus: 'analyzing',
      requestType: 'bug',
      hasPendingApproval: false
    })
    expect(result.actions).toHaveLength(0)
    expect(result.tone).toBe('info')
  })

  it('ready without pending approval: choose + regenerate', () => {
    const result = getSolutionPresentation({
      solution: makeSolution({ status: 'ready' }),
      requestStatus: 'analyzing',
      requestType: 'bug',
      hasPendingApproval: false
    })
    expect(result.actions).toContain('choose')
    expect(result.actions).toContain('regenerate')
  })

  it('ready with pending approval: approve + reject', () => {
    const result = getSolutionPresentation({
      solution: makeSolution({ status: 'ready' }),
      requestStatus: 'awaiting_analysis_approval',
      requestType: 'change_request',
      hasPendingApproval: true
    })
    expect(result.actions).toContain('approve')
    expect(result.actions).toContain('reject')
  })

  it('chosen: generatePlan action', () => {
    const result = getSolutionPresentation({
      solution: makeSolution({ status: 'chosen' }),
      requestStatus: 'awaiting_analysis_approval',
      requestType: 'bug',
      hasPendingApproval: false
    })
    expect(result.actions).toContain('generatePlan')
  })

  it('rejected: regenerate action', () => {
    const result = getSolutionPresentation({
      solution: makeSolution({ status: 'rejected' }),
      requestStatus: 'analyzing',
      requestType: 'change_request',
      hasPendingApproval: false
    })
    expect(result.actions).toContain('regenerate')
  })
})

describe('buildComparisonRows', () => {
  const opt1: SolutionOption = {
    id: 'o1', title: 'A',
    summary: 'Fast', pros: ['quick'], cons: ['risky'], estimatedEffort: '1d'
  }
  const opt2: SolutionOption = {
    id: 'o2', title: 'B',
    summary: 'Slow', pros: ['safe'], cons: ['expensive'], estimatedEffort: '3d'
  }

  it('builds rows for all 5 criteria', () => {
    const rows = buildComparisonRows([opt1, opt2])
    expect(rows).toHaveLength(5)
    expect(rows.map((r) => r.criterion)).toEqual(['summary', 'pros', 'cons', 'effort', 'risk'])
  })

  it('marks differs=true when options differ', () => {
    const rows = buildComparisonRows([opt1, opt2])
    const summary = rows.find((r) => r.criterion === 'summary')!
    expect(summary.differs).toBe(true)
  })

  it('marks differs=false when options are identical', () => {
    const sameOpt = { ...opt1, id: 'o2' }
    const rows = buildComparisonRows([opt1, sameOpt])
    const summary = rows.find((r) => r.criterion === 'summary')!
    expect(summary.differs).toBe(false)
  })

  it('missing pros/cons become empty string', () => {
    const sparseOpt: SolutionOption = { id: 'o1', title: 'T' }
    const rows = buildComparisonRows([sparseOpt])
    const pros = rows.find((r) => r.criterion === 'pros')!
    expect(pros.cells[0]).toBe('')
  })
})

describe('canApproveSolution', () => {
  it('change_request + solution kind with 1 option → needTwoOptions', () => {
    const result = canApproveSolution({
      kind: 'solution',
      requestType: 'change_request',
      options: [{ id: 'o1', title: 'Only one' }],
      chosenOptionId: undefined
    })
    expect(result.ok).toBe(false)
    expect(result.reasonKey).toBe('needTwoOptions')
  })

  it('change_request + solution kind with 2 options but no chosen → chooseOne', () => {
    const result = canApproveSolution({
      kind: 'solution',
      requestType: 'change_request',
      options: [{ id: 'o1', title: 'A' }, { id: 'o2', title: 'B' }],
      chosenOptionId: undefined
    })
    expect(result.ok).toBe(false)
    expect(result.reasonKey).toBe('chooseOne')
  })

  it('diagnosis kind → always ok (no option requirements)', () => {
    const result = canApproveSolution({
      kind: 'diagnosis',
      requestType: 'bug',
      options: undefined,
      chosenOptionId: undefined
    })
    expect(result.ok).toBe(true)
  })
})

describe('validateRejectReason', () => {
  it('returns ok=false for whitespace only', () => {
    expect(validateRejectReason('         ').ok).toBe(false)
  })

  it('returns ok=false for short text', () => {
    expect(validateRejectReason('ngắn').ok).toBe(false)
  })

  it('returns ok=true for exactly 10 chars after trim', () => {
    const tenChars = 'a'.repeat(REJECT_REASON_MIN_LENGTH)
    expect(validateRejectReason(`  ${tenChars}  `).ok).toBe(true)
  })

  it('counts Unicode code points not bytes', () => {
    // Each emoji is 1 code point in JS spread
    const tenEmoji = '😀'.repeat(10)
    const result = validateRejectReason(tenEmoji)
    expect(result.ok).toBe(true)
    expect(result.length).toBe(10)
  })
})

describe('clampFeedback', () => {
  it('returns text unchanged when under limit', () => {
    const text = 'hello'
    expect(clampFeedback(text)).toBe(text)
  })

  it('truncates text over 2000 code points', () => {
    const long = 'a'.repeat(FEEDBACK_MAX_LENGTH + 10)
    const result = clampFeedback(long)
    expect([...result].length).toBe(FEEDBACK_MAX_LENGTH)
  })
})

describe('pickPendingApproval', () => {
  it('prefers exact subjectId match', () => {
    const solution = makeSolution({ id: 'sol-2' })
    const exact = makeApproval({ subjectId: 'sol-2' })
    const other = makeApproval({ id: 'apv-2', subjectId: 'sol-99' })
    const result = pickPendingApproval([other, exact], solution)
    expect(result?.id).toBe('apv-1')
  })

  it('falls back to any pending same-type approval when no exact match', () => {
    const solution = makeSolution({ id: 'sol-missing' })
    const fallback = makeApproval({ subjectId: 'sol-other' })
    const result = pickPendingApproval([fallback], solution)
    expect(result?.id).toBe('apv-1')
  })

  it('returns null when no pending approvals', () => {
    const solution = makeSolution()
    const approved = makeApproval({ status: 'approved' })
    expect(pickPendingApproval([approved], solution)).toBeNull()
  })
})
