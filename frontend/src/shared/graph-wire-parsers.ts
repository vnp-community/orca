/**
 * Graph wire parsers — FE-REQ-TASK-032-03
 *
 * Never throw. Unknown risk becomes `unknown` (never `low`); unknown change
 * becomes `unchanged`; unknown kind/status strings are kept verbatim.
 *
 * @module shared/graph-wire-parsers
 */

import type {
  GraphChange,
  GraphEdge,
  GraphLens,
  GraphNode,
  GraphPayload,
  GraphRisk
} from './graph-types'

const RISKS: readonly GraphRisk[] = ['low', 'medium', 'high', 'critical', 'unknown']
const CHANGES: readonly GraphChange[] = ['added', 'removed', 'unchanged']

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function str(v: unknown): string | null {
  return typeof v === 'string' && v.length > 0 ? v : null
}

export function parseGraphRisk(raw: unknown): GraphRisk {
  return typeof raw === 'string' && (RISKS as readonly string[]).includes(raw)
    ? (raw as GraphRisk)
    : 'unknown'
}

export function parseGraphChange(raw: unknown): GraphChange {
  return typeof raw === 'string' && (CHANGES as readonly string[]).includes(raw)
    ? (raw as GraphChange)
    : 'unchanged'
}

function parseMeta(raw: unknown): GraphNode['meta'] {
  if (!isRecord(raw)) {return undefined}
  // Why: backend sends snake_case `finding_ids` (CR-REQ-030 2.8); GraphNode.meta is frontend-defined.
  const ids = raw.findingIds ?? raw.finding_ids
  const meta: NonNullable<GraphNode['meta']> = {}
  if (Array.isArray(ids)) {meta.findingIds = ids.filter((x): x is string => typeof x === 'string')}
  const taskId = str(raw.taskId ?? raw.task_id)
  if (taskId) {meta.taskId = taskId}
  const optionId = str(raw.optionId ?? raw.option_id)
  if (optionId) {meta.optionId = optionId}
  return Object.keys(meta).length > 0 ? meta : undefined
}

function parseNode(raw: unknown): GraphNode | null {
  if (!isRecord(raw)) {return null}
  const id = str(raw.id)
  const label = str(raw.label)
  if (!id || !label) {return null}
  const node: GraphNode = {
    id,
    kind: str(raw.kind) ?? 'unknown',
    label,
    group: str(raw.group),
    risk: parseGraphRisk(raw.risk),
    status: str(raw.status)
  }
  const meta = parseMeta(raw.meta)
  if (meta) {node.meta = meta}
  return node
}

export function emptyGraphPayload(lens: GraphLens): GraphPayload {
  return {
    lens,
    nodes: [],
    edges: [],
    totalNodes: 0,
    truncated: false,
    assessedAt: null,
    tool: null,
    stale: false,
    droppedEdges: 0
  }
}

export function parseGraphPayload(raw: unknown, expectedLens: GraphLens): GraphPayload {
  if (!isRecord(raw)) {return emptyGraphPayload(expectedLens)}

  const nodes: GraphNode[] = []
  const seen = new Set<string>()
  for (const item of Array.isArray(raw.nodes) ? raw.nodes : []) {
    const node = parseNode(item)
    if (!node || seen.has(node.id)) {continue}
    seen.add(node.id)
    nodes.push(node)
  }

  const edges: GraphEdge[] = []
  let droppedEdges = 0
  for (const item of Array.isArray(raw.edges) ? raw.edges : []) {
    if (!isRecord(item)) {continue}
    const from = str(item.from)
    const to = str(item.to)
    if (!from || !to || !seen.has(from) || !seen.has(to)) {
      droppedEdges += 1
      continue
    }
    edges.push({
      from,
      to,
      kind: str(item.kind) ?? 'unknown',
      change: parseGraphChange(item.change)
    })
  }

  const total = typeof raw.totalNodes === 'number' ? raw.totalNodes : raw.total_nodes
  return {
    // Why: the caller asked for this lens; a mismatched echo must not switch the canvas.
    lens: expectedLens,
    nodes,
    edges,
    totalNodes: typeof total === 'number' && total >= nodes.length ? total : nodes.length,
    truncated: raw.truncated === true,
    assessedAt: str(raw.assessedAt ?? raw.assessed_at),
    tool: str(raw.tool),
    stale: raw.stale === true,
    droppedEdges
  }
}
