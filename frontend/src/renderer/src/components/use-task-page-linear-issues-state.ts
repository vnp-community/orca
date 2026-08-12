import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { useTaskPageLinearIssuesFetch } from '@/components/use-task-page-linear-issues-fetch'
import { DEFAULT_LINEAR_DISPLAY_PROPERTIES } from '@/components/task-page-linear-collection-helpers'
import { emptyLinearIssueAttributeFilter } from '../../../shared/linear-issue-attribute-filter'
import type { LinearIssueAttributeFilter } from '../../../shared/linear-issue-attribute-filter'
import type { LinearIssue, TaskProvider } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'
import type {
  LinearDisplayProperty,
  LinearGroupBy,
  LinearMode,
  LinearOrderBy,
  LinearViewMode
} from '@/components/task-page-localized-options'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) to stay under the
// 300-line file budget — the plain "all issues" Linear list slice: mode,
// search, attribute filter, sort/group/display settings, and pagination for
// the un-scoped issues view. Composes use-task-page-linear-issues-fetch for
// the actual network fetch.
const TASK_SEARCH_DEBOUNCE_MS = 300
const LINEAR_ITEM_LIMIT = 36

type UseTaskPageLinearIssuesStateParams = {
  taskSource: TaskProvider
  taskResumeApplied: boolean
  linearConnected: boolean
  selectedLinearWorkspaceId: string | null
  linearTaskSourceContext: TaskSourceContext | null | undefined
  linearListInvalidationVersionForSource: number
  selectedLinearProjectId: string | undefined
  selectedLinearCustomViewId: string | undefined
  // Why: the mode/refresh-nonce are shared across the issues/projects/
  // custom-views slices (mode gates all three fetches; the nonce forces a
  // refetch across all three), so both are owned by the browse-state
  // orchestrator rather than this slice.
  linearMode: LinearMode
  linearRefreshNonce: number
}

export function useTaskPageLinearIssuesState({
  taskSource,
  taskResumeApplied,
  linearConnected,
  selectedLinearWorkspaceId,
  linearTaskSourceContext,
  linearListInvalidationVersionForSource,
  selectedLinearProjectId,
  selectedLinearCustomViewId,
  linearMode,
  linearRefreshNonce
}: UseTaskPageLinearIssuesStateParams) {
  const setTaskResumeState = useAppStore((s) => s.setTaskResumeState)

  const [linearIssues, setLinearIssues] = useState<LinearIssue[]>([])
  const [linearIssueLimit, setLinearIssueLimit] = useState(LINEAR_ITEM_LIMIT)
  const [linearIssuePage, setLinearIssuePage] = useState(0)
  const [linearIssueLoadingTargetPage, setLinearIssueLoadingTargetPage] = useState<number | null>(
    null
  )
  const [linearIssuesHasMore, setLinearIssuesHasMore] = useState(false)
  const [linearLoading, setLinearLoading] = useState(false)
  const [linearError, setLinearError] = useState<string | null>(null)
  const [linearSearchInput, setLinearSearchInput] = useState('')
  const [appliedLinearSearch, setAppliedLinearSearch] = useState('')
  const [linearAttributeFilter, setLinearAttributeFilter] = useState<LinearIssueAttributeFilter>(
    () => emptyLinearIssueAttributeFilter()
  )
  const [linearViewMode, setLinearViewMode] = useState<LinearViewMode>('list')
  const [linearGroupBy, setLinearGroupBy] = useState<LinearGroupBy>('none')
  const [linearOrderBy, setLinearOrderBy] = useState<LinearOrderBy>('priority')
  const [linearDisplayProperties, setLinearDisplayProperties] = useState<
    ReadonlySet<LinearDisplayProperty>
  >(() => new Set(DEFAULT_LINEAR_DISPLAY_PROPERTIES))
  const [linearTeamPropertyTouched, setLinearTeamPropertyTouched] = useState(false)

  const linearSearchPersistReadyRef = useRef(false)

  const applyLinearAttributeFilter = useCallback((next: LinearIssueAttributeFilter) => {
    // Why: batch filter + limit/page reset in one transition so the fetch
    // effect never issues an old expanded-limit request for the new filter.
    setLinearAttributeFilter(next)
    setLinearIssueLimit(LINEAR_ITEM_LIMIT)
    setLinearIssuePage(0)
    setLinearIssueLoadingTargetPage(null)
  }, [])

  const toggleLinearDisplayProperty = useCallback((property: LinearDisplayProperty): void => {
    if (property === 'team') {
      setLinearTeamPropertyTouched(true)
    }
    setLinearDisplayProperties((prev) => {
      const next = new Set(prev)
      if (next.has(property)) {
        next.delete(property)
      } else {
        next.add(property)
      }
      return next
    })
  }, [])

  // Why: debounce the Linear search input so we don't fire a request on every
  // keystroke — matches the 300ms cadence used for GitHub search.
  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    const timeout = window.setTimeout(() => {
      setAppliedLinearSearch(linearSearchInput)
    }, TASK_SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timeout)
  }, [linearSearchInput, taskResumeApplied])

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    if (!linearSearchPersistReadyRef.current) {
      linearSearchPersistReadyRef.current = true
      return
    }
    setTaskResumeState({ linearQuery: appliedLinearSearch.trim() })
  }, [appliedLinearSearch, setTaskResumeState, taskResumeApplied])

  useEffect(() => {
    setLinearIssueLimit(LINEAR_ITEM_LIMIT)
    setLinearIssuePage(0)
    setLinearIssueLoadingTargetPage(null)
  }, [
    appliedLinearSearch,
    linearMode,
    selectedLinearCustomViewId,
    selectedLinearProjectId,
    selectedLinearWorkspaceId,
    taskSource
  ])

  useTaskPageLinearIssuesFetch({
    taskResumeApplied,
    taskSource,
    linearMode,
    linearConnected,
    selectedLinearWorkspaceId,
    appliedLinearSearch,
    linearIssueLimit,
    linearRefreshNonce,
    linearAttributeFilter,
    linearListInvalidationVersionForSource,
    linearTaskSourceContext,
    setLinearError,
    setLinearIssuesHasMore,
    setLinearIssues,
    setLinearLoading
  })

  return {
    linearIssues,
    setLinearIssues,
    linearIssueLimit,
    setLinearIssueLimit,
    linearIssuePage,
    setLinearIssuePage,
    linearIssueLoadingTargetPage,
    setLinearIssueLoadingTargetPage,
    linearIssuesHasMore,
    linearLoading,
    setLinearLoading,
    linearError,
    setLinearError,
    linearSearchInput,
    setLinearSearchInput,
    appliedLinearSearch,
    setAppliedLinearSearch,
    linearAttributeFilter,
    applyLinearAttributeFilter,
    linearViewMode,
    setLinearViewMode,
    linearGroupBy,
    setLinearGroupBy,
    linearOrderBy,
    setLinearOrderBy,
    linearDisplayProperties,
    linearTeamPropertyTouched,
    toggleLinearDisplayProperty
  }
}
