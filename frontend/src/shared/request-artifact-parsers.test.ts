import { describe, expect, it } from 'vitest'
import {
  parseClarification, parseDecision, parseExecutionResult, parseImpactComparison, parseImpactDrift,
  parseImpactFinding, parseImpactSummary, parseTaskReadinessReport
} from './request-artifact-parsers'
import { ARTIFACT_EVENT_TYPES } from './request-artifact-types'
import { isInterruptStatus } from './request-flow-registry'
import { classifyRequestRpcError } from './request-errors'

describe('parseClarification', () => {
  it('normalises both kind families, sorts by seq and parses JSON-in-string fields', () => {
    const c = parseClarification({
      id: 'c1', display_id: 'CL-1', request_id: 'r1', source: 'readiness', status: 'open', round: 2, version: 3,
      resume_status: 'planning', due_at: '2026-10-08T00:00:00Z', assignee_ids: ['u1'],
      questions: [
        { id: 'q2', seq: 2, kind: 'CONFIRM', prompt: 'Sure?' },
        { id: 'q1', seq: 1, kind: 'single', prompt: 'Pick', options_json: '["a","b"]', suggested_default_json: '"a"', required: false },
        { id: 'q3', seq: 3, kind: 'MULTI_CHOICE', prompt: 'Many', options: [{ value: 'x', label: 'X' }], answer_json: '{broken' }
      ]
    })
    expect(c?.questions.map((q) => [q.id, q.kind])).toEqual([['q1', 'single_choice'], ['q2', 'boolean'], ['q3', 'multi_choice']])
    expect(c?.questions[0]).toMatchObject({ options: [{ value: 'a', label: 'a' }, { value: 'b', label: 'b' }], suggestedDefault: 'a', required: false })
    expect(c?.questions[2].answer).toBe('{broken')
    expect(c).toMatchObject({ displayId: 'CL-1', requestId: 'r1', resumeStatus: 'planning', assigneeIds: ['u1'], round: 2 })
  })

  it('maps unknown enums to unknown and drops items without id', () => {
    expect(parseClarification({ id: 'c', source: 'alien', status: 'weird', questions: [{ prompt: 'no id' }] })).toMatchObject({ source: 'unknown', status: 'unknown', questions: [] })
    expect(parseClarification({})).toBeNull()
    expect(parseClarification(null)).toBeNull()
  })
})

describe('parseDecision', () => {
  it('parses snake_case and defaults risk to normal', () => {
    expect(parseDecision({ id: 'd', status: 'effective', risk_level: 'high', chosen_option_id: 'o1', subject_digest: 'x' })).toMatchObject({ status: 'effective', riskLevel: 'high', chosenOptionId: 'o1', subjectDigest: 'x' })
    expect(parseDecision({ id: 'd', status: 'zzz' })).toMatchObject({ status: 'unknown', riskLevel: 'normal' })
    expect(parseDecision(undefined)).toBeNull()
  })
})

describe('impact parsers', () => {
  it('summary: missing level is unknown (not low), max three reasons', () => {
    const s = parseImpactSummary({ assessmentId: 'a', top_reasons: ['1', '2', '3', '4'], status: 'nope', mode: 'enforce', stale: true })
    expect(s).toMatchObject({ level: 'unknown', status: 'unknown', mode: 'enforce', stale: true, confidence: null, score: null })
    expect(s?.topReasons).toHaveLength(3)
    expect(parseImpactSummary({ level: 'high' })).toBeNull()
  })
  it('finding / comparison / drift', () => {
    expect(parseImpactFinding({ id: 'f', level: 'critical', dimension: 'data', node_ids: ['n'] })).toMatchObject({ level: 'critical', nodeIds: ['n'] })
    expect(parseImpactFinding({ title: 'x' })).toBeNull()
    expect(parseImpactComparison({ option_id: 'o', dimensions: { data: { risk: 'high', score: 3 }, junk: 'x' } })).toEqual({ optionId: 'o', dimensions: { data: { level: 'high', score: 3 } } })
    expect(parseImpactDrift({ phase_id: 'p', items: [{ task_id: 't', expected: 'a', actual: 'b' }] })).toMatchObject({ drifted: true })
  })
})

describe('readiness / execution parsers', () => {
  it('readiness: unknown outcome stays unknown', () => {
    expect(parseTaskReadinessReport({ task_id: 't', outcome: 'maybe', findings: [{ code: 'C', tier: 'structure', message: 'm' }, { message: 'no code' }] })).toMatchObject({ outcome: 'unknown', findings: [{ code: 'C' }] })
    expect(parseTaskReadinessReport({ outcome: 'ready' })).toBeNull()
  })
  it('execution result: invalid parse status falls back, verdict parsed', () => {
    const r = parseExecutionResult({ task_id: 't', parse_status: 'ok', status: 'failed', files_changed: ['a'], checks_run: [{ id: 'lint', exit: 1 }], verdict: { status: 'passed', findings: [{ code: 'CHECK_MISMATCH', message: 'm' }] }, failure_class: 'weird' })
    expect(r).toMatchObject({ parseStatus: 'ok', failureClass: 'unknown', checksRun: [{ id: 'lint', exit: 1 }], verdict: { status: 'passed' } })
    expect(parseExecutionResult({ task_id: 't', parse_status: 'zzz' })?.parseStatus).toBe('invalid')
  })
})

describe('registry and errors additions', () => {
  it('awaiting_information is an interrupt status', () => {
    expect(isInterruptStatus('awaiting_information')).toBe(true)
    expect(isInterruptStatus('planning')).toBe(false)
    expect(ARTIFACT_EVENT_TYPES).toContain('impact.assessed')
  })
  it.each([
    ['REQUEST_CLARIFICATION_NOT_ASSIGNEE', 'forbidden'],
    ['REQUEST_CLARIFICATION_VERSION_CONFLICT', 'conflict'],
    ['REQUEST_CLARIFICATION_EXPIRED', 'expired'],
    ['REQUEST_DECISION_CONFIRMATION_MISMATCH', 'validation'],
    ['REQUEST_RISK_ASSESSMENT_PENDING', 'pending'],
    ['REQUEST_IMPACT_NO_CONNECTION', 'no_dev_server']
  ])('classifies %s as %s', (code, kind) => {
    expect(classifyRequestRpcError({ code: 'internal', message: `${code}: x` }).kind).toBe(kind)
  })
})
