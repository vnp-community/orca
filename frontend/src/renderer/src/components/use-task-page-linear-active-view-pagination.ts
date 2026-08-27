import { useCallback } from 'react'
import {
  clampLinearIssueListLimit,
  LINEAR_ISSUE_LIST_MAX
} from '../../../shared/linear-issue-read-limits'
import type {
  LinearCollectionResult,
  LinearCustomViewSummary,
  LinearIssue,
  LinearProjectSummary
} from '../../../shared/types'

// Why: split out of use-task-page-linear-active-view-state.ts
// (TASK-BIGFILE-241) to stay under the 300-line file budget — the mode-
// routing pagination callbacks (which of issues/project-issues/custom-view-
// issues owns the active page) and the scoped-issue cache patch used by
// board drag&drop.
type UseTaskPageLinearActiveViewPaginationParams = {
  selectedLinearProject: LinearProjectSummary | null
  linearProjectTab: string
  setLinearProjectIssuesResult: (
    updater: (result: LinearCollectionResult<LinearIssue>) => LinearCollectionResult<LinearIssue>
  ) => void
  setLinearProjectIssueLimit: (updater: (limit: number) => number) => void
  setLinearProjectIssuePage: (page: number) => void
  setLinearProjectIssueLoadingTargetPage: (page: number | null) => void
  selectedLinearCustomView: LinearCustomViewSummary | null
  setLinearCustomViewIssuesResult: (
    updater: (result: LinearCollectionResult<LinearIssue>) => LinearCollectionResult<LinearIssue>
  ) => void
  setLinearCustomViewIssueLimit: (updater: (limit: number) => number) => void
  setLinearCustomViewIssuePage: (page: number) => void
  setLinearCustomViewIssueLoadingTargetPage: (page: number | null) => void
  setLinearIssueLimit: (updater: (limit: number) => number) => void
  setLinearIssuePage: (page: number) => void
  setLinearIssueLoadingTargetPage: (page: number | null) => void
}

export function useTaskPageLinearActiveViewPagination({
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
}: UseTaskPageLinearActiveViewPaginationParams) {
  const patchScopedLinearIssue = useCallback(
    (issueId: string, patch: Partial<LinearIssue>) => {
      const patchResult = (result: LinearCollectionResult<LinearIssue>) => ({
        ...result,
        items: result.items.map((item) => (item.id === issueId ? { ...item, ...patch } : item))
      })
      setLinearProjectIssuesResult(patchResult)
      setLinearCustomViewIssuesResult(patchResult)
    },
    [setLinearCustomViewIssuesResult, setLinearProjectIssuesResult]
  )

  const setActiveLinearIssuePage = useCallback(
    (page: number) => {
      if (selectedLinearProject && linearProjectTab === 'issues') {
        setLinearProjectIssuePage(page)
      } else if (selectedLinearCustomView?.model === 'issue') {
        setLinearCustomViewIssuePage(page)
      } else {
        setLinearIssuePage(page)
      }
    },
    [
      linearProjectTab,
      selectedLinearCustomView?.model,
      selectedLinearProject,
      setLinearCustomViewIssuePage,
      setLinearIssuePage,
      setLinearProjectIssuePage
    ]
  )

  const setActiveLinearIssueLoadingTargetPage = useCallback(
    (page: number | null) => {
      if (selectedLinearProject && linearProjectTab === 'issues') {
        setLinearProjectIssueLoadingTargetPage(page)
      } else if (selectedLinearCustomView?.model === 'issue') {
        setLinearCustomViewIssueLoadingTargetPage(page)
      } else {
        setLinearIssueLoadingTargetPage(page)
      }
    },
    [
      linearProjectTab,
      selectedLinearCustomView?.model,
      selectedLinearProject,
      setLinearCustomViewIssueLoadingTargetPage,
      setLinearIssueLoadingTargetPage,
      setLinearProjectIssueLoadingTargetPage
    ]
  )

  const ensureActiveLinearIssueLimit = useCallback(
    (targetLimit: number) => {
      const nextLimit = Math.min(clampLinearIssueListLimit(targetLimit), LINEAR_ISSUE_LIST_MAX)
      if (selectedLinearProject && linearProjectTab === 'issues') {
        setLinearProjectIssueLimit((limit) => Math.max(limit, nextLimit))
      } else if (selectedLinearCustomView?.model === 'issue') {
        setLinearCustomViewIssueLimit((limit) => Math.max(limit, nextLimit))
      } else {
        setLinearIssueLimit((limit) => Math.max(limit, nextLimit))
      }
    },
    [
      linearProjectTab,
      selectedLinearCustomView?.model,
      selectedLinearProject,
      setLinearCustomViewIssueLimit,
      setLinearIssueLimit,
      setLinearProjectIssueLimit
    ]
  )

  return {
    patchScopedLinearIssue,
    setActiveLinearIssuePage,
    setActiveLinearIssueLoadingTargetPage,
    ensureActiveLinearIssueLimit
  }
}
