import { useEffect, useMemo, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { reconcileLinearTeamSelection } from '@/components/task-page-linear-team-selection'
import { resolveLinearIssueAttributeFilterPrimaryTeam } from '@/components/linear-issue-attribute-filter-primary-team'
import { teamDerivedFacetsForPrimaryTeamChange } from '@/components/task-page-linear-issue-request'
import {
  emptyLinearIssueAttributeFilter,
  linearIssueAttributeFilterSignature,
  type LinearIssueAttributeFilter
} from '../../../shared/linear-issue-attribute-filter'
import type { LinearTeam, TaskProvider } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-240) — the Linear teams
// selector slice: fetched team list, team refresh nonce, and the selected
// team subset. `linearAttributeFilter` / `applyLinearAttributeFilter` are
// read(+write) params rather than owned state — they belong to the Linear
// browse domain (TASK-BIGFILE-241, not yet extracted when this hook was
// written); this hook only reconciles them when the primary team changes,
// it doesn't own them. Likewise `linearIssueTeams` (derived from displayed
// Linear issues) is a browse-domain read param.
type UseTaskPageLinearTeamsStateParams = {
  taskSource: TaskProvider
  taskResumeApplied: boolean
  linearConnected: boolean
  selectedLinearWorkspaceId: string | null
  linearTaskSourceContext: TaskSourceContext | null | undefined
  defaultLinearTeamSelection: readonly string[] | null | undefined
  linearIssueTeams: LinearTeam[]
  linearAttributeFilter: LinearIssueAttributeFilter
  applyLinearAttributeFilter: (next: LinearIssueAttributeFilter) => void
}

export function useTaskPageLinearTeamsState({
  taskSource,
  taskResumeApplied,
  linearConnected,
  selectedLinearWorkspaceId,
  linearTaskSourceContext,
  defaultLinearTeamSelection,
  linearIssueTeams,
  linearAttributeFilter,
  applyLinearAttributeFilter
}: UseTaskPageLinearTeamsStateParams) {
  const getCachedLinearTeams = useAppStore((s) => s.getCachedLinearTeams)
  const listLinearTeams = useAppStore((s) => s.listLinearTeams)

  // Why: fetch the full team list from the Linear API so the selector shows
  // all teams the user belongs to, not just teams with issues in the current
  // fetch window. Fetched once when the Linear tab is active and connected.
  const [availableTeams, setAvailableTeams] = useState<LinearTeam[]>([])
  const [linearTeamRefreshNonce, setLinearTeamRefreshNonce] = useState(0)

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    if (taskSource !== 'linear' || !linearConnected) {
      setAvailableTeams([])
      return
    }
    let cancelled = false
    const cachedTeams = getCachedLinearTeams(selectedLinearWorkspaceId, {
      sourceContext: linearTaskSourceContext
    })
    // Why: workspace switches must not leave the prior workspace's teams
    // available for new-issue creation while the replacement fetch is pending,
    // but a workspace-scoped cache can keep the selector usable immediately.
    setAvailableTeams(cachedTeams ?? [])
    void listLinearTeams(selectedLinearWorkspaceId, { sourceContext: linearTaskSourceContext })
      .then((teams) => {
        if (!cancelled) {
          setAvailableTeams(teams)
        }
      })
      .catch(() => {
        if (!cancelled) {
          console.warn('[TaskPage] Failed to fetch Linear teams')
        }
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    taskSource,
    linearConnected,
    selectedLinearWorkspaceId,
    linearTeamRefreshNonce,
    taskResumeApplied,
    getCachedLinearTeams,
    listLinearTeams,
    linearTaskSourceContext
  ])

  const [linearTeamSelection, setLinearTeamSelection] = useState<ReadonlySet<string>>(() => {
    if (!defaultLinearTeamSelection) {
      return new Set<string>()
    }
    return new Set(defaultLinearTeamSelection)
  })

  // Why: the full Linear team fetch is async and can temporarily be empty.
  // Keep the selector usable from issue metadata until the complete list lands.
  const linearTeamOptions = useMemo(() => {
    if (availableTeams.length === 0) {
      return linearIssueTeams
    }
    const issueTeamById = new Map(linearIssueTeams.map((team) => [team.id, team]))
    return availableTeams.map((team) => {
      if (team.url) {
        return team
      }
      return {
        ...team,
        url: issueTeamById.get(team.id)?.url
      }
    })
  }, [availableTeams, linearIssueTeams])

  // Why: team IDs belong to one Linear workspace. Switching workspaces while a
  // saved subset exists must not leave the task list filtered by stale team IDs.
  useEffect(() => {
    if (linearTeamOptions.length === 0) {
      return
    }
    setLinearTeamSelection(
      reconcileLinearTeamSelection(linearTeamOptions, defaultLinearTeamSelection)
    )
  }, [linearTeamOptions, defaultLinearTeamSelection])

  const linearAttributePrimaryTeam = useMemo(
    () =>
      resolveLinearIssueAttributeFilterPrimaryTeam({
        selectedTeamIds: [...linearTeamSelection],
        availableTeams: linearTeamOptions
      }),
    [linearTeamOptions, linearTeamSelection]
  )

  const previousLinearWorkspaceIdForFiltersRef = useRef<string | null | undefined>(undefined)
  useEffect(() => {
    const workspaceId = selectedLinearWorkspaceId ?? null
    const previous = previousLinearWorkspaceIdForFiltersRef.current
    previousLinearWorkspaceIdForFiltersRef.current = workspaceId
    if (previous === undefined || previous === workspaceId) {
      return
    }
    applyLinearAttributeFilter(emptyLinearIssueAttributeFilter())
  }, [applyLinearAttributeFilter, selectedLinearWorkspaceId])

  const linearPrimaryTeamIdRef = useRef<string | null>(null)
  useEffect(() => {
    const nextId = linearAttributePrimaryTeam?.id ?? null
    const previousId = linearPrimaryTeamIdRef.current
    linearPrimaryTeamIdRef.current = nextId
    if (previousId === null || previousId === nextId) {
      return
    }
    // Why: status/assignee/labels are team-scoped; clearing them is a filter change
    // and must reset limit/page via applyLinearAttributeFilter (R6), not a bare set.
    const next = teamDerivedFacetsForPrimaryTeamChange(linearAttributeFilter)
    if (
      linearIssueAttributeFilterSignature(linearAttributeFilter) ===
      linearIssueAttributeFilterSignature(next)
    ) {
      return
    }
    applyLinearAttributeFilter(next)
  }, [applyLinearAttributeFilter, linearAttributeFilter, linearAttributePrimaryTeam?.id])

  return {
    availableTeams,
    setAvailableTeams,
    linearTeamRefreshNonce,
    setLinearTeamRefreshNonce,
    linearTeamSelection,
    setLinearTeamSelection,
    linearTeamOptions,
    linearAttributePrimaryTeam
  }
}
