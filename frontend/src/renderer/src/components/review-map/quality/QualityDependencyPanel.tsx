/**
 * QualityDependencyPanel.tsx — FE-CV-TASK-087-19
 *
 * Dependency structure matrix of the module graph (`imports` edges). At most 60 rows: the
 * busiest modules plus one "Other (N)" row. Cycles are outlined and marked with ▲, not only
 * coloured. Selecting a cell lists the edges between the two modules.
 *
 * @module components/review-map/quality/QualityDependencyPanel
 */

import { useMemo, useState } from 'react'
import { useQualityDependencyMatrix } from '@/hooks/useQualityDependencyMatrix'
import { Button } from '../../ui/button'
import { DependencyMatrix } from '../../quality-charts/DependencyMatrix'
import { chartCopy } from '../../quality-charts/quality-chart-copy'
import { describeQualityBlockError } from './quality-block-error-message'
import { QualityBlockStateFrame } from './QualityBlockStateFrame'
import { reduceDependencyGraph } from './quality-dependency-graph-reduction'
import type { QualityBlockProps } from './quality-lens-block-types'
import { qv } from './quality-visualization-copy'

const STRUCTURE_LENS_ID = 'structure'

export function QualityDependencyPanel({
  worktreeId,
  onOpenLens
}: QualityBlockProps): React.JSX.Element | null {
  const dsm = useQualityDependencyMatrix(worktreeId)
  const [cell, setCell] = useState<{ from: string; to: string } | null>(null)
  const model = useMemo(
    () => reduceDependencyGraph(dsm.graph, (count) => chartCopy('matrix.other', { count })),
    [dsm.graph]
  )
  if (dsm.support === 'disabled' || dsm.support === 'unsupported') {
    return null
  }
  const frameBase = {
    id: 'quality-dependency',
    title: qv('dependency.title'),
    description: qv('dependency.description')
  }
  const { status } = dsm
  if (status === 'idle' || status === 'loading') {
    return <QualityBlockStateFrame {...frameBase} status="loading" />
  }
  if (status === 'error') {
    return (
      <QualityBlockStateFrame
        {...frameBase}
        status="error"
        errorMessage={describeQualityBlockError(dsm.error)}
        onRetry={dsm.refetch}
      />
    )
  }
  if (status === 'unavailable') {
    return (
      <QualityBlockStateFrame {...frameBase} status="empty" emptyReason={qv('block.unavailable')} />
    )
  }
  if (model.edges.length === 0) {
    return (
      <QualityBlockStateFrame
        {...frameBase}
        status="empty"
        emptyReason={qv('dependency.emptyReason')}
      />
    )
  }
  const label = new Map(model.nodes.map((n) => [n.id, n.label]))
  const pair = cell
    ? model.edges.filter(
        (e) =>
          (e.from === cell.from && e.to === cell.to) || (e.from === cell.to && e.to === cell.from)
      )
    : []
  const backendCut = dsm.truncated && dsm.totalCount > (dsm.graph?.nodes?.length ?? 0)
  return (
    <section
      aria-label={frameBase.title}
      className="flex flex-col gap-2"
      data-quality-dependency
      data-status={status}
    >
      {status === 'stale' ? (
        <p className="text-xs text-muted-foreground" data-chart-stale>
          {qv('block.stale')}
        </p>
      ) : null}
      <DependencyMatrix
        frame={{
          ...frameBase,
          emptyReason: qv('dependency.emptyReason'),
          legend: (
            <>
              {model.otherCount > 0 ? (
                <p className="text-xs text-muted-foreground">
                  {chartCopy('frame.showing', { shown: model.shown, total: model.total })}
                </p>
              ) : null}
              {backendCut ? (
                <p className="text-xs text-muted-foreground">
                  {qv('dependency.backendTruncated', {
                    shown: dsm.graph?.nodes?.length ?? 0,
                    total: dsm.totalCount
                  })}
                </p>
              ) : null}
            </>
          )
        }}
        nodes={model.nodes}
        edges={model.edges}
        onSelectCell={(from, to) => setCell({ from, to })}
      />
      {cell ? (
        <div
          className="flex flex-col gap-1 rounded-md border border-border p-2 text-xs text-foreground"
          data-dependency-edges
        >
          <div className="flex items-center justify-between gap-2">
            <span className="font-medium">
              {qv('dependency.edgesTitle', {
                from: label.get(cell.from) ?? cell.from,
                to: label.get(cell.to) ?? cell.to
              })}
            </span>
            <Button type="button" size="sm" variant="ghost" onClick={() => setCell(null)}>
              {qv('dependency.close')}
            </Button>
          </div>
          {pair.length === 0 ? (
            <p className="text-muted-foreground">{qv('dependency.noEdge')}</p>
          ) : (
            <ul className="flex flex-col gap-0.5">
              {pair.map((e) => (
                <li key={`${e.from}>${e.to}`} className="tabular-nums">
                  {qv('dependency.edgeLine', {
                    from: label.get(e.from) ?? e.from,
                    to: label.get(e.to) ?? e.to,
                    count: e.weight
                  })}
                </li>
              ))}
            </ul>
          )}
          {onOpenLens ? (
            <div>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => onOpenLens(STRUCTURE_LENS_ID)}
              >
                {qv('dependency.openStructure')}
              </Button>
            </div>
          ) : null}
        </div>
      ) : null}
    </section>
  )
}

export default QualityDependencyPanel
