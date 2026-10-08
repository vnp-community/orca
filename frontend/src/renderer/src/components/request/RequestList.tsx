/**
 * RequestList — CR-REQ-019-02
 *
 * Listbox of request rows with j/k/Enter/Escape keyboard navigation.
 *
 * @module components/request/RequestList
 */

import React from 'react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { RequestRow } from './RequestRow'
import { handleRequestListKey } from './request-list-keyboard'
import type { OrcaRequest } from '../../../../shared/request-types'

type Props = {
  requests: OrcaRequest[]
  activeId: string | null
  hasMore: boolean
  isLoadingMore: boolean
  onSelect: (id: string) => void
  onClose: () => void
  onLoadMore: () => void
}

export function RequestList({
  requests,
  activeId,
  hasMore,
  isLoadingMore,
  onSelect,
  onClose,
  onLoadMore
}: Props): React.JSX.Element {
  const ids = React.useMemo(() => requests.map((r) => r.id), [requests])

  return (
    <div
      role="listbox"
      tabIndex={0}
      aria-label={translate('auto.components.request.RequestList.label', 'Requests')}
      className="flex-1 overflow-y-auto focus-visible:outline-none"
      onKeyDown={(e) =>
        handleRequestListKey(e, { ids, activeId, onMove: onSelect, onOpen: onSelect, onClose })
      }
    >
      {requests.map((request) => (
        <RequestRow key={request.id} request={request} selected={request.id === activeId} onSelect={onSelect} />
      ))}
      {hasMore && (
        <div className="flex justify-center p-3">
          <Button variant="outline" size="sm" disabled={isLoadingMore} onClick={onLoadMore}>
            {translate('auto.components.request.RequestList.loadMore', 'Load more')}
          </Button>
        </div>
      )}
    </div>
  )
}
