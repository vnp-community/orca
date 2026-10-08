/**
 * DataFlowLens.tsx — FE-CV-TASK-056-06
 *
 * Flow list + sequence diagram + step list. Flows are server-built and heuristic; `partial`
 * flows are always labeled as such.
 */

import { useMemo, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import type { ReviewLensProps } from '../review-lens-registry'
import { useDataFlows } from '../../../hooks/useDataFlows'
import { DataFlowDetailPane } from './DataFlowDetailPane'
import { DataFlowFilters } from './DataFlowFilters'
import { DataFlowListPane } from './DataFlowListPane'
import { usePaneWidthBelow } from './use-pane-width-below'
import { filterFlowsTouchingChange } from './data-flow-overlay'

export default function DataFlowLens(props: ReviewLensProps): React.JSX.Element {
  const { worktreeId, environmentId, overlay, chipFilter, onSelectSymbol } = props
  const flowId = useAppStore((s) => s.reviewUiByWorktree[worktreeId]?.dataFlowId ?? null)
  const setFlowId = useAppStore((s) => s.setReviewDataFlowId)
  const [query, setQuery] = useState('')
  // Arriving from the "flows" chip means "show me what my change touches".
  const [touchOnly, setTouchOnly] = useState(chipFilter === 'flows')

  const [triggerKind, setTriggerKind] = useState<string | null>(null)
  const [service, setService] = useState<string | null>(null)
  const sectionRef = useRef<HTMLElement>(null)
  const compact = usePaneWidthBelow(sectionRef, 720)

  const list = useDataFlows(worktreeId, environmentId, { query, triggerKind, service })
  const services = useMemo(
    () => [...new Set(list.flows.map((f) => f.entryService).filter(Boolean))],
    [list.flows]
  )
  const touch = useMemo(() => filterFlowsTouchingChange(list.flows, overlay), [list.flows, overlay])
  const selected = list.flows.find((f) => f.id === flowId)

  return (
    <section
      ref={sectionRef}
      data-compact={compact}
      aria-label={translate('auto.components.reviewMap.lens.dataflow.label', 'Flows')}
      className={`flex min-h-0 flex-1 flex-col gap-2 p-2 ${compact ? '' : 'md:flex-row'}`}
    >
      <DataFlowListPane
        status={list.status}
        flows={list.flows}
        total={list.total}
        hasMore={list.hasMore}
        loadingMore={list.loadingMore}
        errorMessage={list.error?.message}
        query={query}
        onQueryChange={setQuery}
        touchOnly={touchOnly}
        onTouchOnlyChange={setTouchOnly}
        touch={touch}
        selectedId={flowId}
        onSelect={(id) => setFlowId(worktreeId, id)}
        onLoadMore={list.loadMore}
        onRetry={list.refetch}
        compact={compact}
        filters={
          <DataFlowFilters
            triggerKind={triggerKind}
            onTriggerKindChange={setTriggerKind}
            service={service}
            services={services}
            onServiceChange={setService}
          />
        }
      />
      {flowId ? (
        <DataFlowDetailPane
          key={flowId}
          worktreeId={worktreeId}
          environmentId={environmentId}
          flowId={flowId}
          fallbackLabel={selected?.label}
          overlay={overlay}
          onSelectSymbol={onSelectSymbol}
        />
      ) : (
        <p className="flex-1 p-3 text-sm text-muted-foreground">
          {translate('auto.components.reviewMap.dataflow.pick', 'Pick a flow to see its sequence.')}
        </p>
      )}
    </section>
  )
}
