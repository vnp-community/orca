/**
 * finding-sort.ts — FE-CV-TASK-059-02
 *
 * Deterministic ordering: severity, then origin (changes introduced here first), then key.
 *
 * @module components/review-map/findings/finding-sort
 */

import type { FindingKindState, FindingRowModel } from './finding-view-model'

const SEVERITY_RANK = { error: 0, warning: 1, info: 2, unknown: 3 } as const
const ORIGIN_RANK = { introduced: 0, touched: 1, preexisting: 2, unknown: 3 } as const

export function sortFindings(rows: readonly FindingRowModel[]): FindingRowModel[] {
  return [...rows].sort((a, b) => {
    const s = SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity]
    if (s !== 0) {
      return s
    }
    const o = ORIGIN_RANK[a.origin] - ORIGIN_RANK[b.origin]
    if (o !== 0) {
      return o
    }
    return a.findingKey < b.findingKey ? -1 : a.findingKey > b.findingKey ? 1 : 0
  })
}

export type FindingKindGroup = { kind: FindingKindState; rows: FindingRowModel[] }

/** Groups already-sorted rows by kind; groups ordered by their most severe row. */
export function groupFindingsByKind(rows: readonly FindingRowModel[]): FindingKindGroup[] {
  const groups = new Map<FindingKindState, FindingRowModel[]>()
  for (const row of sortFindings(rows)) {
    const list = groups.get(row.kind)
    if (list) {
      list.push(row)
    } else {
      groups.set(row.kind, [row])
    }
  }
  return [...groups.entries()]
    .map(([kind, list]) => ({ kind, rows: list }))
    .sort((a, b) => {
      const s = SEVERITY_RANK[a.rows[0].severity] - SEVERITY_RANK[b.rows[0].severity]
      return s !== 0 ? s : a.kind < b.kind ? -1 : 1
    })
}
