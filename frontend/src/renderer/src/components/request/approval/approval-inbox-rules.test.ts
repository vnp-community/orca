import { describe, expect, it } from 'vitest'
import {
  canQuickApprove, compareApprovals, groupByRequest, isOverdue, matchesGroup, openTargetFor,
  quickApproveConsequenceKey, serverSubjectTypeFor, subjectKindOf, SUBJECT_GROUPS
} from './approval-inbox-rules'
import type { Approval } from '../../../../../shared/request-types'

const NOW = Date.parse('2026-10-07T12:00:00Z')
const ap = (over: Partial<Approval> & { id: string }): Approval => ({
  requestId: 'r1', subjectType: 'solution', subjectId: 's', status: 'pending', subjectDigest: 'dg',
  createdAt: '2026-10-01T00:00:00Z', updatedAt: '2026-10-01T00:00:00Z', ...over
})
const kind = (k: string): Approval => ap({ id: k, rawSubjectType: k })

describe('canQuickApprove', () => {
  const table: [string, boolean][] = [
    ['request_type', true], ['solution', false], ['findings', true], ['answer', true],
    ['plan', false], ['phase', true], ['task_list', true], ['pre_deploy', true], ['weird', false]
  ]
  it.each(table)('%s -> %s', (k, expected) => {
    expect(canQuickApprove(kind(k))).toBe(expected)
  })
  it('needs a digest and a pending status', () => {
    expect(canQuickApprove({ ...kind('phase'), subjectDigest: undefined })).toBe(false)
    expect(canQuickApprove({ ...kind('phase'), status: 'approved' })).toBe(false)
  })
})

describe('compareApprovals', () => {
  it('orders overdue, nearest due, no due', () => {
    const rows = [
      ap({ id: 'none' }),
      ap({ id: 'soon', dueAt: '2026-10-08T00:00:00Z' }),
      ap({ id: 'late', dueAt: '2026-10-06T00:00:00Z' })
    ]
    expect(rows.sort(compareApprovals(NOW)).map((r) => r.id)).toEqual(['late', 'soon', 'none'])
  })
  it('breaks ties by newest created then id', () => {
    const rows = [
      ap({ id: 'b', dueAt: '2026-10-09T00:00:00Z', createdAt: '2026-10-01T00:00:00Z' }),
      ap({ id: 'a', dueAt: '2026-10-09T00:00:00Z', createdAt: '2026-10-02T00:00:00Z' }),
      ap({ id: 'c', dueAt: '2026-10-09T00:00:00Z', createdAt: '2026-10-02T00:00:00Z' })
    ]
    expect(rows.sort(compareApprovals(NOW)).map((r) => r.id)).toEqual(['a', 'c', 'b'])
  })
  it('treats expiresAt as the due date when dueAt is absent', () => {
    expect(isOverdue(ap({ id: 'x', expiresAt: '2026-10-01T00:00:00Z' }), NOW)).toBe(true)
  })
})

describe('grouping and filters', () => {
  it('keeps the position of the first row of each request', () => {
    const rows = [ap({ id: '1', requestId: 'A' }), ap({ id: '2', requestId: 'B' }), ap({ id: '3', requestId: 'A' })]
    expect(groupByRequest(rows).map((g) => [g.requestId, g.rows.map((r) => r.id)])).toEqual([
      ['A', ['1', '3']], ['B', ['2']]
    ])
  })
  it('maps only single-type groups to the server', () => {
    expect(SUBJECT_GROUPS.map(serverSubjectTypeFor)).toEqual([
      undefined, 'request_type', 'solution', undefined, 'phase', 'pre_deploy', undefined
    ])
  })
  it('groups task_list with plan and findings/answer with other', () => {
    expect(matchesGroup(kind('task_list'), 'plan')).toBe(true)
    expect(matchesGroup(kind('findings'), 'other')).toBe(true)
    expect(matchesGroup(kind('plan'), 'solution')).toBe(false)
    expect(matchesGroup(kind('plan'), 'all')).toBe(true)
  })
  it('falls back to subjectType when no raw value exists', () => {
    expect(subjectKindOf(ap({ id: 'x', subjectType: 'phase' }))).toBe('phase')
  })
})

describe('openTargetFor', () => {
  const table: [string, string | undefined][] = [
    ['request_type', 'type_confirmation'], ['solution', 'analysis'], ['findings', 'analysis'],
    ['answer', 'analysis'], ['plan', 'plan'], ['task_list', 'plan'], ['phase', 'plan'],
    ['pre_deploy', 'plan'], ['weird', undefined]
  ]
  it.each(table)('%s -> %s', (k, focus) => {
    expect(openTargetFor(kind(k))).toEqual({ section: 'requests', requestId: 'r1', focus })
  })
  it('builds the consequence i18n key', () => {
    expect(quickApproveConsequenceKey(kind('phase'))).toBe('auto.components.request.approval.ApprovalRow.confirmApprove.phase')
  })
})
