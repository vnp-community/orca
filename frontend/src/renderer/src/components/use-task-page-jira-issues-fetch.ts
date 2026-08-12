import { useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import {
  getSingleJiraProjectScope,
  getTaskPageJiraStatusOrderScopeKey,
  loadTaskPageJiraProjectStatusOrder
} from '@/components/task-page-jira-status-order'
import {
  createTaskPageJiraLoadFailureState,
  type TaskPageJiraLoadError
} from '@/components/task-page-jira-load-state'
import type { JiraPresetId } from '@/components/task-page-localized-options'
import type {
  GlobalSettings,
  JiraIssue,
  JiraProjectStatusOrder,
  TaskProvider
} from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of use-task-page-jira-browse-state.ts (TASK-BIGFILE-238) to
// stay under the 300-line file budget — the Jira issues data-fetch slice:
// search-box debounce, resume-state persistence, the main list fetch, and
// the selected-issue-fallback reconcile. Pure effects hook — owns no state
// of its own beyond its private debounce-persist-readiness ref.
const TASK_SEARCH_DEBOUNCE_MS = 300
const JIRA_ITEM_LIMIT = 50

type UseTaskPageJiraIssuesFetchParams = {
  taskSource: TaskProvider
  taskResumeApplied: boolean
  jiraConnected: boolean
  selectedJiraSiteId: string | null
  jiraSearchInput: string
  appliedJiraSearch: string
  activeJiraPreset: JiraPresetId
  jiraRefreshNonce: number
  jiraTaskSourceContext: TaskSourceContext | null | undefined
  jiraTaskSourceScopeKey: string
  settings: GlobalSettings | null
  displayedJiraIssues: readonly JiraIssue[]
  selectedJiraIssueKey: string | null
  selectedJiraIssueFallback: JiraIssue | null
  setAppliedJiraSearch: (value: string) => void
  setJiraLoading: (value: boolean) => void
  setJiraError: (value: TaskPageJiraLoadError | null) => void
  setJiraErrorDetailsOpen: (value: boolean) => void
  setJiraIssues: (value: JiraIssue[]) => void
  setJiraProjectStatusOrder: (
    value: { order: JiraProjectStatusOrder; scopeKey: string } | null
  ) => void
  setSelectedJiraIssueKey: (value: string | null) => void
  setSelectedJiraIssueFallback: (value: JiraIssue | null) => void
}

export function useTaskPageJiraIssuesFetch({
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
}: UseTaskPageJiraIssuesFetchParams): void {
  const searchJiraIssues = useAppStore((s) => s.searchJiraIssues)
  const listJiraIssues = useAppStore((s) => s.listJiraIssues)
  const setTaskResumeState = useAppStore((s) => s.setTaskResumeState)

  const jiraSearchPersistReadyRef = useRef(false)

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    const timeout = window.setTimeout(() => {
      setAppliedJiraSearch(jiraSearchInput)
    }, TASK_SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timeout)
  }, [jiraSearchInput, setAppliedJiraSearch, taskResumeApplied])

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    if (!jiraSearchPersistReadyRef.current) {
      jiraSearchPersistReadyRef.current = true
      return
    }
    setTaskResumeState({ jiraQuery: appliedJiraSearch.trim() })
  }, [appliedJiraSearch, setTaskResumeState, taskResumeApplied])

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    if (taskSource !== 'jira') {
      return
    }
    if (!jiraConnected) {
      return
    }

    let cancelled = false
    setJiraLoading(true)
    setJiraError(null)
    setJiraErrorDetailsOpen(false)

    const trimmed = appliedJiraSearch.trim()
    const request =
      trimmed.length > 0
        ? searchJiraIssues(trimmed, JIRA_ITEM_LIMIT, { sourceContext: jiraTaskSourceContext })
        : listJiraIssues(activeJiraPreset, JIRA_ITEM_LIMIT, {
            sourceContext: jiraTaskSourceContext
          })

    void request
      .then((issues) => {
        if (cancelled) {
          return
        }
        setJiraIssues(issues)
        setJiraLoading(false)
        const projectScope = getSingleJiraProjectScope(issues)
        if (!projectScope) {
          return
        }
        const statusOrderScopeKey = getTaskPageJiraStatusOrderScopeKey(
          jiraTaskSourceScopeKey,
          projectScope
        )
        void loadTaskPageJiraProjectStatusOrder(
          jiraTaskSourceContext ?? settings,
          jiraTaskSourceScopeKey,
          projectScope
        ).then((order) => {
          if (!cancelled) {
            setJiraProjectStatusOrder({
              order,
              scopeKey: statusOrderScopeKey
            })
          }
        })
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        const failureState = createTaskPageJiraLoadFailureState(err)
        setJiraIssues(failureState.issues)
        setJiraError(failureState.error)
        setJiraLoading(false)
      })

    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    taskSource,
    jiraConnected,
    selectedJiraSiteId,
    appliedJiraSearch,
    activeJiraPreset,
    jiraRefreshNonce,
    taskResumeApplied,
    jiraTaskSourceContext,
    jiraTaskSourceScopeKey
  ])

  useEffect(() => {
    if (!taskResumeApplied || taskSource !== 'jira') {
      return
    }
    if (!jiraConnected || displayedJiraIssues.length === 0) {
      if (selectedJiraIssueKey !== null) {
        setSelectedJiraIssueKey(null)
      }
      if (selectedJiraIssueFallback !== null) {
        setSelectedJiraIssueFallback(null)
      }
      return
    }
    if (
      selectedJiraIssueKey &&
      !displayedJiraIssues.some((issue) => issue.key === selectedJiraIssueKey)
    ) {
      setSelectedJiraIssueKey(null)
      setSelectedJiraIssueFallback(null)
    }
  }, [
    displayedJiraIssues,
    jiraConnected,
    selectedJiraIssueFallback,
    selectedJiraIssueKey,
    setSelectedJiraIssueFallback,
    setSelectedJiraIssueKey,
    taskResumeApplied,
    taskSource
  ])
}
