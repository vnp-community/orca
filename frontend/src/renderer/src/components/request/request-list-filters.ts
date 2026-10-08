/**
 * request-list-filters.ts — CR-REQ-019-02
 *
 * Pure helpers for the Requests list filter bar (option lists, quick chips,
 * toggling). Kept free of React so the rules are unit-testable.
 *
 * @module components/request/request-list-filters
 */

import type { RequestListFilters } from '../../store/slices/request'
import type {
  RequestSourceProvider,
  RequestStatus,
  RequestType
} from '../../../../shared/request-types'

export const FILTERABLE_STATUSES: Exclude<RequestStatus, 'unknown'>[] = [
  'submitted',
  'classifying',
  'awaiting_type_confirmation',
  'analyzing',
  'awaiting_analysis_approval',
  'awaiting_information',
  'planning',
  'awaiting_plan_approval',
  'executing',
  'completed',
  'cancelled',
  'request_backlog'
]

export const FILTERABLE_TYPES: Exclude<RequestType, 'unknown'>[] = [
  'bug',
  'task',
  'docs',
  'question',
  'hotfix',
  'security',
  'ops_request',
  'change_request',
  'refactor',
  'spike',
  'performance'
]

export const FILTERABLE_SOURCES: Exclude<RequestSourceProvider, 'unknown'>[] = [
  'jira',
  'github',
  'gitlab',
  'linear',
  'mcp',
  'manual',
  'webhook'
]

export type QuickFilterId = 'needsMyConfirmation' | 'running' | 'awaitingInfo'

export const QUICK_FILTER_STATUSES: Record<QuickFilterId, RequestStatus[]> = {
  needsMyConfirmation: ['awaiting_type_confirmation'],
  running: ['analyzing', 'planning', 'executing'],
  awaitingInfo: ['awaiting_information']
}

export function toggleFilterValue<T>(values: T[] | undefined, value: T): T[] | undefined {
  const current = values ?? []
  const next = current.includes(value) ? current.filter((v) => v !== value) : [...current, value]
  return next.length > 0 ? next : undefined
}

export function isQuickFilterActive(filters: RequestListFilters, id: QuickFilterId): boolean {
  const wanted = QUICK_FILTER_STATUSES[id]
  const status = filters.status ?? []
  return status.length === wanted.length && wanted.every((s) => status.includes(s))
}

/** Applies or clears a quick chip; chips replace the status filter. */
export function applyQuickFilter(
  filters: RequestListFilters,
  id: QuickFilterId
): RequestListFilters {
  return isQuickFilterActive(filters, id)
    ? { ...filters, status: undefined }
    : { ...filters, status: [...QUICK_FILTER_STATUSES[id]] }
}

export function hasActiveListFilters(filters: RequestListFilters): boolean {
  return Boolean(filters.status?.length || filters.type?.length || filters.sourceProvider)
}

/** Clears everything except the project scope, which lives in the page header. */
export function clearListFilters(filters: RequestListFilters): RequestListFilters {
  return filters.projectId ? { projectId: filters.projectId } : {}
}
