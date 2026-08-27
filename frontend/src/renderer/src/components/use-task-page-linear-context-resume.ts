import { useEffect } from 'react'
import type { MutableRefObject } from 'react'
import { useAppStore } from '@/store'
import type {
  LinearCustomViewSummary,
  LinearProjectDetail,
  LinearProjectSummary,
  TaskProvider
} from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) — restoring a saved
// Linear project/view context (worktree nav history, page reload) once per
// mount. Spans the projects (C) and custom-views (D) slices, so it lives in
// the browse-state orchestrator rather than either sub-hook.
type UseTaskPageLinearContextResumeParams = {
  taskResumeApplied: boolean
  taskSource: TaskProvider
  linearConnected: boolean
  linearTaskSourceContext: TaskSourceContext | null | undefined
  linearContextResumeAttemptedRef: MutableRefObject<boolean>
  setSelectedLinearProject: (value: LinearProjectSummary | null) => void
  setSelectedLinearProjectDetail: (value: LinearProjectDetail | null) => void
  setLinearProjectParentView: (value: LinearCustomViewSummary | null) => void
  setLinearProjectsError: (value: string | null) => void
  setLinearMode: (value: 'issues' | 'projects' | 'views') => void
  setLinearCustomViewsLoading: (value: boolean) => void
  setLinearCustomViewsError: (value: string | null) => void
  setSelectedLinearCustomView: (value: LinearCustomViewSummary | null) => void
}

export function useTaskPageLinearContextResume({
  taskResumeApplied,
  taskSource,
  linearConnected,
  linearTaskSourceContext,
  linearContextResumeAttemptedRef,
  setSelectedLinearProject,
  setSelectedLinearProjectDetail,
  setLinearProjectParentView,
  setLinearProjectsError,
  setLinearMode,
  setLinearCustomViewsLoading,
  setLinearCustomViewsError,
  setSelectedLinearCustomView
}: UseTaskPageLinearContextResumeParams): void {
  const taskResumeState = useAppStore((s) => s.taskResumeState)
  const setTaskResumeState = useAppStore((s) => s.setTaskResumeState)
  const fetchLinearProject = useAppStore((s) => s.fetchLinearProject)
  const fetchLinearCustomView = useAppStore((s) => s.fetchLinearCustomView)
  const listLinearCustomViews = useAppStore((s) => s.listLinearCustomViews)

  useEffect(() => {
    const context = taskResumeState?.linearContext
    if (
      linearContextResumeAttemptedRef.current ||
      !taskResumeApplied ||
      taskSource !== 'linear' ||
      !linearConnected ||
      !context
    ) {
      return
    }
    linearContextResumeAttemptedRef.current = true
    let cancelled = false

    if (context.kind === 'project') {
      void fetchLinearProject(context.id, context.workspaceId, {
        force: true,
        sourceContext: linearTaskSourceContext
      })
        .then((project) => {
          if (cancelled) {
            return
          }
          if (!project) {
            setSelectedLinearProject(null)
            setSelectedLinearProjectDetail(null)
            setLinearProjectParentView(null)
            setLinearProjectsError('Saved Linear project was not found.')
            setTaskResumeState({ linearContext: undefined })
            return
          }
          setSelectedLinearProject(project)
          setSelectedLinearProjectDetail(project)
          setLinearMode('projects')
        })
        .catch(() => {
          if (!cancelled) {
            setSelectedLinearProject(null)
            setSelectedLinearProjectDetail(null)
            setLinearProjectParentView(null)
            setLinearProjectsError('Failed to restore saved Linear project.')
            setTaskResumeState({ linearContext: undefined })
          }
        })
      return () => {
        cancelled = true
      }
    }

    if (context.kind === 'view' && context.model) {
      setLinearMode('views')
      setLinearCustomViewsLoading(true)
      setLinearCustomViewsError(null)
      void fetchLinearCustomView(context.id, context.workspaceId, context.model, {
        force: true,
        sourceContext: linearTaskSourceContext
      })
        .then((restoredView) => {
          if (cancelled) {
            return
          }
          setLinearCustomViewsLoading(false)
          if (!restoredView) {
            setSelectedLinearCustomView(null)
            setLinearCustomViewsError('Saved Linear view was not found.')
            setTaskResumeState({ linearContext: undefined })
            return
          }
          setSelectedLinearCustomView(restoredView)
        })
        .catch(() => {
          if (!cancelled) {
            setSelectedLinearCustomView(null)
            setLinearCustomViewsLoading(false)
            setLinearCustomViewsError('Failed to restore saved Linear view.')
            setTaskResumeState({ linearContext: undefined })
          }
        })
      return () => {
        cancelled = true
      }
    }
    return undefined
  }, [
    fetchLinearCustomView,
    fetchLinearProject,
    listLinearCustomViews,
    linearConnected,
    linearContextResumeAttemptedRef,
    linearTaskSourceContext,
    setLinearCustomViewsError,
    setLinearCustomViewsLoading,
    setLinearMode,
    setLinearProjectParentView,
    setLinearProjectsError,
    setSelectedLinearCustomView,
    setSelectedLinearProject,
    setSelectedLinearProjectDetail,
    setTaskResumeState,
    taskResumeApplied,
    taskResumeState?.linearContext,
    taskSource
  ])
}
