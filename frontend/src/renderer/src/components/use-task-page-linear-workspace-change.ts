import { useCallback } from 'react'
import type { MutableRefObject } from 'react'
import { toast } from 'sonner'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import type {
  LinearCollectionResult,
  LinearCustomViewSummary,
  LinearIssue,
  LinearProjectDetail,
  LinearProjectSummary,
  LinearWorkspaceSelection
} from '../../../shared/types'
import type { LinearMode } from '@/components/task-page-localized-options'
import type { LinearProjectTab } from '@/components/task-page-types'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241), separate from
// use-task-page-linear-navigation-actions.ts — this handler also needs the
// Linear teams slice's (TASK-BIGFILE-240, already extracted)
// `setLinearTeamRefreshNonce` to refresh the team list post-switch. 240's
// hook call itself needs this file's caller's `linearIssueTeams` /
// `linearAttributeFilter` (from the browse-state orchestrator) as inputs, so
// this handler can only be built at the TaskPage.tsx call site, after both
// hooks have run.
type UseTaskPageLinearWorkspaceChangeParams = {
  linearMode: LinearMode
  clearSelectedLinearIssue: () => void
  setSelectedLinearProject: (value: LinearProjectSummary | null) => void
  setSelectedLinearProjectDetail: (value: LinearProjectDetail | null) => void
  setSelectedLinearCustomView: (value: LinearCustomViewSummary | null) => void
  setLinearProjectParentView: (value: LinearCustomViewSummary | null) => void
  setLinearProjectTab: (value: LinearProjectTab) => void
  setLinearProjectsResult: (value: LinearCollectionResult<LinearProjectSummary>) => void
  setLinearProjectsError: (value: string | null) => void
  setLinearProjectIssuesResult: (value: LinearCollectionResult<LinearIssue>) => void
  setLinearProjectDetailError: (value: string | null) => void
  setLinearCustomViewsResult: (value: LinearCollectionResult<LinearCustomViewSummary>) => void
  setLinearCustomViewsError: (value: string | null) => void
  setLinearCustomViewIssuesResult: (value: LinearCollectionResult<LinearIssue>) => void
  setLinearCustomViewProjectsResult: (value: LinearCollectionResult<LinearProjectSummary>) => void
  setLinearCustomViewContentsError: (value: string | null) => void
  setLinearIssues: (value: LinearIssue[]) => void
  setLinearError: (value: string | null) => void
  setLinearLoading: (value: boolean) => void
  linearContextResumeAttemptedRef: MutableRefObject<boolean>
  setLinearTeamRefreshNonce: (updater: (nonce: number) => number) => void
}

export function useTaskPageLinearWorkspaceChange({
  linearMode,
  clearSelectedLinearIssue,
  setSelectedLinearProject,
  setSelectedLinearProjectDetail,
  setSelectedLinearCustomView,
  setLinearProjectParentView,
  setLinearProjectTab,
  setLinearProjectsResult,
  setLinearProjectsError,
  setLinearProjectIssuesResult,
  setLinearProjectDetailError,
  setLinearCustomViewsResult,
  setLinearCustomViewsError,
  setLinearCustomViewIssuesResult,
  setLinearCustomViewProjectsResult,
  setLinearCustomViewContentsError,
  setLinearIssues,
  setLinearError,
  setLinearLoading,
  linearContextResumeAttemptedRef,
  setLinearTeamRefreshNonce
}: UseTaskPageLinearWorkspaceChangeParams) {
  const setTaskResumeState = useAppStore((s) => s.setTaskResumeState)
  const selectLinearWorkspace = useAppStore((s) => s.selectLinearWorkspace)

  const handleLinearWorkspaceChange = useCallback(
    (workspaceId: LinearWorkspaceSelection): void => {
      clearSelectedLinearIssue()
      setSelectedLinearProject(null)
      setSelectedLinearProjectDetail(null)
      setSelectedLinearCustomView(null)
      setLinearProjectParentView(null)
      setLinearProjectTab('overview')
      setLinearProjectsResult({ items: [] })
      setLinearCustomViewsResult({ items: [] })
      setLinearProjectIssuesResult({ items: [] })
      setLinearCustomViewIssuesResult({ items: [] })
      setLinearCustomViewProjectsResult({ items: [] })
      setLinearProjectDetailError(null)
      setLinearProjectsError(null)
      setLinearCustomViewsError(null)
      setLinearCustomViewContentsError(null)
      setTaskResumeState({
        linearMode,
        linearContext: undefined
      })
      linearContextResumeAttemptedRef.current = false
      setLinearIssues([])
      setLinearError(null)
      setLinearLoading(true)
      void selectLinearWorkspace(workspaceId)
        .then(() => {
          setLinearTeamRefreshNonce((n) => n + 1)
        })
        .catch(() => {
          setLinearLoading(false)
          toast.error(
            translate('auto.components.TaskPage.d0d570b306', 'Failed to switch Linear workspace.')
          )
        })
    },
    [
      clearSelectedLinearIssue,
      linearContextResumeAttemptedRef,
      linearMode,
      selectLinearWorkspace,
      setLinearCustomViewContentsError,
      setLinearCustomViewIssuesResult,
      setLinearCustomViewProjectsResult,
      setLinearCustomViewsError,
      setLinearCustomViewsResult,
      setLinearError,
      setLinearIssues,
      setLinearLoading,
      setLinearProjectDetailError,
      setLinearProjectIssuesResult,
      setLinearProjectParentView,
      setLinearProjectTab,
      setLinearProjectsError,
      setLinearProjectsResult,
      setLinearTeamRefreshNonce,
      setSelectedLinearCustomView,
      setSelectedLinearProject,
      setSelectedLinearProjectDetail,
      setTaskResumeState
    ]
  )

  return { handleLinearWorkspaceChange }
}
