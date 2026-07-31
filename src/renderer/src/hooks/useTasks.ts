import { useState, useMemo, useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaTask } from '../types/task-types'

export function useTasks(projectId: string) {
  // Use Option B: flat filter since store doesn't have tasksByProject index yet
  const allTasks = useAppStore(s => ((s as any).tasks ?? []).filter((t: OrcaTask) => t.projectId === projectId)) as OrcaTask[]

  // Store actions
  const setTasks = useAppStore(s => (s as any).setTasks)
  const setActiveTask = useAppStore(s => (s as any).setActiveTask)

  // Local UI state
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set())
  const [filterStatus, setFilterStatus] = useState<'all' | string>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  // Fetch tasks when projectId changes
  useEffect(() => {
    if (!projectId) return
    setIsLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<OrcaTask[]>(target, 'tasks.list', { projectId })
      .then(tasks => {
        if (typeof setTasks === 'function') {
          // Since our setTasks currently replaces all tasks in the store (as per Option B flat approach),
          // We only set the tasks for the current project for now. Real world we'd append or use tasksByProject
          setTasks(tasks)
        }
      })
      .catch(() => { /* silently fail */ })
      .finally(() => setIsLoading(false))
  }, [projectId, setTasks])

  // Filter + search
  const filteredTasks = useMemo(() => {
    return allTasks.filter(task => {
      if (filterStatus !== 'all' && task.status !== filterStatus) return false
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        return (
          task.title.toLowerCase().includes(q) ||
          task.id.toLowerCase().includes(q)
        )
      }
      return true
    })
  }, [allTasks, filterStatus, searchQuery])

  const toggleExpanded = useCallback((id: string) => {
    setExpandedNodes(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
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
    dagView: null, // future: DAG graph data
  }
}
