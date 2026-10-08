/**
 * QualityLens.tsx — FE-CV-TASK-087-07
 *
 * The `quality` review lens: run toolbar, scorecard, then lazily loaded blocks (coverage,
 * trend, hotspot, dependency). It renders nothing unless quality is enabled. The findings list
 * itself lives in the bottom dock; this lens only filters it and links to it.
 *
 * @module components/review-map/quality/QualityLens
 */

import { useCallback, useMemo } from 'react'
import { useAppStore } from '@/store'
import { useQualityGate } from '../../../hooks/useQualityGate'
import type { ReviewLensProps } from '../review-lens-registry'
import { QualityLensBlockHost } from './QualityLensBlockHost'
import { QualityLensToolbar } from './QualityLensToolbar'
import { QualityNotices, QualityStateScreen, isQualityStateScreenKind } from './QualityStateScreen'
import { QualityScorecard } from './QualityScorecard'
import type { ReasonFilter } from './QualityGateReasonRow'
import { QUALITY_LENS_BLOCKS } from './quality-lens-blocks'
import { splitProfileRef } from './quality-profile-selection'
import { toQualityRunTarget } from './quality-run-scope-model'
import { indexDiffersFromHead, isGateStale } from './quality-stale-model'
import { deriveQualityViewState } from './quality-view-state'

export default function QualityLens({
  worktreeId,
  scope,
  onOpenDiff
}: ReviewLensProps): React.JSX.Element | null {
  const target = toQualityRunTarget(scope)
  const gate = useQualityGate(worktreeId, { base: target.base })
  const runs = useAppStore((s) => s.codeIntelQualityByWorktree[worktreeId]?.runs?.data ?? null)
  const run = useAppStore((s) => s.codeIntelQualityByWorktree[worktreeId]?.run ?? null)
  const runError = useAppStore((s) => s.codeIntelQualityByWorktree[worktreeId]?.runError ?? null)
  const openBlocks = useAppStore((s) => s.codeIntelQualityByWorktree[worktreeId]?.ui.openBlocks)
  const currentHead = useAppStore(
    (s) => s.gitBranchCompareSummaryByWorktree[worktreeId]?.headOid ?? null
  )

  const response = gate.response
  const staleness = isGateStale({
    gate: response?.gate ?? null,
    cacheStale: gate.cacheStale,
    runs,
    currentHead
  })
  const view = useMemo(
    () =>
      deriveQualityViewState({
        gateStatus: gate.status,
        gate: response,
        gateErrorKind: gate.error?.kind ?? null,
        resultStale: staleness.stale,
        indexBehindHead: indexDiffersFromHead(response?.gate ?? null, currentHead),
        runs,
        run,
        runError
      }),
    [gate.status, response, gate.error, staleness.stale, currentHead, runs, run, runError]
  )

  const setBlockOpen = useCallback(
    (id: string, open: boolean) => {
      const current =
        useAppStore.getState().codeIntelQualityByWorktree[worktreeId]?.ui.openBlocks ?? []
      const next = open ? [...current.filter((b) => b !== id), id] : current.filter((b) => b !== id)
      useAppStore.getState().setQualityUi(worktreeId, { openBlocks: next })
    },
    [worktreeId]
  )
  const showFindings = useCallback(
    (filter: ReasonFilter | null) => {
      useAppStore.getState().setQualityUi(worktreeId, {
        source: 'quality',
        category: filter?.category ? [filter.category as never] : [],
        selectedFingerprint: null
      })
    },
    [worktreeId]
  )

  if (gate.support !== 'enabled') {
    return null
  }
  const profileName = response ? splitProfileRef(response.gate.profile).name : ''
  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-3"
      data-testid="quality-lens"
    >
      <QualityLensToolbar
        worktreeId={worktreeId}
        gateProfileRef={response?.gate.profile ?? null}
        scope={scope}
        locked={view.lockRun}
      />
      <QualityNotices notices={view.notices} />
      {isQualityStateScreenKind(view.primary) ? (
        <QualityStateScreen kind={view.primary} profile={profileName} onRetry={gate.refetch} />
      ) : null}
      {view.showScorecard && response ? (
        <QualityScorecard
          response={response}
          runs={runs}
          cacheStale={gate.cacheStale}
          currentHead={currentHead}
          runLocked={view.lockRun || (run !== null && run.phase !== 'finished')}
          onShowFindings={showFindings}
        />
      ) : null}
      <div className="flex flex-col gap-1">
        {QUALITY_LENS_BLOCKS.map((block) => (
          <QualityLensBlockHost
            key={block.id}
            block={block}
            open={openBlocks?.includes(block.id) ?? false}
            onOpenChange={(open) => setBlockOpen(block.id, open)}
            blockProps={{ worktreeId, onOpenDiff }}
          />
        ))}
      </div>
    </div>
  )
}
