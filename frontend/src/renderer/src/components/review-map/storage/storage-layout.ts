/**
 * storage-layout.ts — FE-CV-TASK-058-02
 *
 * Fixed four-lane layout (service | store | topic | secret). Deterministic: rows sorted
 * by name, then one barycenter pass against service-lane neighbours. Empty lanes take no
 * space. No timers, no DOM.
 */

import { STORAGE_LANES } from './storage-view-model'
import type { StorageEdge, StorageLane, StorageNode } from './storage-view-model'

export const STORAGE_NODE_WIDTH = 224
export const STORAGE_NODE_HEIGHT = 64
const LANE_GAP = 120
const ROW_GAP = 24

export type StorageRect = { x: number; y: number; width: number; height: number }
export type StorageLaneBound = { lane: StorageLane } & StorageRect

export type StorageLayoutInput = {
  nodes: readonly StorageNode[]
  edges: readonly StorageEdge[]
  /** Only these ids are laid out (the "changed + related" filter); undefined = all. */
  filter?: ReadonlySet<string>
}

export function layoutStorageLanes(input: StorageLayoutInput): {
  positions: Map<string, StorageRect>
  laneBounds: StorageLaneBound[]
} {
  const nodes = input.nodes.filter((n) => !input.filter || input.filter.has(n.id))
  const ids = new Set(nodes.map((n) => n.id))
  const byLane = new Map<StorageLane, StorageNode[]>()
  for (const lane of STORAGE_LANES) {
    byLane.set(
      lane,
      nodes
        .filter((n) => n.lane === lane)
        .sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : a.id < b.id ? -1 : 1))
    )
  }
  const serviceIndex = new Map((byLane.get('service') ?? []).map((n, i) => [n.id, i]))
  const neighbours = new Map<string, number[]>()
  for (const e of input.edges) {
    if (!ids.has(e.from) || !ids.has(e.to)) {
      continue
    }
    for (const [a, b] of [
      [e.from, e.to],
      [e.to, e.from]
    ] as const) {
      const idx = serviceIndex.get(b)
      if (idx !== undefined && !serviceIndex.has(a)) {
        neighbours.set(a, [...(neighbours.get(a) ?? []), idx])
      }
    }
  }
  const score = (id: string, fallback: number): number => {
    const ns = neighbours.get(id)
    return ns?.length ? ns.reduce((s, n) => s + n, 0) / ns.length : fallback
  }

  const positions = new Map<string, StorageRect>()
  const laneBounds: StorageLaneBound[] = []
  let laneIndex = 0
  for (const lane of STORAGE_LANES) {
    let list = byLane.get(lane) ?? []
    if (list.length === 0) {
      continue
    }
    if (lane !== 'service') {
      list = list
        .map((n, i) => ({ n, s: score(n.id, i) }))
        .sort((a, b) => a.s - b.s)
        .map((x) => x.n)
    }
    const x = laneIndex * (STORAGE_NODE_WIDTH + LANE_GAP)
    list.forEach((n, i) => {
      positions.set(n.id, {
        x,
        y: i * (STORAGE_NODE_HEIGHT + ROW_GAP),
        width: STORAGE_NODE_WIDTH,
        height: STORAGE_NODE_HEIGHT
      })
    })
    laneBounds.push({
      lane,
      x,
      y: 0,
      width: STORAGE_NODE_WIDTH,
      height: list.length * (STORAGE_NODE_HEIGHT + ROW_GAP) - ROW_GAP
    })
    laneIndex += 1
  }
  return { positions, laneBounds }
}
