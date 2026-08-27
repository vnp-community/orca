import { useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import {
  LINEAR_CUSTOM_VIEW_MODELS,
  mergeLinearCollectionResults
} from '@/components/task-page-linear-collection-helpers'
import { clampLinearIssueListLimit } from '../../../shared/linear-issue-read-limits'
import type {
  LinearCollectionResult,
  LinearCustomViewSummary,
  LinearIssue,
  LinearProjectSummary,
  TaskProvider
} from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'
import type { LinearMode } from '@/components/task-page-localized-options'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) — the Linear "custom
// views" sub-view slice: the saved-view list and the selected view's
// contents (issues or projects, per the view's model).
const LINEAR_ITEM_LIMIT = 36

type UseTaskPageLinearCustomViewsStateParams = {
  taskResumeApplied: boolean
  taskSource: TaskProvider
  linearMode: LinearMode
  linearConnected: boolean
  selectedLinearWorkspaceId: string | null
  linearTaskSourceContext: TaskSourceContext | null | undefined
  linearRefreshNonce: number
}

export function useTaskPageLinearCustomViewsState({
  taskResumeApplied,
  taskSource,
  linearMode,
  linearConnected,
  selectedLinearWorkspaceId,
  linearTaskSourceContext,
  linearRefreshNonce
}: UseTaskPageLinearCustomViewsStateParams) {
  const getCachedLinearCustomViews = useAppStore((s) => s.getCachedLinearCustomViews)
  const listLinearCustomViews = useAppStore((s) => s.listLinearCustomViews)
  const listLinearCustomViewIssues = useAppStore((s) => s.listLinearCustomViewIssues)
  const listLinearCustomViewProjects = useAppStore((s) => s.listLinearCustomViewProjects)

  const [linearCustomViewsResult, setLinearCustomViewsResult] = useState<
    LinearCollectionResult<LinearCustomViewSummary>
  >({ items: [] })
  const [linearCustomViewsLoading, setLinearCustomViewsLoading] = useState(false)
  const [linearCustomViewsError, setLinearCustomViewsError] = useState<string | null>(null)
  const [selectedLinearCustomView, setSelectedLinearCustomView] =
    useState<LinearCustomViewSummary | null>(null)
  const [linearProjectParentView, setLinearProjectParentView] =
    useState<LinearCustomViewSummary | null>(null)
  const [linearCustomViewIssuesResult, setLinearCustomViewIssuesResult] = useState<
    LinearCollectionResult<LinearIssue>
  >({ items: [] })
  const [linearCustomViewIssueLimit, setLinearCustomViewIssueLimit] = useState(LINEAR_ITEM_LIMIT)
  const [linearCustomViewIssuePage, setLinearCustomViewIssuePage] = useState(0)
  const [linearCustomViewIssueLoadingTargetPage, setLinearCustomViewIssueLoadingTargetPage] =
    useState<number | null>(null)
  const [linearCustomViewProjectsResult, setLinearCustomViewProjectsResult] = useState<
    LinearCollectionResult<LinearProjectSummary>
  >({ items: [] })
  const [linearCustomViewContentsLoading, setLinearCustomViewContentsLoading] = useState(false)
  const [linearCustomViewContentsError, setLinearCustomViewContentsError] = useState<string | null>(
    null
  )

  useEffect(() => {
    if (!taskResumeApplied || taskSource !== 'linear' || linearMode !== 'views') {
      return
    }
    if (!linearConnected || selectedLinearCustomView) {
      return
    }
    let cancelled = false
    const cachedResults = LINEAR_CUSTOM_VIEW_MODELS.map((model) =>
      getCachedLinearCustomViews(model, LINEAR_ITEM_LIMIT, undefined, {
        sourceContext: linearTaskSourceContext
      })
    )
    const allCached = cachedResults.every(
      (result): result is LinearCollectionResult<LinearCustomViewSummary> => result !== null
    )
    if (allCached) {
      setLinearCustomViewsResult(mergeLinearCollectionResults(cachedResults))
    }
    const force = linearRefreshNonce > 0
    setLinearCustomViewsLoading(force || !allCached)
    setLinearCustomViewsError(null)
    // Why: the Views tab already has a Model column, so listing both view
    // models avoids a second, redundant Issues/Projects switch.
    void Promise.all(
      LINEAR_CUSTOM_VIEW_MODELS.map((model) =>
        listLinearCustomViews(model, LINEAR_ITEM_LIMIT, undefined, {
          force,
          sourceContext: linearTaskSourceContext
        })
      )
    )
      .then((result) => {
        if (!cancelled) {
          setLinearCustomViewsResult(mergeLinearCollectionResults(result))
          setLinearCustomViewsLoading(false)
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setLinearCustomViewsError(
            error instanceof Error ? error.message : 'Failed to load views.'
          )
          setLinearCustomViewsLoading(false)
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
    selectedLinearCustomView,
    linearRefreshNonce,
    getCachedLinearCustomViews,
    listLinearCustomViews,
    linearTaskSourceContext
  ])

  useEffect(() => {
    if (!selectedLinearCustomView?.workspaceId) {
      setLinearCustomViewIssuesResult({ items: [] })
      setLinearCustomViewProjectsResult({ items: [] })
      return
    }
    let cancelled = false
    setLinearCustomViewContentsLoading(true)
    setLinearCustomViewContentsError(null)
    const issueLimit = clampLinearIssueListLimit(linearCustomViewIssueLimit)
    const request =
      selectedLinearCustomView.model === 'issue'
        ? listLinearCustomViewIssues(
            selectedLinearCustomView.id,
            selectedLinearCustomView.workspaceId,
            issueLimit,
            { force: linearRefreshNonce > 0, sourceContext: linearTaskSourceContext }
          )
        : listLinearCustomViewProjects(
            selectedLinearCustomView.id,
            selectedLinearCustomView.workspaceId,
            LINEAR_ITEM_LIMIT,
            { force: linearRefreshNonce > 0, sourceContext: linearTaskSourceContext }
          )
    void request
      .then((result) => {
        if (cancelled) {
          return
        }
        if (selectedLinearCustomView.model === 'issue') {
          setLinearCustomViewIssuesResult(result as LinearCollectionResult<LinearIssue>)
        } else {
          setLinearCustomViewProjectsResult(result as LinearCollectionResult<LinearProjectSummary>)
        }
        setLinearCustomViewContentsLoading(false)
      })
      .catch((error) => {
        if (!cancelled) {
          setLinearCustomViewContentsError(
            error instanceof Error ? error.message : 'Failed to load view contents.'
          )
          setLinearCustomViewContentsLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [
    linearRefreshNonce,
    linearCustomViewIssueLimit,
    listLinearCustomViewIssues,
    listLinearCustomViewProjects,
    linearTaskSourceContext,
    selectedLinearCustomView
  ])

  return {
    linearCustomViewsResult,
    setLinearCustomViewsResult,
    linearCustomViewsLoading,
    setLinearCustomViewsLoading,
    linearCustomViewsError,
    setLinearCustomViewsError,
    selectedLinearCustomView,
    setSelectedLinearCustomView,
    linearProjectParentView,
    setLinearProjectParentView,
    linearCustomViewIssuesResult,
    setLinearCustomViewIssuesResult,
    linearCustomViewIssueLimit,
    setLinearCustomViewIssueLimit,
    linearCustomViewIssuePage,
    setLinearCustomViewIssuePage,
    linearCustomViewIssueLoadingTargetPage,
    setLinearCustomViewIssueLoadingTargetPage,
    linearCustomViewProjectsResult,
    setLinearCustomViewProjectsResult,
    linearCustomViewContentsLoading,
    linearCustomViewContentsError,
    setLinearCustomViewContentsError
  }
}
