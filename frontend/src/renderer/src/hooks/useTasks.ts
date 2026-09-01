import { useState, useMemo, useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../shared/task-types'

export function useTasks(projectId: string) {
  // Use Option B: flat filter since store doesn't have tasksByProject index yet.
  // Why select the raw `tasks` array here instead of filtering inside the
  // selector: `.filter()` allocates a brand-new array on every single call,
  // so useSyncExternalStore's snapshot never stabilizes — React detects the
  // "changed every render" snapshot and throws error #185 (Maximum update
  // depth exceeded), live-reproduced opening this page for a real project.
  // Selecting the stable `tasks` reference and filtering in a memo below
  // keeps the snapshot stable across renders that don't touch `tasks`.
  const tasks = useAppStore((s) => s.tasks)
  const allTasks = useMemo(() => tasks.filter((t) => t.projectId === projectId), [tasks, projectId])

  // Store actions
  const setTasks = useAppStore((s) => s.setTasks)
  const setActiveTask = useAppStore((s) => s.setActiveTask)

  // Local UI state
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set())
  const [filterStatus, setFilterStatus] = useState<'all' | string>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  // Fetch tasks when projectId changes
  useEffect(() => {
    if (!projectId) {
      return
    }
    setIsLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<OrcaTask[]>(target, 'task.list', { projectId })
      .then((fetchedTasks) => {
        if (typeof setTasks === 'function') {
          // Since our setTasks currently replaces all tasks in the store (as per Option B flat approach),
          // We only set the tasks for the current project for now. Real world we'd append or use tasksByProject
          setTasks(fetchedTasks)
        }
      })
      .catch(() => {
        /* silently fail */
      })
      .finally(() => setIsLoading(false))
  }, [projectId, setTasks])

  // Filter + search
  const filteredTasks = useMemo(() => {
    return allTasks.filter((task) => {
      if (filterStatus !== 'all' && task.status !== filterStatus) {
        return false
      }
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        return task.title.toLowerCase().includes(q) || task.id.toLowerCase().includes(q)
      }
      return true
    })
  }, [allTasks, filterStatus, searchQuery])

  const toggleExpanded = useCallback((id: string) => {
    setExpandedNodes((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }, [])

  return {
    filteredTasks,
    expandedNodes,
    toggleExpanded,
    setActiveTask: setActiveTask ?? (() => {}),
    filterStatus,
    setFilterStatus,
    searchQuery,
    setSearchQuery,
    isLoading,
    dagView: null // future: DAG graph data
  }
}
