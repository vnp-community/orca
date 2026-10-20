/**
 * OutcomeReviewStates — FE-OR-T123
 *
 * Everything the tab shows besides the review itself: the banners above it, the invitation when there is no review yet, and the
 * loading, unavailable and error states. A failure is explained in one sentence for its code; the server's own text is never
 * printed.
 *
 * @module components/request/outcome-review/OutcomeReviewStates
 */

import React from 'react'
import { Info, Loader2, TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  bannerText,
  errorText,
  estimateLines,
  factsOnlyText,
  generatingText,
  inviteActionText,
  inviteBodyText,
  inviteTitleText,
  loadingText,
  reasonText,
  unavailableText
} from './outcome-review-state-copy'
import { failedContextRun, type ReviewBannerId } from './outcome-review-view-model'
import type {
  OutcomeReview,
  OutcomeReviewRun,
  ReviewEstimate
} from '../../../../../shared/outcome-review-types'

const WARN: ReadonlySet<ReviewBannerId> = new Set([
  'stale',
  'run_failed',
  'drawing_failed',
  'kg_unavailable',
  'design_not_reviewed',
  'truncated'
])

type BannersProps = {
  ids: ReviewBannerId[]
  review: OutcomeReview
  runs: OutcomeReviewRun[]
  activeRun: OutcomeReviewRun | null
  /** Starts the after-code review (Execution finished) or builds the review again (stale). */
  onGenerate?: () => void
  onRegenerate?: () => void
  busy?: boolean
}

export function ReviewBanners({
  ids,
  review,
  runs,
  activeRun,
  onGenerate,
  onRegenerate,
  busy = false
}: BannersProps): React.JSX.Element | null {
  if (ids.length === 0) {
    return null
  }
  const percent = Math.round(review.model.verdict.architecture.reviewedRatio * 100)
  const reason = reasonText(failedContextRun(runs, activeRun)?.errorCode)
  return (
    <div className="flex flex-col gap-2" data-testid="outcome-review-banners">
      {ids.map((id) => {
        const Icon = WARN.has(id) ? TriangleAlert : Info
        return (
          <div
            key={id}
            role="status"
            className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm"
            data-testid={`outcome-banner-${id}`}
          >
            <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            <span className="min-w-0 flex-1">{bannerText(id, { percent, reason })}</span>
            {id === 'execution_finished' && onGenerate && (
              <Button
                size="xs"
                disabled={busy}
                onClick={onGenerate}
                data-testid="outcome-banner-generate"
              >
                {inviteActionText('delivered')}
              </Button>
            )}
            {id === 'stale' && onRegenerate && (
              <Button
                size="xs"
                variant="outline"
                disabled={busy}
                onClick={onRegenerate}
                data-testid="outcome-banner-rebuild"
              >
                {translate(
                  'auto.components.request.outcomeReview.banner.rebuild',
                  'Rebuild the review'
                )}
              </Button>
            )}
          </div>
        )
      })}
    </div>
  )
}

type InviteProps = {
  kind: 'design' | 'delivered'
  estimate: ReviewEstimate | null
  generating: boolean
  onGenerate: (runContext: boolean) => void
}

export function ReviewInvite({
  kind,
  estimate,
  generating,
  onGenerate
}: InviteProps): React.JSX.Element {
  const lines = estimate
    ? estimateLines({
        contextCached: estimate.contextCached,
        contextRunnable: estimate.contextRunnable,
        designLocked: estimate.designLocked,
        gaps: estimate.contextGaps
      })
    : []
  return (
    <section
      className="flex flex-col gap-3 rounded-md border border-border p-4"
      data-testid="outcome-review-invite"
      data-kind={kind}
    >
      <h3 className="text-sm font-semibold">{inviteTitleText(kind)}</h3>
      <p className="text-sm text-muted-foreground">{inviteBodyText(kind)}</p>
      {lines.length > 0 && (
        <ul
          className="list-disc pl-5 text-xs text-muted-foreground"
          data-testid="outcome-review-estimate"
        >
          {lines.map((line) => (
            <li key={line}>{line}</li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap gap-2">
        <Button
          size="sm"
          disabled={generating}
          onClick={() => onGenerate(true)}
          data-testid="outcome-review-generate"
        >
          {inviteActionText(kind)}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={generating}
          onClick={() => onGenerate(false)}
          data-testid="outcome-review-facts-only"
        >
          {factsOnlyText()}
        </Button>
      </div>
    </section>
  )
}

export function ReviewLoading(): React.JSX.Element {
  return (
    <div className="flex flex-col gap-2 p-1" aria-busy="true" data-testid="outcome-review-loading">
      <span className="sr-only">{loadingText()}</span>
      <Skeleton className="h-6 w-1/2" />
      <Skeleton className="h-24 w-full" />
    </div>
  )
}

export function ReviewUnavailable(): React.JSX.Element {
  return (
    <p
      className="rounded-md border border-dashed border-border p-3 text-sm text-muted-foreground"
      data-testid="outcome-review-unavailable"
    >
      {unavailableText()}
    </p>
  )
}

export function ReviewError({
  code,
  onRetry
}: {
  code: string
  onRetry: () => void
}): React.JSX.Element {
  return (
    <div
      role="alert"
      className="flex items-center gap-3 rounded-md border border-destructive/40 p-3 text-sm"
      data-testid="outcome-review-error"
      data-code={code}
    >
      <TriangleAlert className="size-4 shrink-0 text-destructive" aria-hidden />
      <span className="min-w-0 flex-1">{errorText(code)}</span>
      <Button size="xs" variant="outline" onClick={onRetry}>
        {translate('auto.components.request.error.retry', 'Retry')}
      </Button>
    </div>
  )
}

/** The progress line a screen reader hears; the region stays mounted while empty so what appears in it is announced. */
export function GeneratingLine({
  phase,
  active
}: {
  phase: 'design' | 'delivered'
  active: boolean
}): React.JSX.Element {
  return (
    <div
      aria-live="polite"
      className={active ? 'flex items-center gap-2 text-sm text-muted-foreground' : 'sr-only'}
      data-testid="outcome-review-progress"
    >
      {active && (
        <>
          <Loader2 className="size-4 animate-spin" aria-hidden />
          <span>{generatingText(phase)}</span>
        </>
      )}
    </div>
  )
}

/** Stands in for a block that waits for the agent. */
export function PendingBlock({ label }: { label: string }): React.JSX.Element {
  return (
    <div
      className="flex flex-col gap-2 rounded-md border border-border p-3"
      aria-busy="true"
      data-testid="outcome-review-pending-block"
    >
      <span className="sr-only">{label}</span>
      <Skeleton className="h-4 w-1/3" />
      <Skeleton className="h-16 w-full" />
    </div>
  )
}
