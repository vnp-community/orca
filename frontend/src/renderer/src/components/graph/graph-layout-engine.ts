/**
 * Graph layout engine — FE-REQ-TASK-032-05
 *
 * Pluggable layered ("wave") layout. The wave algorithm mirrors
 * TaskDAGView's buildDAGLayout, plus group-aware ordering inside a wave.
 * No layout library is added (README v7 O5); elkjs would plug in here.
 *
 * @module components/graph/graph-layout-engine
 */

import type { GraphEdge, GraphNode } from '../../../../shared/graph-types'

export type LayoutPositions = Record<string, { x: number; y: number }>
export type LayoutOptions = {
  direction: 'LR' | 'TB'
  groupOf: (nodeId: string) => string | null
}
export type LayoutEngine = (
  nodes: readonly GraphNode[],
  edges: readonly GraphEdge[],
  options: LayoutOptions
) => Promise<LayoutPositions>

export const HORIZONTAL_GAP = 220
export const VERTICAL_GAP = 90

export function computeWaveLayout(
  nodes: readonly GraphNode[],
  edges: readonly GraphEdge[],
  options: LayoutOptions
): LayoutPositions {
  const ids = new Set(nodes.map((n) => n.id))
  const incoming = new Map<string, string[]>()
  for (const e of edges) {
    if (!ids.has(e.from) || !ids.has(e.to) || e.from === e.to) {continue}
    const list = incoming.get(e.to) ?? []
    list.push(e.from)
    incoming.set(e.to, list)
  }

  // Why: nodes on a cycle fall back to wave 0 so layout always terminates.
  const wave = new Map<string, number>()
  const onStack = new Set<string>()
  const waveOf = (id: string): number => {
    const known = wave.get(id)
    if (known !== undefined) {return known}
    if (onStack.has(id)) {return 0}
    onStack.add(id)
    let w = 0
    for (const dep of incoming.get(id) ?? []) {w = Math.max(w, waveOf(dep) + 1)}
    onStack.delete(id)
    wave.set(id, w)
    return w
  }
  for (const n of nodes) {waveOf(n.id)}

  const byWave = new Map<number, GraphNode[]>()
  for (const n of nodes) {
    const w = wave.get(n.id) ?? 0
    const list = byWave.get(w) ?? []
    list.push(n)
    byWave.set(w, list)
  }

  const positions: LayoutPositions = {}
  for (const [w, list] of byWave) {
    list.sort((a, b) => {
      const ga = options.groupOf(a.id) ?? ''
      const gb = options.groupOf(b.id) ?? ''
      if (ga !== gb) {return ga < gb ? -1 : 1}
      return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
    })
    list.forEach((n, idx) => {
      positions[n.id] =
        options.direction === 'LR'
          ? { x: w * HORIZONTAL_GAP, y: idx * VERTICAL_GAP }
          : { x: idx * HORIZONTAL_GAP, y: w * VERTICAL_GAP }
    })
  }
  return positions
}

export const waveLayoutEngine: LayoutEngine = async (nodes, edges, options) =>
  computeWaveLayout(nodes, edges, options)

export function layoutCacheKey(payloadId: string, lens: string, openGroupsKey: string): string {
  return `${payloadId}|${lens}|${openGroupsKey}`
}
