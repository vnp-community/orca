/**
 * BacklogTab — CR-REQ-023-04
 *
 * Request / Task / Execute backlog. One hook instance per view, only the open
 * one talks to the server, so switching views reuses its cache and a failing
 * view cannot blank another.
 *
 * @module components/request/backlog/BacklogTab
 */

import React, { useCallback, useMemo, useState } from 'react'
import { TooltipProvider } from '@/components/ui/tooltip'
import { i18n } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useExecuteBacklog, useRequestBacklog, useTaskBacklog } from '../../../hooks/useBacklog'
import { useMinuteClock } from '../../../hooks/useMinuteClock'
import {
  BacklogEmptyFiltered, BacklogEmptyState, BacklogErrorState, BacklogSkeleton
} from './BacklogStates'
import {
  BACKLOG_VIEWS, BacklogSegmentControl, backlogViewForKeyEvent, type BacklogCount
} from './BacklogSegmentControl'
import { BacklogToolbar } from './BacklogToolbar'
import {
  BACKLOG_COLUMNS, EMPTY_BACKLOG_FILTERS, filterGroups, filterRequestRows, hasActiveBacklogFilters,
  type BacklogClientFilters
} from './backlog-view-columns'
import { ExecuteBacklogTable } from './ExecuteBacklogTable'
import { RequestBacklogTable } from './RequestBacklogTable'
import { TaskBacklogTable } from './TaskBacklogTable'
import type { BacklogState } from '../../../hooks/useBacklog'
import type { BacklogView } from '../../../../../shared/request-types'

export function BacklogTab(): React.JSX.Element | null {
  const view = useAppStore((s) => s.requestPage.backlogView)
  const projectId = useAppStore((s) => s.requestPage.listFilters.projectId)
  const setRequestPageData = useAppStore((s) => s.setRequestPageData)
  const [filters, setFilters] = useState<BacklogClientFilters>(EMPTY_BACKLOG_FILTERS)
  const now = useMinuteClock()
  const locale = i18n.language || 'en'

  const server = useMemo(() => ({ projectId }), [projectId])
  const requests = useRequestBacklog(server, { active: view === 'requests' })
  const tasks = useTaskBacklog(server, { active: view === 'tasks' })
  const execute = useExecuteBacklog(server, { active: view === 'execute' })
  const states: Record<BacklogView, BacklogState<unknown>> = { requests, tasks, execute }
  const current = states[view]

  const changeView = useCallback(
    (next: BacklogView) => setRequestPageData({ backlogView: next }),
    [setRequestPageData]
  )

  const counts = Object.fromEntries(
    BACKLOG_VIEWS.map((v): [BacklogView, BacklogCount] => [v, states[v].loadedOnce ? states[v].countLabel() : null])
  ) as Record<BacklogView, BacklogCount>

  const visibleRequests = useMemo(() => filterRequestRows(requests.items, filters), [requests.items, filters])
  const visibleTasks = useMemo(() => filterGroups(tasks.items, filters), [tasks.items, filters])
  const visibleExecute = useMemo(() => filterGroups(execute.items, filters), [execute.items, filters])

  if (!current.supported) {return null}

  const visibleCount = view === 'requests' ? visibleRequests.length : view === 'tasks' ? visibleTasks.length : visibleExecute.length
  const filtered = hasActiveBacklogFilters(filters)
  const errorKind = current.error?.kind
  const showSkeleton = !current.loadedOnce && !current.error

  let body: React.ReactNode
  if (errorKind === 'forbidden') {
    body = <BacklogErrorState kind="forbidden" onRetry={current.refetch} />
  } else if (showSkeleton) {
    body = <BacklogSkeleton columns={BACKLOG_COLUMNS[view].length} />
  } else if (!current.loadedOnce) {
    body = <BacklogErrorState kind={errorKind ?? 'network'} onRetry={current.refetch} />
  } else {
    const banner = current.error && <BacklogErrorState kind={errorKind ?? 'network'} onRetry={current.refetch} />
    if (visibleCount === 0) {
      const rawEmpty = current.items.length === 0
      body = (
        <>
          {banner}
          {rawEmpty || !filtered ? (
            <BacklogEmptyState view={view} />
          ) : (
            <BacklogEmptyFiltered onClear={() => setFilters(EMPTY_BACKLOG_FILTERS)} />
          )}
        </>
      )
    } else {
      const dimmed = Boolean(current.error)
      const common = { hasMore: current.hasMore, isLoadingMore: current.isLoadingMore, dimmed, onLoadMore: current.loadMore }
      body = (
        <>
          {banner}
          {view === 'requests' && (
            <RequestBacklogTable rows={visibleRequests} now={now} locale={locale} onChanged={requests.refetch} {...common} />
          )}
          {view === 'tasks' && <TaskBacklogTable groups={visibleTasks} onTaskSheetClosed={tasks.refetch} {...common} />}
          {view === 'execute' && <ExecuteBacklogTable groups={visibleExecute} onTaskSheetClosed={execute.refetch} {...common} />}
        </>
      )
    }
  }

  return (
    <TooltipProvider>
      <div
        className="flex h-full flex-col overflow-hidden"
        data-testid="request-tab-backlog"
        onKeyDown={(e) => {
          const next = backlogViewForKeyEvent(e)
          if (next) {
            e.preventDefault()
            changeView(next)
          }
        }}
      >
        <div className="flex flex-wrap items-center gap-3 border-b border-border px-3 py-2">
          <BacklogSegmentControl value={view} onChange={changeView} counts={counts} />
          <div className="min-w-0 flex-1">
            <BacklogToolbar
              view={view}
              filters={filters}
              onFiltersChange={setFilters}
              onRefresh={current.refetch}
              isRefreshing={current.isLoading}
            />
          </div>
        </div>
        {body}
      </div>
    </TooltipProvider>
  )
}
