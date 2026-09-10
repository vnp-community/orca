import { useState } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { useWorkspace } from '../context/WorkspaceContext'
import { Tracers } from '../../../shared/trace/tracers'

const MAX_CONCURRENCY = 3 // avoid dispatching dozens of agents at once onto a single dev-server

export function useTaskBatchExecution() {
  const [running, setRunning] = useState(false)
  const [results, setResults] = useState<Map<string, 'ok' | 'error'>>(new Map())
  const { project, currentWorktree } = useWorkspace()

  // Every task in a single selection belongs to the same project (useTasks filters by
  // projectId), so the currently active worktree is reused for all of them — no need to
  // resolve a worktree per task (task-service's Execute only accepts one worktreePath
  // anyway — see TaskDetail.tsx's handleRunAgent).
  const runSelected = async (taskIds: string[]): Promise<Map<string, 'ok' | 'error'>> => {
    if (!project || !currentWorktree) {
      return new Map()
    }
    setRunning(true)
    setResults(new Map())
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const queue = [...taskIds]
    const runResults = new Map<string, 'ok' | 'error'>()

    const runOne = async (taskId: string) => {
      const span = Tracers.uiTaskGraphExecuteFlow.start({ taskId, entryPoint: 'batch-run' })
      try {
        await callRuntimeRpc(target, 'task.execute', {
          taskId,
          projectId: project.id,
          worktreePath: currentWorktree.path,
          traceId: span.id
        })
        span.ok({ taskId })
        runResults.set(taskId, 'ok')
        setResults((prev) => new Map(prev).set(taskId, 'ok'))
      } catch (err) {
        span.fail(err, { taskId })
        runResults.set(taskId, 'error')
        setResults((prev) => new Map(prev).set(taskId, 'error'))
      }
    }
    // Simple bounded pool, no extra library.
    const workers = Array.from({ length: MAX_CONCURRENCY }, async () => {
      while (queue.length > 0) {
        const taskId = queue.shift()
        if (taskId) {
          await runOne(taskId)
        }
      }
    })
    await Promise.all(workers)
    setRunning(false)
    return runResults
  }

  return { running, results, runSelected }
}
