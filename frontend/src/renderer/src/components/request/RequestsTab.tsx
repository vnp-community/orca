/**
 * RequestsTab — CR-REQ-019-02
 *
 * Request list (filters, keyboard navigation, states) beside the detail pane.
 * Below 768px the detail replaces the list.
 *
 * @module components/request/RequestsTab
 */

import React, { useCallback, useEffect, useRef } from 'react'
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useRequestNarrowLayout } from '../../hooks/useRequestNarrowLayout'
import { useRequests } from '../../hooks/useRequests'
import { useRequestSubscription } from '../../hooks/useRequestSubscription'
import { RequestDetailPane } from './RequestDetailPane'
import { RequestList } from './RequestList'
import { RequestListToolbar } from './RequestListToolbar'
import { RequestListEmpty, RequestListError, RequestListSkeleton } from './RequestListStates'
import { clearListFilters, hasActiveListFilters } from './request-list-filters'
import type { RequestListFilters } from '../../store/slices/request'

const REFETCH_DEBOUNCE_MS = 400

type Props = {
  /** Opens the create dialog (CR-REQ-019-06). */
  onCreate?: () => void
}

export function RequestsTab({ onCreate }: Props): React.JSX.Element {
  const filters = useAppStore((s) => s.requestPage.listFilters)
  const selectedId = useAppStore((s) => s.requestPage.requestId)
  const setRequestPageData = useAppStore((s) => s.setRequestPageData)
  const setRequestPageRequest = useAppStore((s) => s.setRequestPageRequest)
  const narrow = useRequestNarrowLayout()

  const { requests, isLoading, hasLoaded, error, nextPageToken, nextPage, refetch } = useRequests({
    projectId: filters.projectId,
    status: filters.status,
    type: filters.type,
    sourceProvider: filters.sourceProvider as never
  })

  // Why: coalesce bursts of stream events into one list refetch.
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => void (timer.current && clearTimeout(timer.current)), [])
  useRequestSubscription({
    onEvent: () => {
      if (timer.current) {clearTimeout(timer.current)}
      timer.current = setTimeout(refetch, REFETCH_DEBOUNCE_MS)
    }
  })

  const setFilters = useCallback(
    (next: RequestListFilters) => setRequestPageData({ listFilters: next }),
    [setRequestPageData]
  )
  const close = useCallback(() => setRequestPageRequest(null), [setRequestPageRequest])

  const filtered = hasActiveListFilters(filters)
  const showSkeleton = requests.length === 0 && (!hasLoaded || (isLoading && !error))

  const list = (
    <div className="flex h-full flex-col overflow-hidden" data-testid="request-tab-requests">
      <RequestListToolbar
        filters={filters}
        count={requests.length}
        isLoading={isLoading}
        onFiltersChange={setFilters}
        onRefresh={refetch}
      />
      {error && requests.length === 0 ? (
        <RequestListError kind={error} onRetry={refetch} />
      ) : showSkeleton ? (
        <RequestListSkeleton />
      ) : requests.length === 0 ? (
        <RequestListEmpty filtered={filtered} onClearFilters={() => setFilters(clearListFilters(filters))} onCreate={onCreate} />
      ) : (
        <>
          {error && <RequestListError kind={error} onRetry={refetch} />}
          <RequestList
            requests={requests}
            activeId={selectedId}
            hasMore={nextPageToken !== null}
            isLoadingMore={isLoading}
            onSelect={setRequestPageRequest}
            onClose={close}
            onLoadMore={nextPage}
          />
        </>
      )}
    </div>
  )

  const detail = selectedId ? (
    <RequestDetailPane key={selectedId} requestId={selectedId} onBackToList={narrow ? close : undefined} />
  ) : (
    <div className="flex h-full items-center justify-center p-6 text-sm text-muted-foreground" data-testid="request-detail-placeholder">
      {translate('auto.components.request.RequestsTab.selectPrompt', 'Select a request to see its details.')}
    </div>
  )

  if (narrow) {return selectedId ? detail : list}

  return (
    <ResizablePanelGroup orientation="horizontal" className="h-full">
      <ResizablePanel defaultSize="40" minSize="25">
        {list}
      </ResizablePanel>
      <ResizableHandle />
      <ResizablePanel defaultSize="60" minSize="30">
        {detail}
      </ResizablePanel>
    </ResizablePanelGroup>
  )
}
