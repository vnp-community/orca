/**
 * erd-table-filter.ts — FE-CV-TASK-057-03
 *
 * Search, schema filter, "changed + adjacent" focus mode and the render cap. The cap
 * never cuts silently: `truncated` always reports shown/total.
 */

import { ERD_COLLAPSED_COLUMN_LIMIT } from './erd-layout'
import type { ErdColumnView } from './erd-column-changes'
import type { ErdRelationView, ErdTableView } from './erd-view-model'

export const ERD_FOCUS_DEFAULT_THRESHOLD = 25
export const ERD_MAX_RENDERED_TABLES = 150

export type ErdFilterMode = 'focus' | 'all'

export function defaultErdFilterMode(tableCount: number): ErdFilterMode {
  return tableCount > ERD_FOCUS_DEFAULT_THRESHOLD ? 'focus' : 'all'
}

export function normalizeErdSearch(text: string): string {
  return text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .trim()
}

export function erdTableMatchesQuery(table: ErdTableView, normalizedQuery: string): boolean {
  if (!normalizedQuery) {
    return false
  }
  const hay = (s: string): boolean => normalizeErdSearch(s).includes(normalizedQuery)
  return (
    hay(table.name) ||
    table.columns.some((c) => hay(c.name) || hay(c.type) || hay(c.canonicalType)) ||
    table.accessedBy.some((a) => hay(a.symbol.name))
  )
}

export type ErdFilterInput = {
  tables: readonly ErdTableView[]
  relations?: readonly ErdRelationView[]
  query: string
  /** null or empty = every schema. */
  schemas: ReadonlySet<string> | null
  mode: ErdFilterMode
  limit?: number
}

export type ErdFilterResult = {
  visible: ErdTableView[]
  /** Visible but not matching the active query. */
  dimmed: Set<string>
  /** Always present; `capped` is true when the render cap hid tables. */
  truncated: { shown: number; total: number; capped: boolean }
  matchCount: number
}

function relationCounts(relations: readonly ErdRelationView[]): Map<string, number> {
  const counts = new Map<string, number>()
  for (const r of relations) {
    for (const name of [r.from.table, r.to.table]) {
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
  }
  return counts
}

export function filterErdTables(input: ErdFilterInput): ErdFilterResult {
  const limit = input.limit ?? ERD_MAX_RENDERED_TABLES
  const q = normalizeErdSearch(input.query)
  const inSchema = (t: ErdTableView): boolean =>
    !input.schemas || input.schemas.size === 0 || input.schemas.has(t.schema)
  const pool = input.tables.filter(inSchema)
  const matches = new Set(pool.filter((t) => erdTableMatchesQuery(t, q)).map((t) => t.key))
  const isContext = (t: ErdTableView): boolean => t.tableChange !== 'untouched'

  let selected =
    input.mode === 'focus' ? pool.filter((t) => isContext(t) || matches.has(t.key)) : pool
  const counts = relationCounts(input.relations ?? [])
  const rank = (t: ErdTableView): number =>
    (t.tableChange !== 'untouched' && t.tableChange !== 'adjacent'
      ? 3
      : t.tableChange === 'adjacent'
        ? 2
        : 0) + (matches.has(t.key) ? 1.5 : 0)
  const total = selected.length
  const capped = total > limit
  if (capped) {
    selected = [...selected]
      .sort(
        (a, b) =>
          rank(b) - rank(a) ||
          (counts.get(b.name) ?? 0) - (counts.get(a.name) ?? 0) ||
          (a.key < b.key ? -1 : 1)
      )
      .slice(0, limit)
  }
  const keep = new Set(selected.map((t) => t.key))
  const visible = pool.filter((t) => keep.has(t.key))
  const dimmed = new Set(q ? visible.filter((t) => !matches.has(t.key)).map((t) => t.key) : [])
  return {
    visible,
    dimmed,
    truncated: { shown: visible.length, total, capped },
    matchCount: matches.size
  }
}

/** Collapsed order: PK, FK, changed, search match, rest; expanded/short keeps source order. */
export function selectCollapsedColumns(
  columns: readonly ErdColumnView[],
  query: string,
  expanded: boolean,
  limit: number = ERD_COLLAPSED_COLUMN_LIMIT
): { shown: ErdColumnView[]; hidden: number } {
  if (expanded || columns.length <= limit) {
    return { shown: [...columns], hidden: 0 }
  }
  const q = normalizeErdSearch(query)
  const weight = (c: ErdColumnView): number =>
    c.isPk ? 0 : c.isFk ? 1 : c.change ? 2 : q && normalizeErdSearch(c.name).includes(q) ? 3 : 4
  const ordered = columns
    .map((c, i) => ({ c, i }))
    .sort((a, b) => weight(a.c) - weight(b.c) || a.i - b.i)
  return { shown: ordered.slice(0, limit).map((x) => x.c), hidden: columns.length - limit }
}
