/**
 * DataFlowDiagram.tsx — FE-CV-TASK-056-06
 *
 * Wraps the shared MermaidBlock (unmodified: strict security level + DOMPurify live there).
 * The step list is the full-fidelity equivalent; this is a visual aid with hard size limits.
 */

import { forwardRef } from 'react'
import { translate } from '@/i18n/i18n'
import MermaidBlock from '../../editor/MermaidBlock'
import type { SequenceBuildResult } from './data-flow-mermaid'

export const DATA_FLOW_ZOOM_MIN = 50
export const DATA_FLOW_ZOOM_MAX = 200

export const DataFlowDiagram = forwardRef<
  HTMLDivElement,
  { result: SequenceBuildResult; isDark: boolean; zoom: number; label: string; loading?: boolean }
>(function DataFlowDiagram({ result, isDark, zoom, label, loading }, ref) {
  if (loading) {
    return <div role="status" className="h-40 animate-pulse rounded-md bg-muted" aria-label={translate('auto.components.reviewMap.dataflow.diagramLoading', 'Loading diagram')} />
  }
  if (!result.ok) {
    const reason = {
      empty: translate('auto.components.reviewMap.dataflow.diagram.empty', 'This flow has no messages to draw.'),
      'too-many-participants': translate('auto.components.reviewMap.dataflow.diagram.tooManyParticipants', 'Too many participants to draw; use the step list below.'),
      'too-long': translate('auto.components.reviewMap.dataflow.diagram.tooLong', 'The diagram is too large to draw; use the step list below.')
    }[result.reason]
    return <p role="status" className="rounded-md border border-dashed p-3 text-xs text-muted-foreground">{reason}</p>
  }
  return (
    <div className="space-y-1">
      {result.renderedMessages < result.totalMessages ? (
        <p role="status" className="text-xs text-muted-foreground">
          {translate('auto.components.reviewMap.dataflow.diagram.truncated', 'Showing {{rendered}}/{{total}} messages; the step list has all of them.', {
            rendered: result.renderedMessages,
            total: result.totalMessages
          })}
        </p>
      ) : null}
      <div className="overflow-auto rounded-md border p-2">
        <div
          ref={ref}
          role="img"
          aria-label={label}
          style={{ transform: `scale(${zoom / 100})`, transformOrigin: 'top left', width: 'max-content' }}
        >
          <MermaidBlock content={result.source} isDark={isDark} htmlLabels={false} />
        </div>
      </div>
    </div>
  )
})
