/**
 * SolutionVersionSwitcher — CR-REQ-020-02, CR-REQ-041
 *
 * History of the analysis versions (newest first): number, what was asked for, when, and a status badge that always
 * has text. Hidden for a single version. A height-bounded list with pages keeps a hundred revisions usable by keyboard.
 *
 * @module components/request/solution/SolutionVersionSwitcher
 */

import React, { useEffect, useMemo, useRef, useState } from 'react'
import { CheckCircle2, CircleDot, History, XCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate, i18n } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatAbsoluteTime, formatRequestRelativeTime } from '../../../lib/request-relative-time'
import type { Solution } from '../../../../../shared/request-types'
import {
  VERSION_PAGE_SIZE,
  currentSolutionOf,
  solutionVersionNumbers,
  versionBadge,
  versionNote,
  versionPageCount,
  versionPageOf,
  type VersionBadge
} from './solution-version-history'

const BADGE_STYLE: Record<VersionBadge, string> = {
  current: 'border-primary/30 bg-primary/10 text-primary',
  superseded: 'border-border bg-muted text-muted-foreground',
  rejected: 'border-destructive/30 bg-destructive/5 text-destructive',
  approved:
    'border-[color:var(--status-success-border)] bg-[color:var(--status-success-background)] text-[color:var(--status-success)]'
}

function badgeText(badge: VersionBadge): string {
  switch (badge) {
    case 'current':
      return translate('auto.components.request.SolutionVersionSwitcher.badgeCurrent', 'Current')
    case 'superseded':
      return translate(
        'auto.components.request.SolutionVersionSwitcher.badgeSuperseded',
        'Superseded'
      )
    case 'rejected':
      return translate('auto.components.request.SolutionVersionSwitcher.badgeRejected', 'Rejected')
    case 'approved':
      return translate('auto.components.request.SolutionVersionSwitcher.badgeApproved', 'Approved')
  }
}

function VersionBadgeChip({ badge }: { badge: VersionBadge }): React.JSX.Element {
  const Icon =
    badge === 'approved'
      ? CheckCircle2
      : badge === 'rejected'
        ? XCircle
        : badge === 'current'
          ? CircleDot
          : History
  return (
    <Badge variant="outline" data-badge={badge} className={BADGE_STYLE[badge]}>
      <Icon aria-hidden />
      {badgeText(badge)}
    </Badge>
  )
}

export type SolutionVersionSwitcherProps = {
  /** Newest first. */
  solutions: Solution[]
  /** The version on screen. */
  currentId: string
  /** The version that can still be decided; defaults to the newest one not replaced. */
  latestId?: string
  onSelect: (id: string) => void
}

export function SolutionVersionSwitcher({
  solutions,
  currentId,
  latestId,
  onSelect
}: SolutionVersionSwitcherProps): React.JSX.Element | null {
  const [page, setPage] = useState(() => versionPageOf(solutions, currentId))
  // Why: choosing a version from outside the list (Back to current) must bring its page into view, but a refetch
  // that only adds a version must not yank a reader off the page they are browsing; hence the ref, not a dependency.
  const newestFirst = useRef(solutions)
  newestFirst.current = solutions
  useEffect(() => {
    setPage(versionPageOf(newestFirst.current, currentId))
  }, [currentId])
  const numbers = useMemo(() => solutionVersionNumbers(solutions), [solutions])
  if (solutions.length < 2) {
    return null
  }
  const liveId = latestId ?? currentSolutionOf(solutions)?.id
  const pages = versionPageCount(solutions.length)
  const safePage = Math.min(page, pages - 1)
  const first = safePage * VERSION_PAGE_SIZE
  const visible = solutions.slice(first, first + VERSION_PAGE_SIZE)

  return (
    <section
      data-testid="solution-version-list"
      aria-label={translate('auto.components.request.SolutionVersionSwitcher.label', 'Versions')}
      className="flex flex-col gap-1"
    >
      <ul className="flex max-h-60 flex-col gap-1 overflow-y-auto scrollbar-sleek">
        {visible.map((s) => {
          const selected = s.id === currentId
          const badge = versionBadge(s, s.id === liveId)
          const note = versionNote(s.revisionFeedback)
          return (
            <li key={s.id}>
              <button
                type="button"
                aria-pressed={selected}
                data-version-id={s.id}
                onClick={() => onSelect(s.id)}
                className={cn(
                  'flex w-full flex-col gap-0.5 rounded-md border px-2 py-1 text-left text-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
                  selected
                    ? 'border-primary bg-primary/10 text-foreground'
                    : 'border-border text-muted-foreground hover:bg-accent'
                )}
              >
                <span className="flex flex-wrap items-center gap-2">
                  <span className="font-medium text-foreground">
                    {translate(
                      'auto.components.request.SolutionVersionSwitcher.versionN',
                      'Version {{n}}',
                      {
                        n: numbers.get(s.id)
                      }
                    )}
                  </span>
                  {badge && <VersionBadgeChip badge={badge} />}
                  {s.generatedAt && (
                    <time
                      dateTime={s.generatedAt}
                      title={formatAbsoluteTime(s.generatedAt, i18n.language || 'en')}
                    >
                      {formatRequestRelativeTime(s.generatedAt)}
                    </time>
                  )}
                </span>
                <span
                  className={cn('truncate', note === null && 'italic')}
                  data-testid="solution-version-note"
                >
                  {note ??
                    translate(
                      'auto.components.request.SolutionVersionSwitcher.firstProposal',
                      'First proposal'
                    )}
                </span>
              </button>
            </li>
          )
        })}
      </ul>
      {pages > 1 && (
        <nav
          className="flex items-center justify-between gap-2 text-xs text-muted-foreground"
          aria-label={translate(
            'auto.components.request.SolutionVersionSwitcher.pages',
            'Version pages'
          )}
        >
          <Button
            size="xs"
            variant="outline"
            disabled={safePage === 0}
            onClick={() => setPage(safePage - 1)}
            data-testid="solution-version-newer"
          >
            {translate('auto.components.request.SolutionVersionSwitcher.newer', 'Newer')}
          </Button>
          <span role="status">
            {translate(
              'auto.components.request.SolutionVersionSwitcher.range',
              '{{from}}–{{to}} of {{total}}',
              {
                from: first + 1,
                to: first + visible.length,
                total: solutions.length
              }
            )}
          </span>
          <Button
            size="xs"
            variant="outline"
            disabled={safePage >= pages - 1}
            onClick={() => setPage(safePage + 1)}
            data-testid="solution-version-older"
          >
            {translate('auto.components.request.SolutionVersionSwitcher.older', 'Older')}
          </Button>
        </nav>
      )}
    </section>
  )
}
