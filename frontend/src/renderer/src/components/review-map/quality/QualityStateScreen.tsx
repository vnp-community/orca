/**
 * QualityStateScreen.tsx — FE-CV-TASK-087-07
 *
 * Inline, persistent explanation for the lens states that have no scorecard (loading, never
 * run, load failures, a clean-looking run). Run progress and run errors live in the toolbar.
 * Wording never turns missing data into a pass.
 *
 * @module components/review-map/quality/QualityStateScreen
 */

import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { QualityViewKind } from './quality-view-state'
import { scorecardCopy } from './quality-scorecard-copy'

const SCREEN_KINDS = new Set<QualityViewKind>([
  'loading',
  'not-run',
  'load-error',
  'forbidden',
  'offline',
  'ready-empty'
])

export function isQualityStateScreenKind(kind: QualityViewKind): boolean {
  return SCREEN_KINDS.has(kind)
}

export function QualityStateScreen({
  kind,
  profile,
  onRetry
}: {
  kind: QualityViewKind
  profile: string
  onRetry?: () => void
}): React.JSX.Element | null {
  if (!SCREEN_KINDS.has(kind)) {
    return null
  }
  const retry = onRetry ? (
    <Button type="button" variant="outline" size="xs" onClick={onRetry}>
      {scorecardCopy('state.retry')}
    </Button>
  ) : null
  let body: React.ReactNode
  switch (kind) {
    case 'loading':
      body = (
        <p className="flex items-center gap-2">
          <Loader2 className="size-3.5 animate-spin motion-reduce:animate-none" aria-hidden />
          {scorecardCopy('state.loading')}
        </p>
      )
      break
    case 'not-run':
      body = (
        <>
          <p className="font-medium text-foreground">{scorecardCopy('state.notRun.title')}</p>
          <p>{scorecardCopy('state.notRun.body')}</p>
        </>
      )
      break
    case 'ready-empty':
      body = <p className="text-foreground">{scorecardCopy('state.empty', { profile })}</p>
      break
    case 'forbidden':
      body = <p>{scorecardCopy('state.forbidden')}</p>
      break
    case 'offline':
      body = (
        <>
          <p>{scorecardCopy('state.offline')}</p>
          {retry}
        </>
      )
      break
    default:
      body = (
        <>
          <p>{scorecardCopy('state.error')}</p>
          {retry}
        </>
      )
  }
  return (
    <div
      role="status"
      className="flex flex-col items-start gap-2 rounded-md border border-dashed border-border p-3 text-xs text-muted-foreground"
      data-testid="quality-state"
      data-kind={kind}
    >
      {body}
    </div>
  )
}

export function QualityNotices({
  notices
}: {
  notices: readonly QualityViewKind[]
}): React.JSX.Element | null {
  const lines = notices.flatMap((n) =>
    n === 'index-stale'
      ? [scorecardCopy('state.indexStale')]
      : n === 'offline'
        ? [scorecardCopy('state.offline')]
        : []
  )
  if (lines.length === 0) {
    return null
  }
  return (
    <ul
      role="status"
      className="space-y-0.5 text-xs text-muted-foreground"
      data-testid="quality-notices"
    >
      {lines.map((l) => (
        <li key={l}>{l}</li>
      ))}
    </ul>
  )
}
