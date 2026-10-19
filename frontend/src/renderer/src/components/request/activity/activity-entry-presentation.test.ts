import { describe, expect, it } from 'vitest'
import type { RequestActivityEntry } from '../../../../../shared/request-execution-types'
import {
  ACTIVITY_GROUP_KINDS,
  activityAttempt,
  activityIcon,
  activityTargetTab,
  activityTaskName,
  activityTone,
  groupOfKind,
  isKnownActivityKind
} from './activity-entry-presentation'

const entry = (over: Partial<RequestActivityEntry> = {}): RequestActivityEntry => ({
  id: 'e1',
  kind: 'task.failed',
  occurredAt: '2026-10-16T09:00:00Z',
  actorKind: 'system',
  summary: 's',
  severity: 'info',
  details: {},
  ...over
})

describe('groupOfKind', () => {
  it.each([
    ['approval.requested', 'approvals'],
    ['approval.decided', 'approvals'],
    ['task.failed', 'tasks'],
    ['task.accepted', 'tasks'],
    ['phase.started', 'tasks'],
    ['plan.done', 'tasks'],
    ['clarification.asked', 'clarifications'],
    ['clarification.answered', 'clarifications'],
    ['request.type_changed', 'changes'],
    ['request.classified', 'changes'],
    ['analysis.generated', 'changes'],
    ['request.returned', 'changes'],
    ['request.created', null],
    ['something.new', null]
  ])('%s -> %s', (kind, group) => {
    expect(groupOfKind(kind)).toBe(group)
  })

  it('never lists a kind in two groups', () => {
    const all = Object.values(ACTIVITY_GROUP_KINDS).flat()
    expect(new Set(all).size).toBe(all.length)
  })
})

describe('activityTone', () => {
  it.each([
    ['task.failed', 'error', {}, 'error'],
    ['task.failed', 'attention', {}, 'attention'],
    ['request.returned', 'error', {}, 'error'],
    ['task.succeeded', 'info', {}, 'success'],
    ['task.accepted', 'info', {}, 'success'],
    ['plan.done', 'info', {}, 'success'],
    ['approval.decided', 'info', { result: 'approved' }, 'success'],
    ['approval.decided', 'attention', { result: 'rejected' }, 'attention'],
    ['task.started', 'info', {}, 'neutral'],
    ['something.new', 'info', {}, 'neutral']
  ])('%s/%s %j -> %s', (kind, severity, details, tone) => {
    expect(
      activityTone(entry({ kind, severity: severity as RequestActivityEntry['severity'], details }))
    ).toBe(tone)
  })
})

describe('activityIcon', () => {
  it('has an icon for every grouped kind and for an unknown kind', () => {
    for (const kind of [
      ...Object.values(ACTIVITY_GROUP_KINDS).flat(),
      'request.created',
      'something.new'
    ]) {
      expect(activityIcon(entry({ kind }))).toBeTruthy()
    }
  })

  it('uses another shape for a rejected approval', () => {
    const approved = activityIcon(
      entry({ kind: 'approval.decided', details: { result: 'approved' } })
    )
    const rejected = activityIcon(
      entry({ kind: 'approval.decided', details: { result: 'rejected' } })
    )
    expect(rejected).not.toBe(approved)
  })

  it('knows which kinds are built in', () => {
    expect(isKnownActivityKind('task.failed')).toBe(true)
    expect(isKnownActivityKind('task.exploded')).toBe(false)
  })
})

describe('activityTaskName', () => {
  it.each([
    [{ taskTitle: 'Fix login', taskNumber: 12 }, '#TG-12 Fix login'],
    [{ taskTitle: 'Fix login' }, 'Fix login'],
    [{ taskNumber: 7 }, '#TG-7'],
    [{}, undefined],
    [{ taskTitle: '   ' }, undefined]
  ])('%j -> %s', (details, expected) => {
    expect(activityTaskName({ details })).toBe(expected)
  })
})

describe('activityAttempt', () => {
  it.each([
    [
      { attempt: 2, details: { maxAttempts: 3 } },
      { attempt: 2, maxAttempts: 3 }
    ],
    [{ attempt: 2, details: {} }, { attempt: 2 }],
    [{ details: { attempt: 1, maxAttempts: 2 } }, { attempt: 1, maxAttempts: 2 }],
    [{ attempt: 4, details: { maxAttempts: 3 } }, { attempt: 4 }],
    [{ attempt: 0, details: {} }, undefined],
    [{ details: {} }, undefined]
  ])('%j -> %j', (input, expected) => {
    expect(activityAttempt(input)).toEqual(expected)
  })
})

describe('activityTargetTab', () => {
  it.each([
    ['task.failed', 'task', 'execution'],
    ['phase.started', 'phase', 'execution'],
    ['plan.done', 'plan', 'execution'],
    ['approval.requested', 'pre_deploy', 'execution'],
    ['approval.decided', 'plan', 'plan'],
    ['approval.decided', 'phase', 'plan'],
    ['approval.decided', 'solution', 'analysis'],
    ['analysis.generated', 'solution', 'analysis'],
    ['clarification.asked', 'clarification', 'overview']
  ] as const)('%s on %s opens %s', (kind, subjectType, tab) => {
    expect(activityTargetTab({ kind, subjectType })).toBe(tab)
  })

  it('lines with nowhere to go have no tab', () => {
    expect(activityTargetTab({ kind: 'request.created', subjectType: 'request' })).toBeNull()
    expect(activityTargetTab({ kind: 'something.new', subjectType: undefined })).toBeNull()
  })
})
