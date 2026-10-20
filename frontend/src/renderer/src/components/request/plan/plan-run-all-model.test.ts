import { describe, expect, it } from 'vitest'
import { canOfferRunAll, runAllDialogModel, type RunAllOfferInput } from './plan-run-all-model'
import { backendFlowOf, planApproval } from './plan-test-fixtures'
import { fallbackRequestFlow } from '../../../../../shared/request-flow-view'

const ok: RunAllOfferInput = {
  request: { executionMode: 'step_by_step' },
  approval: planApproval('ap1'),
  flow: backendFlowOf('change_request'),
  supported: true,
  approverHidden: false,
  alreadyPinned: false
}

describe('canOfferRunAll', () => {
  it('offers the choice when every condition holds', () => {
    expect(canOfferRunAll(ok)).toBe(true)
  })

  it('offers it for a request whose mode is still empty (not pinned yet)', () => {
    expect(canOfferRunAll({ ...ok, request: { executionMode: undefined } })).toBe(true)
  })

  it('offers it for a task-list approval, by rawSubjectType or subjectType', () => {
    expect(
      canOfferRunAll({ ...ok, approval: planApproval('t', { rawSubjectType: 'task_list' }) })
    ).toBe(true)
    expect(
      canOfferRunAll({ ...ok, approval: planApproval('t', { subjectType: 'task_list' as never }) })
    ).toBe(true)
  })

  it.each<[string, Partial<RunAllOfferInput>]>([
    ['no approval', { approval: null }],
    ['approval already approved', { approval: planApproval('a', { status: 'approved' }) }],
    ['approval rejected', { approval: planApproval('a', { status: 'rejected' }) }],
    ['a phase approval', { approval: planApproval('a', { subjectType: 'phase' }) }],
    ['a pre_deploy approval', { approval: planApproval('a', { subjectType: 'pre_deploy' }) }],
    ['no flow yet', { flow: null }],
    ['the built-in table answered', { flow: fallbackRequestFlow('change_request', 'L') }],
    ['a flow without phases', { flow: backendFlowOf('docs', 'M') }],
    ['already continuous', { request: { executionMode: 'continuous' } }],
    ['gateway without the flag', { supported: false }],
    ['approve hidden by the risk rule', { approverHidden: true }],
    ['pinned to step by step', { alreadyPinned: true }]
  ])('does not offer it with %s', (_name, patch) => {
    expect(canOfferRunAll({ ...ok, ...patch })).toBe(false)
  })

  it('keeps a raw task_list kind winning over a folded plan subject', () => {
    const folded = planApproval('t', { subjectType: 'plan', rawSubjectType: 'phase' as never })
    expect(canOfferRunAll({ ...ok, approval: folded })).toBe(false)
  })
})

describe('runAllDialogModel', () => {
  it('carries the step count and says nothing about pre-deploy for a plain flow', () => {
    expect(runAllDialogModel({ stepCount: 3, flow: backendFlowOf('change_request') })).toEqual({
      stepCount: 3,
      mentionsPreDeploy: false
    })
  })

  it('mentions pre-deploy for a start gate or an execution gate of that name', () => {
    expect(
      runAllDialogModel({ stepCount: 2, flow: backendFlowOf('security') }).mentionsPreDeploy
    ).toBe(true)
    expect(
      runAllDialogModel({
        stepCount: 2,
        flow: backendFlowOf('ops_request', 'L', { executionGates: ['pre_deploy'] })
      }).mentionsPreDeploy
    ).toBe(true)
  })

  it('does not mention it without a flow', () => {
    expect(runAllDialogModel({ stepCount: 2, flow: null }).mentionsPreDeploy).toBe(false)
  })
})
