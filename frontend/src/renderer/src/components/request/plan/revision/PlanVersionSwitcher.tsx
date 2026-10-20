/**
 * PlanVersionSwitcher — CR-REQ-045
 *
 * History of the plan's versions (newest first): number, what was asked for, when, a badge that always has text, how many
 * files it changed and what the run cost. Hidden for a single version. The list scrolls inside a bounded box and older
 * versions are read page by page from the server, so a hundred versions stay usable by keyboard without nested page scrolling.
 *
 * @module components/request/plan/revision/PlanVersionSwitcher
 */

import React from 'react'
import { CircleDot, History } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate, i18n } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  formatAbsoluteTime,
  formatRequestRelativeTime
} from '../../../../lib/request-relative-time'
import type {
  PlanRevision,
  PlanRevisionStatus
} from '../../../../../../shared/request-plan-revision'
import {
  filesChangedLabel,
  planVersionBadge,
  planVersionNote,
  versionStatsLabel
} from './plan-version-history'

const BADGE_STYLE: Record<PlanRevisionStatus, string> = {
  current: 'border-primary/30 bg-primary/10 text-primary',
  superseded: 'border-border bg-muted text-muted-foreground'
}

function VersionBadge({ status }: { status: PlanRevisionStatus }): React.JSX.Element {
  const Icon = status === 'current' ? CircleDot : History
  return (
    <Badge variant="outline" data-badge={status} className={BADGE_STYLE[status]}>
      <Icon aria-hidden />
      {status === 'current'
        ? translate('auto.components.request.plan.revision.badgeCurrent', 'Current')
        : translate('auto.components.request.plan.revision.badgeSuperseded', 'Superseded')}
    </Badge>
  )
}

export type PlanVersionSwitcherProps = {
  /** Newest first. */
  revisions: readonly PlanRevision[]
  /** The version on screen. */
  shownRevision: number
  hasMore: boolean
  loadingMore: boolean
  onLoadMore: () => void
  onSelect: (revision: number) => void
}

export function PlanVersionSwitcher({
  revisions,
  shownRevision,
  hasMore,
  loadingMore,
  onLoadMore,
  onSelect
}: PlanVersionSwitcherProps): React.JSX.Element | null {
  if (revisions.length < 2) {
    return null
  }
  return (
    <section
      data-testid="plan-version-list"
      aria-label={translate('auto.components.request.plan.revision.versions', 'Versions')}
      className="flex flex-col gap-1"
    >
      <ul className="flex max-h-60 flex-col gap-1 overflow-y-auto scrollbar-sleek">
        {revisions.map((r) => {
          const selected = r.revision === shownRevision
          const files = filesChangedLabel(r)
          const stats = versionStatsLabel(r.stats)
          return (
            <li key={r.revision}>
              <button
                type="button"
                aria-pressed={selected}
                data-revision={r.revision}
                onClick={() => onSelect(r.revision)}
                className={cn(
                  'flex w-full flex-col gap-0.5 rounded-md border px-2 py-1 text-left text-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
                  selected
                    ? 'border-primary bg-primary/10 text-foreground'
                    : 'border-border text-muted-foreground hover:bg-accent'
                )}
              >
                <span className="flex flex-wrap items-center gap-2">
                  <span className="font-medium text-foreground">
                    {translate('auto.components.request.plan.revision.versionN', 'Version {{n}}', {
                      n: r.revision
                    })}
                  </span>
                  <VersionBadge status={planVersionBadge(r)} />
                  {r.generatedAt !== '' && (
                    <time
                      dateTime={r.generatedAt}
                      title={formatAbsoluteTime(r.generatedAt, i18n.language || 'en')}
                    >
                      {formatRequestRelativeTime(r.generatedAt)}
                    </time>
                  )}
                  {files !== null && <span>{files}</span>}
                  {stats !== null && <span>{stats}</span>}
                </span>
                <span className="truncate" data-testid="plan-version-note">
                  {planVersionNote(r)}
                </span>
                {!r.textKept && (
                  <span className="italic" data-testid="plan-version-pruned">
                    {translate(
                      'auto.components.request.plan.revision.textPruned',
                      'The text of this version is no longer kept.'
                    )}
                  </span>
                )}
              </button>
            </li>
          )
        })}
      </ul>
      {hasMore && (
        <Button
          size="xs"
          variant="outline"
          className="self-start"
          disabled={loadingMore}
          onClick={onLoadMore}
          data-testid="plan-version-older"
        >
          {translate('auto.components.request.plan.revision.showOlder', 'Show older')}
        </Button>
      )}
    </section>
  )
}
