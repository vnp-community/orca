/**
 * FindingsPanel.tsx — FE-CV-TASK-059-05
 *
 * Structural findings (source `Finding`, not the quality gate's `QualityFinding`). Defaults to the
 * findings introduced by this change, lets the user ignore / resolve / reopen, and never claims
 * the code is free of problems: an empty list only means nothing was found in the indexed scope.
 *
 * @module components/review-map/findings/FindingsPanel
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import { isEditableTarget } from '@/lib/editable-target'
import { trackReviewFindingsSummary } from '@/lib/review-telemetry'
import { useCodeIntelFindings } from '../../../hooks/useCodeIntelFindings'
import { useFindingDismissal } from '../../../hooks/useFindingDismissal'
import { countByKindAndSeverity, filterFindings } from './finding-filter'
import { openFindingInGraph, resolveFindingGraphTarget } from './finding-graph-target'
import { toFindingRow } from './finding-view-model'
import type { FindingRowModel } from './finding-view-model'
import { tf } from './findings-i18n'
import { ReviewNoteButton } from '../notes/ReviewNoteButton'
import { FindingRow } from './FindingRow'
import { FindingsList } from './FindingsList'
import { FindingsToolbar } from './FindingsToolbar'
import type { FindingsToolbarState } from './FindingsToolbar'

const INITIAL_TOOLBAR: FindingsToolbarState = {
  kinds: [],
  severities: [],
  onlyIntroduced: true,
  includeDismissed: false,
  query: ''
}

export type FindingsPanelProps = {
  worktreeId: string
  environmentId: string | null
  /** Paths changed in the current scope; decides "View diff" vs "Open file". */
  changedFiles: ReadonlySet<string>
  /** Symbol keys of the nodes currently drawn, for "View in graph". */
  graphSymbolKeys: ReadonlySet<string> | null
  /** Lenses that can actually be shown. */
  availableLensIds: ReadonlySet<string>
  onOpenDiff: (path: string, line?: number) => void
  onOpenFile: (path: string, line?: number) => void
}

export function FindingsPanel(props: FindingsPanelProps): React.JSX.Element {
  const { worktreeId, environmentId, changedFiles, graphSymbolKeys, availableLensIds } = props
  const [toolbar, setToolbar] = useState<FindingsToolbarState>(INITIAL_TOOLBAR)
  const [activeKey, setActiveKey] = useState<string | null>(null)
  // Last write per row so "Retry" repeats exactly what failed.
  const retryRef = useRef(new Map<string, () => void>())

  const findings = useCodeIntelFindings(worktreeId, environmentId, {
    scope: 'changed',
    includeDismissed: toolbar.includeDismissed,
    severities: toolbar.severities
  })
  const dismissal = useFindingDismissal({
    worktreeId,
    environmentId,
    setOverride: findings.setOverride,
    clearOverride: findings.clearOverride,
    reload: findings.reload
  })
  const setReviewLens = useAppStore((s) => s.setReviewLens)
  const selectReviewSymbol = useAppStore((s) => s.selectReviewSymbol)
  const setErdService = useAppStore((s) => s.setErdService)
  const selectErdTable = useAppStore((s) => s.selectErdTable)

  const allRows = useMemo(() => findings.findings.map((f) => toFindingRow(f, translate)), [findings.findings])
  const counts = useMemo(() => countByKindAndSeverity(allRows.filter((r) => !r.isDismissed)), [allRows])
  const rows = useMemo(
    () =>
      filterFindings(allRows, {
        kinds: toolbar.kinds,
        origins: toolbar.onlyIntroduced ? ['introduced'] : undefined,
        query: toolbar.query,
        includeDismissed: toolbar.includeDismissed
      }),
    [allRows, toolbar]
  )

  // One summary per panel mount, after the first successful load (counts become buckets).
  const summarizedRef = useRef(false)
  useEffect(() => {
    if (findings.status !== 'success' || summarizedRef.current) {
      return
    }
    summarizedRef.current = true
    trackReviewFindingsSummary({
      shown: allRows.filter((r) => !r.isDismissed).length,
      dismissed: allRows.filter((r) => r.disposition === 'ignored').length,
      waived: 0,
      resolved: allRows.filter((r) => r.disposition === 'resolved').length
    })
  }, [findings.status, allRows])

  const onKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>) => {
      if (event.metaKey || event.ctrlKey || event.altKey || isEditableTarget(event.target)) {
        return
      }
      const keys = rows.map((r) => r.findingKey)
      const index = activeKey ? keys.indexOf(activeKey) : -1
      const active = index >= 0 ? rows[index] : null
      if (event.key === 'j' || event.key === 'k') {
        const next = keys[Math.max(0, Math.min(keys.length - 1, index + (event.key === 'j' ? 1 : -1)))]
        setActiveKey(next ?? null)
      } else if (event.key === 'Escape') {
        setActiveKey(null)
      } else if (!active) {
        return
      } else if (event.key === 'Enter' && active.locationPath) {
        openLocation(active)
      } else if (event.key === 'd' && !active.isDismissed) {
        const rowEl = [...event.currentTarget.querySelectorAll<HTMLElement>('[data-finding-key]')].find(
          (el) => el.dataset.findingKey === active.findingKey
        )
        rowEl?.querySelector<HTMLElement>('[data-action="ignore"]')?.click()
      } else if (event.key === 'r' && active.isDismissed) {
        void dismissal.restore(active.finding)
      } else {
        return
      }
      event.preventDefault()
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps -- openLocation reads props only
    [rows, activeKey, dismissal]
  )

  function openLocation(row: FindingRowModel): void {
    if (!row.locationPath) {
      return
    }
    if (changedFiles.has(row.locationPath)) {
      props.onOpenDiff(row.locationPath, row.locationLine ?? undefined)
    } else {
      props.onOpenFile(row.locationPath, row.locationLine ?? undefined)
    }
  }

  if (findings.status === 'idle' || findings.status === 'loading') {
    return (
      <div role="status" aria-live="polite" className="space-y-2 p-3">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 className="size-3.5 animate-spin" aria-hidden />
          {tf('loading', 'Loading findings...')}
        </div>
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    )
  }
  if (findings.status === 'error') {
    const noIndex = findings.error?.kind === 'index-missing' || findings.error?.kind === 'tool-unavailable'
    return (
      <div className="space-y-2 p-3 text-xs">
        <p className="text-muted-foreground">
          {noIndex
            ? tf('error.noIndex', 'The code index is not available, so structural findings cannot be listed yet.')
            : tf('error.load', 'Could not load findings.')}
        </p>
        <Button type="button" variant="outline" size="xs" onClick={findings.reload}>
          {tf('retry', 'Retry')}
        </Button>
      </div>
    )
  }

  const freshness = findings.indexFreshness
  return (
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions -- list-local key handling
    <div
      className="flex min-h-0 flex-1 flex-col"
      onKeyDown={onKeyDown}
      aria-label={tf('label', 'Structural findings')}
      role="region"
    >
      <FindingsToolbar
        state={toolbar}
        counts={counts}
        dismissedCount={findings.dismissedCount}
        freshness={freshness}
        onChange={setToolbar}
      />
      {rows.length === 0 ? (
        <p className="p-3 text-xs text-muted-foreground">
          {tf('empty', 'No problems were found in the indexed scope.')}
          {freshness?.indexedCommit
            ? ` ${tf('emptyIndexed', 'Index: {{commit}} at {{date}}.', { commit: freshness.indexedCommit.slice(0, 8), date: freshness.generatedAt })}`
            : ''}
        </p>
      ) : (
        <FindingsList
          rows={rows}
          hasNextPage={findings.hasNextPage}
          isFetchingNextPage={findings.isFetchingNextPage}
          nextPageFailed={findings.nextPageError !== null}
          onLoadMore={findings.loadMore}
          renderRow={(row) => {
            const target = resolveFindingGraphTarget(row.finding, { availableLensIds, graphSymbolKeys })
            const inDiff = row.locationPath ? changedFiles.has(row.locationPath) : false
            return (
              <FindingRow
                row={row}
                busy={dismissal.pendingKeys.has(row.findingKey)}
                error={dismissal.errorsByKey[row.findingKey] ?? null}
                active={row.findingKey === activeKey}
                onViewInGraph={
                  target
                    ? () =>
                        openFindingInGraph(worktreeId, target, {
                          setReviewLens,
                          selectReviewSymbol,
                          setErdService,
                          selectErdTable
                        })
                    : null
                }
                onOpenLocation={row.locationPath ? () => openLocation(row) : null}
                openLocationLabel={inDiff ? tf('action.viewDiff', 'View diff') : tf('action.openFile', 'Open file')}
                noteSlot={
                  row.locationPath ? (
                    <ReviewNoteButton
                      worktreeId={worktreeId}
                      anchor={{
                        kind: 'finding',
                        findingKey: row.findingKey,
                        filePath: row.locationPath,
                        ...(row.locationLine ? { startLine: row.locationLine } : {}),
                        label: row.title
                      }}
                    />
                  ) : undefined
                }
                onDismiss={(input) => {
                  const run = (): void => void dismissal.dismiss(row.finding, input)
                  retryRef.current.set(row.findingKey, run)
                  run()
                }}
                onResolve={() => {
                  const run = (): void => void dismissal.resolve(row.finding)
                  retryRef.current.set(row.findingKey, run)
                  run()
                }}
                onRestore={() => {
                  const run = (): void => void dismissal.restore(row.finding)
                  retryRef.current.set(row.findingKey, run)
                  run()
                }}
                onRetry={() => retryRef.current.get(row.findingKey)?.()}
              />
            )
          }}
        />
      )}
    </div>
  )
}
