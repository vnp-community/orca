/**
 * FindingRow.tsx — FE-CV-TASK-059-05
 *
 * One structural finding: kind icon, title, severity (icon + text), location, origin, owner and
 * the row actions. Errors from a failed write stay on the row until dismissed.
 *
 * @module components/review-map/findings/FindingRow
 */

import { AlertCircle, AlertTriangle, GitBranch, Info, Loader2, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { CodeIntelQueryError } from '../../../hooks/useCodeIntelQuery'
import { FindingDismissPopover, presetLabel } from './FindingDismissPopover'
import { tf } from './findings-i18n'
import type { FindingOriginState, FindingRowModel, FindingSeverityState } from './finding-view-model'

export function severityLabel(severity: FindingSeverityState): string {
  switch (severity) {
    case 'error':
      return tf('severity.error', 'Error')
    case 'warning':
      return tf('severity.warning', 'Warning')
    case 'info':
      return tf('severity.info', 'Info')
    default:
      return tf('severity.unknown', 'Unclassified')
  }
}

export function originLabel(origin: FindingOriginState): string {
  switch (origin) {
    case 'introduced':
      return tf('origin.introduced', 'Introduced by this change')
    case 'touched':
      return tf('origin.touched', 'In a changed file')
    case 'preexisting':
      return tf('origin.preexisting', 'Pre-existing')
    default:
      return tf('origin.unknown', 'Origin unknown')
  }
}

function SeverityIcon({ severity }: { severity: FindingSeverityState }): React.JSX.Element {
  if (severity === 'error') {
    return <AlertCircle className="size-3.5 text-destructive" aria-hidden />
  }
  if (severity === 'warning') {
    return <AlertTriangle className="size-3.5" aria-hidden />
  }
  return <Info className="size-3.5 text-muted-foreground" aria-hidden />
}

export function describeFindingError(error: CodeIntelQueryError): string {
  switch (error.kind) {
    case 'forbidden':
      return tf('error.forbidden', 'You do not have permission to dismiss findings.')
    case 'offline':
      return tf('error.offline', 'Offline. The finding was left unchanged.')
    case 'not-found':
      return tf('error.notFound', 'This finding no longer exists. The list was reloaded.')
    case 'conflict':
      return tf('error.conflict', 'This finding was changed by someone else. The list was reloaded.')
    case 'validation':
      return tf('error.reasonRequired', 'Choose a reason first.')
    default:
      return tf('error.generic', 'Could not update the finding.')
  }
}

export type FindingRowProps = {
  row: FindingRowModel
  busy: boolean
  error: CodeIntelQueryError | null
  active?: boolean
  /** Null hides the "View in graph" action. */
  onViewInGraph: (() => void) | null
  onOpenLocation: (() => void) | null
  openLocationLabel: string
  /** The Note action (ReviewNoteButton); omitted when the finding has no file to attach to. */
  noteSlot?: React.ReactNode
  onDismiss: (input: { reason: string; note?: string }) => void
  onResolve: () => void
  onRestore: () => void
  onRetry: () => void
}

export function FindingRow(props: FindingRowProps): React.JSX.Element {
  const { row, busy, error } = props
  const label = `${severityLabel(row.severity)}: ${row.title}`
  return (
    <div
      role="listitem"
      aria-label={label}
      data-finding-key={row.findingKey}
      className={cn('space-y-1 border-b px-2 py-1.5 text-xs', props.active && 'bg-accent', row.isDismissed && 'opacity-70')}
    >
      <div className="flex items-start gap-2">
        <SeverityIcon severity={row.severity} />
        <div className="min-w-0 flex-1">
          <div className="font-medium">{row.title}</div>
          <div className="flex flex-wrap items-center gap-x-2 text-[11px] text-muted-foreground">
            <span>{severityLabel(row.severity)}</span>
            {row.locationLabel ? <span className="truncate" title={row.locationLabel}>{row.locationLabel}</span> : null}
            <span>{originLabel(row.origin)}</span>
            {row.ownerLabel ? <span>{row.ownerLabel}</span> : null}
            <span>{tf('confidence', 'Confidence: {{level}}', { level: row.confidence })}</span>
          </div>
          {row.isDismissed ? (
            <div className="text-[11px] text-muted-foreground">
              {row.disposition === 'resolved'
                ? tf('state.resolved', 'Marked as resolved')
                : tf('state.ignored', 'Ignored: {{reason}}', { reason: presetLabel(row.dismissReason ?? '') })}
              {row.dismissNote ? ` · ${row.dismissNote}` : ''}
            </div>
          ) : null}
        </div>
        {busy ? <Loader2 className="size-3.5 animate-spin" aria-label={tf('saving', 'Saving')} /> : null}
      </div>
      <div className="flex flex-wrap gap-1 pl-5">
        {props.onViewInGraph ? (
          <Button type="button" variant="ghost" size="xs" onClick={props.onViewInGraph}>
            <GitBranch aria-hidden />
            {tf('action.viewInGraph', 'View in graph')}
          </Button>
        ) : null}
        {props.onOpenLocation ? (
          <Button type="button" variant="ghost" size="xs" onClick={props.onOpenLocation}>
            {props.openLocationLabel}
          </Button>
        ) : null}
        {props.noteSlot ?? null}
        {row.isDismissed ? (
          <Button type="button" variant="ghost" size="xs" disabled={busy} onClick={props.onRestore}>
            <RotateCcw aria-hidden />
            {tf('action.restore', 'Reopen')}
          </Button>
        ) : (
          <>
            <FindingDismissPopover disabled={busy} onConfirm={props.onDismiss}>
              <Button type="button" variant="ghost" size="xs" disabled={busy} data-action="ignore">
                {tf('action.ignore', 'Ignore')}
              </Button>
            </FindingDismissPopover>
            <Button type="button" variant="ghost" size="xs" disabled={busy} onClick={props.onResolve}>
              {tf('action.resolve', 'Mark resolved')}
            </Button>
          </>
        )}
      </div>
      {error ? (
        <div role="alert" className="flex items-center gap-2 pl-5 text-[11px] text-destructive">
          <span>{describeFindingError(error)}</span>
          {error.retryable !== false && error.kind !== 'forbidden' ? (
            <Button type="button" variant="outline" size="xs" onClick={props.onRetry}>
              {tf('retry', 'Retry')}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
