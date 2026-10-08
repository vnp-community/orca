/**
 * Tests for request-stage-timeline-model.ts (CR-REQ-019-01)
 */

import { describe, it, expect } from 'vitest'
import {
  buildStageTimeline,
  isLowConfidence,
  CHILD_REQUEST_RULES
} from './request-stage-timeline-model'

describe('buildStageTimeline — type unknown or status unknown', () => {
  it('returns empty steps for unknown type', () => {
    const result = buildStageTimeline({ type: 'unknown', status: 'submitted' })
    expect(result.steps).toHaveLength(0)
    expect(result.current).toBeNull()
  })

  it('returns empty steps for unknown status', () => {
    const result = buildStageTimeline({ type: 'bug', status: 'unknown' })
    expect(result.steps).toHaveLength(0)
    expect(result.current).toBeNull()
  })
})

describe('buildStageTimeline — question (no plan step)', () => {
  it('does not include plan step', () => {
    const result = buildStageTimeline({ type: 'question', status: 'analyzing', size: 'M' })
    const ids = result.steps.map((s) => s.id)
    expect(ids).not.toContain('plan')
    expect(ids).toContain('analysis')
  })

  it('does not include phase step', () => {
    const result = buildStageTimeline({ type: 'question', status: 'analyzing', size: 'L' })
    const ids = result.steps.map((s) => s.id)
    expect(ids).not.toContain('phase')
  })
})

describe('buildStageTimeline — bug', () => {
  it('has phase step only when size=L', () => {
    const withL = buildStageTimeline({ type: 'bug', status: 'executing', size: 'L' })
    expect(withL.steps.map((s) => s.id)).toContain('phase')

    const withM = buildStageTimeline({ type: 'bug', status: 'executing', size: 'M' })
    expect(withM.steps.map((s) => s.id)).not.toContain('phase')
  })

  it('analysis step has variant "diagnosis"', () => {
    const result = buildStageTimeline({ type: 'bug', status: 'analyzing', size: 'M' })
    const analysis = result.steps.find((s) => s.id === 'analysis')
    expect(analysis?.variant).toBe('diagnosis')
  })
})

describe('buildStageTimeline — change_request', () => {
  it('always has phase step', () => {
    const result = buildStageTimeline({ type: 'change_request', status: 'submitted', size: 'S' })
    expect(result.steps.map((s) => s.id)).toContain('phase')
  })

  it('analysis step has variant "solution"', () => {
    const result = buildStageTimeline({ type: 'change_request', status: 'analyzing', size: 'M' })
    const analysis = result.steps.find((s) => s.id === 'analysis')
    expect(analysis?.variant).toBe('solution')
  })

  it('maps every status to the correct current step', () => {
    const cases: [string, string][] = [
      ['submitted', 'classification'],
      ['classifying', 'classification'],
      ['awaiting_type_confirmation', 'classification'],
      ['analyzing', 'analysis'],
      ['awaiting_analysis_approval', 'analysis'],
      ['awaiting_information', 'analysis'],
      ['planning', 'plan'],
      ['awaiting_plan_approval', 'plan'],
      ['executing', 'execution']
    ]
    for (const [status, expectedCurrentId] of cases) {
      const result = buildStageTimeline({ type: 'change_request', status: status as never })
      expect(result.current, `status=${status}`).toBe(expectedCurrentId)
    }
  })

  it('awaiting_information uses the clarification resumeStatus step when given', () => {
    const at = (resumeStatus?: string) =>
      buildStageTimeline({
        type: 'change_request',
        status: 'awaiting_information',
        size: 'M',
        resumeStatus
      }).current
    expect(at('planning')).toBe('plan')
    expect(at('executing')).toBe('execution')
    expect(at('bogus')).toBe('analysis')
    expect(at(undefined)).toBe('analysis')
    // resumeStatus is ignored outside awaiting_information
    expect(
      buildStageTimeline({ type: 'change_request', status: 'analyzing', resumeStatus: 'executing' })
        .current
    ).toBe('analysis')
  })

  it('cancelled marks current step as skipped', () => {
    const result = buildStageTimeline({ type: 'change_request', status: 'cancelled', size: 'M' })
    const current = result.steps.find((s) => s.id === result.current!)
    expect(current?.state).toBe('skipped')
  })
})

describe('buildStageTimeline — task/docs (no analysis)', () => {
  it('task does not have analysis step', () => {
    const result = buildStageTimeline({ type: 'task', status: 'executing', size: 'M' })
    expect(result.steps.map((s) => s.id)).not.toContain('analysis')
  })

  it('task plan step has variant "task_list"', () => {
    const result = buildStageTimeline({ type: 'task', status: 'planning', size: 'M' })
    const plan = result.steps.find((s) => s.id === 'plan')
    expect(plan?.variant).toBe('task_list')
  })
})

describe('buildStageTimeline — hotfix', () => {
  it('has note hotfix_fast_diagnosis', () => {
    const result = buildStageTimeline({ type: 'hotfix', status: 'analyzing', size: 'M' })
    expect(result.note).toBe('hotfix_fast_diagnosis')
  })

  it('has no phase step (never)', () => {
    const result = buildStageTimeline({ type: 'hotfix', status: 'executing', size: 'L' })
    expect(result.steps.map((s) => s.id)).not.toContain('phase')
  })
})

describe('buildStageTimeline — request_backlog', () => {
  it('preserves current at plan when returnedFromStage=plan', () => {
    const result = buildStageTimeline({
      type: 'change_request',
      status: 'request_backlog',
      returnedFromStage: 'plan'
    })
    expect(result.current).toBe('plan')
  })

  it('falls back to classification when returnedFromStage absent', () => {
    const result = buildStageTimeline({ type: 'change_request', status: 'request_backlog' })
    expect(result.current).toBe('classification')
  })
})

describe('isLowConfidence', () => {
  it('returns true for values below threshold (0.59)', () => {
    expect(isLowConfidence(0.59)).toBe(true)
  })

  it('returns false at exactly threshold (0.6)', () => {
    expect(isLowConfidence(0.6)).toBe(false)
  })

  it('returns true for undefined', () => {
    expect(isLowConfidence(undefined)).toBe(true)
  })

  it('returns false for values above threshold', () => {
    expect(isLowConfidence(0.9)).toBe(false)
  })
})

describe('CHILD_REQUEST_RULES', () => {
  it('spike has reason spawned_by_spike', () => {
    expect(CHILD_REQUEST_RULES.spike?.reason).toBe('spawned_by_spike')
  })

  it('question has reason spawned_by_question', () => {
    expect(CHILD_REQUEST_RULES.question?.reason).toBe('spawned_by_question')
  })

  it('hotfix has reason followup_hotfix', () => {
    expect(CHILD_REQUEST_RULES.hotfix?.reason).toBe('followup_hotfix')
  })
})
