// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import type { RequestActivityEntry } from '../../../../../shared/request-execution-types'
import { activityAttemptLabel, activityDetailText, activityTitle } from './activity-entry-text'

const entry = (over: Partial<RequestActivityEntry>): RequestActivityEntry => ({
  id: 'e1',
  kind: 'task.failed',
  occurredAt: '2026-10-16T09:00:00Z',
  actorKind: 'system',
  summary: 'backend summary',
  severity: 'info',
  details: {},
  ...over
})

describe('activityTitle', () => {
  it.each<[string, Partial<RequestActivityEntry>, string]>([
    [
      'task failed with number',
      { kind: 'task.failed', details: { taskTitle: 'Fix login', taskNumber: 3 } },
      '#TG-3 Fix login failed'
    ],
    ['task failed without a name', { kind: 'task.failed' }, 'A task failed'],
    [
      'task accepted',
      { kind: 'task.accepted', details: { taskTitle: 'Fix login' } },
      'Fix login marked done'
    ],
    [
      'task started',
      { kind: 'task.started', details: { taskTitle: 'Fix login' } },
      'Fix login started'
    ],
    [
      'approval approved',
      { kind: 'approval.decided', subjectType: 'plan', details: { result: 'approved' } },
      'The plan was approved'
    ],
    [
      'approval rejected',
      { kind: 'approval.decided', subjectType: 'plan', details: { result: 'rejected' } },
      'The plan was rejected'
    ],
    [
      'approval requested',
      { kind: 'approval.requested', subjectType: 'solution' },
      'Approval requested for the solution'
    ],
    [
      'phase auto',
      { kind: 'phase.started', details: { trigger: 'auto' } },
      'Phase started automatically'
    ],
    ['phase manual', { kind: 'phase.started', details: { trigger: 'user' } }, 'Phase started'],
    ['returned', { kind: 'request.returned' }, 'Returned to backlog'],
    ['clarification asked', { kind: 'clarification.asked' }, 'Clarification asked'],
    [
      'analysis version',
      { kind: 'analysis.generated', details: { analysisKind: 'solution', version: 2 } },
      'solution version 2 generated'
    ],
    [
      'analysis revision requested',
      { kind: 'analysis.revision_requested', details: { solutionId: 's-1' } },
      'Requested changes to the analysis'
    ],
    [
      'plan revision requested',
      { kind: 'plan.revision_requested', details: { runId: 'run-3' } },
      'Requested changes to the plan'
    ],
    [
      'unknown kind uses the summary',
      { kind: 'thing.happened', summary: 'Something happened' },
      'Something happened'
    ],
    [
      'unknown kind without summary uses the kind',
      { kind: 'thing.happened', summary: ' ' },
      'thing.happened'
    ]
  ])('%s', (_name, over, expected) => {
    expect(activityTitle(entry(over))).toBe(expected)
  })

  it('names the types of a type change', () => {
    const title = activityTitle(
      entry({ kind: 'request.type_changed', details: { from: 'bug', to: 'task' } })
    )
    expect(title).toMatch(/^Type changed from .+ to .+$/)
  })
})

describe('activityAttemptLabel', () => {
  it.each<[Partial<RequestActivityEntry>, string | null]>([
    [{ attempt: 2, details: { maxAttempts: 3 } }, 'Attempt 2 of 3'],
    [{ attempt: 1, details: {} }, 'Attempt 1'],
    [{ kind: 'request.returned' }, null]
  ])('%j', (over, expected) => {
    expect(activityAttemptLabel(entry(over))).toBe(expected)
  })
})

describe('activityDetailText', () => {
  it.each<[string, Partial<RequestActivityEntry>, string | null]>([
    [
      'failure cause and error',
      { details: { cause: 'timed_out', error: 'deadline exceeded' } },
      'Timed out: deadline exceeded'
    ],
    ['failure cause only', { details: { cause: 'verify_failed' } }, 'Verification failed'],
    ['unknown cause kept raw', { details: { cause: 'weird' } }, 'weird'],
    [
      'starting branch not found',
      { details: { cause: 'base_ref_not_found', error: 'unknown revision release/9' } },
      'Starting branch not found: unknown revision release/9'
    ],
    [
      'approval comment',
      { kind: 'approval.decided', details: { comment: 'Looks good' } },
      'Looks good'
    ],
    [
      'returned reason',
      { kind: 'request.returned', details: { reason: 'Failed twice' } },
      'Failed twice'
    ],
    [
      'type change reason',
      { kind: 'request.type_changed', details: { reason: 'It is a bug' } },
      'It is a bug'
    ],
    ['no detail', { kind: 'task.started' }, null],
    [
      'a revision request never shows text, even if the backend sent some',
      {
        kind: 'analysis.revision_requested',
        details: { solutionId: 's-1', feedback: 'secret reviewer text', reason: 'also hidden' }
      },
      null
    ],
    [
      'a plan revision request never shows text either',
      {
        kind: 'plan.revision_requested',
        details: { feedback: 'secret text', reason: 'also hidden', comment: 'and this' }
      },
      null
    ]
  ])('%s', (_name, over, expected) => {
    expect(activityDetailText(entry(over))).toBe(expected)
  })

  it('shows the reviewer text of a plan revision on no line at all', () => {
    const e = entry({
      kind: 'plan.revision_requested',
      summary: 'Requested changes to the plan',
      details: { feedback: 'secret text', reason: 'secret text', error: 'secret text' }
    })
    const lines = [activityTitle(e), activityDetailText(e), activityAttemptLabel(e)]
    expect(lines.join('|')).not.toContain('secret text')
  })
})
