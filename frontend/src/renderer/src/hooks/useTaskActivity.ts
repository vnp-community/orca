import { useEffect, useReducer } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../shared/task-types'

// Same interval constant style as useWorkflowExecution.ts's EXECUTION_POLL_INTERVAL_MS —
// keep the two polling hooks consistent rather than inventing a second value.
const TASK_ACTIVITY_POLL_INTERVAL_MS = 4_000

// Polls `task.get` for post-Run task STATUS (cross-platform, works on the desktop `local` target too).
// The agent's live OUTPUT is a separate push channel: see useTaskAgentOutput.ts
// (`task.activity.subscribe`), which only has a client for runtime environments.
export type TaskActivityState = {
  task: OrcaTask | null
  // Always false: this hook only polls. Kept so callers need no change if status ever goes push.
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
