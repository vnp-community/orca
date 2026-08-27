import { useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { jiraListCreateFields, jiraListIssueTypes } from '@/runtime/runtime-jira-client'
import type {
  GlobalSettings,
  JiraCreateField,
  JiraIssueType,
  JiraProject
} from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-237) — the Jira "new issue"
// draft state + connect-dialog draft state. The effect that closes this
// draft when a Linear draft opens (cross-domain coordination) is NOT
// included here — it stays in TaskPage.tsx, reading `newJiraIssueOpen` /
// `setNewJiraIssueOpen` from this hook's return value. `sortedAvailableJiraProjects`
// is a read param rather than owned state — it derives from
// `availableJiraProjects`, which belongs to the Jira browse domain
// (TASK-BIGFILE-238, a separate hook); `newJiraIssueTargetProject` /
// `newJiraIssueTargetType` are computed here (from owned state) and
// returned so TaskPage.tsx doesn't need to re-derive them.
type UseTaskPageJiraDraftStateParams = {
  settings: GlobalSettings | null
  jiraConnected: boolean
  jiraTaskSourceContext: TaskSourceContext | null | undefined
  sortedAvailableJiraProjects: readonly JiraProject[]
  getJiraProjectSelectionKey: (project: JiraProject) => string
}

export function useTaskPageJiraDraftState({
  settings,
  jiraConnected,
  jiraTaskSourceContext,
  sortedAvailableJiraProjects,
  getJiraProjectSelectionKey
}: UseTaskPageJiraDraftStateParams) {
  const [newJiraIssueOpen, setNewJiraIssueOpen] = useState(false)
  const [newJiraIssueTitle, setNewJiraIssueTitle] = useState('')
  const [newJiraIssueBody, setNewJiraIssueBody] = useState('')
  const [newJiraIssueProjectId, setNewJiraIssueProjectId] = useState<string | null>(null)
  const [newJiraIssueProjectComboboxOpen, setNewJiraIssueProjectComboboxOpen] = useState(false)
  const [newJiraIssueProjectQuery, setNewJiraIssueProjectQuery] = useState('')
  const [newJiraIssueProjectCommandValue, setNewJiraIssueProjectCommandValue] = useState('')
  const [newJiraIssueTypeId, setNewJiraIssueTypeId] = useState<string | null>(null)
  const [newJiraIssueSubmitting, setNewJiraIssueSubmitting] = useState(false)
  const newJiraIssueProjectSearchInputRef = useRef<HTMLInputElement | null>(null)
  const [availableJiraIssueTypes, setAvailableJiraIssueTypes] = useState<JiraIssueType[]>([])
  const [jiraIssueTypesLoading, setJiraIssueTypesLoading] = useState(false)
  const [jiraCreateFields, setJiraCreateFields] = useState<JiraCreateField[]>([])
  const [jiraCreateFieldsLoading, setJiraCreateFieldsLoading] = useState(false)
  const [jiraCreateFieldsError, setJiraCreateFieldsError] = useState<string | null>(null)
  const [newJiraIssueCustomFieldValues, setNewJiraIssueCustomFieldValues] = useState<
    Record<string, string>
  >({})
  const [jiraConnectOpen, setJiraConnectOpen] = useState(false)
  const [jiraSiteUrlDraft, setJiraSiteUrlDraft] = useState('')
  const [jiraEmailDraft, setJiraEmailDraft] = useState('')
  const [jiraApiTokenDraft, setJiraApiTokenDraft] = useState('')
  const [jiraConnectState, setJiraConnectState] = useState<'idle' | 'connecting' | 'error'>('idle')
  const [jiraConnectError, setJiraConnectError] = useState<string | null>(null)

  const newJiraIssueTargetProject = useMemo(
    () =>
      sortedAvailableJiraProjects.find(
        (project) => getJiraProjectSelectionKey(project) === newJiraIssueProjectId
      ) ??
      sortedAvailableJiraProjects[0] ??
      null,
    [getJiraProjectSelectionKey, newJiraIssueProjectId, sortedAvailableJiraProjects]
  )

  const newJiraIssueTargetType = useMemo(
    () =>
      availableJiraIssueTypes.find((issueType) => issueType.id === newJiraIssueTypeId) ??
      availableJiraIssueTypes[0] ??
      null,
    [availableJiraIssueTypes, newJiraIssueTypeId]
  )

  useEffect(() => {
    if (!newJiraIssueProjectComboboxOpen) {
      return
    }
    const frame = requestAnimationFrame(() => {
      const input = newJiraIssueProjectSearchInputRef.current
      if (!input) {
        return
      }
      input.focus()
      const end = input.value.length
      input.setSelectionRange(end, end)
    })
    return () => cancelAnimationFrame(frame)
  }, [newJiraIssueProjectComboboxOpen])

  useEffect(() => {
    if (!newJiraIssueOpen || !jiraConnected || !newJiraIssueTargetProject) {
      setAvailableJiraIssueTypes([])
      setJiraIssueTypesLoading(false)
      return
    }
    let cancelled = false
    setAvailableJiraIssueTypes([])
    setJiraIssueTypesLoading(true)
    void jiraListIssueTypes(
      jiraTaskSourceContext ?? settings,
      newJiraIssueTargetProject.id,
      newJiraIssueTargetProject.siteId
    )
      .then((issueTypes) => {
        if (cancelled) {
          return
        }
        setAvailableJiraIssueTypes(issueTypes)
        setNewJiraIssueTypeId(issueTypes[0]?.id ?? null)
      })
      .catch(() => {
        if (!cancelled) {
          toast.error(
            translate('auto.components.TaskPage.af2a8371de', 'Failed to load Jira issue types.')
          )
        }
      })
      .finally(() => {
        if (!cancelled) {
          setJiraIssueTypesLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [settings, jiraConnected, newJiraIssueOpen, newJiraIssueTargetProject, jiraTaskSourceContext])

  useEffect(() => {
    if (
      !newJiraIssueOpen ||
      !jiraConnected ||
      !newJiraIssueTargetProject ||
      !newJiraIssueTargetType
    ) {
      setJiraCreateFields([])
      setJiraCreateFieldsLoading(false)
      setJiraCreateFieldsError(null)
      setNewJiraIssueCustomFieldValues({})
      return
    }
    let cancelled = false
    setJiraCreateFields([])
    setJiraCreateFieldsLoading(true)
    setJiraCreateFieldsError(null)
    setNewJiraIssueCustomFieldValues({})
    void jiraListCreateFields(
      jiraTaskSourceContext ?? settings,
      newJiraIssueTargetProject.id,
      newJiraIssueTargetType.id,
      newJiraIssueTargetProject.siteId
    )
      .then((fields) => {
        if (!cancelled) {
          setJiraCreateFields(fields)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setJiraCreateFieldsError('Failed to load required Jira fields.')
        }
      })
      .finally(() => {
        if (!cancelled) {
          setJiraCreateFieldsLoading(false)
        }
      })
    return () => {
      // Why: create fields are scoped to project + issue type; ignore late
      // responses after the user switches either selector.
      cancelled = true
    }
  }, [
    settings,
    jiraConnected,
    newJiraIssueOpen,
    newJiraIssueTargetProject,
    newJiraIssueTargetType,
    jiraTaskSourceContext
  ])

  return {
    newJiraIssueOpen,
    setNewJiraIssueOpen,
    newJiraIssueTitle,
    setNewJiraIssueTitle,
    newJiraIssueBody,
    setNewJiraIssueBody,
    newJiraIssueProjectId,
    setNewJiraIssueProjectId,
    newJiraIssueProjectComboboxOpen,
    setNewJiraIssueProjectComboboxOpen,
    newJiraIssueProjectQuery,
    setNewJiraIssueProjectQuery,
    newJiraIssueProjectCommandValue,
    setNewJiraIssueProjectCommandValue,
    newJiraIssueTypeId,
    setNewJiraIssueTypeId,
    newJiraIssueSubmitting,
    setNewJiraIssueSubmitting,
    newJiraIssueProjectSearchInputRef,
    availableJiraIssueTypes,
    setAvailableJiraIssueTypes,
    jiraIssueTypesLoading,
    setJiraIssueTypesLoading,
    jiraCreateFields,
    setJiraCreateFields,
    jiraCreateFieldsLoading,
    setJiraCreateFieldsLoading,
    jiraCreateFieldsError,
    setJiraCreateFieldsError,
    newJiraIssueCustomFieldValues,
    setNewJiraIssueCustomFieldValues,
    jiraConnectOpen,
    setJiraConnectOpen,
    jiraSiteUrlDraft,
    setJiraSiteUrlDraft,
    jiraEmailDraft,
    setJiraEmailDraft,
    jiraApiTokenDraft,
    setJiraApiTokenDraft,
    jiraConnectState,
    setJiraConnectState,
    jiraConnectError,
    setJiraConnectError,
    newJiraIssueTargetProject,
    newJiraIssueTargetType
  }
}
