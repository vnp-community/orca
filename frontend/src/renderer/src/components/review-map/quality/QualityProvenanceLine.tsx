/**
 * QualityProvenanceLine.tsx — FE-CV-TASK-087-05
 *
 * Where the verdict came from (local or CI, when, HEAD, index) plus the banners that qualify it:
 * out-of-date result, uncommitted or changing working tree, widened scope, findings outside the
 * changed scope. Staleness keeps the data visible and says so.
 *
 * @module components/review-map/quality/QualityProvenanceLine
 */

import { TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { GateStaleness } from './quality-stale-model'
import { shortCommit } from './quality-scorecard-model'
import type { ScorecardRunSummary } from './quality-scorecard-model'
import { scorecardCopy } from './quality-scorecard-copy'

function formatRunTime(iso: string | null): string {
  const date = iso ? new Date(iso) : null
  return date && !Number.isNaN(date.getTime())
    ? date.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
    : scorecardCopy('provenance.unknownValue')
}

function sourceLabel(source: string): string {
  if (source === 'local') {
    return scorecardCopy('source.local')
  }
  return source === 'ci' ? scorecardCopy('source.ci') : scorecardCopy('source.unknown')
}

function Banner({
  children,
  action
}: {
  children: React.ReactNode
  action?: React.ReactNode
}): React.JSX.Element {
  return (
    <div
      role="status"
      className="flex items-start gap-2 rounded-md border border-quality-warning-border bg-quality-warning-background px-2 py-1.5 text-xs text-foreground"
    >
      <TriangleAlert className="mt-0.5 size-3.5 shrink-0 text-quality-warning" aria-hidden />
      <div className="min-w-0 flex-1">{children}</div>
      {action}
    </div>
  )
}

export function QualityProvenanceLine({
  summary,
  staleness,
  currentHead,
  indexBehindHead,
  onRerun,
  runLocked
}: {
  summary: ScorecardRunSummary
  staleness: GateStaleness
  currentHead: string | null | undefined
  indexBehindHead: boolean
  onRerun?: () => void
  runLocked?: boolean
}): React.JSX.Element {
  const unknown = scorecardCopy('provenance.unknownValue')
  return (
    <div className="flex flex-col gap-1.5" data-testid="quality-provenance">
      {summary.sources.map((s) => (
        <p key={s.runId} className="break-words text-xs text-muted-foreground">
          {scorecardCopy('provenance.line', {
            source: sourceLabel(s.source),
            time: formatRunTime(s.finishedAt),
            head: shortCommit(s.headCommit) ?? unknown,
            index: shortCommit(s.indexCommit) ?? unknown
          })}
        </p>
      ))}
      {indexBehindHead ? (
        <p className="text-xs text-muted-foreground">{scorecardCopy('provenance.indexBehind')}</p>
      ) : null}
      {staleness.stale ? (
        <Banner
          action={
            onRerun ? (
              <Button
                type="button"
                variant="outline"
                size="xs"
                disabled={runLocked}
                onClick={onRerun}
              >
                {scorecardCopy('stale.rerun')}
              </Button>
            ) : undefined
          }
        >
          <p className="font-medium">{scorecardCopy('stale.title')}</p>
          {staleness.reasons.includes('head-moved') ? (
            <p>
              {scorecardCopy('stale.headMoved')}
              {currentHead ? ` (${shortCommit(currentHead)})` : ''}
            </p>
          ) : null}
          {staleness.reasons.includes('backend') ? <p>{scorecardCopy('stale.backend')}</p> : null}
          {staleness.reasons.includes('cache') ? <p>{scorecardCopy('stale.cache')}</p> : null}
        </Banner>
      ) : null}
      {summary.dirty ? <Banner>{scorecardCopy('banner.dirty')}</Banner> : null}
      {summary.changedDuringRun ? (
        <Banner>{scorecardCopy('banner.changedDuringRun')}</Banner>
      ) : null}
      {summary.scopeWidened ? <Banner>{scorecardCopy('banner.scopeWidened')}</Banner> : null}
      {summary.truncated ? <Banner>{scorecardCopy('banner.truncated')}</Banner> : null}
      {summary.outsideScope > 0 ? (
        <p className="text-xs text-muted-foreground">
          {scorecardCopy('banner.outsideScope', { count: summary.outsideScope })}
        </p>
      ) : null}
    </div>
  )
}
