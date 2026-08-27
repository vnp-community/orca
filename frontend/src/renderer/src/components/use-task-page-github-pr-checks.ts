import { useCallback, useEffect } from 'react'
import { useAppStore } from '@/store'
import { deriveTaskPagePRCheckSummary } from '@/components/task-page-pr-check-summary'
import { sameGitHubOwnerRepo } from '@/components/github/IssueSourceIndicator'
import type { GitHubOwnerRepo, GitHubWorkItem, Repo, TaskProvider } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of use-task-page-github-state.ts (TASK-BIGFILE-236) to stay
// under the 300-line file budget — this is the PR-checks eager-prefetch
// slice: load check-run summaries for the first page of visible PR rows so
// the checks column doesn't pop in lazily on scroll.
const PR_CHECKS_EAGER_PREFETCH_LIMIT = 20

function sameOptionalGitHubOwnerRepo(
  left: GitHubOwnerRepo | null | undefined,
  right: GitHubOwnerRepo | null | undefined
): boolean {
  const leftValue = left ?? null
  const rightValue = right ?? null
  return leftValue === null && rightValue === null
    ? true
    : sameGitHubOwnerRepo(leftValue, rightValue)
}

type UseTaskPageGitHubPRChecksParams = {
  taskSource: TaskProvider
  githubMode: 'items' | 'project'
  repoMap: ReadonlyMap<string, Repo>
  filteredWorkItems: readonly GitHubWorkItem[]
  showPRManagementColumns: boolean
  patchTaskPageWorkItemRows: (
    itemKey: { id: string; repoId: string },
    patch: Partial<GitHubWorkItem>,
    shouldPatch?: (item: GitHubWorkItem) => boolean
  ) => void
  getTaskPageRepoSourceContext: (
    repo: Repo | null | undefined,
    provider: 'github' | 'gitlab'
  ) => TaskSourceContext | null
}

export function useTaskPageGitHubPRChecks({
  taskSource,
  githubMode,
  repoMap,
  filteredWorkItems,
  showPRManagementColumns,
  patchTaskPageWorkItemRows,
  getTaskPageRepoSourceContext
}: UseTaskPageGitHubPRChecksParams) {
  const fetchPRChecks = useAppStore((s) => s.fetchPRChecks)

  const ensurePRChecksLoaded = useCallback(
    (item: GitHubWorkItem): void => {
      if (item.type !== 'pr' || item.checksSummary) {
        return
      }
      const repo = repoMap.get(item.repoId)
      if (!repo) {
        return
      }
      const requestedHeadSha = item.headSha
      const requestedPRRepo = item.prRepo ?? null
      void fetchPRChecks(
        repo.path,
        item.number,
        item.branchName,
        item.headSha,
        item.prRepo ?? null,
        { repoId: repo.id, sourceContext: getTaskPageRepoSourceContext(repo, 'github') }
      ).then((checks) => {
        patchTaskPageWorkItemRows(
          { id: item.id, repoId: item.repoId },
          { checksSummary: deriveTaskPagePRCheckSummary(checks) },
          (currentItem) =>
            currentItem.type === 'pr' &&
            currentItem.headSha === requestedHeadSha &&
            sameOptionalGitHubOwnerRepo(currentItem.prRepo, requestedPRRepo)
        )
      })
    },
    [fetchPRChecks, getTaskPageRepoSourceContext, patchTaskPageWorkItemRows, repoMap]
  )

  useEffect(() => {
    if (taskSource !== 'github' || githubMode !== 'items' || !showPRManagementColumns) {
      return
    }

    for (const item of filteredWorkItems.slice(0, PR_CHECKS_EAGER_PREFETCH_LIMIT)) {
      ensurePRChecksLoaded(item)
    }
  }, [ensurePRChecksLoaded, filteredWorkItems, githubMode, showPRManagementColumns, taskSource])

  return { ensurePRChecksLoaded }
}
