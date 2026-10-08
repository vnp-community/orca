/**
 * erd-layout.ts — FE-CV-TASK-057-03
 *
 * Deterministic ERD layout (no layout library, O5): connected components, layered by
 * reference depth, one barycenter pass, isolated tables on a grid below. Same input
 * => same output, so positions stay stable between opens. No timers or DOM.
 */

import type { ErdRelationView, ErdTableView } from './erd-view-model'

export const ERD_COLLAPSED_COLUMN_LIMIT = 12
export const ERD_NODE_WIDTH = 264
export const ERD_HEADER_HEIGHT = 36
export const ERD_ROW_HEIGHT = 22
const FOOTER_HEIGHT = 22
const GAP_X = 96
const GAP_Y = 32
const SECTION_GAP = 64
const GRID_COLUMNS = 4
const GROUP_PADDING = 16

export type ErdRect = { x: number; y: number; width: number; height: number }
export type ErdSchemaGroup = ErdRect & { schema: string }

export function erdNodeHeight(table: { columns: readonly unknown[] }, expanded: boolean): number {
  const total = table.columns.length
  const shown = expanded ? total : Math.min(total, ERD_COLLAPSED_COLUMN_LIMIT)
  const footer = total > ERD_COLLAPSED_COLUMN_LIMIT ? FOOTER_HEIGHT : 0
  return ERD_HEADER_HEIGHT + shown * ERD_ROW_HEIGHT + footer
}

export type ErdLayoutInput = {
  tables: readonly ErdTableView[]
  relations: readonly ErdRelationView[]
  expandedTables: ReadonlySet<string>
  groupBySchema: boolean
}

export type ErdLayoutResult = {
  positions: Map<string, ErdRect>
  groups: ErdSchemaGroup[]
}

function cmp(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0
}

/** Same-service edges as [fromKey, toKey]; endpoints are matched by table name. */
function localEdges(
  tables: readonly ErdTableView[],
  relations: readonly ErdRelationView[]
): [string, string][] {
  const byName = new Map<string, string>()
  for (const t of [...tables].sort((a, b) => cmp(a.key, b.key))) {
    if (!byName.has(t.name)) {
      byName.set(t.name, t.key)
    }
    byName.set(t.key, t.key)
  }
  const edges: [string, string][] = []
  for (const r of relations) {
    if (r.crossService) {
      continue
    }
    const from = byName.get(r.from.table)
    const to = byName.get(r.to.table)
    if (from && to && from !== to) {
      edges.push([from, to])
    }
  }
  return edges
}

function components(keys: string[], edges: [string, string][]): string[][] {
  const parent = new Map(keys.map((k) => [k, k]))
  const find = (k: string): string => {
    let root = k
    while (parent.get(root) !== root) {
      root = parent.get(root) as string
    }
    parent.set(k, root)
    return root
  }
  for (const [a, b] of edges) {
    const ra = find(a)
    const rb = find(b)
    if (ra !== rb) {
      parent.set(ra < rb ? rb : ra, ra < rb ? ra : rb)
    }
  }
  const groups = new Map<string, string[]>()
  for (const k of keys) {
    const root = find(k)
    groups.set(root, [...(groups.get(root) ?? []), k])
  }
  return [...groups.values()].map((g) => g.sort(cmp)).sort((a, b) => cmp(a[0], b[0]))
}

function layoutConnected(
  keys: string[],
  edges: [string, string][],
  height: (key: string) => number,
  originY: number,
  positions: Map<string, ErdRect>
): number {
  const level = new Map(keys.map((k) => [k, 0]))
  // Why: iteration cap makes FK cycles terminate with deterministic levels.
  for (let i = 0; i < keys.length; i++) {
    let changed = false
    for (const [from, to] of edges) {
      const next = Math.min((level.get(to) as number) + 1, keys.length - 1)
      if (next > (level.get(from) as number)) {
        level.set(from, next)
        changed = true
      }
    }
    if (!changed) {
      break
    }
  }
  const layers: string[][] = []
  for (const k of keys) {
    const l = level.get(k) as number
    ;(layers[l] ??= []).push(k)
  }
  const index = new Map<string, number>()
  layers.forEach((layer) => layer?.sort(cmp).forEach((k, i) => index.set(k, i)))
  layers.forEach((layer, l) => {
    if (!layer || l === 0) {
      return
    }
    const score = (k: string): number => {
      const ns = edges
        .filter(
          ([a, b]) => (a === k && level.get(b) === l - 1) || (b === k && level.get(a) === l - 1)
        )
        .map(([a, b]) => index.get(a === k ? b : a) as number)
      return ns.length ? ns.reduce((s, n) => s + n, 0) / ns.length : (index.get(k) as number)
    }
    const scored = layer
      .map((k) => [k, score(k)] as const)
      .sort((a, b) => a[1] - b[1] || cmp(a[0], b[0]))
    scored.forEach(([k], i) => {
      layer[i] = k
      index.set(k, i)
    })
  })
  let maxY = originY
  layers.forEach((layer, l) => {
    if (!layer) {
      return
    }
    let y = originY
    for (const k of layer) {
      const h = height(k)
      positions.set(k, { x: l * (ERD_NODE_WIDTH + GAP_X), y, width: ERD_NODE_WIDTH, height: h })
      y += h + GAP_Y
    }
    maxY = Math.max(maxY, y - GAP_Y)
  })
  return maxY
}

function layoutPartition(
  input: ErdLayoutInput,
  tables: readonly ErdTableView[],
  originY: number,
  out: Map<string, ErdRect>
): number {
  const heightOf = new Map(
    tables.map((t) => [t.key, erdNodeHeight(t, input.expandedTables.has(t.key))])
  )
  const height = (k: string): number => heightOf.get(k) as number
  const edges = localEdges(tables, input.relations).filter(
    ([a, b]) => heightOf.has(a) && heightOf.has(b)
  )
  const comps = components([...heightOf.keys()], edges)
  let y = originY
  for (const comp of comps.filter((c) => c.length > 1)) {
    const set = new Set(comp)
    y =
      layoutConnected(
        comp,
        edges.filter(([a, b]) => set.has(a) && set.has(b)),
        height,
        y,
        out
      ) + SECTION_GAP
  }
  const isolated = comps.filter((c) => c.length === 1).map((c) => c[0])
  for (let i = 0; i < isolated.length; i += GRID_COLUMNS) {
    const row = isolated.slice(i, i + GRID_COLUMNS)
    const rowHeight = Math.max(...row.map(height))
    row.forEach((k, col) =>
      out.set(k, { x: col * (ERD_NODE_WIDTH + GAP_X), y, width: ERD_NODE_WIDTH, height: height(k) })
    )
    y += rowHeight + GAP_Y
  }
  return y
}

export function layoutErdTables(input: ErdLayoutInput): ErdLayoutResult {
  const positions = new Map<string, ErdRect>()
  const groups: ErdSchemaGroup[] = []
  const schemas = [...new Set(input.tables.map((t) => t.schema))].sort(cmp)
  if (!input.groupBySchema || schemas.length < 2) {
    layoutPartition(input, input.tables, 0, positions)
    return { positions, groups }
  }
  let y = GROUP_PADDING
  for (const schema of schemas) {
    const part = input.tables.filter((t) => t.schema === schema)
    const before = new Set(positions.keys())
    const end = layoutPartition(input, part, y + GROUP_PADDING, positions)
    const rects = [...positions.entries()].filter(([k]) => !before.has(k)).map(([, r]) => r)
    const maxX = Math.max(...rects.map((r) => r.x + r.width))
    groups.push({
      schema,
      x: -GROUP_PADDING,
      y,
      width: maxX + GROUP_PADDING * 2,
      height: end - y - GAP_Y + GROUP_PADDING * 2 - SECTION_GAP + GAP_Y
    })
    y = end + SECTION_GAP
  }
  return { positions, groups }
}
