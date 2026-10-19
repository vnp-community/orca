/**
 * ExecutionFailureCard — FE-REQ-TASK-037-05
 *
 * Replaces the one-line backlog banner when a task out of attempts sent the Request back: which task, how many times, why, what
 * was kept, and the three things the viewer can do.
 *
 * @module components/request/execution/ExecutionFailureCard
 */

import React from 'react'
import { OctagonX } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/store'
import { supportsBaseRef } from '@/store/slices/request'
import { BaseRefChangeControl } from '../BaseRefChangeControl'
import { BaseRefSaveFeedback } from '../BaseRefSaveFeedback'
import { useFreshestRequest, useRequestBaseRefSave } from '../use-request-base-ref-save'
import { failureCauseAdvice, failureCauseLabel, shortError } from './execution-failure-cause'
import type {
  ExecutionFailedTask,
  ExecutionProgress
} from '../../../../../shared/request-execution-types'
import type { OrcaRequest } from '../../../../../shared/request-types'

const T = 'auto.components.request.execution.ExecutionFailureCard.'

type Props = {
  failed: ExecutionFailedTask
  progress: ExecutionProgress
  busy: boolean
  onReopen: () => void
  onViewLog: (taskId: string) => void
  onCancel: () => void
  /** The Request this card is about, for "Change starting branch"; the open Request of the page when absent. */
  request?: OrcaRequest
}

/** Offered when the starting branch is gone and no worktree exists yet, so choosing another one can still fix the run. */
function ChangeStartingBranch({
  request,
  busy
}: {
  request: OrcaRequest
  busy: boolean
}): React.JSX.Element | null {
  const current = useFreshestRequest(request)
  const { state, save, applyConflict, dismiss } = useRequestBaseRefSave(current)
  if (current.baseRefLocked === true || state.forbidden) {
    return null
  }
  return (
    <>
      <BaseRefChangeControl request={current} onSave={(ref) => void save(ref)}>
        <Button
          size="sm"
          variant="outline"
          disabled={busy || state.saving}
          data-testid="execution-failure-change-branch"
        >
          {translate(`${T}changeBranch`, 'Change starting branch')}
        </Button>
      </BaseRefChangeControl>
      <div className="basis-full">
        <BaseRefSaveFeedback
          state={state}
          currentBaseRef={current.baseRef ?? ''}
          unverified={state.unverifiedRef !== null && state.unverifiedRef === current.baseRef}
          onApplyConflict={() => void applyConflict()}
          onDismiss={dismiss}
          onPickSuggestion={(ref) => void save(ref)}
        />
      </div>
    </>
  )
}

export function ExecutionFailureCard({
  failed,
  progress,
  busy,
  onReopen,
  onViewLog,
  onCancel,
  request
}: Props): React.JSX.Element {
  const error = shortError(failed.lastError)
  const supported = useAppStore(supportsBaseRef)
  const openRequest = useAppStore((s) =>
    s.requestPage.requestId ? s.requestsById[s.requestPage.requestId] : undefined
  )
  const baseRefRequest = request ?? openRequest
  return (
    <div
      role="alert"
      className="flex flex-col gap-2 rounded-md border border-destructive/40 px-3 py-3 text-sm"
      data-testid="execution-failure-card"
    >
      <div className="flex items-start gap-2">
        <OctagonX className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <p className="font-medium">
            {translate(`${T}title`, 'Stopped: “{{task}}” failed {{attempts}}/{{max}} times', {
              task: failed.title,
              attempts: failed.attempts,
              max: failed.maxAttempts
            })}
          </p>
          <p>
            {translate(`${T}cause`, 'Cause: {{cause}}', {
              cause: failureCauseLabel(failed.lastCause)
            })}
          </p>
          {error && <p className="break-words text-xs text-muted-foreground">{error}</p>}
          <p className="text-xs text-muted-foreground">{failureCauseAdvice(failed.lastCause)}</p>
          <p className="text-xs text-muted-foreground">
            {translate(
              `${T}kept`,
              'Everything before it is done and kept ({{done}}/{{total}} tasks).',
              {
                done: progress.tasksDone,
                total: progress.tasksTotal
              }
            )}
          </p>
        </div>
      </div>
      <div className="flex flex-wrap gap-2 pl-6">
        <Button size="sm" disabled={busy} onClick={onReopen} data-testid="execution-failure-reopen">
          {translate(`${T}reopen`, 'Reopen and rerun')}
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => onViewLog(failed.taskId)}
          data-testid="execution-failure-log"
        >
          {translate(`${T}log`, 'View last log')}
        </Button>
        {failed.lastCause === 'base_ref_not_found' && supported && baseRefRequest && (
          <ChangeStartingBranch request={baseRefRequest} busy={busy} />
        )}
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={onCancel}
          data-testid="execution-failure-cancel"
        >
          {translate(`${T}cancel`, 'Cancel request')}
        </Button>
      </div>
    </div>
  )
}
