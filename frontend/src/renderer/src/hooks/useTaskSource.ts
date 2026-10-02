import { useEffect, useState } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'

export type TaskSource = { provider: string; ref: string; url?: string }

// The external issue (Jira/Linear/...) a task was started from, or null.
// Failures resolve to null on purpose: a runtime without task.getSource (older
// backend, local desktop) must simply show no badge, never an error.
export function useTaskSource(taskId: string | null | undefined): TaskSource | null {
  const [source, setSource] = useState<TaskSource | null>(null)

  useEffect(() => {
    setSource(null)
    if (!taskId) {
      return
    }
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<TaskSource | null>(target, 'task.getSource', { taskId })
      .then((result) => {
        if (!cancelled && result && typeof result.ref === 'string' && result.ref) {
          setSource(result)
        }
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [taskId])

  return source
}
