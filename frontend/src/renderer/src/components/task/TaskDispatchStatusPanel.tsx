import { useEffect, useState } from 'react'
import { useAppStore } from '../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { focusRuntimeOrchestrationTask } from '../terminal-pane/terminal-orchestration-task-links'
import { Button } from '../ui/button'
import { translate } from '@/i18n/i18n'

const T = 'auto.components.task.dispatch.'

type DispatchView = {
  id: string
  orchestration_task_id: string
  assignee_handle: string
  status: string
}

// Shows the dispatch status of exactly the 1 task being viewed — NOT a list of multiple
// dispatches/coordinator runs (that data doesn't exist under orchestration.* today, see
// BUG-TASKV1-005). Reuses the exact RPC + logic terminal-orchestration-task-links.ts
// already uses for real.
type DispatchState = 'loading' | 'not-dispatched' | DispatchView

export function TaskDispatchStatusPanel({ taskId }: { taskId: string }) {
  // Folded loading into the same union (rather than a separate boolean) so TS narrows
  // `dispatch` to DispatchView below without a redundant null check — once loading
  // clears, the effect has always set a definite 'not-dispatched' or DispatchView.
  const [dispatch, setDispatch] = useState<DispatchState>('loading')

  useEffect(() => {
    let cancelled = false
    setDispatch('loading')
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<{ dispatch: DispatchView | null }>(target, 'orchestration.dispatchShow', {
      task: taskId
    })
      .then((r) => {
        if (!cancelled) {
          setDispatch(r.dispatch ?? 'not-dispatched')
        }
      })
      .catch(() => {
        if (!cancelled) {
          setDispatch('not-dispatched')
        }
      })
    return () => {
      cancelled = true
    }
  }, [taskId])

  if (dispatch === 'loading') {
    return (
      <div className="text-xs text-muted-foreground p-2">
        {translate(`${T}loading`, 'Loading dispatch status…')}
      </div>
    )
  }

  if (dispatch === 'not-dispatched') {
    return (
      <div className="text-xs text-muted-foreground p-2" data-testid="task-dispatch-none">
        {translate(
          `${T}none`,
          'This task has no orchestration dispatch yet (it has not run through Engine 2, or it is running directly through Engine 1).'
        )}
      </div>
    )
  }

  const focusTerminal = (): void => {
    // Use the actually-active target (SSH/environment aware) instead of hard-coding
    // null — the open task can belong to an SSH-hosted environment, not just local
    // (see AGENTS.md "SSH Use Case").
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const environmentId = target.kind === 'environment' ? target.environmentId : null
    focusRuntimeOrchestrationTask(taskId, environmentId).catch(() => {})
  }

  return (
    <div
      className="text-xs border border-border rounded-md p-2 space-y-1"
      data-testid="task-dispatch-status"
    >
      <div>
        <span className="font-medium">{translate(`${T}status`, 'Status:')}</span> {dispatch.status}
      </div>
      <div>
        <span className="font-medium">{translate(`${T}assignee`, 'Assignee:')}</span>{' '}
        {dispatch.assignee_handle || '—'}
      </div>
      {dispatch.assignee_handle && (
        <Button
          variant="link"
          size="xs"
          className="h-auto p-0"
          onClick={focusTerminal}
          data-testid="task-dispatch-focus-terminal"
        >
          {translate(`${T}focusTerminal`, 'Focus the terminal running this dispatch')}
        </Button>
      )}
    </div>
  )
}
