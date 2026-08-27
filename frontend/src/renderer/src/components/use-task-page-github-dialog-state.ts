import { useCallback, useEffect, useMemo, useState } from 'react'
import { useAppStore } from '@/store'
import { findTaskPageDialogWorkItem } from '@/components/task-page-cache-selectors'
import type { ItemDialogTab } from '@/components/GitHubItemDialog'
import type { GitHubWorkItem, Repo } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of use-task-page-github-state.ts (TASK-BIGFILE-236) to stay
// under the 300-line file budget — this is the GitHub item-dialog slice:
// which work item the dialog shows, and the deep-link effect that opens it.
type UseTaskPageGitHubDialogStateParams = {
  repoMap: ReadonlyMap<string, Repo>
  openGitHubWorkItem: GitHubWorkItem | null | undefined
  openGitHubInitialTab: ItemDialogTab | undefined
  openGitHubSourceContext: TaskSourceContext | null | undefined
  setGithubMode: (mode: 'items' | 'project') => void
  setPages: React.Dispatch<React.SetStateAction<GitHubWorkItem[][]>>
  getTaskPageRepoSourceContext: (
    repo: Repo | null | undefined,
    provider: 'github' | 'gitlab'
  ) => TaskSourceContext | null
}

export function useTaskPageGitHubDialogState({
  repoMap,
  openGitHubWorkItem,
  openGitHubInitialTab,
  openGitHubSourceContext,
  setGithubMode,
  setPages,
  getTaskPageRepoSourceContext
}: UseTaskPageGitHubDialogStateParams) {
  // Why: clicking a GitHub row (or completing the create-issue flow) opens
  // this dialog for a read/review surface. The dialog's "Use" button routes
  // through the same direct-launch flow as the row-level "Use" CTA so
  // behavior is consistent regardless of entry point.
  const githubTaskDrawerWorkItem = useAppStore((s) => s.githubTaskDrawerWorkItem)
  const setGithubTaskDrawerWorkItem = useAppStore((s) => s.setGithubTaskDrawerWorkItem)
  const [dialogInitialTab, setDialogInitialTab] = useState<ItemDialogTab>('conversation')
  const dialogWorkItemKey = githubTaskDrawerWorkItem
    ? { id: githubTaskDrawerWorkItem.id, repoId: githubTaskDrawerWorkItem.repoId }
    : null

  // Why: derive the dialog's work item from the store cache so it reflects
  // optimistic patches (e.g. table-cell status toggle). Falls back to the
  // snapshot stored at click time for newly-created stubs not yet in the cache.
  // Disambiguates by repoId so issues with the same number fetched from
  // multiple repos (e.g. fork + non-fork, both routed through the same
  // upstream) resolve to the clicked row's repo, not the first one scanned.
  const cachedDialogWorkItem = useAppStore((s) =>
    findTaskPageDialogWorkItem(s.workItemsCache, dialogWorkItemKey)
  )
  const dialogWorkItem = dialogWorkItemKey
    ? (cachedDialogWorkItem ?? githubTaskDrawerWorkItem)
    : null
  const dialogRepoPath = dialogWorkItem ? (repoMap.get(dialogWorkItem.repoId)?.path ?? null) : null
  const dialogSourceContext = useMemo(() => {
    if (!dialogWorkItem) {
      return null
    }
    if (
      openGitHubSourceContext?.provider === 'github' &&
      openGitHubWorkItem?.id === dialogWorkItem.id &&
      openGitHubWorkItem.repoId === dialogWorkItem.repoId
    ) {
      return openGitHubSourceContext
    }
    return getTaskPageRepoSourceContext(repoMap.get(dialogWorkItem.repoId), 'github')
  }, [
    dialogWorkItem,
    getTaskPageRepoSourceContext,
    openGitHubSourceContext,
    openGitHubWorkItem,
    repoMap
  ])

  const setDialogWorkItem = useCallback(
    (item: GitHubWorkItem | null, initialTab: ItemDialogTab = 'conversation') => {
      setDialogInitialTab(item ? initialTab : 'conversation')
      setGithubTaskDrawerWorkItem(item)
    },
    [setGithubTaskDrawerWorkItem]
  )

  useEffect(() => {
    if (!openGitHubWorkItem) {
      setDialogWorkItem(null)
      return
    }
    setGithubMode('items')
    setDialogWorkItem(openGitHubWorkItem, openGitHubInitialTab)
  }, [openGitHubInitialTab, openGitHubWorkItem, setDialogWorkItem, setGithubMode])

  const patchTaskPageWorkItemRows = useCallback(
    (
      itemKey: { id: string; repoId: string },
      patch: Partial<GitHubWorkItem>,
      shouldPatch?: (item: GitHubWorkItem) => boolean
    ): void => {
      setPages((current) => {
        let changed = false
        const nextPages = current.map((page) => {
          let pageChanged = false
          const nextPage = page.map((item) => {
            if (item.id !== itemKey.id || item.repoId !== itemKey.repoId) {
              return item
            }
            if (shouldPatch && !shouldPatch(item)) {
              return item
            }
            pageChanged = true
            changed = true
            return { ...item, ...patch }
          })
          return pageChanged ? nextPage : page
        })
        return changed ? nextPages : current
      })
    },
    [setPages]
  )

  return {
    dialogInitialTab,
    dialogWorkItem,
    dialogRepoPath,
    dialogSourceContext,
    setDialogWorkItem,
    patchTaskPageWorkItemRows
  }
}
