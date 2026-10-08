/**
 * Semantic zoom levels — FE-REQ-TASK-032-04
 *
 * @module components/graph/graph-zoom-levels
 */

import { maxRiskOf, groupIdOf } from './graph-grouping'
import type { GraphGroup } from './graph-grouping'
import type { GraphNode, GraphRisk } from '../../../../shared/graph-types'

// Why: thresholds are proposals (unmeasured); keep them as named constants.
export const ZOOM_SERVICE_MAX = 0.5
export const ZOOM_MODULE_MAX = 1.2

export type GraphZoomLevel = 'service' | 'module' | 'leaf'

export function levelForZoom(zoom: number): GraphZoomLevel {
  if (!Number.isFinite(zoom) || zoom < 0) {return 'module'}
  if (zoom < ZOOM_SERVICE_MAX) {return 'service'}
  if (zoom <= ZOOM_MODULE_MAX) {return 'module'}
  return 'leaf'
}

export type GraphRepresentative = {
  id: string
  label: string
  count: number
  maxRisk: GraphRisk
  memberIds: string[]
}

function serviceOf(group: string): string {
  return group.split('/')[0] ?? ''
}

export function collapseToLevel(
  visible: readonly GraphNode[],
  groups: readonly GraphGroup[],
  level: GraphZoomLevel
): GraphRepresentative[] {
  if (level === 'leaf') {
    return [
      ...visible.map((n) => ({ id: n.id, label: n.label, count: 1, maxRisk: n.risk, memberIds: [n.id] })),
      ...groups.map((g) => ({ id: `group:${g.id}`, label: g.label, count: g.count, maxRisk: g.maxRisk, memberIds: g.memberIds }))
    ]
  }
  const keyOf = (group: string): string => (level === 'service' ? serviceOf(group) : group)
  const buckets = new Map<string, { members: string[]; nodes: GraphNode[]; risks: GraphRisk[] }>()
  const bucket = (key: string) => {
    let b = buckets.get(key)
    if (!b) {
      b = { members: [], nodes: [], risks: [] }
      buckets.set(key, b)
    }
    return b
  }
  for (const n of visible) {
    const b = bucket(keyOf(groupIdOf(n)))
    b.members.push(n.id)
    b.nodes.push(n)
  }
  for (const g of groups) {
    const b = bucket(keyOf(g.id))
    b.members.push(...g.memberIds)
    b.risks.push(g.maxRisk)
  }
  return [...buckets.entries()]
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([key, b]) => {
      const pseudo = [...b.nodes, ...b.risks.map((risk) => ({ risk }) as GraphNode)]
      return {
        id: `rep:${key}`,
        label: key === '' ? '(ungrouped)' : key,
        count: b.members.length,
        maxRisk: maxRiskOf(pseudo),
        memberIds: b.members
      }
    })
}

export type GraphZoomPresentation = {
  /** full: label+risk+status; compact: label+risk; minimal: label only; risk stays in border style + aria-label. */
  nodeDetail: 'full' | 'compact' | 'minimal'
  showEdgeSigns: boolean
  edgeOpacityFactor: number
}

// Why: zoomed-out views stay readable by dropping detail; risk is never dropped (border + aria).
export function zoomPresentation(level: GraphZoomLevel): GraphZoomPresentation {
  if (level === 'service') {return { nodeDetail: 'minimal', showEdgeSigns: false, edgeOpacityFactor: 0.5 }}
  if (level === 'module') {return { nodeDetail: 'compact', showEdgeSigns: true, edgeOpacityFactor: 1 }}
  return { nodeDetail: 'full', showEdgeSigns: true, edgeOpacityFactor: 1 }
}
