/**
 * Graph grouping — FE-REQ-TASK-032-04
 *
 * Pure: picks at most VISIBLE_NODE_LIMIT nodes and folds the rest by `group`.
 *
 * @module components/graph/graph-grouping
 */

import { GRAPH_RISK_ORDER } from '../../../../shared/graph-types'
import type { GraphEdge, GraphNode, GraphPayload, GraphRisk } from '../../../../shared/graph-types'

// Why: 50 comes from research and is unmeasured; keep it a named constant.
export const VISIBLE_NODE_LIMIT = 50

export const GROUP_NODE_PREFIX = 'group:'

export type GraphGroup = {
  id: string
  label: string
  count: number
  byKind: Record<string, number>
  memberIds: string[]
  maxRisk: GraphRisk
}

export type GraphViewEdge = GraphEdge & { weight: number; dim?: boolean }

type PickOptions = {
  selectedId?: string | null
  openGroups: ReadonlySet<string>
  limit?: number
}

const CHANGED_STATUSES = new Set(['added', 'removed', 'modified'])

export function groupIdOf(node: GraphNode): string {
  return node.group ?? ''
}

export function maxRiskOf(nodes: readonly GraphNode[]): GraphRisk {
  let best: GraphRisk = 'unknown'
  for (const n of nodes) {
    if (GRAPH_RISK_ORDER[n.risk] > GRAPH_RISK_ORDER[best]) {best = n.risk}
  }
  return best
}

function changedNodeIds(payload: GraphPayload): Set<string> {
  const ids = new Set<string>()
  for (const e of payload.edges) {
    if (e.change !== 'unchanged') {
      ids.add(e.from)
      ids.add(e.to)
    }
  }
  for (const n of payload.nodes) {
    if (n.status && CHANGED_STATUSES.has(n.status)) {ids.add(n.id)}
  }
  return ids
}

export function pickVisibleNodes(
  payload: GraphPayload,
  opts: PickOptions
): { visible: GraphNode[]; groups: GraphGroup[] } {
  const limit = opts.limit ?? VISIBLE_NODE_LIMIT
  if (payload.nodes.length <= limit) {
    return { visible: [...payload.nodes], groups: [] }
  }

  const changed = changedNodeIds(payload)
  const neighbors = new Set<string>()
  if (opts.selectedId) {
    for (const e of payload.edges) {
      if (e.from === opts.selectedId) {neighbors.add(e.to)}
      if (e.to === opts.selectedId) {neighbors.add(e.from)}
    }
  }

  const forced = new Set<string>()
  const candidates: GraphNode[] = []
  for (const n of payload.nodes) {
    if (n.id === opts.selectedId || opts.openGroups.has(groupIdOf(n))) {forced.add(n.id)}
    else {candidates.push(n)}
  }

  // Why: lexicographic keys (change, risk, neighbor, id) keep the choice stable across renders.
  candidates.sort((a, b) => {
    const ca = changed.has(a.id) ? 1 : 0
    const cb = changed.has(b.id) ? 1 : 0
    if (ca !== cb) {return cb - ca}
    const ra = GRAPH_RISK_ORDER[a.risk]
    const rb = GRAPH_RISK_ORDER[b.risk]
    if (ra !== rb) {return rb - ra}
    const na = neighbors.has(a.id) ? 1 : 0
    const nb = neighbors.has(b.id) ? 1 : 0
    if (na !== nb) {return nb - na}
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
  })

  const picked = new Set(candidates.slice(0, limit).map((n) => n.id))
  const visible: GraphNode[] = []
  const hidden = new Map<string, GraphNode[]>()
  for (const n of payload.nodes) {
    if (forced.has(n.id) || picked.has(n.id)) {visible.push(n)}
    else {
      const gid = groupIdOf(n)
      const list = hidden.get(gid) ?? []
      list.push(n)
      hidden.set(gid, list)
    }
  }

  const groups: GraphGroup[] = [...hidden.entries()]
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([id, members]) => {
      const byKind: Record<string, number> = {}
      for (const m of members) {byKind[m.kind] = (byKind[m.kind] ?? 0) + 1}
      return {
        id,
        label: id === '' ? '(ungrouped)' : id,
        count: members.length,
        byKind,
        memberIds: members.map((m) => m.id),
        maxRisk: maxRiskOf(members)
      }
    })

  return { visible, groups }
}

/** Lifts edges touching folded nodes onto their group node; merges duplicates; drops intra-group edges. */
export function buildGroupEdges(
  payload: GraphPayload,
  visible: readonly GraphNode[],
  groups: readonly GraphGroup[]
): GraphViewEdge[] {
  const endpoint = new Map<string, string>()
  for (const n of visible) {endpoint.set(n.id, n.id)}
  for (const g of groups) {
    for (const id of g.memberIds) {endpoint.set(id, `${GROUP_NODE_PREFIX}${g.id}`)}
  }

  const merged = new Map<string, GraphViewEdge>()
  for (const e of payload.edges) {
    const from = endpoint.get(e.from)
    const to = endpoint.get(e.to)
    if (!from || !to || from === to) {continue}
    // Why: before/after views mark removed edges `dim`; keep it through merging.
    const dim = (e as GraphEdge & { dim?: boolean }).dim === true
    const key = `${from}|${to}|${e.kind}|${e.change}|${dim}`
    const existing = merged.get(key)
    if (existing) {existing.weight += 1}
    else {merged.set(key, { from, to, kind: e.kind, change: e.change, weight: 1, ...(dim ? { dim } : {}) })}
  }
  return [...merged.values()]
}
