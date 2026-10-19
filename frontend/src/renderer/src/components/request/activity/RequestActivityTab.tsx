/**
 * RequestActivityTab — FE-REQ-TASK-037-07
 *
 * The History tab: what happened to the Request, newest first, grouped by day. A runtime without `request.activity`
 * falls back to the type-change list.
 *
 * @module components/request/activity/RequestActivityTab
 */

import React, { useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { i18n, translate } from '@/i18n/i18n'
import { useRequestActivity } from '../../../hooks/useRequestActivity'
import { requestErrorMessage } from '../request-error-message'
import { RequestHistoryTab } from '../RequestHistoryTab'
import type { RequestTypeHistoryEntry } from '../../../../../shared/request-types'
import { ActivityEntryRow } from './ActivityEntryRow'
import type { ActivityTargetTab } from './activity-entry-presentation'
import { ActivityFilterBar } from './ActivityFilterBar'
import { groupActivityByDay, type ActivityDayGroup } from './activity-day-grouping'
import {
  ACTIVITY_FILTER_DEFAULT,
  activityFilterParams,
  isActivityFilterDefault,
  type ActivityFilterState
} from './activity-filter-params'

const T = 'auto.components.request.activity.'

type Props = {
  requestId: string
  /** Goes to the tab an entry is about; omitted hides the "Open" links. */
  onOpenTab?: (tab: ActivityTargetTab) => void
  active: boolean
  typeHistory: RequestTypeHistoryEntry[]
  typeHistoryLoading: boolean
  typeHistoryError: string | null
  onRetryTypeHistory: () => void
}

function dayHeading(group: ActivityDayGroup): string {
  if (group.relative === 'today') {
    return translate(`${T}day.today`, 'Today')
  }
  if (group.relative === 'yesterday') {
    return translate(`${T}day.yesterday`, 'Yesterday')
  }
  const first = group.entries[0]
  const time = first ? Date.parse(first.occurredAt) : Number.NaN
  if (Number.isNaN(time)) {
    return translate(`${T}day.unknown`, 'Unknown date')
  }
  return new Intl.DateTimeFormat(i18n.language || 'en', { dateStyle: 'medium' }).format(time)
}

export function RequestActivityTab({
  requestId,
  onOpenTab,
  active,
  typeHistory,
  typeHistoryLoading,
  typeHistoryError,
  onRetryTypeHistory
}: Props): React.JSX.Element | null {
  const [filterState, setFilterState] = useState<ActivityFilterState>(ACTIVITY_FILTER_DEFAULT)
  const filter = useMemo(() => activityFilterParams(filterState), [filterState])
  const { entries, hasMore, isLoading, isLoadingMore, error, unsupported, loadMore, refetch } =
    useRequestActivity(requestId, {
      enabled: active,
      filter
    })
  const groups = useMemo(() => groupActivityByDay(entries), [entries])

  if (!active) {
    return null
  }
  // Why: an older runtime has no activity channel; it is not an error, so show what it can (type changes).
  if (unsupported) {
    return (
      <RequestHistoryTab
        entries={typeHistory}
        isLoading={typeHistoryLoading}
        error={typeHistoryError}
        onRetry={onRetryTypeHistory}
      />
    )
  }

  const firstLoad = isLoading && entries.length === 0
  return (
    <div className="flex flex-col" data-testid="request-activity-tab">
      <ActivityFilterBar state={filterState} onChange={setFilterState} />
      {firstLoad && (
        <div className="flex flex-col gap-2 p-4" aria-busy="true" data-testid="activity-loading">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-4 w-3/5" />
        </div>
      )}
      {error && (
        <div role="alert" className="m-4 flex items-center gap-2 text-sm">
          <span>{requestErrorMessage(error)}</span>
          <Button size="xs" variant="outline" onClick={refetch}>
            {translate('auto.components.request.error.retry', 'Retry')}
          </Button>
        </div>
      )}
      {!firstLoad && !error && entries.length === 0 && (
        <p className="p-4 text-sm text-muted-foreground" data-testid="activity-empty">
          {isActivityFilterDefault(filterState)
            ? translate(`${T}empty`, 'No activity yet')
            : translate(`${T}emptyFiltered`, 'No activity matches these filters.')}
        </p>
      )}
      {groups.map((group) => (
        <section
          key={group.dayKey}
          aria-labelledby={`activity-day-${group.dayKey}`}
          data-testid="activity-day-group"
        >
          <h3
            id={`activity-day-${group.dayKey}`}
            className="sticky top-0 border-b border-border bg-background px-4 py-1.5 text-xs font-medium text-muted-foreground"
          >
            {dayHeading(group)}
          </h3>
          <ol
            className="flex flex-col divide-y divide-border"
            aria-labelledby={`activity-day-${group.dayKey}`}
          >
            {group.entries.map((entry) => (
              <ActivityEntryRow key={entry.id} entry={entry} onOpenTab={onOpenTab} />
            ))}
          </ol>
        </section>
      ))}
      {hasMore && (
        <div className="flex justify-center p-3">
          <Button size="xs" variant="outline" disabled={isLoadingMore} onClick={loadMore}>
            {isLoadingMore
              ? translate(`${T}loadingMore`, 'Loading…')
              : translate(`${T}loadMore`, 'Load more')}
          </Button>
        </div>
      )}
    </div>
  )
}
