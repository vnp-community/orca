/**
 * Backlog view columns and client filters — CR-REQ-023-04
 *
 * The server filters only by project/plan/phase, so search, Request type and
 * return category are applied here on the pages already loaded.
 *
 * @module components/request/backlog/backlog-view-columns
 */

import type { BacklogGroupData, RequestBacklogRowData, ReturnedCategory } from '../../../../../shared/request-backlog-types'
import type { BacklogView, RequestType } from '../../../../../shared/request-types'

export type BacklogColumn = { id: string; labelKey: string; fallback: string; sticky?: boolean; narrow?: boolean }

const P = 'auto.components.request.backlog.'

export const BACKLOG_COLUMNS: Record<BacklogView, BacklogColumn[]> = {
  requests: [
    { id: 'request', labelKey: `${P}RequestBacklogTable.col.request`, fallback: 'Request', sticky: true },
    { id: 'source', labelKey: `${P}RequestBacklogTable.col.source`, fallback: 'Source' },
    { id: 'type', labelKey: `${P}RequestBacklogTable.col.type`, fallback: 'Type' },
    { id: 'stage', labelKey: `${P}RequestBacklogTable.col.stage`, fallback: 'Returned from' },
    { id: 'category', labelKey: `${P}RequestBacklogTable.col.category`, fallback: 'Category' },
    { id: 'reason', labelKey: `${P}RequestBacklogTable.col.reason`, fallback: 'Reason' },
    { id: 'by', labelKey: `${P}RequestBacklogTable.col.by`, fallback: 'Returned by' },
    { id: 'at', labelKey: `${P}RequestBacklogTable.col.at`, fallback: 'When' },
    { id: 'actions', labelKey: `${P}RequestBacklogTable.col.actions`, fallback: 'Actions', narrow: true }
  ],
  tasks: [
    { id: 'task', labelKey: `${P}TaskBacklogTable.col.task`, fallback: 'Task', sticky: true },
    { id: 'estimate', labelKey: `${P}TaskBacklogTable.col.estimate`, fallback: 'Estimate', narrow: true },
    { id: 'dependencies', labelKey: `${P}TaskBacklogTable.col.dependencies`, fallback: 'Waiting on' }
  ],
  execute: [
    { id: 'task', labelKey: `${P}ExecuteBacklogTable.col.task`, fallback: 'Task', sticky: true },
    { id: 'blockedBy', labelKey: `${P}ExecuteBacklogTable.col.blockedBy`, fallback: 'Blocked by' },
    { id: 'lastError', labelKey: `${P}ExecuteBacklogTable.col.lastError`, fallback: 'Last error' },
    { id: 'failedAttempts', labelKey: `${P}ExecuteBacklogTable.col.failedAttempts`, fallback: 'Failed runs', narrow: true },
    { id: 'engine', labelKey: `${P}ExecuteBacklogTable.col.engine`, fallback: 'Engine' }
  ]
}

export type BacklogClientFilters = {
  q: string
  type: RequestType | 'all'
  category: ReturnedCategory | 'all'
}

export const EMPTY_BACKLOG_FILTERS: BacklogClientFilters = { q: '', type: 'all', category: 'all' }

export function hasActiveBacklogFilters(f: BacklogClientFilters): boolean {
  return f.q.trim() !== '' || f.type !== 'all' || f.category !== 'all'
}

function matches(haystack: (string | number | undefined)[], q: string): boolean {
  const needle = q.trim().toLowerCase().replace(/^#/, '')
  if (!needle) {return true}
  return haystack.some((h) => h !== undefined && String(h).toLowerCase().includes(needle))
}

export function filterRequestRows(
  rows: RequestBacklogRowData[],
  f: Pick<BacklogClientFilters, 'q' | 'type' | 'category'>
): RequestBacklogRowData[] {
  return rows.filter(
    (r) =>
      (f.type === 'all' || r.type === f.type) &&
      (f.category === 'all' || r.returnedCategory === f.category) &&
      matches([r.title, r.number, r.sourceRef, r.returnReason], f.q)
  )
}

/** Keeps a group when the query hits its Plan/Phase title or any task; a hit on the group keeps all its tasks. */
export function filterGroups(groups: BacklogGroupData[], f: Pick<BacklogClientFilters, 'q'>): BacklogGroupData[] {
  if (!f.q.trim()) {return groups}
  const out: BacklogGroupData[] = []
  for (const g of groups) {
    if (matches([g.planTitle, g.phaseTitle], f.q)) {
      out.push(g)
      continue
    }
    const tasks = g.tasks.filter((t) => matches([t.title, t.taskId], f.q))
    if (tasks.length > 0) {out.push({ ...g, tasks })}
  }
  return out
}
