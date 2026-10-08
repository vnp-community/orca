/**
 * BacklogToolbar — CR-REQ-023-04
 *
 * Client-side search plus (Request view only) type and return-category filters.
 *
 * @module components/request/backlog/BacklogToolbar
 */

import React, { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import type { ReturnedCategory } from '../../../../../shared/request-backlog-types'
import type { BacklogView, RequestType } from '../../../../../shared/request-types'
import type { BacklogClientFilters } from './backlog-view-columns'

const T = 'auto.components.request.backlog.'
export const SEARCH_DEBOUNCE_MS = 200

const TYPES: RequestType[] = [
  'bug', 'task', 'docs', 'question', 'hotfix', 'security', 'ops_request', 'change_request', 'refactor', 'spike', 'performance'
]
const CATEGORIES: Exclude<ReturnedCategory, 'unknown'>[] = ['missing_info', 'infeasible', 'blocked_dependency', 'rejected', 'other']
const CATEGORY_LABELS: Record<string, string> = {
  missing_info: 'Missing information', infeasible: 'Not feasible', blocked_dependency: 'Blocked by dependency',
  rejected: 'Rejected', other: 'Other'
}

export function BacklogToolbar({
  view,
  filters,
  onFiltersChange,
  onRefresh,
  isRefreshing
}: {
  view: BacklogView
  filters: BacklogClientFilters
  onFiltersChange: (next: BacklogClientFilters) => void
  onRefresh: () => void
  isRefreshing: boolean
}): React.JSX.Element {
  const [text, setText] = useState(filters.q)
  // Keep the box in step when filters are cleared from outside.
  useEffect(() => { setText(filters.q) }, [filters.q])
  useEffect(() => {
    if (text === filters.q) {return}
    const timer = setTimeout(() => onFiltersChange({ ...filters, q: text }), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [text, filters, onFiltersChange])

  return (
    <div className="flex flex-wrap items-center gap-2" data-testid="backlog-toolbar">
      <Input
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={translate(`${T}BacklogToolbar.search`, 'Search title or #number')}
        aria-label={translate(`${T}BacklogToolbar.search`, 'Search title or #number')}
        className="h-8 w-56"
      />
      {view === 'requests' && (
        <>
          <Select value={filters.type} onValueChange={(v) => onFiltersChange({ ...filters, type: v as RequestType | 'all' })}>
            <SelectTrigger size="sm" aria-label={translate(`${T}BacklogToolbar.type`, 'Type')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{translate(`${T}BacklogToolbar.allTypes`, 'All types')}</SelectItem>
              {TYPES.map((t) => (
                <SelectItem key={t} value={t}>
                  {translate(`auto.components.request.RequestType.${t}.label`, t)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={filters.category} onValueChange={(v) => onFiltersChange({ ...filters, category: v as ReturnedCategory | 'all' })}>
            <SelectTrigger size="sm" aria-label={translate(`${T}BacklogToolbar.category`, 'Category')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{translate(`${T}BacklogToolbar.allCategories`, 'All categories')}</SelectItem>
              {CATEGORIES.map((c) => (
                <SelectItem key={c} value={c}>
                  {translate(`${T}ReturnedCategory.${c}`, CATEGORY_LABELS[c])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </>
      )}
      <Button variant="outline" size="sm" className="ml-auto" disabled={isRefreshing} onClick={onRefresh}>
        <RefreshCw className={isRefreshing ? 'size-3.5 animate-spin' : 'size-3.5'} aria-hidden />
        {translate(`${T}BacklogToolbar.refresh`, 'Refresh')}
      </Button>
    </div>
  )
}
