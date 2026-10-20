import { fallbackRequestFlow } from '../../../../../shared/request-flow-view'
import type { OrcaTask } from '../../../../../shared/task-types'
import type { Approval, RequestSize, RequestType } from '../../../../../shared/request-types'
import type { PlanSubtree } from '../../../../../shared/task-hierarchy'
import type { RequestFlowView } from '../../../../../shared/request-execution-types'

export function planTask(id: string, o: Partial<OrcaTask> = {}): OrcaTask {
  return {
    id,
    projectId: 'p1',
    title: `Title ${id}`,
    type: 'task',
    status: 'todo',
    priority: 'medium',
    labels: [],
    visibility: 'private',
    progressPercent: 0,
    createdAt: new Date(),
    updatedAt: new Date(),
    ...o
  }
}

export function planApproval(id: string, o: Partial<Approval> = {}): Approval {
  return {
    id,
    requestId: 'r1',
    subjectType: 'plan',
    subjectId: 'plan1',
    subjectDigest: 'dg',
    status: 'pending',
    version: 2,
    createdAt: '2026-10-01',
    updatedAt: '2026-10-01',
    ...o
  }
}

/** Plan with 2 phases x 3 tasks. */
export function twoPhaseTree(): PlanSubtree {
  const phases = [
    planTask('ph1', { type: 'phase', parentId: 'plan1', title: 'Phase One' }),
    planTask('ph2', { type: 'phase', parentId: 'plan1', title: 'Phase Two' })
  ]
  return {
    plan: planTask('plan1', { type: 'plan', title: 'The Plan' }),
    phases,
    tasksByPhase: {
      ph1: [
        planTask('a1', { parentId: 'ph1', status: 'done', taskNumber: 7 }),
        planTask('a2', { parentId: 'ph1', status: 'in_progress' }),
        planTask('a3', { parentId: 'ph1', status: 'blocked' })
      ],
      ph2: [
        planTask('b1', { parentId: 'ph2' }),
        planTask('b2', { parentId: 'ph2' }),
        planTask('b3', { parentId: 'ph2' })
      ]
    },
    flatTasks: []
  }
}

/** A flow as `request.flow` answers it (the built-in table only answers 'fallback'). */
export function backendFlowOf(
  type: Exclude<RequestType, 'unknown'>,
  size: RequestSize | undefined = 'L',
  overrides: Partial<RequestFlowView> = {}
): RequestFlowView {
  return { ...fallbackRequestFlow(type, size)!, source: 'backend', ...overrides }
}
