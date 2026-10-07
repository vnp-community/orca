/**
 * Tests for request-flow-registry.ts
 */

import { describe, it, expect } from 'vitest'
import { REQUEST_FLOW_REGISTRY, flowHasPhase, LOW_CONFIDENCE_THRESHOLD } from './request-flow-registry'

describe('REQUEST_FLOW_REGISTRY', () => {
  it('has exactly 11 entries (one per RequestType excluding unknown)', () => {
    expect(Object.keys(REQUEST_FLOW_REGISTRY)).toHaveLength(11)
  })

  // Spot-check every row matches README v6 §3.4
  it.each([
    ['bug', 'diagnosis', 'task_list', 'size_l', false, true, false],
    ['task', null, 'task_list', 'never', false, true, false],
    ['docs', null, 'task_list', 'never', false, false, false],
    ['question', 'answer', 'none', 'never', false, false, false],
    ['hotfix', 'diagnosis', 'single_task', 'never', false, false, true],
    ['security', 'findings', 'plan', 'size_l', true, true, true],
    ['ops_request', null, 'task_list', 'never', false, true, false],
    ['change_request', 'solution', 'plan', 'always', true, true, true],
    ['refactor', 'solution', 'plan', 'size_l', true, true, false],
    ['spike', 'findings', 'none', 'never', true, false, false],
    ['performance', 'findings', 'task_list', 'size_l', true, true, false]
  ] as const)(
    '%s: analysisKind=%s plan=%s phase=%s analysisApproval=%s planApproval=%s preDeployApproval=%s',
    (type, analysisKind, plan, phase, analysisApproval, planApproval, preDeployApproval) => {
      const entry = REQUEST_FLOW_REGISTRY[type]
      expect(entry.analysisKind).toBe(analysisKind)
      expect(entry.plan).toBe(plan)
      expect(entry.phase).toBe(phase)
      expect(entry.gates.analysisApproval).toBe(analysisApproval)
      expect(entry.gates.planApproval).toBe(planApproval)
      expect(entry.gates.preDeployApproval).toBe(preDeployApproval)
    }
  )
})

describe('flowHasPhase', () => {
  it('bug+L → true (size_l)', () => {
    expect(flowHasPhase('bug', 'L')).toBe(true)
  })

  it('bug+M → false (size_l rule, not L)', () => {
    expect(flowHasPhase('bug', 'M')).toBe(false)
  })

  it('bug+S → false', () => {
    expect(flowHasPhase('bug', 'S')).toBe(false)
  })

  it('change_request+S → true (always)', () => {
    expect(flowHasPhase('change_request', 'S')).toBe(true)
  })

  it('change_request+M → true (always)', () => {
    expect(flowHasPhase('change_request', 'M')).toBe(true)
  })

  it('change_request+L → true (always)', () => {
    expect(flowHasPhase('change_request', 'L')).toBe(true)
  })

  it('question → always false (never)', () => {
    expect(flowHasPhase('question', 'L')).toBe(false)
  })

  it('task → always false (never)', () => {
    expect(flowHasPhase('task', 'L')).toBe(false)
  })

  it('unknown → false', () => {
    expect(flowHasPhase('unknown', 'L')).toBe(false)
  })
})

describe('LOW_CONFIDENCE_THRESHOLD', () => {
  it('is 0.6', () => {
    expect(LOW_CONFIDENCE_THRESHOLD).toBe(0.6)
  })
})
