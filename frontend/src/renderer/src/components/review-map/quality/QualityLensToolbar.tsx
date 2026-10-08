/**
 * QualityLensToolbar.tsx — FE-CV-TASK-087-06
 *
 * Connects the run hooks to the presentational run control and the persistent notices
 * (start errors, runs that did not complete). Nothing is rendered unless quality is enabled.
 *
 * @module components/review-map/quality/QualityLensToolbar
 */

import { useCallback } from 'react'
import { useAppStore } from '@/store'
import { useCodeIntelSelector } from '@/lib/code-intel-worktree-selector'
import { useQualityProfiles } from '../../../hooks/useQualityProfiles'
import { useQualityRun } from '../../../hooks/useQualityRun'
import type { ReviewScope } from '../review-scope-model'
import { QualityRunControl } from './QualityRunControl'
import { QualityRunErrorNotice, QualityRunFinishedNotice } from './QualityRunNotice'
import {
  buildQualityRunRequest,
  resolveRunScope,
  toQualityRunTarget
} from './quality-run-scope-model'
import type { QualityRunScopeChoice } from './quality-run-scope-model'
import { SPINNER_DELAY_LOCAL_MS, SPINNER_DELAY_REMOTE_MS } from './use-delayed-flag'

export function QualityLensToolbar({
  worktreeId,
  gateProfileRef,
  scope,
  locked
}: {
  worktreeId: string
  gateProfileRef: string | null
  scope: ReviewScope | null
  locked: boolean
}): React.JSX.Element {
  const { run, runError, start, cancel, dismiss } = useQualityRun(worktreeId)
  const profiles = useQualityProfiles(worktreeId, gateProfileRef)
  const selector = useCodeIntelSelector(worktreeId)
  const preferred = useAppStore(
    (s) => s.codeIntelQualityByWorktree[worktreeId]?.ui.runScope ?? 'changed'
  )
  const target = toQualityRunTarget(scope)
  const effectiveScope = resolveRunScope(preferred, profiles.selectedProfile) ?? preferred
  const remote = selector.state === 'ready' && selector.environmentId !== null

  const onStart = useCallback(() => {
    if (!profiles.selected) {
      return
    }
    const request = buildQualityRunRequest(
      profiles.selected,
      { scope: effectiveScope, base: target.base },
      profiles.selectedProfile
    )
    if (request) {
      void start(request)
    }
  }, [profiles.selected, profiles.selectedProfile, effectiveScope, target.base, start])

  const onSelectScope = useCallback(
    (next: QualityRunScopeChoice) =>
      useAppStore.getState().setQualityUi(worktreeId, { runScope: next }),
    [worktreeId]
  )

  return (
    <div className="flex flex-col gap-2" data-testid="quality-toolbar">
      <QualityRunControl
        runnable={profiles.runnable}
        selected={profiles.selected}
        selectedProfile={profiles.selectedProfile}
        scope={effectiveScope}
        run={run}
        locked={locked}
        spinnerDelayMs={remote ? SPINNER_DELAY_REMOTE_MS : SPINNER_DELAY_LOCAL_MS}
        onSelectProfile={profiles.select}
        onSelectScope={onSelectScope}
        onStart={onStart}
        onCancel={() => void cancel()}
      />
      {runError ? (
        <QualityRunErrorNotice
          error={runError}
          onRecheck={() => {
            dismiss()
            profiles.refetch()
          }}
          onDismiss={dismiss}
        />
      ) : null}
      {run?.phase === 'finished' ? (
        <QualityRunFinishedNotice run={run} onDismiss={dismiss} />
      ) : null}
    </div>
  )
}
