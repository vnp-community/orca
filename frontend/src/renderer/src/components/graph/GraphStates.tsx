/**
 * Graph empty / error / skeleton / status states — FE-REQ-TASK-032-06
 *
 * Copy never says "safe"; absence of nodes means "none affected", not "verified".
 *
 * @module components/graph/GraphStates
 */

import React from 'react'
import { AlertTriangle, Info } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import type { RequestRpcError } from '../../../../shared/request-errors'
import { splitErrorCode } from '../../../../shared/request-errors'
import type { GraphPayload } from '../../../../shared/graph-types'

const T = 'auto.components.graph.State.'

export function GraphSkeleton(): React.JSX.Element {
  return (
    <div className="flex h-full min-h-[240px] flex-col gap-2 p-3" role="status" aria-label={translate(`${T}loading`, 'Loading graph')}>
      <Skeleton className="h-6 w-1/3" />
      <Skeleton className="h-full min-h-[160px] w-full" />
    </div>
  )
}

export function GraphEmptyState({
  kind, onRunAssessment
}: {
  kind: 'noAssessment' | 'empty'
  onRunAssessment?: () => void
}): React.JSX.Element {
  return (
    <div className="flex h-full min-h-[200px] flex-col items-center justify-center gap-2 p-4 text-center text-sm text-muted-foreground">
      <Info className="size-5" aria-hidden />
      <p>
        {kind === 'noAssessment'
          ? translate(`${T}noAssessment`, 'No impact assessment yet')
          : translate(`${T}empty`, 'No affected items found for this view')}
      </p>
      {kind === 'noAssessment' && onRunAssessment ? (
        <Button size="sm" variant="outline" onClick={onRunAssessment}>
          {translate(`${T}runAssessment`, 'Run assessment')}
        </Button>
      ) : null}
    </div>
  )
}

export function graphErrorMessage(error: RequestRpcError): string {
  const code = splitErrorCode(error.message).code || error.code
  if (code === 'REQUEST_IMPACT_NO_CONNECTION') {return translate(`${T}noDevServer`, 'No dev server connected')}
  if (error.kind === 'forbidden') {return translate(`${T}forbidden`, 'You do not have permission to view the assessment')}
  return translate(`${T}error`, 'Could not load the graph')
}

export function isNoDevServerError(error: RequestRpcError): boolean {
  return (splitErrorCode(error.message).code || error.code) === 'REQUEST_IMPACT_NO_CONNECTION'
}

export function GraphErrorState({
  error, onRetry, hasStalePayload, onConnectDevServer
}: {
  error: RequestRpcError
  onRetry: () => void
  hasStalePayload?: boolean
  onConnectDevServer?: () => void
}): React.JSX.Element {
  const canRetry = error.kind !== 'forbidden'
  return (
    <div
      role="alert"
      className="flex items-center gap-2 border-b border-risk-medium-border bg-risk-medium-background px-3 py-2 text-xs"
      data-stale-payload={hasStalePayload ? 'true' : undefined}
    >
      <AlertTriangle className="size-3.5 shrink-0" aria-hidden />
      <span className="flex-1">{graphErrorMessage(error)}</span>
      {isNoDevServerError(error) && onConnectDevServer ? (
        <Button size="xs" variant="outline" onClick={onConnectDevServer}>
          {translate(`${T}connectDevServer`, 'Connect dev server')}
        </Button>
      ) : null}
      {canRetry ? (
        <Button size="xs" variant="outline" onClick={onRetry}>
          {translate(`${T}retry`, 'Retry')}
        </Button>
      ) : null}
    </div>
  )
}

export function GraphStatusBanner({ payload }: { payload: GraphPayload }): React.JSX.Element | null {
  if (!payload.stale && !payload.truncated) {return null}
  return (
    <div className="flex flex-col gap-1 border-b border-border bg-muted px-3 py-1.5 text-xs text-muted-foreground">
      {payload.stale ? (
        <span>{translate(`${T}stale`, 'The analysis index is out of date; results may be incomplete')}</span>
      ) : null}
      {payload.truncated ? (
        <span>
          {translate(`${T}truncated`, 'Showing {{shown}} of {{total}} nodes', {
            shown: payload.nodes.length,
            total: payload.totalNodes
          })}
        </span>
      ) : null}
    </div>
  )
}
