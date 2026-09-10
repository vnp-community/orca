import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../shared/task-types'

export type TaskEdgeMap = Map<string, { blockedBy: string[]; blocks: string[] }>

const FETCH_CONCURRENCY = 4

// N+1 calls because task.getDependencies has no batch-by-projectId variant at
// backend-go today. The real response is a flat Task[] (NO edgeType — see
// task.proto:169-177 + channels_automation_task.go:296-308), always the
// "this task depends on" direction — the "blocks" direction is derived by
// cross-scanning once every task in the batch has been fetched.
export function useTaskDependencyEdges(tasks: OrcaTask[]): {
  edges: TaskEdgeMap
  loading: boolean
  error: boolean
  refetch: () => void
} {
  const [edges, setEdges] = useState<TaskEdgeMap>(new Map())
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)
  // Bumped by refetch() to re-run the fetch effect without needing a new `tasks`
  // reference — e.g. after TASK-FE-TASKV1-04's onEdgeAdded, when the new edge
  // doesn't itself change any field on the OrcaTask objects passed in.
  const [refetchKey, setRefetchKey] = useState(0)
  const tasksRef = useRef(tasks)
  tasksRef.current = tasks
  // Key the effect off the set of task ids, not the `tasks` array's own reference — a
  // caller that (accidentally or not) passes a freshly-allocated array every render
  // would otherwise re-trigger this effect every render, setEdges(new Map()) every
  // time (a new object, so React never bails out), and infinite-loop.
  const taskIdsKey = useMemo(() => tasks.map((t) => t.id).join(','), [tasks])

  const refetch = useCallback(() => setRefetchKey((k) => k + 1), [])

  useEffect(() => {
    const currentTasks = tasksRef.current
    if (currentTasks.length === 0) {
      setEdges(new Map())
      return
    }
    setLoading(true)
    setError(false)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const blockedByMap = new Map<string, string[]>()
    let hadError = false
    const queue = currentTasks.map((t) => t.id)

    const worker = async () => {
      while (queue.length > 0) {
        const taskId = queue.shift()
        if (!taskId) {
          continue
        }
        try {
          const deps = await callRuntimeRpc<OrcaTask[]>(target, 'task.getDependencies', { taskId })
          blockedByMap.set(
            taskId,
            (deps ?? []).map((d) => d.id)
          )
        } catch {
          // Previously TaskDetail.tsx's `.catch(() => {})` swallowed this entirely — now
          // an error flag is surfaced instead of failing silently.
          hadError = true
        }
      }
    }
    Promise.all(Array.from({ length: FETCH_CONCURRENCY }, worker)).then(() => {
      // Derive "blocks" (the reverse direction) from blockedByMap once it's fully
      // populated for every task in this batch.
      const result: TaskEdgeMap = new Map()
      for (const taskId of blockedByMap.keys()) {
        result.set(taskId, { blockedBy: blockedByMap.get(taskId) ?? [], blocks: [] })
      }
      for (const [taskId, blockedBy] of blockedByMap) {
        for (const depId of blockedBy) {
          const entry = result.get(depId)
          if (entry) {
            entry.blocks.push(taskId)
          }
        }
      }
      setEdges(result)
      setError(hadError)
      setLoading(false)
    })
  }, [taskIdsKey, refetchKey])

  return { edges, loading, error, refetch }
}
