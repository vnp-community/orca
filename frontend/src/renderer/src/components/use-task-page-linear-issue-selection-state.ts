import { useCallback, useEffect, useMemo, useState } from 'react'
import { useShallow } from 'zustand/react/shallow'
import { useAppStore } from '@/store'
import { findTaskPageLinearIssue } from '@/components/task-page-cache-selectors'
import type { LinearIssue } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) — the "which Linear
// issue is open" slice: selection id/fallback/float-outside-list state, the
// cache-joined selected issue, and the deep-link-open handlers. Kept apart
// from the browse/pagination slice because it's read by task-page-linear-
// draft-state (TASK-BIGFILE-239, already extracted) as well.
type UseTaskPageLinearIssueSelectionStateParams = {
  openLinearIssue: LinearIssue | null | undefined
  openLinearSourceContext: TaskSourceContext | null | undefined
  linearTaskSourceContext: TaskSourceContext | null | undefined
}

export function useTaskPageLinearIssueSelectionState({
  openLinearIssue,
  openLinearSourceContext,
  linearTaskSourceContext
}: UseTaskPageLinearIssueSelectionStateParams) {
  const openTaskPage = useAppStore((s) => s.openTaskPage)

  const [selectedLinearIssueId, setSelectedLinearIssueId] = useState<string | null>(null)
  const [selectedLinearIssueFallback, setSelectedLinearIssueFallback] =
    useState<LinearIssue | null>(null)
  const [selectedLinearIssueCanFloat, setSelectedLinearIssueCanFloat] = useState(false)

  // Why: the Linear list keeps its own fetched array, while cell edits patch
  // the shared caches. Subscribing to just the Linear caches lets the list and
  // inline detail reflect optimistic mutations without a second durable cache.
  const linearCacheSnapshot = useAppStore(
    useShallow((s) => ({
      issueCache: s.linearIssueCache,
      searchCache: s.linearSearchCache,
      listCache: s.linearListCache
    }))
  )
  const cachedSelectedLinearIssue = findTaskPageLinearIssue(
    linearCacheSnapshot.issueCache,
    linearCacheSnapshot.searchCache,
    linearCacheSnapshot.listCache,
    selectedLinearIssueId
  )
  const selectedLinearIssue = selectedLinearIssueId
    ? (cachedSelectedLinearIssue ?? selectedLinearIssueFallback)
    : null
  const linearDetailSourceContext = useMemo(() => {
    if (
      selectedLinearIssue &&
      openLinearSourceContext?.provider === 'linear' &&
      openLinearIssue?.id === selectedLinearIssue.id
    ) {
      return openLinearSourceContext
    }
    return linearTaskSourceContext
  }, [linearTaskSourceContext, openLinearIssue, openLinearSourceContext, selectedLinearIssue])

  const setSelectedLinearIssue = useCallback(
    (issue: LinearIssue | null, options?: { allowOutsideList?: boolean }) => {
      setSelectedLinearIssueCanFloat(Boolean(issue && options?.allowOutsideList))
      setSelectedLinearIssueId(issue?.id ?? null)
      setSelectedLinearIssueFallback(issue)
    },
    []
  )

  const clearSelectedLinearIssue = useCallback(() => {
    setSelectedLinearIssueCanFloat(false)
    setSelectedLinearIssueId(null)
    setSelectedLinearIssueFallback(null)
  }, [])

  useEffect(() => {
    if (!openLinearIssue) {
      clearSelectedLinearIssue()
      return
    }
    setSelectedLinearIssue(openLinearIssue, { allowOutsideList: true })
  }, [clearSelectedLinearIssue, openLinearIssue, setSelectedLinearIssue])

  const openLinearDetailPage = useCallback(
    (issue: LinearIssue) => {
      openTaskPage(
        {
          taskSource: 'linear',
          openLinearIssue: issue,
          openLinearSourceContext: linearTaskSourceContext
        },
        { recordTasksInteraction: false }
      )
    },
    [linearTaskSourceContext, openTaskPage]
  )

  const openRelatedLinearIssue = useCallback(
    (issue: LinearIssue) => {
      openLinearDetailPage(issue)
    },
    [openLinearDetailPage]
  )

  return {
    selectedLinearIssueId,
    setSelectedLinearIssueId,
    selectedLinearIssueFallback,
    setSelectedLinearIssueFallback,
    selectedLinearIssueCanFloat,
    selectedLinearIssue,
    linearDetailSourceContext,
    setSelectedLinearIssue,
    clearSelectedLinearIssue,
    openLinearDetailPage,
    openRelatedLinearIssue
  }
}
