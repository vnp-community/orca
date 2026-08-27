import { useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import { clampLinearIssueListLimit } from '../../../shared/linear-issue-read-limits'
import type {
  LinearCollectionResult,
  LinearCustomViewSummary,
  LinearIssue,
  LinearProjectDetail,
  LinearProjectSummary,
  TaskProvider
} from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'
import type { LinearMode } from '@/components/task-page-localized-options'
import type { LinearProjectTab } from '@/components/task-page-types'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) — the Linear "projects"
// sub-view slice: project search/list, the selected project's detail, and
// its scoped issues page. `setLinearProjectParentView` is a write-only param
// into the custom-views slice (TASK-BIGFILE-241's own D file) because a
// project opened from a saved view must keep that view as its back target —
// this hook doesn't otherwise own or read custom-view state.
const TASK_SEARCH_DEBOUNCE_MS = 300
const LINEAR_ITEM_LIMIT = 36

type UseTaskPageLinearProjectsStateParams = {
  taskResumeApplied: boolean
  taskSource: TaskProvider
  linearMode: LinearMode
  linearConnected: boolean
  selectedLinearWorkspaceId: string | null
  linearTaskSourceContext: TaskSourceContext | null | undefined
  linearRefreshNonce: number
  setLinearProjectParentView: (value: LinearCustomViewSummary | null) => void
}

export function useTaskPageLinearProjectsState({
  taskResumeApplied,
  taskSource,
  linearMode,
  linearConnected,
  selectedLinearWorkspaceId,
  linearTaskSourceContext,
  linearRefreshNonce,
  setLinearProjectParentView
}: UseTaskPageLinearProjectsStateParams) {
  const getCachedLinearProjects = useAppStore((s) => s.getCachedLinearProjects)
  const listLinearProjectsFromStore = useAppStore((s) => s.listLinearProjects)
  const fetchLinearProject = useAppStore((s) => s.fetchLinearProject)
  const listLinearProjectIssues = useAppStore((s) => s.listLinearProjectIssues)
  const setTaskResumeState = useAppStore((s) => s.setTaskResumeState)

  const [linearProjectSearchInput, setLinearProjectSearchInput] = useState('')
  const [appliedLinearProjectSearch, setAppliedLinearProjectSearch] = useState('')
  const [linearProjectsResult, setLinearProjectsResult] = useState<
    LinearCollectionResult<LinearProjectSummary>
  >({ items: [] })
  const [linearProjectsLoading, setLinearProjectsLoading] = useState(false)
  const [linearProjectsError, setLinearProjectsError] = useState<string | null>(null)
  const [selectedLinearProject, setSelectedLinearProject] = useState<LinearProjectSummary | null>(
    null
  )
  const [selectedLinearProjectDetail, setSelectedLinearProjectDetail] =
    useState<LinearProjectDetail | null>(null)
  const [linearProjectDetailLoading, setLinearProjectDetailLoading] = useState(false)
  const [linearProjectDetailError, setLinearProjectDetailError] = useState<string | null>(null)
  const [linearProjectTab, setLinearProjectTab] = useState<LinearProjectTab>('overview')
  const [linearProjectIssuesResult, setLinearProjectIssuesResult] = useState<
    LinearCollectionResult<LinearIssue>
  >({ items: [] })
  const [linearProjectIssueLimit, setLinearProjectIssueLimit] = useState(LINEAR_ITEM_LIMIT)
  const [linearProjectIssuePage, setLinearProjectIssuePage] = useState(0)
  const [linearProjectIssueLoadingTargetPage, setLinearProjectIssueLoadingTargetPage] = useState<
    number | null
  >(null)
  const [linearProjectIssuesLoading, setLinearProjectIssuesLoading] = useState(false)
  const [linearProjectIssuesError, setLinearProjectIssuesError] = useState<string | null>(null)

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    const timeout = window.setTimeout(() => {
      setAppliedLinearProjectSearch(linearProjectSearchInput)
    }, TASK_SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timeout)
  }, [linearProjectSearchInput, taskResumeApplied])

  useEffect(() => {
    if (!taskResumeApplied || taskSource !== 'linear' || linearMode !== 'projects') {
      return
    }
    if (!linearConnected || selectedLinearProject) {
      return
    }
    let cancelled = false
    const query = appliedLinearProjectSearch.trim()
    const cached = getCachedLinearProjects(query || undefined, LINEAR_ITEM_LIMIT, undefined, {
      sourceContext: linearTaskSourceContext
    })
    if (cached) {
      setLinearProjectsResult(cached)
    }
    const force = linearRefreshNonce > 0
    setLinearProjectsLoading(force || cached === null)
    setLinearProjectsError(null)
    void listLinearProjectsFromStore(query || undefined, LINEAR_ITEM_LIMIT, undefined, {
      force,
      sourceContext: linearTaskSourceContext
    })
      .then((result) => {
        if (!cancelled) {
          setLinearProjectsResult(result)
          setLinearProjectsLoading(false)
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setLinearProjectsError(
            error instanceof Error ? error.message : 'Failed to load projects.'
          )
          setLinearProjectsLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    taskResumeApplied,
    taskSource,
    linearMode,
    linearConnected,
    selectedLinearWorkspaceId,
    selectedLinearProject,
    appliedLinearProjectSearch,
    linearRefreshNonce,
    getCachedLinearProjects,
    linearTaskSourceContext
  ])

  useEffect(() => {
    if (!selectedLinearProject?.workspaceId) {
      setSelectedLinearProjectDetail(null)
      return
    }
    let cancelled = false
    setLinearProjectDetailLoading(true)
    setLinearProjectDetailError(null)
    void fetchLinearProject(selectedLinearProject.id, selectedLinearProject.workspaceId, {
      force: linearRefreshNonce > 0,
      sourceContext: linearTaskSourceContext
    })
      .then((project) => {
        if (!cancelled) {
          setSelectedLinearProjectDetail(project)
          setLinearProjectDetailLoading(false)
          if (!project) {
            setSelectedLinearProject(null)
            setLinearProjectParentView(null)
            setLinearProjectDetailError(null)
            setLinearProjectsError('Project was not found.')
            setTaskResumeState({ linearContext: undefined })
          }
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setLinearProjectDetailError(
            error instanceof Error ? error.message : 'Failed to load project.'
          )
          setLinearProjectDetailLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [
    fetchLinearProject,
    linearRefreshNonce,
    selectedLinearProject,
    setLinearProjectParentView,
    setTaskResumeState,
    linearTaskSourceContext
  ])

  useEffect(() => {
    if (!selectedLinearProject?.workspaceId || linearProjectTab !== 'issues') {
      return
    }
    let cancelled = false
    setLinearProjectIssuesLoading(true)
    setLinearProjectIssuesError(null)
    const effectiveLimit = clampLinearIssueListLimit(linearProjectIssueLimit)
    void listLinearProjectIssues(
      selectedLinearProject.id,
      selectedLinearProject.workspaceId,
      effectiveLimit,
      { force: linearRefreshNonce > 0, sourceContext: linearTaskSourceContext }
    )
      .then((result) => {
        if (!cancelled) {
          setLinearProjectIssuesResult(result)
          setLinearProjectIssuesLoading(false)
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setLinearProjectIssuesError(
            error instanceof Error ? error.message : 'Failed to load project issues.'
          )
          setLinearProjectIssuesLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [
    linearProjectIssueLimit,
    linearProjectTab,
    linearRefreshNonce,
    listLinearProjectIssues,
    linearTaskSourceContext,
    selectedLinearProject
  ])

  return {
    linearProjectSearchInput,
    setLinearProjectSearchInput,
    appliedLinearProjectSearch,
    setAppliedLinearProjectSearch,
    linearProjectsResult,
    setLinearProjectsResult,
    linearProjectsLoading,
    linearProjectsError,
    setLinearProjectsError,
    selectedLinearProject,
    setSelectedLinearProject,
    selectedLinearProjectDetail,
    setSelectedLinearProjectDetail,
    linearProjectDetailLoading,
    linearProjectDetailError,
    setLinearProjectDetailError,
    linearProjectTab,
    setLinearProjectTab,
    linearProjectIssuesResult,
    setLinearProjectIssuesResult,
    linearProjectIssueLimit,
    setLinearProjectIssueLimit,
    linearProjectIssuePage,
    setLinearProjectIssuePage,
    linearProjectIssueLoadingTargetPage,
    setLinearProjectIssueLoadingTargetPage,
    linearProjectIssuesLoading,
    linearProjectIssuesError
  }
}
