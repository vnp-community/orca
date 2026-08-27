import { useRef, useState } from 'react'
import { useTaskPageLinearIssueSelectionState } from '@/components/use-task-page-linear-issue-selection-state'
import { useTaskPageLinearIssuesState } from '@/components/use-task-page-linear-issues-state'
import { useTaskPageLinearProjectsState } from '@/components/use-task-page-linear-projects-state'
import { useTaskPageLinearCustomViewsState } from '@/components/use-task-page-linear-custom-views-state'
import { useTaskPageLinearContextResume } from '@/components/use-task-page-linear-context-resume'
import { useTaskPageLinearActiveViewState } from '@/components/use-task-page-linear-active-view-state'
import { useTaskPageLinearNavigationActions } from '@/components/use-task-page-linear-navigation-actions'
import type { LinearIssue, TaskProvider } from '../../../shared/types'
import type { LinearMode } from '@/components/task-page-localized-options'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) — the Linear tab's
// browse-state orchestrator (51 useState + ~12 effects across the domain's
// largest remaining cluster). Composes the issue-selection, issues,
// projects, and custom-views slices, plus the cross-slice active-view and
// navigation-action derivations that read more than one of them.
// `linearMode` and `linearRefreshNonce` live here (not in the issues slice)
// because the projects and custom-views fetches gate on/force-refresh with
// them too. `handleLinearWorkspaceChange` and the team-filtered issue
// pipeline (pagination target-page effects, board drag&drop, grouped
// sections) are NOT here — see TaskPage.tsx's own note — they need
// TASK-BIGFILE-240's `linearTeamSelection`, which itself needs this hook's
// `linearIssueTeams`/`linearAttributeFilter` output, an unavoidable
// browse<->teams circular read that can only be resolved by computing them
// after both hooks have run.
type UseTaskPageLinearBrowseStateParams = {
  taskSource: TaskProvider
  taskResumeApplied: boolean
  linearConnected: boolean
  selectedLinearWorkspaceId: string | null
  linearTaskSourceContext: TaskSourceContext | null | undefined
  linearListInvalidationVersionForSource: number
  openLinearIssue: LinearIssue | null | undefined
  openLinearSourceContext: TaskSourceContext | null | undefined
}

export function useTaskPageLinearBrowseState({
  taskSource,
  taskResumeApplied,
  linearConnected,
  selectedLinearWorkspaceId,
  linearTaskSourceContext,
  linearListInvalidationVersionForSource,
  openLinearIssue,
  openLinearSourceContext
}: UseTaskPageLinearBrowseStateParams) {
  const [linearMode, setLinearMode] = useState<LinearMode>('issues')
  const [linearRefreshNonce, setLinearRefreshNonce] = useState(0)
  const [linearBoardDraggingIssueId, setLinearBoardDraggingIssueId] = useState<string | null>(null)
  const [linearBoardDragOverKey, setLinearBoardDragOverKey] = useState<string | null>(null)
  const [linearBoardUpdatingIssueIds, setLinearBoardUpdatingIssueIds] = useState<
    ReadonlySet<string>
  >(() => new Set())
  const linearContextResumeAttemptedRef = useRef(false)

  const issueSelection = useTaskPageLinearIssueSelectionState({
    openLinearIssue,
    openLinearSourceContext,
    linearTaskSourceContext
  })

  const customViews = useTaskPageLinearCustomViewsState({
    taskResumeApplied,
    taskSource,
    linearMode,
    linearConnected,
    selectedLinearWorkspaceId,
    linearTaskSourceContext,
    linearRefreshNonce
  })

  const projects = useTaskPageLinearProjectsState({
    taskResumeApplied,
    taskSource,
    linearMode,
    linearConnected,
    selectedLinearWorkspaceId,
    linearTaskSourceContext,
    linearRefreshNonce,
    setLinearProjectParentView: customViews.setLinearProjectParentView
  })

  const issues = useTaskPageLinearIssuesState({
    taskSource,
    taskResumeApplied,
    linearConnected,
    selectedLinearWorkspaceId,
    linearTaskSourceContext,
    linearListInvalidationVersionForSource,
    selectedLinearProjectId: projects.selectedLinearProject?.id,
    selectedLinearCustomViewId: customViews.selectedLinearCustomView?.id,
    linearMode,
    linearRefreshNonce
  })

  useTaskPageLinearContextResume({
    taskResumeApplied,
    taskSource,
    linearConnected,
    linearTaskSourceContext,
    linearContextResumeAttemptedRef,
    setSelectedLinearProject: projects.setSelectedLinearProject,
    setSelectedLinearProjectDetail: projects.setSelectedLinearProjectDetail,
    setLinearProjectParentView: customViews.setLinearProjectParentView,
    setLinearProjectsError: projects.setLinearProjectsError,
    setLinearMode,
    setLinearCustomViewsLoading: customViews.setLinearCustomViewsLoading,
    setLinearCustomViewsError: customViews.setLinearCustomViewsError,
    setSelectedLinearCustomView: customViews.setSelectedLinearCustomView
  })

  const activeView = useTaskPageLinearActiveViewState({
    selectedLinearProject: projects.selectedLinearProject,
    linearProjectTab: projects.linearProjectTab,
    linearProjectIssuesResult: projects.linearProjectIssuesResult,
    setLinearProjectIssuesResult: projects.setLinearProjectIssuesResult,
    linearProjectIssuesLoading: projects.linearProjectIssuesLoading,
    linearProjectIssuesError: projects.linearProjectIssuesError,
    linearProjectIssueLimit: projects.linearProjectIssueLimit,
    setLinearProjectIssueLimit: projects.setLinearProjectIssueLimit,
    linearProjectIssuePage: projects.linearProjectIssuePage,
    setLinearProjectIssuePage: projects.setLinearProjectIssuePage,
    linearProjectIssueLoadingTargetPage: projects.linearProjectIssueLoadingTargetPage,
    setLinearProjectIssueLoadingTargetPage: projects.setLinearProjectIssueLoadingTargetPage,
    selectedLinearCustomView: customViews.selectedLinearCustomView,
    linearCustomViewIssuesResult: customViews.linearCustomViewIssuesResult,
    setLinearCustomViewIssuesResult: customViews.setLinearCustomViewIssuesResult,
    linearCustomViewContentsLoading: customViews.linearCustomViewContentsLoading,
    linearCustomViewContentsError: customViews.linearCustomViewContentsError,
    linearCustomViewIssueLimit: customViews.linearCustomViewIssueLimit,
    setLinearCustomViewIssueLimit: customViews.setLinearCustomViewIssueLimit,
    linearCustomViewIssuePage: customViews.linearCustomViewIssuePage,
    setLinearCustomViewIssuePage: customViews.setLinearCustomViewIssuePage,
    linearCustomViewIssueLoadingTargetPage: customViews.linearCustomViewIssueLoadingTargetPage,
    setLinearCustomViewIssueLoadingTargetPage:
      customViews.setLinearCustomViewIssueLoadingTargetPage,
    linearIssues: issues.linearIssues,
    linearLoading: issues.linearLoading,
    linearError: issues.linearError,
    linearIssuesHasMore: issues.linearIssuesHasMore,
    linearIssueLimit: issues.linearIssueLimit,
    setLinearIssueLimit: issues.setLinearIssueLimit,
    linearIssuePage: issues.linearIssuePage,
    setLinearIssuePage: issues.setLinearIssuePage,
    linearIssueLoadingTargetPage: issues.linearIssueLoadingTargetPage,
    setLinearIssueLoadingTargetPage: issues.setLinearIssueLoadingTargetPage,
    linearSearchInput: issues.linearSearchInput,
    appliedLinearSearch: issues.appliedLinearSearch,
    linearMode
  })

  const navigationActions = useTaskPageLinearNavigationActions({
    setLinearMode,
    clearSelectedLinearIssue: issueSelection.clearSelectedLinearIssue,
    setSelectedLinearProject: projects.setSelectedLinearProject,
    setSelectedLinearProjectDetail: projects.setSelectedLinearProjectDetail,
    setSelectedLinearCustomView: customViews.setSelectedLinearCustomView,
    setLinearProjectParentView: customViews.setLinearProjectParentView,
    setLinearProjectTab: projects.setLinearProjectTab,
    setLinearProjectIssuesResult: projects.setLinearProjectIssuesResult,
    setLinearProjectIssueLimit: projects.setLinearProjectIssueLimit,
    setLinearProjectIssuePage: projects.setLinearProjectIssuePage,
    setLinearProjectIssueLoadingTargetPage: projects.setLinearProjectIssueLoadingTargetPage,
    setLinearCustomViewIssuesResult: customViews.setLinearCustomViewIssuesResult,
    setLinearCustomViewIssueLimit: customViews.setLinearCustomViewIssueLimit,
    setLinearCustomViewIssuePage: customViews.setLinearCustomViewIssuePage,
    setLinearCustomViewIssueLoadingTargetPage:
      customViews.setLinearCustomViewIssueLoadingTargetPage,
    setLinearCustomViewProjectsResult: customViews.setLinearCustomViewProjectsResult
  })

  return {
    ...issueSelection,
    ...issues,
    ...projects,
    ...customViews,
    ...activeView,
    ...navigationActions,
    linearMode,
    setLinearMode,
    linearRefreshNonce,
    setLinearRefreshNonce,
    linearBoardDraggingIssueId,
    setLinearBoardDraggingIssueId,
    linearBoardDragOverKey,
    setLinearBoardDragOverKey,
    linearBoardUpdatingIssueIds,
    setLinearBoardUpdatingIssueIds,
    linearContextResumeAttemptedRef
  }
}
