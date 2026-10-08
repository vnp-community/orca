/**
 * RequestListToolbar — CR-REQ-019-02
 *
 * Result count and refresh above the list; hosts the filter bar.
 *
 * @module components/request/RequestListToolbar
 */

import React from 'react'
import { RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { RequestFilterBar } from './RequestFilterBar'
import type { RequestListFilters } from '../../store/slices/request'

type Props = {
  filters: RequestListFilters
  count: number
  isLoading: boolean
  onFiltersChange: (next: RequestListFilters) => void
  onRefresh: () => void
}

export function RequestListToolbar({ filters, count, isLoading, onFiltersChange, onRefresh }: Props): React.JSX.Element {
  return (
    <div className="shrink-0">
      <div className="flex items-center justify-between px-3 pt-2 text-xs text-muted-foreground">
        <span>{translate('auto.components.request.RequestListToolbar.count', '{{count}} requests', { count })}</span>
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={onRefresh}
          disabled={isLoading}
          aria-label={translate('auto.components.request.RequestListToolbar.refresh', 'Refresh')}
        >
          <RefreshCw className="size-3" aria-hidden />
        </Button>
      </div>
      <RequestFilterBar filters={filters} onChange={onFiltersChange} />
    </div>
  )
}
