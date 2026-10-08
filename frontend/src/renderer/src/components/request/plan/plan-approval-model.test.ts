import { describe, it, expect } from 'vitest'
import { attachApprovals, computePhaseStats } from './plan-approval-model'
import type { Approval } from '../../../../../shared/request-types'
import type { OrcaTask } from '../../../../../shared/task-types'
import type { PlanSubtree } from '../../../../../shared/task-hierarchy'

const task = (id: string, status: string, type = 'task') =>
  ({ id, status, type }) as unknown as OrcaTask
const ap = (
  id: string,
  subjectType: string,
  subjectId: string,
  updatedAt: string,
  status = 'pending'
) => ({ id, subjectType, subjectId, updatedAt, status, requestId: 'r' }) as unknown as Approval

const tree: PlanSubtree = {
  plan: task('p', 'todo', 'plan'),
  phases: [task('ph1', 'todo', 'phase'), task('ph2', 'todo', 'phase')],
  tasksByPhase: { ph1: [], ph2: [] },
  flatTasks: []
}

describe('attachApprovals', () => {
  it('picks the latest approval per subject', () => {
    const map = attachApprovals(tree, [
      ap('a1', 'plan', 'p', '2026-01-01'),
      ap('a2', 'plan', 'p', '2026-02-01', 'approved'),
      ap('a3', 'phase', 'ph1', '2026-01-01')
    ])
    expect(map.plan?.id).toBe('a2')
    expect(map.byPhaseId['ph1']?.id).toBe('a3')
    expect(map.byPhaseId['ph2']).toBeNull()
  })
  it('keeps task_list separate and lists pre_deploy', () => {
    const map = attachApprovals(tree, [
      ap('t1', 'task_list', 'p', '2026-01-01'),
      ap('d1', 'pre_deploy', 'x', '2026-01-01'),
      ap('d2', 'pre_deploy', 'y', '2026-01-02'),
      ap('s1', 'solution', 'z', '2026-01-02')
    ])
    expect(map.taskList?.id).toBe('t1')
    expect(map.preDeployList.map((a) => a.id)).toEqual(['d1', 'd2'])
  })
})

describe('computePhaseStats', () => {
  it('counts done, blocked, running', () => {
    const s = computePhaseStats([
      task('1', 'done'),
      task('2', 'cancelled'),
      task('3', 'blocked'),
      task('4', 'in_progress'),
      task('5', 'todo')
    ])
    expect(s).toEqual({ done: 2, total: 5, blocked: 1, running: 1 })
  })
})
