import { useEffect, useMemo, useRef } from 'react'
import { useShallow } from 'zustand/react/shallow'
import { toast } from 'sonner'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import { getTaskSourceCacheScope } from '../../../shared/task-source-context'
import { PER_REPO_FETCH_LIMIT } from '@/lib/new-workspace'
import {
  buildTaskPageRepoSourceState,
  reconcileTaskPagePagesWithWorkItemsCache,
  selectTaskPageWorkItemsCacheEntries,
  type TaskPageRepoSourceState
} from '@/components/task-page-cache-selectors'
import { stripRepoQualifiers } from '../../../shared/task-query'
import type { GitHubWorkItem, Repo, TaskProvider } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of use-task-page-github-state.ts (TASK-BIGFILE-236) to stay
// under the 300-line file budget — this is the `workItemsCache` read slice:
// per-repo source badges/retry state and the reconcile-into-`pages` +
// fell-back-to-origin-toast effects that react to cache writes.
function getTaskPageRepoCacheInput(
  repo: Repo,
  getTaskPageRepoSourceContext: (
    repo: Repo | null | undefined,
    provider: 'github' | 'gitlab'
  ) => TaskSourceContext | null
): {
  id: string
  path: string
  executionHostId?: string | null
  sourceCacheScope?: string | null
} {
  const sourceContext = getTaskPageRepoSourceContext(repo, 'github')
  return {
    id: repo.id,
    path: repo.path,
    executionHostId: repo.executionHostId,
    sourceCacheScope:
      sourceContext?.provider === 'github' ? getTaskSourceCacheScope(sourceContext) : null
  }
}

type UseTaskPageGitHubWorkItemsCacheParams = {
  taskSource: TaskProvider
  githubMode: 'items' | 'project'
  selectedRepos: readonly Repo[]
  appliedTaskSearch: string
  setPages: React.Dispatch<React.SetStateAction<GitHubWorkItem[][]>>
  getTaskPageRepoSourceContext: (
    repo: Repo | null | undefined,
    provider: 'github' | 'gitlab'
  ) => TaskSourceContext | null
}

export function useTaskPageGitHubWorkItemsCache({
  taskSource,
  githubMode,
  selectedRepos,
  appliedTaskSearch,
  setPages,
  getTaskPageRepoSourceContext
}: UseTaskPageGitHubWorkItemsCacheParams) {
  const appliedWorkItemsCacheQuery = useMemo(
    () => stripRepoQualifiers(appliedTaskSearch.trim()),
    [appliedTaskSearch]
  )
  const selectedWorkItemsCacheEntries = useAppStore(
    useShallow((s) =>
      selectTaskPageWorkItemsCacheEntries(
        s.workItemsCache,
        selectedRepos.map((repo) => getTaskPageRepoCacheInput(repo, getTaskPageRepoSourceContext)),
        PER_REPO_FETCH_LIMIT,
        appliedWorkItemsCacheQuery
      )
    )
  )

  // Why: feature 1 — render the "Issues from {owner}/{repo}" indicator per
  // selected repo whose issue-source and PR-source slugs differ, and surface
  // a per-repo retryable banner when the issue-side fetch failed. Both derive
  // from the same `workItemsCache` entry the list already consumes, so no
  // extra IPC round-trip is needed. The `TaskPageRepoSourceState` shape lives
  // with the cache selectors so the render and guard code share one contract.
  const perRepoSourceState = useMemo<TaskPageRepoSourceState[]>(
    () => buildTaskPageRepoSourceState(selectedRepos, selectedWorkItemsCacheEntries),
    [selectedRepos, selectedWorkItemsCacheEntries]
  )

  useEffect(() => {
    if (taskSource !== 'github' || githubMode !== 'items') {
      return
    }
    // Why: inline/dialog edits patch `workItemsCache`; the paged table renders
    // from a local snapshot so it needs the patched row objects copied across.
    setPages((current) =>
      reconcileTaskPagePagesWithWorkItemsCache(current, selectedWorkItemsCacheEntries)
    )
  }, [githubMode, selectedWorkItemsCacheEntries, setPages, taskSource])

  // Why: surface a one-time toast per session per repo when the user's
  // preferred `'upstream'` is no longer configured and we fell back to
  // origin. Gated on a ref-backed set so repeated list refreshes don't
  // re-toast. We deliberately do NOT auto-reset the preference — the user
  // may re-add `upstream` later and expect it to pick up again.
  const fellBackToastedRef = useRef<Set<string>>(new Set())
  useEffect(() => {
    if (taskSource !== 'github') {
      return
    }
    for (const [index, r] of selectedRepos.entries()) {
      const entry = selectedWorkItemsCacheEntries[index]
      if (!entry?.issueSourceFellBack) {
        continue
      }
      if (fellBackToastedRef.current.has(r.id)) {
        continue
      }
      const prSlug = entry.sources?.prs
        ? `${entry.sources.prs.owner}/${entry.sources.prs.repo}`
        : r.displayName
      toast.message(
        translate(
          'auto.components.TaskPage.f4374519ae',
          'Your preferred issue source (upstream) is no longer configured for {{value0}}. Using origin.',
          { value0: prSlug }
        )
      )
      fellBackToastedRef.current.add(r.id)
    }
  }, [selectedRepos, selectedWorkItemsCacheEntries, taskSource])

  return { perRepoSourceState, selectedWorkItemsCacheEntries }
}
