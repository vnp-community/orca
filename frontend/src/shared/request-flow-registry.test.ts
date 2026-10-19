/**
 * Tests for request-flow-registry.ts
 */

import { describe, it, expect } from 'vitest'
import {
  REQUEST_FLOW_REGISTRY,
  flowHasPhase,
  LOW_CONFIDENCE_THRESHOLD
} from './request-flow-registry'
import fixture from './request-flow-registry.fixture.json'

type FixtureEntry = {
  analysisKind: string
  analysisGate: string
  planKind: string
  phaseRule: string
  startGate: string
  executionGates: string[]
  completesAfterAnalysis: boolean
  humanConfirmRequired: boolean
  openSpecProfile: string
}

// Why: the table is generated from request-service `domain.FlowFor` (a Go test fails when the fixture is stale), so this
// comparison is what keeps the fallback the screens use without `request.flow` from drifting from the backend.
const FIXTURE = fixture as Record<string, FixtureEntry>
const PHASE_RULES: Record<string, string> = {
  never: 'never',
  always: 'always',
  when_size_L: 'size_l'
}

describe('REQUEST_FLOW_REGISTRY', () => {
  it('has exactly 11 entries (one per RequestType excluding unknown)', () => {
    expect(Object.keys(REQUEST_FLOW_REGISTRY)).toHaveLength(11)
    expect(Object.keys(FIXTURE).sort()).toEqual(Object.keys(REQUEST_FLOW_REGISTRY).sort())
  })

  it.each(Object.keys(FIXTURE))('%s matches the backend registry fixture', (type) => {
    const want = FIXTURE[type]
    const got = REQUEST_FLOW_REGISTRY[type as keyof typeof REQUEST_FLOW_REGISTRY]
    expect(got.analysisKind ?? 'none').toBe(want.analysisKind)
    expect(got.analysisGate ?? '').toBe(want.analysisGate)
    expect(got.plan).toBe(want.planKind)
    expect(got.phase).toBe(PHASE_RULES[want.phaseRule])
    expect(got.startGate ?? '').toBe(want.startGate)
    expect([...got.executionGates]).toEqual(want.executionGates)
    expect(got.completesAfterAnalysis).toBe(want.completesAfterAnalysis)
    expect(got.humanConfirmRequired).toBe(want.humanConfirmRequired)
    expect(got.openSpecProfile).toBe(want.openSpecProfile)
  })

  // The three yes/no questions older screens read are derived from the gates, so check the cases that were wrong before.
  it.each([
    ['change_request', true, true, false],
    ['docs', false, true, false],
    ['security', true, false, true],
    ['hotfix', false, false, true],
    ['ops_request', false, true, true],
    ['question', true, false, false],
    ['spike', true, false, false]
  ] as const)(
    '%s: analysisApproval=%s planApproval=%s preDeployApproval=%s',
    (type, analysis, plan, preDeploy) => {
      const g = REQUEST_FLOW_REGISTRY[type].gates
      expect(g.analysisApproval).toBe(analysis)
      expect(g.planApproval).toBe(plan)
      expect(g.preDeployApproval).toBe(preDeploy)
    }
  )

  it('only change_request has a Phase gate in executing', () => {
    const withPhaseGate = Object.entries(REQUEST_FLOW_REGISTRY)
      .filter(([, e]) => e.executionGates.includes('phase'))
      .map(([t]) => t)
    expect(withPhaseGate).toEqual(['change_request'])
  })
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
