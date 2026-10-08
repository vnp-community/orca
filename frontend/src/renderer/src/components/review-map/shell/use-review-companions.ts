/**
 * use-review-companions.ts — FE-CV-TASK-060-07 / 089-04 / 093-04 / 095-05 / 095-06
 *
 * Everything the workspace runs next to the lenses: agent-turn recording and comparison, the AI
 * summary state, the surface telemetry and the "mark reviewed" decision. Split from
 * ReviewWorkspace so the layout file stays about layout.
 *
 * @module components/review-map/shell/use-review-companions
 */

import { useEffect, useMemo, useRef, useState } from 'react'
import { i18n } from '@/i18n/i18n'
import { recordReviewSurfaceDecision } from '@/lib/review-surface-decision'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'
import { useReviewAiSummary } from '../ai-summary/use-review-ai-summary'
import { countSeen } from '../reading-progress-merge'
import type { ReviewDataApi } from '../review-shell-data'
import { useAgentTurnVerification } from '../turns/use-agent-turn-verification'
import { useReviewTurnMarkers } from '../turns/use-review-turn-markers'
import { useReviewTurnRecorderChannel } from '../turns/use-review-turn-recorder-channel'
import type { ReviewTurnViewMode } from '../turns/review-turn-selection'
import type { TurnCompareResult } from '../turns/turn-compare-model'
import { useReviewSurfaceTelemetry } from './use-review-surface-telemetry'
import type { useReviewWorkspaceModel } from './useReviewWorkspaceModel'

type WorkspaceModel = ReturnType<typeof useReviewWorkspaceModel>

export function useReviewCompanions(worktreeId: string, api: ReviewDataApi, m: WorkspaceModel) {
  const { data, scope, activeLensId, viewState } = m
  const flags = useQualityFeatureFlags()
  const supported = m.selector.state === 'ready' && flags.codeIntel
  const turnMarkers = useReviewTurnMarkers(worktreeId, api, supported)
  // Why: recording runs at App level (use-app-agent-turn-recorders) so closed-tab turns are kept;
  // the workspace only feeds it symbol keys and reads the result.
  const turnRecorder = useReviewTurnRecorderChannel(
    worktreeId,
    data.overlay ? data.overlay.changedSymbols.map((c) => c.symbol.key) : null,
    turnMarkers.applySaved
  )
  const verificationByTurn = useAgentTurnVerification({
    worktreeId,
    enabled: flags.quality && turnMarkers.markers.length > 0,
    refreshKey: turnMarkers.markers.length
  })

  const [turnMode, setTurnMode] = useState<ReviewTurnViewMode>('all')
  const [selectedTurnId, setSelectedTurnId] = useState<string | null>(null)
  const [turnCompare, setTurnCompare] = useState<TurnCompareResult | null>(null)
  // "Since previous turn" narrows the reading order to files the last turn added or changed.
  const turnFiles = useMemo(
    () =>
      turnCompare
        ? new Set(
            Object.entries(turnCompare.files)
              .filter(([, label]) => label === 'new_in_turn' || label === 'changed_in_turn')
              .map(([path]) => path)
          )
        : null,
    [turnCompare]
  )
  const readingFilter = useMemo(() => {
    if (!turnFiles) {
      return m.filter
    }
    return m.filter ? new Set([...m.filter].filter((f) => turnFiles.has(f))) : turnFiles
  }, [m.filter, turnFiles])

  const aiSummary = useReviewAiSummary({
    projectId: m.worktree?.projectId,
    worktreeId,
    base: m.summary?.baseRef ?? null,
    locale: i18n.language || 'en'
  })
  const changedFileSet = useMemo(
    () => new Set((data.overlay?.changedFiles ?? []).map((f) => f.path)),
    [data.overlay]
  )

  useReviewSurfaceTelemetry({
    worktreeId,
    ready: viewState.kind !== 'screen' && data.overlay !== null,
    scope,
    defaultBaseRef: m.summary?.baseRef ?? m.worktree?.baseRef ?? null,
    overall: data.status?.overall,
    activeLensId
  })

  const entry = m.progressEntry
  const seenAll =
    entry?.loadStatus === 'ready' &&
    m.items.length > 0 &&
    countSeen(
      entry.progress,
      m.items.map((i) => i.stepKey)
    ) === m.items.length
  const seenAllRef = useRef(false)
  useEffect(() => {
    // Why: finishing the reading order is the "mark reviewed" decision; fire on the transition only.
    if (seenAll && !seenAllRef.current) {
      recordReviewSurfaceDecision(worktreeId, 'mark_reviewed')
    }
    seenAllRef.current = seenAll
  }, [seenAll, worktreeId])

  return {
    turnMarkers: turnMarkers.markers,
    turnSaveFailed: turnRecorder.failed,
    retryTurnSave: turnRecorder.retry,
    verificationByTurn,
    turnMode,
    setTurnMode,
    selectedTurnId,
    setSelectedTurnId,
    setTurnCompare,
    turnFilterActive: turnFiles !== null,
    readingFilter,
    aiSummary,
    changedFileSet
  }
}
