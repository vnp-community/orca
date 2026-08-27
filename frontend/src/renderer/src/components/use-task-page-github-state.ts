import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { PER_REPO_FETCH_LIMIT, CROSS_REPO_DISPLAY_LIMIT } from '@/lib/new-workspace'
import type { TaskSourceContext } from '../../../shared/task-source-context'
import type { ItemDialogTab } from '@/components/GitHubItemDialog'
import { useTaskPageGitHubDialogState } from '@/components/use-task-page-github-dialog-state'
import { useTaskPageGitHubNewIssueDraftState } from '@/components/use-task-page-github-new-issue-draft-state'
import { useTaskPageGitHubWorkItemsCache } from '@/components/use-task-page-github-work-items-cache'
import { useTaskPageGitHubWorkItemsFetch } from '@/components/use-task-page-github-work-items-fetch'
import { useTaskPageGitHubPRChecks } from '@/components/use-task-page-github-pr-checks'
import { getGitHubTaskKind } from '@/components/task-page-github-task-kind'
import type { GitHubWorkItem, Repo, TaskProvider, TaskViewPresetId } from '../../../shared/types'

export {
  getDefaultPresetForGitHubTaskKind,
  scopeGitHubTaskSearch
} from '@/components/task-page-github-task-kind'

type UseTaskPageGitHubStateParams = {
  taskSource: TaskProvider
  taskResumeApplied: boolean
  selectedRepos: readonly Repo[]
  repoMap: ReadonlyMap<string, Repo>
  defaultTaskViewPreset: TaskViewPresetId
  initialTaskQuery: string
  openGitHubWorkItem: GitHubWorkItem | null | undefined
  openGitHubInitialTab: ItemDialogTab | undefined
  openGitHubSourceContext: TaskSourceContext | null | undefined
  // Why: pure module-scope helper owned by TaskPage.tsx and shared across
  // every provider domain — passed in rather than imported back from
  // TaskPage.tsx to avoid a circular module dependency.
  getTaskPageRepoSourceContext: (
    repo: Repo | null | undefined,
    provider: 'github' | 'gitlab'
  ) => TaskSourceContext | null
}

export function useTaskPageGitHubState({
  taskSource,
  taskResumeApplied,
  selectedRepos,
  repoMap,
  defaultTaskViewPreset,
  initialTaskQuery,
  openGitHubWorkItem,
  openGitHubInitialTab,
  openGitHubSourceContext,
  getTaskPageRepoSourceContext
}: UseTaskPageGitHubStateParams) {
  const getCachedWorkItems = useAppStore((s) => s.getCachedWorkItems)
  const workItemsInvalidationNonce = useAppStore((s) => s.workItemsInvalidationNonce)

  const [githubMode, setGithubMode] = useState<'items' | 'project'>('items')

  const [taskSearchInput, setTaskSearchInput] = useState(initialTaskQuery)
  const [appliedTaskSearch, setAppliedTaskSearch] = useState(initialTaskQuery)
  const [activeTaskPreset, setActiveTaskPreset] = useState<TaskViewPresetId | null>(
    defaultTaskViewPreset
  )
  const [tasksLoading, setTasksLoading] = useState(false)
  const [tasksRefreshing, setTasksRefreshing] = useState(false)
  const [tasksFiltering, setTasksFiltering] = useState(false)
  const [tasksError, setTasksError] = useState<string | null>(null)
  // Why: per-repo failure count surfaced through the "N of M" banner. IPC-level
  // rejections populate tasksError instead — the two are mutually exclusive so
  // a successful-with-partial-failure read and a hard-reject don't double-show.
  const [failedCount, setFailedCount] = useState(0)
  const [taskRefreshNonce, setTaskRefreshNonce] = useState(0)
  const paginationGenerationRef = useRef(0)
  // Why: pages holds all fetched pages of work items. Page 0 is seeded from
  // cache for instant first paint; subsequent pages are loaded via date cursors.
  const [pages, setPages] = useState<GitHubWorkItem[][]>(() => {
    const trimmed = initialTaskQuery.trim()
    const merged: GitHubWorkItem[] = []
    for (const r of selectedRepos) {
      const cached = getCachedWorkItems(
        r.id,
        PER_REPO_FETCH_LIMIT,
        trimmed,
        r.path,
        getTaskPageRepoSourceContext(r, 'github')
      )
      if (cached) {
        merged.push(...cached)
      }
    }
    if (merged.length === 0) {
      return [[]]
    }
    const page0 = [...merged]
      .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
      .slice(0, CROSS_REPO_DISPLAY_LIMIT)
    return [page0]
  })
  const [currentPage, setCurrentPage] = useState(0)
  const [paginationLoading, setPaginationLoading] = useState(false)
  const [loadingTargetPage, setLoadingTargetPage] = useState<number | null>(null)
  const [totalItemCount, setTotalItemCount] = useState<number | null>(null)

  useEffect(() => {
    paginationGenerationRef.current += 1
    setPaginationLoading(false)
    setLoadingTargetPage(null)
  }, [selectedRepos, appliedTaskSearch, workItemsInvalidationNonce])

  const {
    dialogInitialTab,
    dialogWorkItem,
    dialogRepoPath,
    dialogSourceContext,
    setDialogWorkItem,
    patchTaskPageWorkItemRows
  } = useTaskPageGitHubDialogState({
    repoMap,
    openGitHubWorkItem,
    openGitHubInitialTab,
    openGitHubSourceContext,
    setGithubMode,
    setPages,
    getTaskPageRepoSourceContext
  })

  const { perRepoSourceState } = useTaskPageGitHubWorkItemsCache({
    taskSource,
    githubMode,
    selectedRepos,
    appliedTaskSearch,
    setPages,
    getTaskPageRepoSourceContext
  })

  // Why: track retry-in-flight per selected source so that clicking Retry
  // on one banner only flips that source's button into its "Retrying…"
  // state — other still-failing banners stay in their "Retry" state rather
  // than misleadingly flipping in lockstep. The fetch sub-hook clears the
  // set when the nonce-driven refresh settles.
  const [retryingSourceKeys, setRetryingSourceKeys] = useState<ReadonlySet<string>>(() => new Set())

  const {
    newIssueOpen,
    setNewIssueOpen,
    newIssueTitle,
    setNewIssueTitle,
    newIssueBody,
    setNewIssueBody,
    newIssueLabels,
    setNewIssueLabels,
    newIssueAssignees,
    setNewIssueAssignees,
    newIssueSubmitting,
    setNewIssueSubmitting,
    newIssueRepoId,
    setNewIssueRepoId
  } = useTaskPageGitHubNewIssueDraftState({ selectedRepos })

  const activeGithubTaskKind = getGitHubTaskKind(activeTaskPreset, appliedTaskSearch)
  const applyTypeFilter = useCallback(
    (items: GitHubWorkItem[]) => {
      return items.filter((item) => {
        return activeGithubTaskKind === 'prs' ? item.type === 'pr' : item.type === 'issue'
      })
    },
    [activeGithubTaskKind]
  )
  const currentPageItems = useMemo(() => pages[currentPage] ?? [], [pages, currentPage])
  const filteredWorkItems = useMemo(
    () => applyTypeFilter(currentPageItems),
    [applyTypeFilter, currentPageItems]
  )
  const showPRManagementColumns = activeGithubTaskKind === 'prs'

  const { ensurePRChecksLoaded } = useTaskPageGitHubPRChecks({
    taskSource,
    githubMode,
    repoMap,
    filteredWorkItems,
    showPRManagementColumns,
    patchTaskPageWorkItemRows,
    getTaskPageRepoSourceContext
  })

  useTaskPageGitHubWorkItemsFetch({
    taskSource,
    taskResumeApplied,
    githubMode,
    selectedRepos,
    taskSearchInput,
    appliedTaskSearch,
    activeTaskPreset,
    activeGithubTaskKind,
    taskRefreshNonce,
    retryingSourceKeys,
    setAppliedTaskSearch,
    setTasksFiltering,
    setTasksLoading,
    setTasksRefreshing,
    setTasksError,
    setFailedCount,
    setPages,
    setCurrentPage,
    setTotalItemCount,
    setRetryingSourceKeys,
    getTaskPageRepoSourceContext
  })

  return {
    githubMode,
    setGithubMode,
    taskSearchInput,
    setTaskSearchInput,
    appliedTaskSearch,
    setAppliedTaskSearch,
    activeTaskPreset,
    setActiveTaskPreset,
    tasksLoading,
    tasksRefreshing,
    setTasksRefreshing,
    tasksFiltering,
    setTasksFiltering,
    tasksError,
    failedCount,
    taskRefreshNonce,
    setTaskRefreshNonce,
    pages,
    setPages,
    currentPage,
    setCurrentPage,
    paginationLoading,
    setPaginationLoading,
    loadingTargetPage,
    setLoadingTargetPage,
    totalItemCount,
    paginationGenerationRef,
    dialogInitialTab,
    dialogWorkItem,
    dialogRepoPath,
    dialogSourceContext,
    setDialogWorkItem,
    patchTaskPageWorkItemRows,
    perRepoSourceState,
    retryingSourceKeys,
    setRetryingSourceKeys,
    newIssueOpen,
    setNewIssueOpen,
    newIssueTitle,
    setNewIssueTitle,
    newIssueBody,
    setNewIssueBody,
    newIssueLabels,
    setNewIssueLabels,
    newIssueAssignees,
    setNewIssueAssignees,
    newIssueSubmitting,
    setNewIssueSubmitting,
    newIssueRepoId,
    setNewIssueRepoId,
    activeGithubTaskKind,
    filteredWorkItems,
    showPRManagementColumns,
    ensurePRChecksLoaded
  }
}
