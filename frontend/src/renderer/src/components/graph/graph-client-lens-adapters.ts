/**
 * Client-built lenses — FE-REQ-TASK-032-04
 *
 * flow, plan and execution are derived locally so they work before the
 * backend impact service exists. Pure; no React, no clock.
 *
 * @module components/graph/graph-client-lens-adapters
 */

import { buildStageTimeline } from '../request/request-stage-timeline-model'
import type { PlanSubtree } from '../../../../shared/task-hierarchy'
import type { OrcaTask } from '../../../../shared/task-types'
import type { OrcaRequest } from '../../../../shared/request-types'
import type { GraphEdge, GraphLens, GraphNode, GraphPayload, GraphRisk } from '../../../../shared/graph-types'
import { parseGraphRisk } from '../../../../shared/graph-wire-parsers'

type DependencyMap = ReadonlyMap<string, { blockedBy: string[] }>

export type PlanHeatmap = {
  byTaskId: Record<string, GraphRisk | string>
  assessedAt?: string | null
  tool?: string | null
}

export type ExecutionDrift = { driftedTaskIds: readonly string[] }

type LabelFn = (key: string) => string

function basePayload(lens: GraphLens, nodes: GraphNode[], edges: GraphEdge[]): GraphPayload {
  return {
    lens,
    nodes,
    edges,
    totalNodes: nodes.length,
    truncated: false,
    assessedAt: null,
    tool: null,
    stale: false,
    droppedEdges: 0
  }
}

export function buildFlowGraph(
  request: Pick<OrcaRequest, 'type' | 'size' | 'status'>,
  label: LabelFn = (k) => k
): GraphPayload {
  const timeline = buildStageTimeline({ type: request.type, size: request.size, status: request.status })
  const nodes: GraphNode[] = []
  const edges: GraphEdge[] = []
  // Why: `unknown` risk means "not applicable" on the flow lens, never "low".
  const push = (id: string, text: string, status: string): void => {
    nodes.push({ id, kind: 'step', label: text, group: null, risk: 'unknown', status })
  }
  for (const step of timeline.steps) {
    push(step.id, label(step.labelKey), step.state)
    if (step.id === 'analysis' && request.status === 'awaiting_information') {
      push('awaiting_information', label('auto.components.request.RequestStatus.awaiting_information'), 'current')
    }
  }
  for (let i = 1; i < nodes.length; i += 1) {
    edges.push({ from: nodes[i - 1].id, to: nodes[i].id, kind: 'transition', change: 'unchanged' })
  }
  return basePayload('flow', nodes, edges)
}

function planNodes(
  tree: PlanSubtree,
  riskOf: (task: OrcaTask) => GraphRisk,
  statusOf: (task: OrcaTask) => string,
  driftedIds?: ReadonlySet<string>
): GraphNode[] {
  const nodes: GraphNode[] = []
  const seen = new Set<string>()
  const add = (task: OrcaTask, kind: 'phase' | 'task', group: string | null): void => {
    if (seen.has(task.id)) {return}
    seen.add(task.id)
    const node: GraphNode = {
      id: task.id,
      kind,
      label: task.title || task.id,
      group,
      risk: riskOf(task),
      status: statusOf(task),
      meta: { taskId: task.id }
    }
    if (driftedIds?.has(task.id) && node.meta) {node.meta.drifted = true}
    nodes.push(node)
  }
  for (const phase of tree.phases) {
    add(phase, 'phase', phase.title)
    for (const t of tree.tasksByPhase[phase.id] ?? []) {add(t, 'task', phase.title)}
  }
  for (const t of tree.flatTasks) {add(t, 'task', null)}
  return nodes
}

function planEdges(tree: PlanSubtree, nodes: readonly GraphNode[], deps: DependencyMap): GraphEdge[] {
  const ids = new Set(nodes.map((n) => n.id))
  const edges: GraphEdge[] = []
  for (const phase of tree.phases) {
    for (const t of tree.tasksByPhase[phase.id] ?? []) {
      if (ids.has(phase.id) && ids.has(t.id)) {edges.push({ from: phase.id, to: t.id, kind: 'contains', change: 'unchanged' })}
    }
  }
  for (const n of nodes) {
    for (const dep of deps.get(n.id)?.blockedBy ?? []) {
      if (ids.has(dep)) {edges.push({ from: dep, to: n.id, kind: 'depends_on', change: 'unchanged' })}
    }
  }
  return edges
}

export function buildPlanGraph(tree: PlanSubtree | null, deps: DependencyMap = new Map(), heatmap?: PlanHeatmap | null): GraphPayload {
  if (!tree) {return basePayload('plan', [], [])}
  const nodes = planNodes(
    tree,
    (t) => parseGraphRisk(heatmap?.byTaskId[t.id]),
    (t) => t.status
  )
  const payload = basePayload('plan', nodes, planEdges(tree, nodes, deps))
  payload.assessedAt = heatmap?.assessedAt ?? null
  payload.tool = heatmap?.tool ?? null
  return payload
}

export function buildExecutionGraph(
  tree: PlanSubtree | null,
  deps: DependencyMap = new Map(),
  outcomes?: Readonly<Record<string, string>>,
  drift?: ExecutionDrift | null
): GraphPayload {
  if (!tree) {return basePayload('execution', [], [])}
  const drifted = new Set(drift?.driftedTaskIds ?? [])
  const nodes = planNodes(tree, () => 'unknown', (t) => outcomes?.[t.id] ?? t.status, drifted)
  return basePayload('execution', nodes, planEdges(tree, nodes, deps))
}
