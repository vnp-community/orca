import { describe, expect, it } from 'vitest'
import {
  keepTasksOfRequest,
  planTaskListParams,
  PLAN_TREE_TASK_TYPES
} from './plan-task-list-filter'
import type { OrcaTask } from '../../../shared/task-types'

const task = (id: string, requestId?: string): OrcaTask => ({ id, requestId }) as OrcaTask

describe('plan-task-list-filter', () => {
  it('names plan, phase and task so task-service stops hiding the plan nodes', () => {
    expect(PLAN_TREE_TASK_TYPES).toEqual(['plan', 'phase', 'task', 'bug', 'feature', 'epic'])
    expect(planTaskListParams({ projectId: 'p', requestId: 'r', pageSize: 50 })).toEqual({
      projectId: 'p',
      requestId: 'r',
      taskTypes: ['plan', 'phase', 'task', 'bug', 'feature', 'epic'],
      pageSize: 50
    })
  })

  it('adds pageToken only when given', () => {
    expect(
      planTaskListParams({ projectId: 'p', requestId: 'r', pageSize: 5, pageToken: 'n' })
    ).toMatchObject({ pageToken: 'n' })
    expect('pageToken' in planTaskListParams({ projectId: 'p', requestId: 'r', pageSize: 5 })).toBe(
      false
    )
  })

  it('drops only tasks that carry another requestId', () => {
    expect(
      keepTasksOfRequest([task('a', 'r'), task('b', 'x'), task('c')], 'r').map((t) => t.id)
    ).toEqual(['a', 'c'])
  })
})
