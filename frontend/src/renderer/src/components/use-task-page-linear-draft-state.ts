import { useEffect, useMemo, useState } from 'react'
import { linearListProjects } from '@/runtime/runtime-linear-client'
import { useTeamLabels, useTeamMembers, useTeamStates } from '@/hooks/useIssueMetadata'
import type { GlobalSettings, LinearProjectSummary, LinearTeam } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-239) — the Linear "new
// project" + "new issue" draft state and the connect-dialog draft state.
// The effect that closes this draft when a Jira draft opens (cross-domain
// coordination) is NOT included here — it stays in TaskPage.tsx, reading
// `newLinearIssueOpen` / `setNewLinearIssueOpen` from this hook's return
// value. `availableTeams` / `selectedLinearProject` / `selectedLinearWorkspaceId`
// are read params rather than owned state — they belong to the Linear teams
// (TASK-BIGFILE-240) and Linear browse (TASK-BIGFILE-241) domains, not yet
// extracted when this hook was written; see those tasks' docs for the
// accepted temporary composition dependency.
type UseTaskPageLinearDraftStateParams = {
  settings: GlobalSettings | null
  linearConnected: boolean
  linearTaskSourceContext: TaskSourceContext | null | undefined
  selectedLinearWorkspaceId: string | null
  availableTeams: readonly LinearTeam[]
  selectedLinearProject: LinearProjectSummary | null
}

export function useTaskPageLinearDraftState({
  settings,
  linearConnected,
  linearTaskSourceContext,
  selectedLinearWorkspaceId,
  availableTeams,
  selectedLinearProject
}: UseTaskPageLinearDraftStateParams) {
  // New Linear project dialog state
  const [newLinearProjectOpen, setNewLinearProjectOpen] = useState(false)
  const [newLinearProjectName, setNewLinearProjectName] = useState('')
  const [newLinearProjectDescription, setNewLinearProjectDescription] = useState('')
  const [newLinearProjectContent, setNewLinearProjectContent] = useState('')
  const [newLinearProjectTeamId, setNewLinearProjectTeamId] = useState<string | null>(null)
  const [newLinearProjectLeadId, setNewLinearProjectLeadId] = useState<string | null>(null)
  const [newLinearProjectMemberIds, setNewLinearProjectMemberIds] = useState<string[]>([])
  const [newLinearProjectLabelIds, setNewLinearProjectLabelIds] = useState<string[]>([])
  const [newLinearProjectPriority, setNewLinearProjectPriority] = useState<number>(0)
  const [newLinearProjectStartDate, setNewLinearProjectStartDate] = useState('')
  const [newLinearProjectTargetDate, setNewLinearProjectTargetDate] = useState('')
  const [newLinearProjectSubmitting, setNewLinearProjectSubmitting] = useState(false)

  const newLinearProjectTargetTeam = useMemo(
    () => availableTeams.find((t) => t.id === newLinearProjectTeamId) ?? availableTeams[0] ?? null,
    [availableTeams, newLinearProjectTeamId]
  )
  const newLinearProjectMembers = useTeamMembers(
    newLinearProjectOpen ? (newLinearProjectTargetTeam?.id ?? null) : null,
    settings,
    newLinearProjectTargetTeam?.workspaceId
  )
  const newLinearProjectLabels = useTeamLabels(
    newLinearProjectOpen ? (newLinearProjectTargetTeam?.id ?? null) : null,
    settings,
    newLinearProjectTargetTeam?.workspaceId
  )

  useEffect(() => {
    setNewLinearProjectLeadId(null)
    setNewLinearProjectMemberIds([])
    setNewLinearProjectLabelIds([])
  }, [newLinearProjectTargetTeam?.id, newLinearProjectTargetTeam?.workspaceId])

  // New Linear issue dialog state
  const [newLinearIssueOpen, setNewLinearIssueOpen] = useState(false)
  const [newLinearIssueTitle, setNewLinearIssueTitle] = useState('')
  const [newLinearIssueBody, setNewLinearIssueBody] = useState('')
  const [newLinearIssueTeamId, setNewLinearIssueTeamId] = useState<string | null>(null)
  const [newLinearIssueSubmitting, setNewLinearIssueSubmitting] = useState(false)

  const [newLinearIssueStateId, setNewLinearIssueStateId] = useState<string | null>(null)
  const [newLinearIssueAssigneeId, setNewLinearIssueAssigneeId] = useState<string | null>(null)
  const [newLinearIssuePriority, setNewLinearIssuePriority] = useState<number>(0)
  const [newLinearIssueProjectId, setNewLinearIssueProjectId] = useState<string | null>(null)
  const [newLinearIssueLabelIds, setNewLinearIssueLabelIds] = useState<string[]>([])

  const newLinearIssueTargetTeam = useMemo(
    () => availableTeams.find((t) => t.id === newLinearIssueTeamId) ?? availableTeams[0] ?? null,
    [availableTeams, newLinearIssueTeamId]
  )

  const [newLinearIssueProjects, setNewLinearIssueProjects] = useState<LinearProjectSummary[]>([])
  const [newLinearIssueProjectsLoading, setNewLinearIssueProjectsLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    if (!newLinearIssueOpen || !linearConnected || !newLinearIssueTargetTeam) {
      setNewLinearIssueProjects([])
      setNewLinearIssueProjectsLoading(false)
      return
    }
    setNewLinearIssueProjectsLoading(true)
    const targetWorkspaceId =
      newLinearIssueTargetTeam.workspaceId ||
      (selectedLinearWorkspaceId !== 'all' ? selectedLinearWorkspaceId : null)
    linearListProjects(linearTaskSourceContext ?? settings, undefined, 100, targetWorkspaceId)
      .then((p) => {
        if (!cancelled) {
          setNewLinearIssueProjects(p.items)
        }
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) {
          setNewLinearIssueProjectsLoading(false)
        }
      })
    return () => {
      // Why: project lists are workspace-scoped; stale responses must not
      // populate the composer after a team/workspace switch.
      cancelled = true
    }
  }, [
    linearConnected,
    newLinearIssueOpen,
    newLinearIssueTargetTeam,
    linearTaskSourceContext,
    settings,
    selectedLinearWorkspaceId
  ])

  useEffect(() => {
    // Why: the selected team can change indirectly when the available Linear
    // teams/workspace list refreshes, even if the explicit picker value did not.
    setNewLinearIssueStateId(null)
    setNewLinearIssueAssigneeId(null)
    setNewLinearIssuePriority(0)
    if (
      selectedLinearProject &&
      selectedLinearProject.workspaceId === newLinearIssueTargetTeam?.workspaceId
    ) {
      setNewLinearIssueProjectId(selectedLinearProject.id)
    } else {
      setNewLinearIssueProjectId(null)
    }
    setNewLinearIssueLabelIds([])
  }, [newLinearIssueTargetTeam?.id, newLinearIssueTargetTeam?.workspaceId, selectedLinearProject])

  const newLinearStates = useTeamStates(
    linearConnected ? newLinearIssueTargetTeam?.id || null : null,
    settings,
    newLinearIssueTargetTeam?.workspaceId
  )
  const newLinearMembers = useTeamMembers(
    linearConnected ? newLinearIssueTargetTeam?.id || null : null,
    settings,
    newLinearIssueTargetTeam?.workspaceId
  )
  const newLinearLabels = useTeamLabels(
    linearConnected ? newLinearIssueTargetTeam?.id || null : null,
    settings,
    newLinearIssueTargetTeam?.workspaceId
  )

  useEffect(() => {
    if (newLinearStates.data.length > 0 && !newLinearIssueStateId) {
      const defaultState =
        newLinearStates.data.find((s) => s.type === 'unstarted') || newLinearStates.data[0]
      if (defaultState) {
        setNewLinearIssueStateId(defaultState.id)
      }
    }
  }, [newLinearStates.data, newLinearIssueStateId])

  const [linearConnectOpen, setLinearConnectOpen] = useState(false)

  return {
    newLinearProjectOpen,
    setNewLinearProjectOpen,
    newLinearProjectName,
    setNewLinearProjectName,
    newLinearProjectDescription,
    setNewLinearProjectDescription,
    newLinearProjectContent,
    setNewLinearProjectContent,
    newLinearProjectTeamId,
    setNewLinearProjectTeamId,
    newLinearProjectLeadId,
    setNewLinearProjectLeadId,
    newLinearProjectMemberIds,
    setNewLinearProjectMemberIds,
    newLinearProjectLabelIds,
    setNewLinearProjectLabelIds,
    newLinearProjectPriority,
    setNewLinearProjectPriority,
    newLinearProjectStartDate,
    setNewLinearProjectStartDate,
    newLinearProjectTargetDate,
    setNewLinearProjectTargetDate,
    newLinearProjectSubmitting,
    setNewLinearProjectSubmitting,
    newLinearProjectTargetTeam,
    newLinearProjectMembers,
    newLinearProjectLabels,
    newLinearIssueOpen,
    setNewLinearIssueOpen,
    newLinearIssueTitle,
    setNewLinearIssueTitle,
    newLinearIssueBody,
    setNewLinearIssueBody,
    newLinearIssueTeamId,
    setNewLinearIssueTeamId,
    newLinearIssueSubmitting,
    setNewLinearIssueSubmitting,
    newLinearIssueStateId,
    setNewLinearIssueStateId,
    newLinearIssueAssigneeId,
    setNewLinearIssueAssigneeId,
    newLinearIssuePriority,
    setNewLinearIssuePriority,
    newLinearIssueProjectId,
    setNewLinearIssueProjectId,
    newLinearIssueLabelIds,
    setNewLinearIssueLabelIds,
    newLinearIssueTargetTeam,
    newLinearIssueProjects,
    setNewLinearIssueProjects,
    newLinearIssueProjectsLoading,
    setNewLinearIssueProjectsLoading,
    newLinearStates,
    newLinearMembers,
    newLinearLabels,
    linearConnectOpen,
    setLinearConnectOpen
  }
}
