/**
 * QualityFindingsList.tsx — FE-CV-TASK-087-12
 *
 * Check findings of the dock. Rows are virtualized above 50 items; load errors are inline with a
 * retry (never a toast). The footer always states how many rows are shown out of the total, so a
 * truncated list is never mistaken for a complete one.
 *
 * @module components/review-map/quality/findings/QualityFindingsList
 */

import React, { useEffect, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { Button } from '../../../ui/button'
import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'
import { QualityFindingRow } from './QualityFindingRow'
import { qf } from './quality-findings-copy'

export const QUALITY_FINDINGS_VIRTUALIZE_ABOVE = 50
const ESTIMATED_ROW_HEIGHT = 64

export type QualityFindingsEmptyReason = 'no-run' | 'none' | 'filtered'

export type QualityFindingsListProps = {
  items: readonly QualityFinding[]
  status: 'idle' | 'loading' | 'ready' | 'error'
  total: number
  truncated: boolean
  hasMore: boolean
  capped: boolean
  loadingMore: boolean
  outsideScopeCount: number
  emptyReason: QualityFindingsEmptyReason
  selectedFingerprint: string | null
  onOpen?: (finding: QualityFinding) => void
  renderWaiveAction?: (finding: QualityFinding) => React.ReactNode
  onLoadMore: () => void
  onRetry: () => void
}

function emptyText(reason: QualityFindingsEmptyReason): string {
  if (reason === 'no-run') {
    return qf('emptyNoRun')
  }
  return reason === 'filtered' ? qf('emptyFiltered') : qf('emptyNone')
}

export function QualityFindingsList(props: QualityFindingsListProps): React.JSX.Element {
  const { items, selectedFingerprint } = props
  const scrollRef = useRef<HTMLDivElement | null>(null)
  const virtualized = items.length > QUALITY_FINDINGS_VIRTUALIZE_ABOVE
  const virtualizer = useVirtualizer({
    count: virtualized ? items.length : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ESTIMATED_ROW_HEIGHT,
    overscan: 8,
    getItemKey: (index) => items[index]?.fingerprint ?? index
  })

  useEffect(() => {
    if (!selectedFingerprint) {
      return
    }
    const index = items.findIndex((f) => f.fingerprint === selectedFingerprint)
    if (index === -1) {
      return
    }
    if (virtualized) {
      virtualizer.scrollToIndex(index, { align: 'center' })
      return
    }
    const node = scrollRef.current?.querySelector(
      `[data-fingerprint="${CSS.escape(selectedFingerprint)}"]`
    )
    node?.scrollIntoView?.({ block: 'nearest' })
    // Why: only a new selection should scroll; a list refresh must not yank the viewport back.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedFingerprint, virtualized])

  const renderRow = (finding: QualityFinding): React.JSX.Element => (
    <QualityFindingRow
      finding={finding}
      selected={finding.fingerprint === selectedFingerprint}
      onOpen={props.onOpen}
      waiveAction={props.renderWaiveAction?.(finding)}
    />
  )

  let body: React.ReactNode
  if (props.status === 'error' && items.length === 0) {
    body = (
      <div className="flex flex-col items-start gap-2 p-3 text-sm" role="alert">
        <span>{qf('loadError')}</span>
        <Button type="button" variant="outline" size="xs" onClick={props.onRetry}>
          {qf('retry')}
        </Button>
      </div>
    )
  } else if (props.status === 'loading' && items.length === 0) {
    body = <div className="p-3 text-sm text-muted-foreground">{qf('loading')}</div>
  } else if (items.length === 0) {
    body = <div className="p-3 text-sm text-muted-foreground">{emptyText(props.emptyReason)}</div>
  } else if (virtualized) {
    body = (
      <div role="rowgroup" style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
        {virtualizer.getVirtualItems().map((item) => (
          <div
            key={item.key}
            data-index={item.index}
            ref={virtualizer.measureElement}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              transform: `translateY(${item.start}px)`
            }}
          >
            {renderRow(items[item.index])}
          </div>
        ))}
      </div>
    )
  } else {
    body = (
      <div role="rowgroup">
        {items.map((finding) => (
          <React.Fragment key={finding.fingerprint}>{renderRow(finding)}</React.Fragment>
        ))}
      </div>
    )
  }

  const shownTotal = Math.max(props.total, items.length)
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div ref={scrollRef} role="table" className="min-h-0 flex-1 overflow-y-auto">
        {body}
        {props.status === 'error' && items.length > 0 ? (
          <div className="flex items-center gap-2 p-3 text-sm" role="alert">
            <span>{qf('loadError')}</span>
            <Button type="button" variant="outline" size="xs" onClick={props.onRetry}>
              {qf('retry')}
            </Button>
          </div>
        ) : null}
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-border px-3 py-1.5 text-xs text-muted-foreground">
        <span>{qf('footerShowing', { shown: items.length, total: shownTotal })}</span>
        {props.outsideScopeCount > 0 ? (
          <span>{qf('footerOutsideScope', { count: props.outsideScopeCount })}</span>
        ) : null}
        {props.capped ? <span>{qf('footerCapped')}</span> : null}
        {props.truncated && !props.capped ? <span>{qf('footerTruncated')}</span> : null}
        {props.hasMore && !props.capped ? (
          <Button
            type="button"
            variant="ghost"
            size="xs"
            disabled={props.loadingMore}
            onClick={props.onLoadMore}
          >
            {props.loadingMore ? qf('loading') : qf('loadMore')}
          </Button>
        ) : null}
      </div>
    </div>
  )
}
