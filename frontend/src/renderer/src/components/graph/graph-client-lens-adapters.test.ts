import { describe, expect, it } from 'vitest'
import { buildExecutionGraph, buildFlowGraph, buildPlanGraph } from './graph-client-lens-adapters'
import type { OrcaTask } from '../../../../shared/task-types'
import type { PlanSubtree } from '../../../../shared/task-hierarchy'

const task = (id: string, over: Partial<OrcaTask> = {}) =>
  ({ id, title: `T ${id}`, type: 'task', status: 'open', priority: 'medium', progressPercent: 0, ...over }) as OrcaTask

const tree: PlanSubtree = {
  plan: task('plan'),
  phases: [task('p1', { title: 'Phase 1' })],
  tasksByPhase: { p1: [task('t1'), task('t2', { status: 'done' })] },
  flatTasks: []
}
const deps = new Map([['t2', { blockedBy: ['t1', 'ghost'] }]])

describe('client lens adapters', () => {
  it('buildFlowGraph yields a chain with step states and unknown risk', () => {
    const p = buildFlowGraph({ type: 'bug', size: 'M', status: 'analyzing' })
    expect(p.lens).toBe('flow')
    expect(p.nodes.length).toBeGreaterThan(2)
    expect(p.nodes.every((n) => n.risk === 'unknown' && n.kind === 'step')).toBe(true)
    expect(p.nodes.some((n) => n.status === 'current')).toBe(true)
    expect(p.edges).toHaveLength(p.nodes.length - 1)
  })

  it('inserts awaiting_information after the analysis step', () => {
    const p = buildFlowGraph({ type: 'bug', size: 'M', status: 'awaiting_information' })
    const ids = p.nodes.map((n) => n.id)
    expect(ids[ids.indexOf('analysis') + 1]).toBe('awaiting_information')
  })

  it('buildPlanGraph builds phase/task nodes, contains and depends_on edges, heatmap risk', () => {
    const p = buildPlanGraph(tree, deps, { byTaskId: { t1: 'high' }, assessedAt: 'x', tool: 'y' })
    expect(p.nodes.map((n) => n.id)).toEqual(['p1', 't1', 't2'])
    expect(p.nodes.find((n) => n.id === 't1')?.risk).toBe('high')
    expect(p.nodes.find((n) => n.id === 't2')?.risk).toBe('unknown')
    expect(p.edges.filter((e) => e.kind === 'depends_on')).toEqual([
      { from: 't1', to: 't2', kind: 'depends_on', change: 'unchanged' }
    ])
    expect(p.edges.filter((e) => e.kind === 'contains')).toHaveLength(2)
    expect(p.assessedAt).toBe('x')
  })

  it('buildExecutionGraph marks drifted tasks and uses outcomes over task status', () => {
    const p = buildExecutionGraph(tree, deps, { t1: 'failed' }, { driftedTaskIds: ['t2'] })
    expect(p.nodes.find((n) => n.id === 't1')?.status).toBe('failed')
    expect(p.nodes.find((n) => n.id === 't2')?.meta?.drifted).toBe(true)
  })

  it('handles an absent tree', () => {
    expect(buildPlanGraph(null).nodes).toEqual([])
    expect(buildExecutionGraph(null).nodes).toEqual([])
  })
})
