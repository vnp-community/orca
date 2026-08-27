import { useMemo } from 'react'
import { useShallow } from 'zustand/react/shallow'
import { useAppStore } from '@/store'
import { findTaskPageLinearIssue } from '@/components/task-page-cache-selectors'
import { isLinearIssueSearchActive } from '@/components/task-page-linear-issue-request'
import { useTaskPageLinearActiveViewPagination } from '@/components/use-task-page-linear-active-view-pagination'
import { LINEAR_ISSUE_LIST_MAX } from '../../../shared/linear-issue-read-limits'
import type {
  LinearCollectionResult,
  LinearCustomViewSummary,
  LinearIssue,
  LinearProjectSummary,
  LinearTeam
} from '../../../shared/types'
import {
  buildLinearTeamUrl,
  getLinearOrganizationUrlKeyFromIssueUrl
} from '../../../shared/linear-links'
import type { LinearMode } from '@/components/task-page-localized-options'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) — the "which Linear
// issue source is active" view-model: picks between the plain issues list,
// the selected project's issues, or the selected custom view's issues, and
// exposes one pagination surface across all three. Reads across the issues
// (B), projects (C), and custom-views (D) slices, so it lives in the
// browse-state orchestrator rather than any one of them.
type UseTaskPageLinearActiveViewStateParams = {
  selectedLinearProject: LinearProjectSummary | null
  linearProjectTab: string
  linearProjectIssuesResult: LinearCollectionResult<LinearIssue>
  setLinearProjectIssuesResult: (
    updater: (result: LinearCollectionResult<LinearIssue>) => LinearCollectionResult<LinearIssue>
  ) => void
  linearProjectIssuesLoading: boolean
  linearProjectIssuesError: string | null
  linearProjectIssueLimit: number
  setLinearProjectIssueLimit: (updater: (limit: number) => number) => void
  linearProjectIssuePage: number
  setLinearProjectIssuePage: (page: number) => void
  linearProjectIssueLoadingTargetPage: number | null
  setLinearProjectIssueLoadingTargetPage: (page: number | null) => void
  selectedLinearCustomView: LinearCustomViewSummary | null
  linearCustomViewIssuesResult: LinearCollectionResult<LinearIssue>
  setLinearCustomViewIssuesResult: (
    updater: (result: LinearCollectionResult<LinearIssue>) => LinearCollectionResult<LinearIssue>
  ) => void
  linearCustomViewContentsLoading: boolean
  linearCustomViewContentsError: string | null
  linearCustomViewIssueLimit: number
  setLinearCustomViewIssueLimit: (updater: (limit: number) => number) => void
  linearCustomViewIssuePage: number
  setLinearCustomViewIssuePage: (page: number) => void
  linearCustomViewIssueLoadingTargetPage: number | null
  setLinearCustomViewIssueLoadingTargetPage: (page: number | null) => void
  linearIssues: LinearIssue[]
  linearLoading: boolean
  linearError: string | null
  linearIssuesHasMore: boolean
  linearIssueLimit: number
  setLinearIssueLimit: (updater: (limit: number) => number) => void
  linearIssuePage: number
  setLinearIssuePage: (page: number) => void
  linearIssueLoadingTargetPage: number | null
  setLinearIssueLoadingTargetPage: (page: number | null) => void
  linearSearchInput: string
  appliedLinearSearch: string
  linearMode: LinearMode
}

export function useTaskPageLinearActiveViewState({
  selectedLinearProject,
  linearProjectTab,
  linearProjectIssuesResult,
  setLinearProjectIssuesResult,
  linearProjectIssuesLoading,
  linearProjectIssuesError,
  linearProjectIssueLimit,
  setLinearProjectIssueLimit,
  linearProjectIssuePage,
  setLinearProjectIssuePage,
  linearProjectIssueLoadingTargetPage,
  setLinearProjectIssueLoadingTargetPage,
  selectedLinearCustomView,
  linearCustomViewIssuesResult,
  setLinearCustomViewIssuesResult,
  linearCustomViewContentsLoading,
  linearCustomViewContentsError,
  linearCustomViewIssueLimit,
  setLinearCustomViewIssueLimit,
  linearCustomViewIssuePage,
  setLinearCustomViewIssuePage,
  linearCustomViewIssueLoadingTargetPage,
  setLinearCustomViewIssueLoadingTargetPage,
  linearIssues,
  linearLoading,
  linearError,
  linearIssuesHasMore,
  linearIssueLimit,
  setLinearIssueLimit,
  linearIssuePage,
  setLinearIssuePage,
  linearIssueLoadingTargetPage,
  setLinearIssueLoadingTargetPage,
  linearSearchInput,
  appliedLinearSearch,
  linearMode
}: UseTaskPageLinearActiveViewStateParams) {
  const linearStatus = useAppStore((s) => s.linearStatus)
  const linearCacheSnapshot = useAppStore(
    useShallow((s) => ({
      issueCache: s.linearIssueCache,
      searchCache: s.linearSearchCache,
      listCache: s.linearListCache
    }))
  )

  const {
    patchScopedLinearIssue,
    setActiveLinearIssuePage,
    setActiveLinearIssueLoadingTargetPage,
    ensureActiveLinearIssueLimit
  } = useTaskPageLinearActiveViewPagination({
    selectedLinearProject,
    linearProjectTab,
    setLinearProjectIssuesResult,
    setLinearProjectIssueLimit,
    setLinearProjectIssuePage,
    setLinearProjectIssueLoadingTargetPage,
    selectedLinearCustomView,
    setLinearCustomViewIssuesResult,
    setLinearCustomViewIssueLimit,
    setLinearCustomViewIssuePage,
    setLinearCustomViewIssueLoadingTargetPage,
    setLinearIssueLimit,
    setLinearIssuePage,
    setLinearIssueLoadingTargetPage
  })

  const activeLinearIssues =
    selectedLinearProject && linearProjectTab === 'issues'
      ? linearProjectIssuesResult.items
      : selectedLinearCustomView?.model === 'issue'
        ? linearCustomViewIssuesResult.items
        : linearIssues
  const activeLinearIssueLoading =
    selectedLinearProject && linearProjectTab === 'issues'
      ? linearProjectIssuesLoading
      : selectedLinearCustomView?.model === 'issue'
        ? linearCustomViewContentsLoading
        : linearLoading
  const activeLinearIssueError =
    linearStatus.credentialError ??
    (selectedLinearProject && linearProjectTab === 'issues'
      ? linearProjectIssuesError
      : selectedLinearCustomView?.model === 'issue'
        ? linearCustomViewContentsError
        : linearError)
  const activeLinearIssueCollectionErrors =
    selectedLinearProject && linearProjectTab === 'issues'
      ? linearProjectIssuesResult.errors
      : selectedLinearCustomView?.model === 'issue'
        ? linearCustomViewIssuesResult.errors
        : undefined
  const activeLinearIssueHasCollectionError = (activeLinearIssueCollectionErrors?.length ?? 0) > 0
  const activeLinearIssueContextLabel = selectedLinearProject
    ? `Project: ${selectedLinearProject.name}`
    : selectedLinearCustomView?.model === 'issue'
      ? `View: ${selectedLinearCustomView.name}`
      : null
  const canLoadMorePlainLinearIssues =
    !activeLinearIssueContextLabel &&
    appliedLinearSearch.trim().length === 0 &&
    linearIssuesHasMore &&
    linearIssueLimit < LINEAR_ISSUE_LIST_MAX
  const canLoadMoreLinearProjectIssues =
    selectedLinearProject !== null &&
    linearProjectTab === 'issues' &&
    Boolean(linearProjectIssuesResult.hasMore) &&
    linearProjectIssueLimit < LINEAR_ISSUE_LIST_MAX
  const canLoadMoreLinearCustomViewIssues =
    selectedLinearCustomView?.model === 'issue' &&
    Boolean(linearCustomViewIssuesResult.hasMore) &&
    linearCustomViewIssueLimit < LINEAR_ISSUE_LIST_MAX
  const activeLinearIssuePage =
    selectedLinearProject && linearProjectTab === 'issues'
      ? linearProjectIssuePage
      : selectedLinearCustomView?.model === 'issue'
        ? linearCustomViewIssuePage
        : linearIssuePage
  const activeLinearIssueLoadingTargetPage =
    selectedLinearProject && linearProjectTab === 'issues'
      ? linearProjectIssueLoadingTargetPage
      : selectedLinearCustomView?.model === 'issue'
        ? linearCustomViewIssueLoadingTargetPage
        : linearIssueLoadingTargetPage
  const activeLinearIssueCanLoadMore =
    selectedLinearProject && linearProjectTab === 'issues'
      ? canLoadMoreLinearProjectIssues
      : selectedLinearCustomView?.model === 'issue'
        ? canLoadMoreLinearCustomViewIssues
        : canLoadMorePlainLinearIssues
  const activeLinearIssueCanRequestMore =
    activeLinearIssueCanLoadMore && !activeLinearIssueHasCollectionError
  const activeLinearIssueLimit =
    selectedLinearProject && linearProjectTab === 'issues'
      ? linearProjectIssueLimit
      : selectedLinearCustomView?.model === 'issue'
        ? linearCustomViewIssueLimit
        : linearIssueLimit

  const displayedLinearIssues = useMemo(
    () =>
      activeLinearIssues.map(
        (issue) =>
          findTaskPageLinearIssue(
            linearCacheSnapshot.issueCache,
            linearCacheSnapshot.searchCache,
            linearCacheSnapshot.listCache,
            issue.id
          ) ?? issue
      ),
    [
      activeLinearIssues,
      linearCacheSnapshot.issueCache,
      linearCacheSnapshot.listCache,
      linearCacheSnapshot.searchCache
    ]
  )

  const linearIssueTeams = useMemo(() => {
    const seen = new Set<string>()
    const teams: LinearTeam[] = []
    for (const issue of displayedLinearIssues) {
      if (!issue.team.id || seen.has(issue.team.id)) {
        continue
      }
      seen.add(issue.team.id)
      teams.push({
        id: issue.team.id,
        workspaceId: issue.workspaceId,
        workspaceName: issue.workspaceName,
        name: issue.team.name,
        key: issue.team.key,
        url:
          buildLinearTeamUrl({
            organizationUrlKey: getLinearOrganizationUrlKeyFromIssueUrl(issue.url),
            teamKey: issue.team.key
          }) ?? undefined
      })
    }
    return teams.sort((a, b) => a.name.localeCompare(b.name))
  }, [displayedLinearIssues])

  const linearSearchActive = isLinearIssueSearchActive(linearSearchInput, appliedLinearSearch)
  const showLinearAttributeFilters =
    linearMode === 'issues' && !activeLinearIssueContextLabel && !linearSearchActive

  return {
    activeLinearIssues,
    activeLinearIssueLoading,
    activeLinearIssueError,
    activeLinearIssueHasCollectionError,
    activeLinearIssueContextLabel,
    activeLinearIssuePage,
    activeLinearIssueLoadingTargetPage,
    activeLinearIssueCanLoadMore,
    activeLinearIssueCanRequestMore,
    activeLinearIssueLimit,
    displayedLinearIssues,
    linearIssueTeams,
    linearSearchActive,
    showLinearAttributeFilters,
    setActiveLinearIssuePage,
    setActiveLinearIssueLoadingTargetPage,
    ensureActiveLinearIssueLimit,
    patchScopedLinearIssue
  }
}
