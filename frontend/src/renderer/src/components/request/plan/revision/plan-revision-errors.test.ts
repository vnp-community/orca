import { describe, expect, it } from 'vitest'
import type { RequestRpcError } from '../../../../../../shared/request-errors'
import {
  classifyPlanRevisionError,
  classifyPlanRevisionKind,
  planRevisionMessage
} from './plan-revision-errors'
import { planRevisionSuggestions } from './plan-revision-suggestions'

const error = (over: Partial<RequestRpcError>): RequestRpcError => ({
  kind: 'unknown',
  code: 'runtime_error',
  message: 'x',
  ...over
})

describe('classifyPlanRevisionError', () => {
  it.each([
    [{ message: 'REQUEST_PLAN_FEEDBACK_TOO_LONG: too long', kind: 'validation' }, 'tooLong'],
    [{ message: 'REQUEST_STATE_STALE: stale', kind: 'conflict' }, 'wrongState'],
    [{ message: 'REQUEST_PLAN_REVISION_IN_PROGRESS: busy', kind: 'conflict' }, 'wrongState'],
    [{ message: 'REQUEST_PLAN_REVISION_IN_PROGRESS: busy' }, 'wrongState'],
    [{ kind: 'conflict' }, 'wrongState'],
    [{ message: 'REQUEST_PLAN_AI_UNAVAILABLE: none', kind: 'unavailable' }, 'noConnection'],
    [{ kind: 'no_dev_server' }, 'noConnection'],
    [{ message: 'REQUEST_PLAN_REVISION_NOT_APPLICABLE: hotfix' }, 'notApplicable'],
    [{ message: 'REQUEST_PLAN_REVISION_NO_CHANGE: same' }, 'noChange'],
    [{ message: 'REQUEST_BASE_REF_NOT_FOUND: gone' }, 'baseRefGone'],
    [{ message: 'REQUEST_PLAN_FORBIDDEN: no' }, 'forbidden'],
    [{ kind: 'forbidden' }, 'forbidden'],
    [{ kind: 'rate_limited' }, 'rateLimited'],
    [{ kind: 'network' }, 'network'],
    [{ kind: 'unavailable' }, 'other'],
    [{ kind: 'unknown' }, 'other']
  ] as [Partial<RequestRpcError>, string][])('maps %j to %s', (over, kind) => {
    expect(classifyPlanRevisionKind(error(over))).toBe(kind)
    expect(classifyPlanRevisionError(error(over)).message.trim()).not.toBe('')
  })

  it('names the limit in the too-long sentence', () => {
    expect(planRevisionMessage('tooLong')).toContain('2000')
  })

  it('tells who may ask for changes by whether the server saves the revised plan', () => {
    const forbidden = error({ message: 'REQUEST_PLAN_FORBIDDEN: no' })
    expect(classifyPlanRevisionError(forbidden, { serverCommit: true }).message).toMatch(
      /approvers of this plan/
    )
    expect(classifyPlanRevisionError(forbidden, { serverCommit: false }).message).toMatch(
      /reported this request or a project admin/
    )
  })

  it('says the text is kept for every refusal that leaves the box as it is', () => {
    for (const kind of ['forbidden', 'wrongState', 'noConnection', 'baseRefGone'] as const) {
      expect(planRevisionMessage(kind)).toMatch(/text is kept/)
    }
  })

  it('has a fixed sentence for the kinds the panel handles itself and none for the generic ones', () => {
    for (const kind of [
      'empty',
      'tooLong',
      'forbidden',
      'wrongState',
      'noConnection',
      'notApplicable',
      'noChange',
      'baseRefGone'
    ] as const) {
      expect(planRevisionMessage(kind)).toBeTruthy()
    }
    for (const kind of ['rateLimited', 'network', 'other'] as const) {
      expect(planRevisionMessage(kind)).toBeNull()
    }
  })

  it('does not echo the server message of a generic failure', () => {
    const result = classifyPlanRevisionError(error({ kind: 'unknown', message: 'SECRET-DETAIL' }))
    expect(result.message).not.toContain('SECRET-DETAIL')
  })
})

describe('planRevisionSuggestions', () => {
  it('offers four chips per engine and a native-style set for an unknown engine', () => {
    const openspec = planRevisionSuggestions('openspec')
    const native = planRevisionSuggestions('native')
    expect(openspec.map((s) => s.id)).toEqual([
      'tests',
      'splitBiggest',
      'tightenDesign',
      'edgeScenario'
    ])
    expect(native.map((s) => s.id)).toEqual(['splitLargest', 'rollback', 'riskyFirst', 'mergeTiny'])
    expect(planRevisionSuggestions(undefined)).toEqual(native)
    for (const s of [...openspec, ...native]) {
      expect(s.text.trim()).not.toBe('')
    }
  })
})
