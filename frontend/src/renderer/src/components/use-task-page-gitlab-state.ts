import { useEffect, useMemo, useState } from 'react'
import { getRepoBackedTaskEmptyState } from '@/components/task-page-empty-state'
import type { GitLabIssueFilter, GitLabTaskFilter } from '@/components/task-page-localized-options'
import type { GitLabTodo, GitLabWorkItem, Repo, TaskProvider } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: kept module-private (not exported) — used only by the GitLab fetch
// effect below, mirrors the original TaskPage.tsx placement.
function isGitLabMRFilter(value: GitLabTaskFilter | GitLabIssueFilter): value is GitLabTaskFilter {
  return value === 'opened' || value === 'merged' || value === 'closed' || value === 'all'
}

function isGitLabIssueFilter(
  value: GitLabTaskFilter | GitLabIssueFilter
): value is GitLabIssueFilter {
  return value === 'opened' || value === 'assigned-to-me'
}

type UseTaskPageGitLabStateParams = {
  taskSource: TaskProvider
  selectedRepos: readonly Repo[]
  openGitLabWorkItem: GitLabWorkItem | null | undefined
  // Why: pure module-scope helper owned by TaskPage.tsx and shared across
  // every provider domain — passed in rather than imported back from
  // TaskPage.tsx to avoid a circular module dependency.
  getTaskPageRepoSourceContext: (
    repo: Repo | null | undefined,
    provider: 'github' | 'gitlab'
  ) => TaskSourceContext | null
}

export function useTaskPageGitLabState({
  taskSource,
  selectedRepos,
  openGitLabWorkItem,
  getTaskPageRepoSourceContext
}: UseTaskPageGitLabStateParams) {
  // ── GitLab task-source state ──────────────────────────────────────
  // Why: parallel to Linear's slim per-source state. Skips workItemsCache
  // and cross-repo aggregation in v1 — the GitLab list fetches directly
  // from `window.api.gl.listMRs` / `listIssues` for the primary repo.
  const [gitlabFilter, setGitlabFilter] = useState<GitLabTaskFilter | GitLabIssueFilter>('opened')
  const [gitlabItems, setGitlabItems] = useState<GitLabWorkItem[]>([])
  const [gitlabLoading, setGitlabLoading] = useState(false)
  const [gitlabError, setGitlabError] = useState<string | null>(null)
  const [gitlabRefreshNonce, setGitlabRefreshNonce] = useState(0)
  // Why: opens GitLabItemDialog when a row is clicked. Separate state from
  // gitlabItems so the dialog target survives a list refresh that might
  // remove the item from the visible filter (e.g. closing an MR while
  // it's open in the dialog).
  const [gitlabDialogItem, setGitlabDialogItem] = useState<GitLabWorkItem | null>(null)

  // Why: GitLab tab has two sub-views — the project's MR/issue list,
  // and the user's cross-project Todos (gitlab.com/dashboard/todos).
  // 'project' is default; 'todos' fetches a separate stream.
  const [gitlabView, setGitlabView] = useState<'issues' | 'mrs' | 'todos'>('mrs')
  const [gitlabTodos, setGitlabTodos] = useState<GitLabTodo[]>([])
  const [gitlabTodosLoading, setGitlabTodosLoading] = useState(false)
  const gitlabEmptyState = useMemo(
    () =>
      getRepoBackedTaskEmptyState({
        provider: 'gitlab',
        selectedRepoCount: selectedRepos.length,
        gitlabView
      }),
    [gitlabView, selectedRepos.length]
  )

  const gitlabFilterIsValid =
    gitlabView === 'issues'
      ? isGitLabIssueFilter(gitlabFilter)
      : gitlabView === 'mrs'
        ? isGitLabMRFilter(gitlabFilter)
        : true
  const activeGitlabFilter = gitlabFilterIsValid ? gitlabFilter : 'opened'
  // Why: Issues and MRs expose different filter sets; repair before commit so
  // fetch Effects never issue `glab` with a stale filter from the other view.
  if (!gitlabFilterIsValid) {
    setGitlabFilter('opened')
  }

  const displayedGitLabItems = useMemo(() => {
    if (gitlabView === 'issues') {
      return gitlabItems.filter((item) => item.type === 'issue')
    }
    if (gitlabView === 'mrs') {
      return gitlabItems.filter((item) => item.type === 'mr')
    }
    return gitlabItems
  }, [gitlabItems, gitlabView])

  useEffect(() => {
    setGitlabDialogItem(openGitLabWorkItem ?? null)
  }, [openGitLabWorkItem])

  const primaryRepo = selectedRepos[0] ?? null

  // Why: stable key for `selectedRepos` so the GitLab fetch effect below
  // doesn't re-run on every parent re-render just because the array
  // reference changed. The memoized string keys off id + path +
  // connectionId — the only fields the effect actually reads.
  const selectedReposKey = useMemo(
    () =>
      selectedRepos
        .map((r) => `${r.id}|${r.path}|${r.connectionId ?? ''}|${r.executionHostId ?? ''}`)
        .join(','),
    [selectedRepos]
  )

  // Why: GitLab task-source data fetch. Issues and MRs are fetched
  // separately (mirrors GitHub's separate Issues / PRs endpoints) so
  // errors are isolated per tab and the backend doesn't need a combined
  // merge+sort that can hide failures.
  useEffect(() => {
    if (taskSource !== 'gitlab') {
      return
    }
    if (gitlabView === 'todos') {
      return
    }
    const activeIssueFilter =
      gitlabView === 'issues' && isGitLabIssueFilter(activeGitlabFilter) ? activeGitlabFilter : null
    const activeMRFilter =
      gitlabView === 'mrs' && isGitLabMRFilter(activeGitlabFilter) ? activeGitlabFilter : null
    if (
      (gitlabView === 'issues' && !activeIssueFilter) ||
      (gitlabView === 'mrs' && !activeMRFilter)
    ) {
      return
    }
    // Why: folder-mode repos have no remotes to derive a GitLab project from;
    // SSH-backed Git repos go through the same provider-aware IPC path.
    const eligibleRepos = selectedRepos
    if (eligibleRepos.length === 0) {
      setGitlabItems([])
      setGitlabLoading(false)
      setGitlabError(null)
      return
    }
    let stale = false
    setGitlabLoading(true)
    setGitlabError(null)

    const fetchItems =
      gitlabView === 'issues'
        ? (repo: (typeof eligibleRepos)[0]) => {
            const isAssignedToMe = activeIssueFilter === 'assigned-to-me'
            return window.api.gl
              .listIssues({
                repoPath: repo.path,
                repoId: repo.id,
                sourceContext: getTaskPageRepoSourceContext(repo, 'gitlab'),
                state: 'opened',
                assignee: isAssignedToMe ? '@me' : undefined,
                limit: 50
              })
              .then((result) => {
                const typed = result as {
                  items: GitLabWorkItem[]
                  error?: { type?: string; message: string }
                }
                // Why: not_found just means "this repo isn't a GitLab project"
                // (e.g. a GitHub-only repo in a mixed selection). Drop it
                // silently so the GitLab list doesn't show false errors.
                const error = typed.error?.type === 'not_found' ? undefined : typed.error
                return { repoId: repo.id, items: typed.items, error }
              })
          }
        : (repo: (typeof eligibleRepos)[0]) =>
            window.api.gl
              .listMRs({
                repoPath: repo.path,
                repoId: repo.id,
                sourceContext: getTaskPageRepoSourceContext(repo, 'gitlab'),
                state: activeMRFilter ?? 'opened',
                page: 1,
                perPage: 50
              })
              .then((result) => {
                const typed = result as {
                  items: GitLabWorkItem[]
                  error?: { type?: string; message: string }
                }
                const error = typed.error?.type === 'not_found' ? undefined : typed.error
                return { repoId: repo.id, items: typed.items, error }
              })

    void Promise.allSettled(eligibleRepos.map(fetchItems))
      .then((results) => {
        if (stale) {
          return
        }
        const merged: GitLabWorkItem[] = []
        const errs: string[] = []
        for (const r of results) {
          if (r.status !== 'fulfilled') {
            errs.push(r.reason instanceof Error ? r.reason.message : String(r.reason))
            continue
          }
          for (const item of r.value.items) {
            merged.push({ ...item, repoId: r.value.repoId })
          }
          if (r.value.error) {
            errs.push(r.value.error.message)
          }
        }
        merged.sort((a, b) => (b.updatedAt ?? '').localeCompare(a.updatedAt ?? ''))
        setGitlabItems(merged)
        // Why: only surface an error banner when EVERY eligible repo failed.
        // Mixed selections often include non-GitLab repos, and a partial
        // banner would hide the working rows.
        if (errs.length > 0 && merged.length === 0) {
          setGitlabError(errs[0])
        }
      })
      .finally(() => {
        if (!stale) {
          setGitlabLoading(false)
        }
      })
    return () => {
      stale = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- selectedReposKey encodes the only selectedRepos fields read above; keying off the array ref would re-run on every parent render.
  }, [
    taskSource,
    gitlabView,
    activeGitlabFilter,
    gitlabRefreshNonce,
    selectedReposKey,
    getTaskPageRepoSourceContext
  ])

  // Why: Todos fetch lives in its own effect — different trigger
  // condition from the project view (no chip filter dependence) and a
  // different data path (`gl.todos` is user-scoped, not repo-scoped).
  useEffect(() => {
    if (taskSource !== 'gitlab' || gitlabView !== 'todos') {
      return
    }
    if (!primaryRepo?.path) {
      setGitlabTodos([])
      setGitlabTodosLoading(false)
      return
    }
    let stale = false
    setGitlabTodosLoading(true)
    void window.api.gl
      .todos({
        repoPath: primaryRepo.path,
        repoId: primaryRepo.id,
        sourceContext: getTaskPageRepoSourceContext(primaryRepo, 'gitlab')
      })
      .then((todos) => {
        if (!stale) {
          setGitlabTodos(todos as GitLabTodo[])
        }
      })
      .catch(() => {
        if (!stale) {
          setGitlabTodos([])
        }
      })
      .finally(() => {
        if (!stale) {
          setGitlabTodosLoading(false)
        }
      })
    return () => {
      stale = true
    }
  }, [taskSource, gitlabView, gitlabRefreshNonce, primaryRepo, getTaskPageRepoSourceContext])

  return {
    gitlabFilter,
    setGitlabFilter,
    gitlabItems,
    setGitlabItems,
    gitlabLoading,
    setGitlabLoading,
    gitlabError,
    setGitlabError,
    gitlabRefreshNonce,
    setGitlabRefreshNonce,
    gitlabDialogItem,
    setGitlabDialogItem,
    gitlabView,
    setGitlabView,
    gitlabTodos,
    setGitlabTodos,
    gitlabTodosLoading,
    setGitlabTodosLoading,
    gitlabEmptyState,
    activeGitlabFilter,
    displayedGitLabItems
  }
}
