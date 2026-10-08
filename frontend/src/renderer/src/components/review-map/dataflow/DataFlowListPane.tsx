/** DataFlowListPane.tsx — FE-CV-TASK-056-05 */

import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import type { DataFlowSummary } from '../../../../../shared/code-intel-architecture-types'

export type TouchFilterState = { ids: ReadonlySet<string>; unknown: boolean }

export function DataFlowListPane({
  status,
  flows,
  total,
  hasMore,
  loadingMore,
  errorMessage,
  query,
  onQueryChange,
  touchOnly,
  onTouchOnlyChange,
  touch,
  selectedId,
  onSelect,
  onLoadMore,
  onRetry,
  filters,
  compact = false
}: {
  status: 'loading' | 'ready' | 'error'
  flows: readonly DataFlowSummary[]
  total: number
  hasMore: boolean
  loadingMore: boolean
  errorMessage?: string
  query: string
  onQueryChange: (q: string) => void
  touchOnly: boolean
  onTouchOnlyChange: (v: boolean) => void
  touch: TouchFilterState
  selectedId: string | null
  onSelect: (id: string) => void
  onLoadMore: () => void
  onRetry: () => void
  /** Trigger / service filter controls rendered under the search box. */
  filters?: React.ReactNode
  /** Narrow pane (< 720 px): the list collapses into a Select. */
  compact?: boolean
}): React.JSX.Element {
  // Only filters the already-listed page by id; the text search itself is server-side.
  const visible = touchOnly && !touch.unknown ? flows.filter((f) => touch.ids.has(f.id)) : flows
  return (
    <div
      className={cn(
        'flex min-h-0 w-full flex-col gap-2',
        !compact && 'md:w-72 md:shrink-0 md:border-r md:pr-2'
      )}
    >
      <Input
        type="search"
        value={query}
        maxLength={128}
        placeholder={translate('auto.components.reviewMap.dataflow.search', 'Search flows')}
        aria-label={translate('auto.components.reviewMap.dataflow.search', 'Search flows')}
        onChange={(e) => onQueryChange(e.target.value)}
      />
      {filters}
      <label className="flex items-center gap-1.5 text-xs">
        <input
          type="checkbox"
          checked={touchOnly}
          onChange={(e) => onTouchOnlyChange(e.target.checked)}
        />
        {translate('auto.components.reviewMap.dataflow.touchOnly', 'Only flows touching changes')}
      </label>
      {touchOnly && touch.unknown ? (
        <p role="status" className="text-xs text-muted-foreground">
          {translate(
            'auto.components.reviewMap.dataflow.touchUnknown',
            'Cannot tell yet which flows touch the changes.'
          )}
        </p>
      ) : null}

      {status === 'loading' ? (
        <p role="status" className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
          {translate('auto.components.reviewMap.dataflow.loading', 'Loading flows...')}
        </p>
      ) : null}
      {status === 'error' ? (
        <div role="alert" className="space-y-1 text-xs">
          <p>{translate('auto.components.reviewMap.dataflow.error', 'Could not load flows.')}</p>
          {errorMessage ? <p className="text-muted-foreground">{errorMessage}</p> : null}
          <Button size="xs" variant="outline" onClick={onRetry}>
            {translate('auto.components.reviewMap.dataflow.retry', 'Retry')}
          </Button>
        </div>
      ) : null}
      {status === 'ready' && flows.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          {translate(
            'auto.components.reviewMap.dataflow.empty',
            'No data flows could be built for this repo yet.'
          )}
        </p>
      ) : null}

      {compact ? (
        <Select value={selectedId ?? undefined} onValueChange={onSelect}>
          <SelectTrigger
            size="sm"
            className="w-full text-xs"
            aria-label={translate('auto.components.reviewMap.dataflow.list', 'Flows')}
          >
            <SelectValue
              placeholder={translate('auto.components.reviewMap.dataflow.pickShort', 'Pick a flow')}
            />
          </SelectTrigger>
          <SelectContent>
            {visible.map((f) => (
              <SelectItem key={f.id} value={f.id}>
                {f.completeness === 'partial'
                  ? `${f.label} · ${translate('auto.components.reviewMap.dataflow.partialTag', 'partial')}`
                  : f.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : null}
      <ul
        hidden={compact}
        aria-label={translate('auto.components.reviewMap.dataflow.list', 'Flows')}
        className="min-h-0 flex-1 space-y-0.5 overflow-auto"
      >
        {visible.map((f) => (
          <li key={f.id}>
            <button
              type="button"
              aria-current={f.id === selectedId ? 'true' : undefined}
              className={cn(
                'w-full rounded-md px-2 py-1.5 text-left text-xs hover:bg-accent',
                f.id === selectedId && 'bg-accent'
              )}
              onClick={() => onSelect(f.id)}
            >
              <span className="block truncate font-medium">{f.label}</span>
              <span className="flex flex-wrap gap-x-2 text-[11px] text-muted-foreground">
                <span className="truncate">{f.trigger.name}</span>
                <span>
                  {translate('auto.components.reviewMap.dataflow.hops', '{{count}} hops', {
                    count: f.serviceHops
                  })}
                </span>
                {f.completeness === 'partial' ? (
                  <span>
                    {translate('auto.components.reviewMap.dataflow.partialTag', 'partial')}
                  </span>
                ) : null}
              </span>
            </button>
          </li>
        ))}
      </ul>
      {status === 'ready' ? (
        <div className="flex items-center justify-between text-[11px] text-muted-foreground">
          <span>
            {translate('auto.components.reviewMap.dataflow.count', '{{shown}} of {{total}}', {
              shown: flows.length,
              total: Math.max(total, flows.length)
            })}
          </span>
          {hasMore ? (
            <Button size="xs" variant="ghost" disabled={loadingMore} onClick={onLoadMore}>
              {translate('auto.components.reviewMap.dataflow.loadMore', 'Load more')}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
