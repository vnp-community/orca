/** DataFlowDetailPane.tsx — FE-CV-TASK-056-06 */

import { useMemo, useRef, useState } from 'react'
import { TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { translate } from '@/i18n/i18n'
import { useDataFlow, type DataFlowDetail } from '../../../hooks/useDataFlow'
import { useIsDarkTheme } from '../../../hooks/useIsDarkTheme'
import type { ChangeOverlayView } from '../review-wire-types'
import { DataFlowDiagram } from './DataFlowDiagram'
import { DataFlowStepDetail } from './DataFlowStepDetail'
import { DataFlowStepList } from './DataFlowStepList'
import { DataFlowToolbar } from './DataFlowToolbar'
import { buildSequenceDiagram } from './data-flow-mermaid'
import type { SequenceBuildResult } from './data-flow-mermaid'
import { buildStepRows, changedMessageSet } from './data-flow-overlay'
import type { DataFlowRow } from './data-flow-overlay'
import { copyMermaidSource, dataFlowExportFilename, downloadMermaidSource, downloadSvgFromContainer } from './data-flow-export'

const COPIED_RESET_MS = 2000

export function DataFlowDetailPane({
  worktreeId,
  environmentId,
  flowId,
  fallbackLabel,
  overlay,
  onSelectSymbol
}: {
  worktreeId: string
  environmentId: string | null
  flowId: string
  fallbackLabel?: string
  overlay: ChangeOverlayView
  onSelectSymbol: (key: string | null) => void
}): React.JSX.Element {
  const [detail, setDetail] = useState<DataFlowDetail>('service')
  const [zoom, setZoom] = useState(100)
  const [copied, setCopied] = useState(false)
  const [openStep, setOpenStep] = useState<Extract<DataFlowRow, { type: 'step' }> | null>(null)
  const diagramRef = useRef<HTMLDivElement | null>(null)
  const isDark = useIsDarkTheme()
  const load = useDataFlow(worktreeId, environmentId, flowId, detail)
  const data = load.data

  const diagram: SequenceBuildResult | null = useMemo(() => {
    if (!data) {
      return null
    }
    if (!data.sequence) {
      return { ok: false, reason: 'empty' }
    }
    return buildSequenceDiagram(data.sequence, {
      changedMessages: changedMessageSet(data.flow, overlay),
      changedPrefix: `[${translate('auto.components.reviewMap.dataflow.changedTag', 'changed')}] `
    })
  }, [data, overlay])

  const rows = useMemo(
    () => (data ? buildStepRows(data.flow, overlay, diagram?.ok ? diagram.renderedMessages : 0) : []),
    [data, overlay, diagram]
  )

  if (load.status === 'error') {
    const notFound = load.error?.kind === 'not-found'
    return (
      <div role="alert" className="space-y-2 p-3 text-sm">
        <p>
          {notFound
            ? translate('auto.components.reviewMap.dataflow.notFound', 'No data flow matches «{{label}}» yet.', { label: fallbackLabel ?? flowId })
            : translate('auto.components.reviewMap.dataflow.detailError', 'Could not load this flow.')}
        </p>
        {!notFound ? (
          <Button size="sm" variant="outline" onClick={load.refetch}>{translate('auto.components.reviewMap.dataflow.retry', 'Retry')}</Button>
        ) : null}
      </div>
    )
  }
  if (!data || !diagram) {
    return (
      <div role="status" aria-label={translate('auto.components.reviewMap.dataflow.loading', 'Loading flows...')} className="space-y-2 p-3">
        <Skeleton className="h-6 w-1/2" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }

  const { flow } = data
  const source = diagram.ok ? diagram.source : null
  const name = (ext: 'mmd' | 'svg'): string => dataFlowExportFilename(flow.label, ext, new Date())

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-3 overflow-auto p-3">
      <DataFlowToolbar
        label={flow.label}
        trigger={`${flow.trigger.kind} · ${flow.trigger.name}`}
        detail={detail}
        onDetailChange={setDetail}
        zoom={zoom}
        onZoomChange={setZoom}
        canExport={source !== null}
        copied={copied}
        onCopy={async () => {
          if (source && (await copyMermaidSource(source))) {
            setCopied(true)
            setTimeout(() => setCopied(false), COPIED_RESET_MS)
          }
        }}
        onExportMmd={() => source && downloadMermaidSource(source, name('mmd'))}
        onExportSvg={() => downloadSvgFromContainer(diagramRef.current, name('svg'))}
      />
      {flow.completeness !== 'complete' ? (
        <div role="status" className="flex items-start gap-2 rounded-md border p-2 text-xs">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          <span>{translate('auto.components.reviewMap.dataflow.partialBanner', 'This flow is incomplete: some steps could not be resolved. Gaps are marked in the step list.')}</span>
        </div>
      ) : null}
      <DataFlowDiagram
        ref={diagramRef}
        result={diagram}
        isDark={isDark}
        zoom={zoom}
        label={translate('auto.components.reviewMap.dataflow.diagramLabel', 'Sequence diagram of {{label}}; see the step list for the text equivalent', { label: flow.label })}
      />
      <DataFlowStepList
        rows={rows}
        onActivate={(row) => {
          if (row.step.symbol) {
            setOpenStep(null)
            onSelectSymbol(row.step.symbol.key)
          } else {
            setOpenStep(row)
          }
        }}
      />
      {openStep ? <DataFlowStepDetail step={openStep.step} stores={openStep.stores} /> : null}
    </div>
  )
}
