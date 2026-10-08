/**
 * Tests for task-hierarchy.ts (CR-REQ-021-01)
 */

import { describe, it, expect } from 'vitest'
import {
  isPlanningTask,
  isWorkTask,
  computeEffectiveParents,
  hidePlanningTasks,
  buildPlanSubtree
} from './task-hierarchy'
import type { OrcaTask } from './task-types'

function makeTask(id: string, overrides: Partial<OrcaTask> = {}): OrcaTask {
  return {
    id,
    title: `Task ${id}`,
    type: 'task',
    status: 'open',
    priority: 'medium',
    labels: [],
    visibility: 'team',
    progressPercent: 0,
    createdAt: new Date(),
    updatedAt: new Date(),
    ...overrides
  }
}

describe('isPlanningTask', () => {
  it('returns true for plan type', () => {
    expect(isPlanningTask(makeTask('t', { type: 'plan' }))).toBe(true)
  })

  it('returns true for phase type', () => {
    expect(isPlanningTask(makeTask('t', { type: 'phase' }))).toBe(true)
  })

  it.each(['task', 'epic', 'story', 'subtask', 'bug', 'spike'] as const)(
    'returns false for type %s',
    (type) => {
      expect(isPlanningTask(makeTask('t', { type }))).toBe(false)
    }
  )
})

describe('isWorkTask', () => {
  it('returns false for plan', () => {
    expect(isWorkTask(makeTask('t', { type: 'plan' }))).toBe(false)
  })

  it('returns true for task', () => {
    expect(isWorkTask(makeTask('t', { type: 'task' }))).toBe(true)
  })
})

describe('computeEffectiveParents', () => {
  it('task with no parent stays at root', () => {
    const tasks = [makeTask('t1')]
    const result = computeEffectiveParents(tasks)
    expect(result.get('t1')?.parentId).toBeNull()
    expect(result.get('t1')?.trail).toHaveLength(0)
  })

  it('task with work task parent keeps its natural parent', () => {
    const parent = makeTask('epic1', { type: 'epic' })
    const child = makeTask('t1', { parentId: 'epic1', type: 'task' })
    const result = computeEffectiveParents([parent, child])
    expect(result.get('t1')?.parentId).toBe('epic1')
    expect(result.get('t1')?.trail).toHaveLength(0)
  })

  it('task under a single Plan is promoted to root with trail', () => {
    const plan = makeTask('plan1', { type: 'plan', title: 'Plan Alpha' })
    const task = makeTask('t1', { parentId: 'plan1' })
    const result = computeEffectiveParents([plan, task])
    expect(result.get('t1')?.parentId).toBeNull()
    expect(result.get('t1')?.trail).toEqual(['Plan Alpha'])
  })

  it('task under Plan > Phase is promoted to root with full trail', () => {
    const plan = makeTask('plan1', { type: 'plan', title: 'Plan Alpha' })
    const phase = makeTask('phase1', { type: 'phase', title: 'Phase Beta', parentId: 'plan1' })
    const task = makeTask('t1', { parentId: 'phase1' })
    const result = computeEffectiveParents([plan, phase, task])
    expect(result.get('t1')?.parentId).toBeNull()
    // trail is in traversal order: phase first, then plan
    expect(result.get('t1')?.trail).toEqual(['Phase Beta', 'Plan Alpha'])
  })

  it('task under Phase whose parent is a work epic gets promoted to epic', () => {
    const epic = makeTask('epic1', { type: 'epic' })
    const phase = makeTask('phase1', { type: 'phase', parentId: 'epic1', title: 'Phase X' })
    const task = makeTask('t1', { parentId: 'phase1' })
    const result = computeEffectiveParents([epic, phase, task])
    expect(result.get('t1')?.parentId).toBe('epic1')
    expect(result.get('t1')?.trail).toEqual(['Phase X'])
  })

  it('handles cycle in parentId without infinite loop', () => {
    // t1 → t2 → t1 (cycle)
    const t1 = makeTask('t1', { parentId: 't2' })
    const t2 = makeTask('t2', { parentId: 't1' })
    expect(() => computeEffectiveParents([t1, t2])).not.toThrow()
    // Both should resolve to null (cycle detected)
    const result = computeEffectiveParents([t1, t2])
    expect(result.get('t1')?.parentId).toBeNull()
  })

  it('returns correct entries when no plan/phase tasks exist', () => {
    const tasks = [
      makeTask('epic', { type: 'epic' }),
      makeTask('story', { type: 'story', parentId: 'epic' })
    ]
    const result = computeEffectiveParents(tasks)
    expect(result.get('story')?.parentId).toBe('epic')
    expect(result.get('story')?.trail).toHaveLength(0)
  })

  it('handles empty task list', () => {
    const result = computeEffectiveParents([])
    expect(result.size).toBe(0)
  })
})

describe('hidePlanningTasks', () => {
  const plan = makeTask('plan1', { type: 'plan', title: 'Plan A' })
  const phase = makeTask('ph1', { type: 'phase', parentId: 'plan1', title: 'Phase 1' })
  const work = makeTask('w1', { parentId: 'ph1' })
  const all = [plan, phase, work]

  it('returns the same array reference when there is no plan/phase', () => {
    const list = [makeTask('a'), makeTask('b', { parentId: 'a' })]
    expect(hidePlanningTasks(list, list, false)).toBe(list)
  })

  it('drops plan/phase and promotes children with a Plan / Phase path', () => {
    const out = hidePlanningTasks(all, all, true)
    expect(out.map((t) => t.id)).toEqual(['w1'])
    expect(out[0].parentId).toBeUndefined()
    expect(out[0].planPath).toEqual(['Plan A', 'Phase 1'])
  })

  it('keeps the work-task parent when a phase hangs under a work epic', () => {
    const epic = makeTask('e1', { type: 'epic' })
    const ph = makeTask('ph2', { type: 'phase', parentId: 'e1', title: 'P2' })
    const t = makeTask('w2', { parentId: 'ph2' })
    const out = hidePlanningTasks([epic, ph, t], [epic, ph, t], true)
    expect(out.find((x) => x.id === 'w2')?.parentId).toBe('e1')
    expect(out.find((x) => x.id === 'e1')).toBeDefined()
  })

  it('leaves untouched tasks as the same object', () => {
    const solo = makeTask('s')
    const out = hidePlanningTasks([plan, solo], [plan, solo], true)
    expect(out[0]).toBe(solo)
  })
})

describe('buildPlanSubtree', () => {
  it('splits phases, tasks per phase and flat tasks', () => {
    const plan = makeTask('p', { type: 'plan' })
    const ph = makeTask('ph', { type: 'phase', parentId: 'p' })
    const a = makeTask('a', { parentId: 'ph' })
    const flat = makeTask('f', { parentId: 'p' })
    const tree = buildPlanSubtree([plan, ph, a, flat], 'p')
    expect(tree.phases.map((x) => x.id)).toEqual(['ph'])
    expect(tree.tasksByPhase['ph'].map((x) => x.id)).toEqual(['a'])
    expect(tree.flatTasks.map((x) => x.id)).toEqual(['f'])
  })

  it('returns plan=null when the plan is not in the list', () => {
    expect(buildPlanSubtree([makeTask('x')], 'missing').plan).toBeNull()
  })

  it('ignores orphans and survives parent cycles', () => {
    const plan = makeTask('p', { type: 'plan' })
    const c1 = makeTask('c1', { parentId: 'c2' })
    const c2 = makeTask('c2', { parentId: 'c1' })
    const tree = buildPlanSubtree([plan, c1, c2, makeTask('orphan', { parentId: 'gone' })], 'p')
    expect(tree.phases).toEqual([])
    expect(tree.flatTasks).toEqual([])
  })
})
