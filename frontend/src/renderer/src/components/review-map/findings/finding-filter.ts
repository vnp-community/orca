/**
 * finding-filter.ts — FE-CV-TASK-059-02
 *
 * Client-side part of the filtering. `severities`, `pathPrefix`, `includeDismissed` and
 * `scope` are also sent to the server (hook); `kind`, `origin` and search only exist here.
 *
 * @module components/review-map/findings/finding-filter
 */

import type {
  FindingKindState,
  FindingOriginState,
  FindingRowModel,
  FindingSeverityState
} from './finding-view-model'

export type FindingFilter = {
  severities?: readonly FindingSeverityState[]
  origins?: readonly FindingOriginState[]
  kinds?: readonly FindingKindState[]
  query?: string
  includeDismissed?: boolean
}

export function foldSearchText(text: string): string {
  return text.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()
}

export function filterFindings(
  rows: readonly FindingRowModel[],
  filter: FindingFilter
): FindingRowModel[] {
  const query = foldSearchText(filter.query?.trim() ?? '')
  return rows.filter((row) => {
    if (row.isDismissed && !filter.includeDismissed) {
      return false
    }
    if (filter.severities?.length && !filter.severities.includes(row.severity)) {
      return false
    }
    if (filter.origins?.length && !filter.origins.includes(row.origin)) {
      return false
    }
    if (filter.kinds?.length && !filter.kinds.includes(row.kind)) {
      return false
    }
    if (query) {
      const haystack = foldSearchText([row.title, row.rule, row.locationPath ?? '', row.symbolKey ?? ''].join('\n'))
      if (!haystack.includes(query)) {
        return false
      }
    }
    return true
  })
}

export type FindingCounts = {
  total: number
  byKind: Partial<Record<FindingKindState, number>>
  bySeverity: Record<FindingSeverityState, number>
}

export function countByKindAndSeverity(rows: readonly FindingRowModel[]): FindingCounts {
  const counts: FindingCounts = {
    total: rows.length,
    byKind: {},
    bySeverity: { error: 0, warning: 0, info: 0, unknown: 0 }
  }
  for (const row of rows) {
    counts.byKind[row.kind] = (counts.byKind[row.kind] ?? 0) + 1
    counts.bySeverity[row.severity] += 1
  }
  return counts
}
