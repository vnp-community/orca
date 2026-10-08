/** DataFlowToolbar.tsx — FE-CV-TASK-056-06 */

import { Check, Copy, Download, Minus, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import type { DataFlowDetail } from '../../../hooks/useDataFlow'
import { DATA_FLOW_ZOOM_MAX, DATA_FLOW_ZOOM_MIN } from './DataFlowDiagram'

export const DATA_FLOW_ZOOM_STEP = 25

export function DataFlowToolbar({
  label,
  trigger,
  detail,
  onDetailChange,
  zoom,
  onZoomChange,
  canExport,
  copied,
  onCopy,
  onExportMmd,
  onExportSvg
}: {
  label: string
  trigger: string
  detail: DataFlowDetail
  onDetailChange: (d: DataFlowDetail) => void
  zoom: number
  onZoomChange: (z: number) => void
  canExport: boolean
  copied: boolean
  onCopy: () => void
  onExportMmd: () => void
  onExportSvg: () => void
}): React.JSX.Element {
  return (
    <div role="toolbar" aria-label={translate('auto.components.reviewMap.dataflow.toolbar', 'Flow options')} className="flex flex-wrap items-center gap-2">
      <div className="min-w-0 flex-1">
        <h3 className="truncate text-sm font-medium">{label}</h3>
        <p className="truncate text-[11px] text-muted-foreground">{trigger}</p>
      </div>
      <div className="flex items-center gap-1">
        {(['service', 'component'] as const).map((d) => (
          <Button key={d} size="xs" variant={detail === d ? 'secondary' : 'ghost'} aria-pressed={detail === d} onClick={() => onDetailChange(d)}>
            {d === 'service'
              ? translate('auto.components.reviewMap.dataflow.detail.service', 'Services')
              : translate('auto.components.reviewMap.dataflow.detail.component', 'Components')}
          </Button>
        ))}
      </div>
      <div className="flex items-center gap-1">
        <Button size="icon-xs" variant="ghost" aria-label={translate('auto.components.reviewMap.dataflow.zoomOut', 'Zoom out')} disabled={zoom <= DATA_FLOW_ZOOM_MIN} onClick={() => onZoomChange(Math.max(DATA_FLOW_ZOOM_MIN, zoom - DATA_FLOW_ZOOM_STEP))}>
          <Minus className="size-3.5" />
        </Button>
        <Button size="xs" variant="ghost" aria-label={translate('auto.components.reviewMap.dataflow.zoomReset', 'Reset zoom')} onClick={() => onZoomChange(100)}>
          {zoom}%
        </Button>
        <Button size="icon-xs" variant="ghost" aria-label={translate('auto.components.reviewMap.dataflow.zoomIn', 'Zoom in')} disabled={zoom >= DATA_FLOW_ZOOM_MAX} onClick={() => onZoomChange(Math.min(DATA_FLOW_ZOOM_MAX, zoom + DATA_FLOW_ZOOM_STEP))}>
          <Plus className="size-3.5" />
        </Button>
      </div>
      <Button size="xs" variant="outline" disabled={!canExport} onClick={onCopy}>
        {copied ? <Check className="size-3.5" aria-hidden="true" /> : <Copy className="size-3.5" aria-hidden="true" />}
        {copied ? translate('auto.components.reviewMap.dataflow.copied', 'Copied') : translate('auto.components.reviewMap.dataflow.copy', 'Copy Mermaid')}
      </Button>
      <Button size="xs" variant="outline" disabled={!canExport} onClick={onExportMmd}>
        <Download className="size-3.5" aria-hidden="true" />
        .mmd
      </Button>
      <Button size="xs" variant="outline" disabled={!canExport} onClick={onExportSvg}>
        <Download className="size-3.5" aria-hidden="true" />
        .svg
      </Button>
    </div>
  )
}
