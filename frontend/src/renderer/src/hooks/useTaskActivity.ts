import { useEffect, useReducer } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../shared/task-types'

// Same interval constant style as useWorkflowExecution.ts's EXECUTION_POLL_INTERVAL_MS —
// keep the two polling hooks consistent rather than inventing a second value.
const TASK_ACTIVITY_POLL_INTERVAL_MS = 4_000

// Polling fallback for post-Run task status: `subscribeRuntimeEvent(target, 'task.activity:
// ${taskId}', cb)` (CR-FLOW-TASK-005's original design) doesn't exist anywhere in the
// codebase, and the one real push-event client (subscribeRuntimeClientEvents) only takes an
// environmentId (no 'local' target) and a closed event union with no task/workflow variant.
// Both the backend WS channel (CR-FLOW-TASK-003) and that client-side variant are still
// unbuilt — polling `task.get` is a real, working, cross-platform stopgap until they land.
export type TaskActivityState = {
  task: OrcaTask | null
  // Always false today — flips meaningful once a real push channel replaces this poll, so UI
  // can distinguish "polling" from "live" without a call-site change later.
  isLive: false
  lastPolledAt: number | null
}

type Action = { type: 'polled'; task: OrcaTask }

function reducer(state: TaskActivityState, action: Action): TaskActivityState {
  switch (action.type) {
    case 'polled':
      return { ...state, task: action.task, lastPolledAt: Date.now() }
  }
}

export function useTaskActivity(taskId: string | null | undefined): TaskActivityState {
  const [state, dispatch] = useReducer(reducer, { task: null, isLive: false, lastPolledAt: null })

  useEffect(() => {
    if (!taskId) {
      return
    }
    let cancelled = false
    const poll = async (): Promise<void> => {
      try {
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        // task.get expects { id } — see channels.go's task.get registration
        // (getArgs.ID, json tag "id"); other task.* channels use "taskId",
        // this one doesn't — verified directly against the handler, not
        // assumed from other channels' convention.
        const task = await callRuntimeRpc<OrcaTask>(target, 'task.get', { id: taskId })
        if (!cancelled) {
          dispatch({ type: 'polled', task })
        }
      } catch {
        // Transient RPC failure — next tick retries; no error state for a background poll.
      }
    }
    void poll() // poll immediately on mount, don't wait for the first interval tick
    const intervalId = setInterval(() => {
      void poll()
    }, TASK_ACTIVITY_POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(intervalId)
    }
  }, [taskId])

  return state
}
