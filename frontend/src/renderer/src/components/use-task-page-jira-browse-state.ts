import { useCallback, useEffect, useMemo, useState } from 'react'
import { useShallow } from 'zustand/react/shallow'
import { useAppStore } from '@/store'
import { jiraListPriorities, jiraListProjects } from '@/runtime/runtime-jira-client'
import { findTaskPageJiraIssue } from '@/components/task-page-jira-cache-selectors'
import {
  getSingleJiraProjectScope,
  getTaskPageJiraStatusOrderScopeKey
} from '@/components/task-page-jira-status-order'
import type { TaskPageJiraLoadError } from '@/components/task-page-jira-load-state'
import {
  sortJiraIssues,
  type JiraIssueSortColumn,
  type JiraIssueSortDirection,
  type JiraPrioritiesBySite
} from './jira-issue-sorter'
import { useTaskPageJiraIssuesFetch } from '@/components/use-task-page-jira-issues-fetch'
import type { JiraPresetId } from '@/components/task-page-localized-options'
import type {
  GlobalSettings,
  JiraIssue,
  JiraPriority,
  JiraProject,
  JiraProjectStatusOrder,
  TaskProvider
} from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

type UseTaskPageJiraBrowseStateParams = {
  taskSource: TaskProvider
  taskResumeApplied: boolean
  jiraConnected: boolean
  settings: GlobalSettings | null
  selectedJiraSiteId: string | null
  jiraTaskSourceContext: TaskSourceContext | null | undefined
  jiraTaskSourceScopeKey: string
  openJiraIssue: JiraIssue | null | undefined
  openJiraSourceContext: TaskSourceContext | null | undefined
}

export function useTaskPageJiraBrowseState({
  taskSource,
  taskResumeApplied,
  jiraConnected,
  settings,
  selectedJiraSiteId,
  jiraTaskSourceContext,
  jiraTaskSourceScopeKey,
  openJiraIssue,
  openJiraSourceContext
}: UseTaskPageJiraBrowseStateParams) {
  // Why: chosen/opened Jira issue — the detail dialog's source of truth.
  const [selectedJiraIssueKey, setSelectedJiraIssueKey] = useState<string | null>(null)
  const [selectedJiraIssueFallback, setSelectedJiraIssueFallback] = useState<JiraIssue | null>(null)
  const jiraCacheSnapshot = useAppStore(
    useShallow((s) => ({
      issueCache: s.jiraIssueCache,
      searchCache: s.jiraSearchCache
    }))
  )
  const cachedSelectedJiraIssue = findTaskPageJiraIssue(
    jiraCacheSnapshot.issueCache,
    jiraCacheSnapshot.searchCache,
    selectedJiraIssueKey,
    {
      sourceContext: jiraTaskSourceContext,
      siteId: selectedJiraIssueFallback?.siteId ?? openJiraIssue?.siteId ?? null
    }
  )
  const selectedJiraIssue = selectedJiraIssueKey
    ? (cachedSelectedJiraIssue ?? selectedJiraIssueFallback)
    : null
  const jiraDetailSourceContext = useMemo(() => {
    if (
      selectedJiraIssue &&
      openJiraSourceContext?.provider === 'jira' &&
      openJiraIssue?.key === selectedJiraIssue.key &&
      openJiraIssue.siteId === selectedJiraIssue.siteId
    ) {
      return openJiraSourceContext
    }
    return jiraTaskSourceContext
  }, [jiraTaskSourceContext, openJiraIssue, openJiraSourceContext, selectedJiraIssue])

  const setSelectedJiraIssue = useCallback((issue: JiraIssue | null) => {
    setSelectedJiraIssueKey(issue?.key ?? null)
    setSelectedJiraIssueFallback(issue)
  }, [])

  useEffect(() => {
    setSelectedJiraIssue(openJiraIssue ?? null)
  }, [openJiraIssue, setSelectedJiraIssue])

  // Jira tab browse state
  const [jiraIssues, setJiraIssues] = useState<JiraIssue[]>([])
  const [jiraLoading, setJiraLoading] = useState(false)
  const [jiraError, setJiraError] = useState<TaskPageJiraLoadError | null>(null)
  const [jiraErrorDetailsOpen, setJiraErrorDetailsOpen] = useState(false)
  const [jiraSearchInput, setJiraSearchInput] = useState('')
  const [appliedJiraSearch, setAppliedJiraSearch] = useState('')
  const [activeJiraPreset, setActiveJiraPreset] = useState<JiraPresetId>('assigned')
  const [jiraRefreshNonce, setJiraRefreshNonce] = useState(0)
  const [jiraProjectStatusOrder, setJiraProjectStatusOrder] = useState<{
    order: JiraProjectStatusOrder
    scopeKey: string
  } | null>(null)
  const [jiraOrderBy, setJiraOrderBy] = useState<JiraIssueSortColumn>('updated')
  const [jiraOrderDirection, setJiraOrderDirection] = useState<JiraIssueSortDirection>('desc')
  const [jiraPrioritiesBySite, setJiraPrioritiesBySite] = useState<JiraPrioritiesBySite>(
    () => new Map()
  )
  const jiraPrioritySiteIdsKey = useMemo(() => {
    const siteIds =
      selectedJiraSiteId && selectedJiraSiteId !== 'all'
        ? [selectedJiraSiteId]
        : jiraIssues.flatMap((issue) => (issue.siteId ? [issue.siteId] : []))
    // Why: result refreshes replace the issue array; depend on the represented sites, not identity.
    return JSON.stringify([...new Set(siteIds)].sort())
  }, [jiraIssues, selectedJiraSiteId])

  useEffect(() => {
    if (taskSource !== 'jira' || !jiraConnected || jiraOrderBy !== 'priority') {
      setJiraPrioritiesBySite((current) => (current.size === 0 ? current : new Map()))
      return
    }
    let cancelled = false
    const jiraPrioritySiteIds = JSON.parse(jiraPrioritySiteIdsKey) as string[]
    void Promise.all(
      jiraPrioritySiteIds.map(async (siteId) => {
        try {
          return [
            siteId,
            await jiraListPriorities(jiraTaskSourceContext ?? settings, siteId)
          ] as const
        } catch {
          return [siteId, [] as JiraPriority[]] as const
        }
      })
    ).then((prioritiesBySite) => {
      if (!cancelled) {
        setJiraPrioritiesBySite(new Map(prioritiesBySite))
      }
    })
    return () => {
      cancelled = true
    }
  }, [
    jiraConnected,
    jiraOrderBy,
    jiraPrioritySiteIdsKey,
    jiraTaskSourceContext,
    settings,
    taskSource
  ])

  const [availableJiraProjects, setAvailableJiraProjects] = useState<JiraProject[]>([])
  const [jiraProjectsLoading, setJiraProjectsLoading] = useState(false)

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    if (taskSource !== 'jira' || !jiraConnected) {
      setAvailableJiraProjects([])
      setJiraProjectsLoading(false)
      return
    }
    let cancelled = false
    setAvailableJiraProjects([])
    setJiraProjectsLoading(true)
    void jiraListProjects(jiraTaskSourceContext ?? settings, selectedJiraSiteId)
      .then((projects) => {
        if (!cancelled) {
          setAvailableJiraProjects(projects)
        }
      })
      .catch(() => {
        if (!cancelled) {
          console.warn('[TaskPage] Failed to fetch Jira projects')
        }
      })
      .finally(() => {
        if (!cancelled) {
          setJiraProjectsLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [
    settings,
    taskSource,
    jiraConnected,
    selectedJiraSiteId,
    taskResumeApplied,
    jiraTaskSourceContext
  ])

  const displayedJiraIssues = useMemo(
    () =>
      jiraIssues.map(
        (issue) =>
          findTaskPageJiraIssue(
            jiraCacheSnapshot.issueCache,
            jiraCacheSnapshot.searchCache,
            issue.key,
            {
              sourceContext: jiraTaskSourceContext,
              siteId: issue.siteId
            }
          ) ?? issue
      ),
    [jiraIssues, jiraCacheSnapshot.issueCache, jiraCacheSnapshot.searchCache, jiraTaskSourceContext]
  )
  const displayedJiraProjectScope = useMemo(
    () => getSingleJiraProjectScope(displayedJiraIssues),
    [displayedJiraIssues]
  )
  const displayedJiraStatusOrderScopeKey = displayedJiraProjectScope
    ? getTaskPageJiraStatusOrderScopeKey(jiraTaskSourceScopeKey, displayedJiraProjectScope)
    : null
  const displayedJiraStatusOrder =
    jiraProjectStatusOrder && displayedJiraStatusOrderScopeKey === jiraProjectStatusOrder.scopeKey
      ? jiraProjectStatusOrder.order
      : null

  const sortedJiraIssues = useMemo(() => {
    return sortJiraIssues(
      displayedJiraIssues,
      jiraOrderBy,
      jiraOrderDirection,
      jiraPrioritiesBySite
    )
  }, [displayedJiraIssues, jiraOrderBy, jiraOrderDirection, jiraPrioritiesBySite])

  useTaskPageJiraIssuesFetch({
    taskSource,
    taskResumeApplied,
    jiraConnected,
    selectedJiraSiteId,
    jiraSearchInput,
    appliedJiraSearch,
    activeJiraPreset,
    jiraRefreshNonce,
    jiraTaskSourceContext,
    jiraTaskSourceScopeKey,
    settings,
    displayedJiraIssues,
    selectedJiraIssueKey,
    selectedJiraIssueFallback,
    setAppliedJiraSearch,
    setJiraLoading,
    setJiraError,
    setJiraErrorDetailsOpen,
    setJiraIssues,
    setJiraProjectStatusOrder,
    setSelectedJiraIssueKey,
    setSelectedJiraIssueFallback
  })

  return {
    selectedJiraIssueKey,
    setSelectedJiraIssueKey,
    selectedJiraIssueFallback,
    setSelectedJiraIssueFallback,
    selectedJiraIssue,
    jiraDetailSourceContext,
    setSelectedJiraIssue,
    jiraIssues,
    setJiraIssues,
    jiraLoading,
    setJiraLoading,
    jiraError,
    setJiraError,
    jiraErrorDetailsOpen,
    setJiraErrorDetailsOpen,
    jiraSearchInput,
    setJiraSearchInput,
    appliedJiraSearch,
    setAppliedJiraSearch,
    activeJiraPreset,
    setActiveJiraPreset,
    jiraRefreshNonce,
    setJiraRefreshNonce,
    jiraOrderBy,
    jiraOrderDirection,
    setJiraOrderBy,
    setJiraOrderDirection,
    availableJiraProjects,
    jiraProjectsLoading,
    displayedJiraIssues,
    displayedJiraStatusOrder,
    sortedJiraIssues
  }
}
