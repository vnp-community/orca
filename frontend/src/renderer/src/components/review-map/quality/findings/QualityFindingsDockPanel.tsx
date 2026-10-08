/**
 * QualityFindingsDockPanel.tsx — FE-CV-TASK-087-13
 *
 * The "Checks" findings panel of the review dock. It is its own dock panel (registered by the
 * lens registration), so check findings are never merged with the structural Findings of the
 * contract lens: different scale, actions and keys (waiver by fingerprint vs dismiss by key).
 * Filters live in the quality ui state; `selectedFingerprint` (set by a diff glyph click)
 * scrolls to and highlights its row.
 *
 * @module components/review-map/quality/findings/QualityFindingsDockPanel
 */

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { useQualitySupport } from '../../../../hooks/useQualitySupport'
import { useLatestQualityRun } from '../../../../hooks/useLatestQualityRun'
import { useQualityFindings } from '../../../../hooks/useQualityFindings'
import { useQualityWaive } from '../../../../hooks/useQualityWaive'
import { cancelAnnotationRevealFrame } from '../../../editor/check-annotation-open'
import { DEFAULT_QUALITY_UI } from '../../../../store/slices/code-intel-quality-state-types'
import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'
import { QualityFindingsList } from './QualityFindingsList'
import { QualityFindingsToolbar } from './QualityFindingsToolbar'
import { QualityWaivePopover } from './QualityWaivePopover'
import { countWaivedFindings, filterQualityFindings } from './quality-finding-filter'
import { openQualityFinding } from './quality-finding-open'
import type { QualityFindingOpenDiff } from './quality-finding-open'
import { sortQualityFindings } from './quality-finding-sort'

export type QualityFindingsDockPanelProps = {
  worktreeId: string
  /** Opens the review diff at a worktree-relative path and line; falls back to opening the file. */
  onOpenDiff?: QualityFindingOpenDiff
}

export default function QualityFindingsDockPanel({
  worktreeId,
  onOpenDiff
}: QualityFindingsDockPanelProps): React.JSX.Element | null {
  const support = useQualitySupport(worktreeId)
  const enabled = support === 'enabled'
  const ui = useAppStore((s) => s.codeIntelQualityByWorktree[worktreeId]?.ui ?? DEFAULT_QUALITY_UI)
  const run = useLatestQualityRun(worktreeId, enabled)
  const [search, setSearch] = useState('')
  const revealRafRef = useRef<number | null>(null)
  const revealInnerRafRef = useRef<number | null>(null)

  const server = useQualityFindings(enabled ? worktreeId : null, {
    severities: ui.severity,
    categories: ui.category,
    inScope: ui.onlyInScope
  })
  const waive = useQualityWaive(worktreeId, server.patchWaiver)

  useEffect(
    () => () => {
      cancelAnnotationRevealFrame(revealRafRef)
      cancelAnnotationRevealFrame(revealInnerRafRef)
    },
    []
  )

  const visible = useMemo(
    () =>
      sortQualityFindings(
        filterQualityFindings(server.items, { search, showWaived: ui.showWaived })
      ),
    [server.items, search, ui.showWaived]
  )
  const setUi = useCallback(
    (patch: Parameters<ReturnType<typeof useAppStore.getState>['setQualityUi']>[1]) =>
      useAppStore.getState().setQualityUi(worktreeId, patch),
    [worktreeId]
  )
  const open = useCallback(
    (finding: QualityFinding) =>
      openQualityFinding({ worktreeId, finding, onOpenDiff, revealRafRef, revealInnerRafRef }),
    [worktreeId, onOpenDiff]
  )

  if (!enabled) {
    return null
  }
  const filtered =
    ui.severity.length > 0 ||
    ui.category.length > 0 ||
    search.trim() !== '' ||
    server.items.length > visible.length
  let emptyReason: 'no-run' | 'none' | 'filtered' = 'none'
  if (run === null && server.items.length === 0) {
    emptyReason = 'no-run'
  } else if (filtered) {
    emptyReason = 'filtered'
  }

  return (
    <div className="flex h-full min-h-0 flex-col" data-testid="quality-findings-dock-panel">
      <QualityFindingsToolbar
        severity={ui.severity}
        category={ui.category}
        onlyInScope={ui.onlyInScope}
        showWaived={ui.showWaived}
        waivedCount={countWaivedFindings(server.items)}
        search={search}
        onSeverityChange={(severity) => setUi({ severity })}
        onCategoryChange={(category) => setUi({ category })}
        onOnlyInScopeChange={(onlyInScope) => setUi({ onlyInScope })}
        onShowWaivedChange={(showWaived) => setUi({ showWaived })}
        onSearchChange={setSearch}
      />
      <QualityFindingsList
        items={visible}
        status={server.status}
        total={server.total}
        truncated={server.truncated}
        hasMore={server.hasMore}
        capped={server.capped}
        loadingMore={server.loadingMore}
        outsideScopeCount={server.outsideScopeCount}
        emptyReason={emptyReason}
        selectedFingerprint={ui.selectedFingerprint}
        onOpen={open}
        renderWaiveAction={(finding) => <QualityWaivePopover finding={finding} waive={waive} />}
        onLoadMore={server.loadMore}
        onRetry={server.retry}
      />
    </div>
  )
}
