import { useCallback } from 'react'
import { toast } from 'sonner'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import type {
  LinearCollectionResult,
  LinearCustomViewSummary,
  LinearIssue,
  LinearProjectDetail,
  LinearProjectSummary
} from '../../../shared/types'
import type { LinearMode } from '@/components/task-page-localized-options'
import type { LinearProjectTab } from '@/components/task-page-types'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) — Linear tab/context
// switch handlers. Each resets state across the issues (B), projects (C),
// and custom-views (D) slices at once, so they live in the browse-state
// orchestrator rather than any one slice. `handleLinearWorkspaceChange` is
// NOT here — it also needs the teams slice's (TASK-BIGFILE-240, already
// extracted) `setLinearTeamRefreshNonce`, which this hook's own caller
// doesn't have yet (240 is constructed from this hook's own output), so it's
// composed at the TaskPage.tsx call site instead (see
// use-task-page-linear-workspace-change.ts).
const LINEAR_ITEM_LIMIT = 36

type UseTaskPageLinearNavigationActionsParams = {
  setLinearMode: (mode: LinearMode) => void
  clearSelectedLinearIssue: () => void
  setSelectedLinearProject: (value: LinearProjectSummary | null) => void
  setSelectedLinearProjectDetail: (value: LinearProjectDetail | null) => void
  setSelectedLinearCustomView: (value: LinearCustomViewSummary | null) => void
  setLinearProjectParentView: (value: LinearCustomViewSummary | null) => void
  setLinearProjectTab: (value: LinearProjectTab) => void
  setLinearProjectIssuesResult: (value: LinearCollectionResult<LinearIssue>) => void
  setLinearProjectIssueLimit: (value: number) => void
  setLinearProjectIssuePage: (value: number) => void
  setLinearProjectIssueLoadingTargetPage: (value: number | null) => void
  setLinearCustomViewIssuesResult: (value: LinearCollectionResult<LinearIssue>) => void
  setLinearCustomViewIssueLimit: (value: number) => void
  setLinearCustomViewIssuePage: (value: number) => void
  setLinearCustomViewIssueLoadingTargetPage: (value: number | null) => void
  setLinearCustomViewProjectsResult: (value: LinearCollectionResult<LinearProjectSummary>) => void
}

export function useTaskPageLinearNavigationActions({
  setLinearMode,
  clearSelectedLinearIssue,
  setSelectedLinearProject,
  setSelectedLinearProjectDetail,
  setSelectedLinearCustomView,
  setLinearProjectParentView,
  setLinearProjectTab,
  setLinearProjectIssuesResult,
  setLinearProjectIssueLimit,
  setLinearProjectIssuePage,
  setLinearProjectIssueLoadingTargetPage,
  setLinearCustomViewIssuesResult,
  setLinearCustomViewIssueLimit,
  setLinearCustomViewIssuePage,
  setLinearCustomViewIssueLoadingTargetPage,
  setLinearCustomViewProjectsResult
}: UseTaskPageLinearNavigationActionsParams) {
  const setTaskResumeState = useAppStore((s) => s.setTaskResumeState)

  const selectLinearMode = useCallback(
    (mode: LinearMode) => {
      clearSelectedLinearIssue()
      setSelectedLinearProject(null)
      setSelectedLinearProjectDetail(null)
      setSelectedLinearCustomView(null)
      setLinearProjectParentView(null)
      setLinearProjectIssuesResult({ items: [] })
      setLinearProjectIssueLimit(LINEAR_ITEM_LIMIT)
      setLinearProjectIssuePage(0)
      setLinearProjectIssueLoadingTargetPage(null)
      setLinearCustomViewIssuesResult({ items: [] })
      setLinearCustomViewIssueLimit(LINEAR_ITEM_LIMIT)
      setLinearCustomViewIssuePage(0)
      setLinearCustomViewIssueLoadingTargetPage(null)
      setLinearCustomViewProjectsResult({ items: [] })
      setLinearMode(mode)
      setTaskResumeState({ linearMode: mode, linearContext: undefined })
    },
    [
      clearSelectedLinearIssue,
      setLinearCustomViewIssueLimit,
      setLinearCustomViewIssueLoadingTargetPage,
      setLinearCustomViewIssuePage,
      setLinearCustomViewIssuesResult,
      setLinearCustomViewProjectsResult,
      setLinearMode,
      setLinearProjectIssueLimit,
      setLinearProjectIssueLoadingTargetPage,
      setLinearProjectIssuePage,
      setLinearProjectIssuesResult,
      setLinearProjectParentView,
      setSelectedLinearCustomView,
      setSelectedLinearProject,
      setSelectedLinearProjectDetail,
      setTaskResumeState
    ]
  )

  const openLinearProjectContext = useCallback(
    (project: LinearProjectSummary, options?: { parentView?: LinearCustomViewSummary | null }) => {
      if (!project.workspaceId) {
        toast.error(
          translate(
            'auto.components.TaskPage.cba2a2b7fb',
            'Linear project is missing workspace context.'
          )
        )
        return
      }
      const parentView = options?.parentView ?? null
      clearSelectedLinearIssue()
      setLinearProjectParentView(parentView)
      if (parentView) {
        setSelectedLinearCustomView(parentView)
      } else {
        setSelectedLinearCustomView(null)
        setLinearCustomViewProjectsResult({ items: [] })
      }
      setLinearProjectIssuesResult({ items: [] })
      setLinearProjectIssueLimit(LINEAR_ITEM_LIMIT)
      setLinearProjectIssuePage(0)
      setLinearProjectIssueLoadingTargetPage(null)
      setLinearCustomViewIssuesResult({ items: [] })
      setLinearCustomViewIssueLimit(LINEAR_ITEM_LIMIT)
      setLinearCustomViewIssuePage(0)
      setLinearCustomViewIssueLoadingTargetPage(null)
      setSelectedLinearProject(project)
      setLinearProjectTab('overview')
      setLinearMode('projects')
      setTaskResumeState({
        linearMode: 'projects',
        linearContext: { kind: 'project', id: project.id, workspaceId: project.workspaceId }
      })
    },
    [
      clearSelectedLinearIssue,
      setLinearCustomViewIssueLimit,
      setLinearCustomViewIssueLoadingTargetPage,
      setLinearCustomViewIssuePage,
      setLinearCustomViewIssuesResult,
      setLinearCustomViewProjectsResult,
      setLinearMode,
      setLinearProjectIssueLimit,
      setLinearProjectIssueLoadingTargetPage,
      setLinearProjectIssuePage,
      setLinearProjectIssuesResult,
      setLinearProjectParentView,
      setLinearProjectTab,
      setSelectedLinearCustomView,
      setSelectedLinearProject,
      setTaskResumeState
    ]
  )

  const openLinearCustomViewContext = useCallback(
    (view: LinearCustomViewSummary) => {
      if (!view.workspaceId) {
        toast.error(
          translate(
            'auto.components.TaskPage.669e419d65',
            'Linear view is missing workspace context.'
          )
        )
        return
      }
      clearSelectedLinearIssue()
      setSelectedLinearProject(null)
      setSelectedLinearProjectDetail(null)
      setLinearProjectParentView(null)
      setLinearProjectIssuesResult({ items: [] })
      setLinearProjectIssueLimit(LINEAR_ITEM_LIMIT)
      setLinearProjectIssuePage(0)
      setLinearProjectIssueLoadingTargetPage(null)
      setLinearCustomViewIssuesResult({ items: [] })
      setLinearCustomViewIssueLimit(LINEAR_ITEM_LIMIT)
      setLinearCustomViewIssuePage(0)
      setLinearCustomViewIssueLoadingTargetPage(null)
      setLinearCustomViewProjectsResult({ items: [] })
      setSelectedLinearCustomView(view)
      setLinearMode('views')
      setTaskResumeState({
        linearMode: 'views',
        linearContext: {
          kind: 'view',
          id: view.id,
          workspaceId: view.workspaceId,
          model: view.model
        }
      })
    },
    [
      clearSelectedLinearIssue,
      setLinearCustomViewIssueLimit,
      setLinearCustomViewIssueLoadingTargetPage,
      setLinearCustomViewIssuePage,
      setLinearCustomViewIssuesResult,
      setLinearCustomViewProjectsResult,
      setLinearMode,
      setLinearProjectIssueLimit,
      setLinearProjectIssueLoadingTargetPage,
      setLinearProjectIssuePage,
      setLinearProjectIssuesResult,
      setLinearProjectParentView,
      setSelectedLinearCustomView,
      setSelectedLinearProject,
      setSelectedLinearProjectDetail,
      setTaskResumeState
    ]
  )

  return {
    selectLinearMode,
    openLinearProjectContext,
    openLinearCustomViewContext
  }
}
