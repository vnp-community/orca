/**
 * QualityRunNotice.tsx — FE-CV-TASK-087-06
 *
 * Persistent inline explanation of why a run did not start or did not complete. Never a toast:
 * the reason stays until the user acts. An interrupted or failed run is never a result.
 *
 * @module components/review-map/quality/QualityRunNotice
 */

import { TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type {
  ActiveQualityRun,
  QualityRunError
} from '../../../store/slices/code-intel-quality-state'
import { scorecardCopy } from './quality-scorecard-copy'
import type { QualityScorecardCopyKey } from './quality-scorecard-copy'

function NoticeFrame({
  children,
  actions
}: {
  children: React.ReactNode
  actions?: React.ReactNode
}): React.JSX.Element {
  return (
    <div
      role="alert"
      className="flex items-start gap-2 rounded-md border border-border bg-muted/40 px-2 py-1.5 text-xs text-foreground"
    >
      <TriangleAlert className="mt-0.5 size-3.5 shrink-0 text-quality-warning" aria-hidden />
      <div className="min-w-0 flex-1 space-y-1">{children}</div>
      <div className="flex shrink-0 items-center gap-1">{actions}</div>
    </div>
  )
}

export function QualityRunErrorNotice({
  error,
  onRecheck,
  onDismiss
}: {
  error: QualityRunError
  onRecheck: () => void
  onDismiss: () => void
}): React.JSX.Element {
  const dismiss = (
    <Button type="button" variant="ghost" size="xs" onClick={onDismiss}>
      {scorecardCopy('run.dismiss')}
    </Button>
  )
  switch (error.kind) {
    case 'env-not-ready':
      return (
        <NoticeFrame
          actions={
            <>
              <Button type="button" variant="outline" size="xs" onClick={onRecheck}>
                {scorecardCopy('error.recheck')}
              </Button>
              {dismiss}
            </>
          }
        >
          <p className="font-medium">{scorecardCopy('error.envNotReady')}</p>
          <MissingList missing={error.missing ?? []} />
        </NoticeFrame>
      )
    case 'profile-unknown':
      return (
        <NoticeFrame actions={dismiss}>
          <p>{scorecardCopy('error.profileUnknown')}</p>
          {error.available && error.available.length > 0 ? (
            <p className="text-muted-foreground">
              {scorecardCopy('error.profileAvailable', { names: error.available.join(', ') })}
            </p>
          ) : null}
        </NoticeFrame>
      )
    case 'rate-limited':
      return (
        <NoticeFrame actions={dismiss}>
          <p>{scorecardCopy('error.rateLimited')}</p>
          {error.retryAfterSeconds ? (
            <p className="text-muted-foreground">
              {scorecardCopy('error.rateLimitedIn', { seconds: error.retryAfterSeconds })}
            </p>
          ) : null}
        </NoticeFrame>
      )
    case 'forbidden':
      return <NoticeFrame actions={dismiss}>{scorecardCopy('error.forbidden')}</NoticeFrame>
    case 'offline':
      return <NoticeFrame actions={dismiss}>{scorecardCopy('error.offline')}</NoticeFrame>
    default:
      return (
        <NoticeFrame actions={dismiss}>
          <p>{scorecardCopy('error.generic')}</p>
          {error.message ? (
            <p className="break-words text-muted-foreground">{error.message}</p>
          ) : null}
        </NoticeFrame>
      )
  }
}

export function MissingList({
  missing
}: {
  missing: readonly { check: string; reason: string; hint?: string }[]
}): React.JSX.Element | null {
  if (missing.length === 0) {
    return null
  }
  return (
    <ul className="space-y-0.5 text-muted-foreground" data-testid="quality-missing">
      {missing.map((m, i) => (
        <li key={`${m.check}:${i}`} className="break-words">
          {scorecardCopy('run.missing', { check: m.check, reason: m.reason || '—' })}
          {m.hint ? <span> {scorecardCopy('run.missingHint', { hint: m.hint })}</span> : null}
        </li>
      ))}
    </ul>
  )
}

/** Shown for a finished run that did not produce a result (failed, interrupted, cancelled). */
export function QualityRunFinishedNotice({
  run,
  onDismiss
}: {
  run: ActiveQualityRun
  onDismiss: () => void
}): React.JSX.Element | null {
  const status = run.status ?? 'unknown'
  if (status === 'succeeded') {
    return null
  }
  const key = (['failed', 'interrupted', 'cancelled'].includes(status) ? status : 'unknown') as
    | 'failed'
    | 'interrupted'
    | 'cancelled'
    | 'unknown'
  const details = [
    run.runId ? scorecardCopy('run.details', { runId: run.runId, status }) : status,
    run.message
  ]
    .filter(Boolean)
    .join('\n')
  return (
    <NoticeFrame
      actions={
        <>
          {status !== 'cancelled' ? (
            <Button
              type="button"
              variant="ghost"
              size="xs"
              onClick={() => void navigator.clipboard?.writeText(details)}
            >
              {scorecardCopy('run.copyDetails')}
            </Button>
          ) : null}
          <Button type="button" variant="ghost" size="xs" onClick={onDismiss}>
            {scorecardCopy('run.dismiss')}
          </Button>
        </>
      }
    >
      <p>{scorecardCopy(`run.finished.${key}` as QualityScorecardCopyKey)}</p>
      {run.message ? <p className="break-words text-muted-foreground">{run.message}</p> : null}
    </NoticeFrame>
  )
}
