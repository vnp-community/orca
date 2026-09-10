import { useEffect, useState } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { TaskComment } from '../../../shared/task-types'

// isSupported feature-detect: probe `task.listComments` on mount and fall back to
// "unsupported" on any error. A dedicated RPC capability-probe is out of scope here
// (TASK-FE-TASKV1-08) — this is a best-effort try/catch gate, same pattern the task
// spec calls for.
//
// `task.listComments` is NOT a confirmed real RPC name — backend-go has neither
// AddComment nor ListComments (task.proto's TaskService has 0 comment RPCs), and
// auditing Node's task-rpc-handler.ts for this task turned up only `task.addComment`;
// no matching read RPC exists there either. So this probe fails (and the UI shows
// "unavailable") on every deploy target today, until a backend task adds the read RPC
// (see TaskComments.tsx's unsupported message for the tracking bug).
export function useTaskComments(taskId: string) {
  const [comments, setComments] = useState<TaskComment[]>([])
  const [isSupported, setIsSupported] = useState(true) // optimistic, flips false if list fails

  useEffect(() => {
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<{ comments: TaskComment[] }>(target, 'task.listComments', { taskId })
      .then((r) => {
        if (!cancelled) {
          setComments(r.comments ?? [])
        }
      })
      .catch(() => {
        if (!cancelled) {
          setIsSupported(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [taskId])

  const addComment = async (content: string): Promise<void> => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // Node's real `task.addComment` handler returns `{ added: true }`, not the created
    // TaskComment (confirmed by reading backend/src/main/task/task-rpc-handler.ts) — so
    // this refetches the list instead of optimistically appending the RPC response,
    // unlike the shape SOL-FE-TASKV1-004's original sample assumed.
    await callRuntimeRpc<{ added: boolean }>(target, 'task.addComment', { taskId, content })
    const r = await callRuntimeRpc<{ comments: TaskComment[] }>(target, 'task.listComments', {
      taskId
    })
    setComments(r.comments ?? [])
  }

  return { comments, addComment, isSupported }
}
