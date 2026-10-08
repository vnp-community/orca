/**
 * SolutionStatusBanner — CR-REQ-020-02
 *
 * Icon + text + token colour (never colour alone) for the current Solution.
 *
 * @module components/request/solution/SolutionStatusBanner
 */

import React from 'react'
import { CheckCircle2, Clock, Info, Loader2, Lock, XCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import type { Approval, Solution } from '../../../../../shared/request-types'
import type { SolutionPresentation } from './solution-view-model'

const BANNER_PREFIX = 'auto.components.request.SolutionPanel.banner.'

const BANNER_FALLBACKS: Record<string, string> = {
  readOnly: 'Analysis is read-only',
  generating: 'AI is drafting this analysis',
  awaitingApproval: 'Awaiting approval',
  ready: 'Ready for review',
  chosen: 'Approved',
  rejected: 'Rejected',
  superseded: 'Older version',
  unknown: 'Status unavailable'
}

/** Resolve the banner key; a read-only view still shows the Solution's own end state. */
export function resolveBannerKey(solution: Solution, presentation: SolutionPresentation): string {
  if (presentation.readOnly && ['chosen', 'rejected', 'superseded'].includes(solution.status)) {
    return BANNER_PREFIX + solution.status
  }
  return presentation.bannerKey
}

const TONE_CLASSES: Record<string, string> = {
  success: 'border-[color:var(--status-success-border)] bg-[color:var(--status-success-background)] text-[color:var(--status-success)]',
  info: 'border-primary/30 bg-primary/5 text-primary',
  warning: 'border-border bg-muted text-foreground',
  neutral: 'border-border bg-muted/50 text-muted-foreground',
  destructive: 'border-destructive/30 bg-destructive/5 text-destructive'
}

export function SolutionStatusBanner({
  solution,
  presentation,
  approval,
  conflictNotice = false
}: {
  solution: Solution
  presentation: SolutionPresentation
  approval?: Approval | null
  conflictNotice?: boolean
}): React.JSX.Element {
  const key = resolveBannerKey(solution, presentation)
  const suffix = key.startsWith(BANNER_PREFIX) ? key.slice(BANNER_PREFIX.length) : 'unknown'
  const rejected = solution.status === 'rejected'
  const tone = rejected ? 'destructive' : presentation.tone
  const Icon = rejected
    ? XCircle
    : presentation.tone === 'success'
      ? CheckCircle2
      : solution.status === 'generating'
        ? Loader2
        : presentation.readOnly
          ? Lock
          : presentation.tone === 'warning'
            ? Clock
            : Info

  return (
    <div
      role="status"
      data-testid="solution-status-banner"
      data-status={solution.status}
      className={cn('flex flex-col gap-1 rounded-md border px-3 py-2 text-sm', TONE_CLASSES[tone])}
    >
      <div className="flex items-center gap-2 font-medium">
        <Icon className={cn('size-4 shrink-0', solution.status === 'generating' && 'animate-spin')} aria-hidden />
        <span>{translate(key, BANNER_FALLBACKS[suffix] ?? BANNER_FALLBACKS.unknown)}</span>
      </div>
      {approval?.status === 'pending' && approval.expiresAt && (
        <p className="text-xs">
          {translate('auto.components.request.SolutionStatusBanner.dueAt', 'Due {{date}}', {
            date: new Date(approval.expiresAt).toLocaleString()
          })}
        </p>
      )}
      {rejected && solution.rejectionReason && (
        <p className="whitespace-pre-wrap text-xs">{solution.rejectionReason}</p>
      )}
      {rejected && (solution.reviewedById || solution.reviewedAt) && (
        <p className="text-xs">
          {[solution.reviewedById, solution.reviewedAt && new Date(solution.reviewedAt).toLocaleString()]
            .filter(Boolean)
            .join(' · ')}
        </p>
      )}
      {conflictNotice && (
        <p role="alert" className="text-xs">
          {translate('auto.components.request.SolutionStatusBanner.updated', 'This solution was just updated.')}
        </p>
      )}
    </div>
  )
}
