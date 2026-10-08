import { useId, useRef, useState } from 'react'
import { Button } from '../ui/button'
import { Skeleton } from '../ui/skeleton'
import { Toggle } from '../ui/toggle'
import { ChartTextAlternative } from './ChartTextAlternative'
import { chartCopy } from './quality-chart-copy'
import type { ChartSize, ChartStatus, ChartTableData } from './chart-frame-types'
import { useChartSize } from './useChartSize'

export type ChartFrameProps = {
  id: string
  title: string
  description?: string
  summary: string
  status: ChartStatus
  emptyReason?: React.ReactNode
  error?: { message: string; onRetry?: () => void }
  staleNote?: string
  legend?: React.ReactNode
  table: ChartTableData
  minHeight: number
  /** 'img' wraps the drawing in role="img"; 'custom' lets the chart own its role (grids). */
  surface?: 'img' | 'custom'
  children: (size: ChartSize) => React.ReactNode
}

// Why: before the first measurement (and in non-layout environments) charts still get a sane size.
const FALLBACK_WIDTH = 480

export function ChartFrame(props: ChartFrameProps): React.JSX.Element {
  const { id, title, description, summary, status, table, minHeight, surface = 'img' } = props
  const [showTable, setShowTable] = useState(false)
  const surfaceRef = useRef<HTMLDivElement | null>(null)
  const measured = useChartSize(surfaceRef)
  const tableId = useId()
  const size: ChartSize = {
    width: measured.width > 0 ? measured.width : FALLBACK_WIDTH,
    height: measured.height > 0 ? measured.height : minHeight
  }
  const drawable = status === 'ready' || status === 'stale'

  let body: React.ReactNode = null
  if (status === 'loading') {
    body = (
      <div style={{ height: minHeight }}>
        <Skeleton className="h-full w-full animate-none" />
        <span className="sr-only">{chartCopy('frame.loading')}</span>
      </div>
    )
  } else if (status === 'empty') {
    // Why: an empty chart must say why; it never implies the scope is healthy.
    body = (
      <p className="p-3 text-sm text-muted-foreground" data-chart-empty>
        {props.emptyReason ?? chartCopy('frame.emptyDefault')}
      </p>
    )
  } else if (status === 'error') {
    body = (
      <div
        role="alert"
        className="flex flex-col items-start gap-2 p-3 text-sm text-foreground"
        data-chart-error
      >
        <p>{props.error?.message ?? chartCopy('frame.errorDefault')}</p>
        {props.error?.onRetry ? (
          <Button type="button" variant="outline" size="sm" onClick={props.error.onRetry}>
            {chartCopy('frame.retry')}
          </Button>
        ) : null}
      </div>
    )
  } else if (!showTable) {
    body =
      surface === 'img' ? (
        <div role="img" aria-label={summary} className="h-full w-full">
          {props.children(size)}
        </div>
      ) : (
        props.children(size)
      )
  }

  return (
    <figure
      id={id}
      data-chart-frame
      data-status={status}
      aria-busy={status === 'loading' ? true : undefined}
      className="m-0 flex flex-col gap-2 rounded-lg border border-border bg-card p-3 text-card-foreground"
    >
      <figcaption className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-sm font-medium">{title}</div>
          {description ? <div className="text-xs text-muted-foreground">{description}</div> : null}
        </div>
        {drawable ? (
          <Toggle
            variant="outline"
            size="sm"
            pressed={showTable}
            onPressedChange={setShowTable}
            aria-controls={tableId}
          >
            {showTable ? chartCopy('frame.viewChart') : chartCopy('frame.viewTable')}
          </Toggle>
        ) : null}
      </figcaption>
      <div ref={surfaceRef} data-chart-surface className="relative w-full" style={{ minHeight }}>
        {body}
      </div>
      {props.legend ? <div data-chart-legend-slot>{props.legend}</div> : null}
      {status === 'stale' && props.staleNote ? (
        <p className="text-xs text-muted-foreground" data-chart-stale>
          {chartCopy('frame.stale')}: {props.staleNote}
        </p>
      ) : null}
      <ChartTextAlternative id={tableId} data={table} visible={showTable && drawable} />
    </figure>
  )
}
