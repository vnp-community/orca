/* eslint-disable max-lines -- Why: straight verbatim extraction of TaskPage.tsx's
GitHub review/PR cell components (ReviewChipAvatar, PRReviewCell, PRChecksCell,
PRMergeCell) plus their private helpers, which were already coupled at this size
inside TaskPage.tsx's own grandfathered max-lines disable before this move
(TASK-BIGFILE-030). Registered in config/max-lines-baseline.txt per AGENTS.md —
NEEDS PR REVIEW. Further internal splitting is a separate, un-tracked refactor
candidate; not addressed here to keep this a pure Move. */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useShallow } from 'zustand/react/shallow'
import {
  AlertCircle,
  Check,
  CheckCircle2,
  ChevronDown,
  Clock3,
  ExternalLink,
  GitMerge,
  LoaderCircle,
  Minus,
  Users
} from 'lucide-react'
import { toast } from 'sonner'

import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { Input } from '@/components/ui/input'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import {
  getGitHubPRPrimaryReviewer,
  getGitHubPRReviewerRows,
  getGitHubPRReviewLabel,
  normalizeGitHubReviewerLogins,
  parseGitHubReviewerInputLogins,
  type GitHubPRPrimaryReviewer
} from '@/components/github-pr-reviewer-display'
import {
  filterGitHubPRReviewerCandidates,
  getGitHubPRReviewerQueryState
} from '@/components/github/github-pr-reviewer-candidate-filter'
import { parseGitHubIssueOrPRLink } from '@/lib/github-links'
import { useRepoAssigneesBySlug } from '@/hooks/useGitHubSlugMetadata'
import { getSettingsForRepoRuntimeOwner } from '@/lib/repo-runtime-owner'
import { cn } from '@/lib/utils'
import {
  getTaskSourceRuntimeSettings,
  type TaskSourceContext
} from '../../../shared/task-source-context'
import { presentGitHubPRMergeState } from '@/components/github-pr-merge-state'
import {
  GITHUB_PR_MERGE_METHOD_LABELS,
  resolveGitHubPRMergeMethods
} from '../../../shared/github-pr-merge-methods'
import { translate } from '@/i18n/i18n'
import type {
  GitHubAssignableUser,
  GitHubPRMergeMethod,
  GitHubWorkItem,
  Repo
} from '../../../shared/types'

export function ReviewChipAvatar({
  reviewer
}: {
  reviewer: GitHubPRPrimaryReviewer | null
}): React.JSX.Element {
  if (reviewer?.login) {
    // Why: `gh pr list --json reviewRequests` can return only logins; GitHub's
    // public avatar endpoint keeps the list visual aligned with assignee cells.
    const avatarUrl = reviewer.avatarUrl || `https://github.com/${reviewer.login}.png?size=40`
    return (
      <img
        src={avatarUrl}
        alt=""
        loading="lazy"
        decoding="async"
        title={reviewer.name ? `${reviewer.name} (${reviewer.login})` : reviewer.login}
        className="size-5 shrink-0 rounded-full border border-border/50 bg-muted object-cover"
      />
    )
  }
  return <Users className="size-5 shrink-0" />
}
function getChecksLabel(item: GitHubWorkItem): string {
  const summary = item.checksSummary
  if (!summary) {
    return 'Checks'
  }
  if (summary.total === 0) {
    return 'No checks'
  }
  if (summary.failed > 0) {
    return `${summary.failed} failing`
  }
  if (summary.pending > 0) {
    return `${summary.pending} pending`
  }
  return `${summary.passed}/${summary.total} passed`
}
function getChecksPillTone(item: GitHubWorkItem): string {
  const state = item.checksSummary?.state
  if (state === 'success') {
    return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-200'
  }
  if (state === 'failure') {
    return 'border-rose-500/30 bg-rose-500/10 text-rose-700 dark:text-rose-200'
  }
  if (state === 'pending') {
    return 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-200'
  }
  return 'border-border/60 bg-background/70 text-muted-foreground'
}
function mergeReviewerSuggestions(
  users: GitHubAssignableUser[],
  seedUsers: GitHubAssignableUser[]
): GitHubAssignableUser[] {
  const byLogin = new Map<string, GitHubAssignableUser>()
  for (const user of [...seedUsers, ...users]) {
    const key = user.login.toLowerCase()
    const existing = byLogin.get(key)
    if (!existing) {
      byLogin.set(key, user)
      continue
    }
    if (!existing.avatarUrl && user.avatarUrl) {
      byLogin.set(key, { ...existing, avatarUrl: user.avatarUrl })
    }
  }
  return Array.from(byLogin.values()).sort((a, b) => a.login.localeCompare(b.login))
}
function buildRequestedReviewUsers(
  logins: string[],
  candidates: GitHubAssignableUser[],
  existingRequests: GitHubAssignableUser[]
): GitHubAssignableUser[] {
  const byLogin = new Map<string, GitHubAssignableUser>()
  for (const user of existingRequests) {
    byLogin.set(user.login.toLowerCase(), user)
  }
  const candidatesByLogin = new Map(candidates.map((user) => [user.login.toLowerCase(), user]))
  for (const login of logins) {
    const key = login.toLowerCase()
    if (byLogin.has(key)) {
      continue
    }
    byLogin.set(key, candidatesByLogin.get(key) ?? { login, name: null, avatarUrl: '' })
  }
  return Array.from(byLogin.values())
}
export function PRReviewCell({
  item,
  repo,
  sourceContext
}: {
  item: GitHubWorkItem
  repo: Repo | null
  sourceContext?: TaskSourceContext | null
}): React.JSX.Element {
  const [open, setOpen] = useState(false)
  const [reviewerInput, setReviewerInput] = useState('')
  const [localReviewRequests, setLocalReviewRequests] = useState<GitHubAssignableUser[]>(
    () => item.reviewRequests ?? []
  )
  const [reviewerPickerSide, setReviewerPickerSide] = useState<'top' | 'bottom'>('bottom')
  const [reviewerPickerMaxHeight, setReviewerPickerMaxHeight] = useState<number | null>(null)
  const [reviewRequestsSource, setReviewRequestsSource] = useState(() => ({
    itemId: item.id,
    repoId: item.repoId,
    reviewRequests: item.reviewRequests
  }))
  const patchWorkItem = useAppStore((s) => s.patchWorkItem)
  const [activeReviewerCursor, setActiveReviewerCursor] = useState({ resetKey: '', index: 0 })
  const [submitting, setSubmitting] = useState(false)
  const repoOwnerSettings = useAppStore(
    useShallow((s) => getSettingsForRepoRuntimeOwner(s, repo?.id ?? null))
  )
  const sourceSettings = useMemo(
    () =>
      sourceContext?.provider === 'github'
        ? ({
            ...repoOwnerSettings,
            ...getTaskSourceRuntimeSettings(sourceContext)
          } as typeof repoOwnerSettings)
        : repoOwnerSettings,
    [repoOwnerSettings, sourceContext]
  )
  const reviewerInputRef = useRef<HTMLInputElement | null>(null)
  const reviewerTriggerRef = useRef<HTMLButtonElement | null>(null)
  const reviewerInputFocusFrameRef = useRef<number | null>(null)

  const cancelReviewerInputFocusFrame = useCallback((): void => {
    if (reviewerInputFocusFrameRef.current === null) {
      return
    }
    cancelAnimationFrame(reviewerInputFocusFrameRef.current)
    reviewerInputFocusFrameRef.current = null
  }, [])

  const setReviewerInputNode = useCallback(
    (node: HTMLInputElement | null): void => {
      // Why: the queued picker focus is only valid while this input is mounted.
      if (!node) {
        cancelReviewerInputFocusFrame()
      }
      reviewerInputRef.current = node
    },
    [cancelReviewerInputFocusFrame]
  )

  // Why: reviewer edits are optimistic, but item switches/refetches must clear
  // stale local requests before paint; a passive Effect leaves one stale render.
  if (
    reviewRequestsSource.itemId !== item.id ||
    reviewRequestsSource.repoId !== item.repoId ||
    reviewRequestsSource.reviewRequests !== item.reviewRequests
  ) {
    setReviewRequestsSource({
      itemId: item.id,
      repoId: item.repoId,
      reviewRequests: item.reviewRequests
    })
    setLocalReviewRequests(item.reviewRequests ?? [])
  }

  const reviewerSeedUsers = useMemo<GitHubAssignableUser[]>(() => {
    const byLogin = new Map<string, GitHubAssignableUser>()
    const add = (user: GitHubAssignableUser): void => {
      if (!user.login) {
        return
      }
      byLogin.set(user.login.toLowerCase(), user)
    }
    for (const user of localReviewRequests) {
      add(user)
    }
    for (const review of item.latestReviews ?? []) {
      add({
        login: review.login,
        name: null,
        avatarUrl: review.avatarUrl ?? ''
      })
    }
    if (item.author) {
      add({ login: item.author, name: null, avatarUrl: '' })
    }
    return Array.from(byLogin.values())
  }, [item.author, item.latestReviews, localReviewRequests])

  const reviewSlug = useMemo(() => parseGitHubIssueOrPRLink(item.url)?.slug ?? null, [item.url])
  const reviewerMetadata = useRepoAssigneesBySlug(
    open && reviewSlug ? reviewSlug.owner : null,
    open && reviewSlug ? reviewSlug.repo : null,
    reviewerSeedUsers.map((user) => user.login),
    sourceSettings
  )

  const authorLogin = item.author?.toLowerCase() ?? null
  const reviewerCandidates = useMemo(
    () =>
      mergeReviewerSuggestions(reviewerMetadata.data, reviewerSeedUsers).filter(
        (user) => user.login.toLowerCase() !== authorLogin
      ),
    [authorLogin, reviewerMetadata.data, reviewerSeedUsers]
  )
  const reviewerCandidatesByLogin = useMemo(
    () => new Map(reviewerCandidates.map((user) => [user.login.toLowerCase(), user])),
    [reviewerCandidates]
  )
  const selectedReviewerLogins = useMemo(
    () =>
      new Set(
        localReviewRequests.map((reviewer) => reviewer.login.trim().toLowerCase()).filter(Boolean)
      ),
    [localReviewRequests]
  )
  const reviewerQueryState = useMemo(
    () => getGitHubPRReviewerQueryState(reviewerInput),
    [reviewerInput]
  )
  const reviewerQuery = reviewerQueryState.query
  const filteredReviewerCandidates = useMemo(
    () =>
      filterGitHubPRReviewerCandidates({
        candidates: reviewerCandidates,
        queryState: reviewerQueryState
      }),
    [reviewerCandidates, reviewerQueryState]
  )
  const suggestedReviewerRows = useMemo(
    () =>
      reviewerQuery.length === 0 && !reviewerQueryState.isTooLarge
        ? reviewerSeedUsers
            .filter((user) => !selectedReviewerLogins.has(user.login.toLowerCase()))
            .filter((user) => user.login.toLowerCase() !== authorLogin)
            .map((user) => reviewerCandidatesByLogin.get(user.login.toLowerCase()) ?? user)
            .slice(0, 1)
        : [],
    [
      authorLogin,
      reviewerCandidatesByLogin,
      reviewerQuery.length,
      reviewerQueryState.isTooLarge,
      reviewerSeedUsers,
      selectedReviewerLogins
    ]
  )
  const everyoneElseReviewerRows = useMemo(() => {
    const suggestedLogins = new Set(suggestedReviewerRows.map((user) => user.login.toLowerCase()))
    return filteredReviewerCandidates.filter(
      (user) => !suggestedLogins.has(user.login.toLowerCase())
    )
  }, [filteredReviewerCandidates, suggestedReviewerRows])
  const actionableReviewerRows = useMemo(
    () => [...suggestedReviewerRows, ...everyoneElseReviewerRows],
    [everyoneElseReviewerRows, suggestedReviewerRows]
  )

  const reviewerCursorResetKey = `${reviewerQuery}\u0000${actionableReviewerRows.length}`
  if (activeReviewerCursor.resetKey !== reviewerCursorResetKey) {
    setActiveReviewerCursor({ resetKey: reviewerCursorResetKey, index: 0 })
  }
  const activeReviewerIndex =
    activeReviewerCursor.resetKey === reviewerCursorResetKey ? activeReviewerCursor.index : 0
  const setActiveReviewerIndex = useCallback(
    (nextIndex: number | ((current: number) => number)): void => {
      setActiveReviewerCursor((current) => {
        const currentIndex = current.resetKey === reviewerCursorResetKey ? current.index : 0
        return {
          resetKey: reviewerCursorResetKey,
          index: typeof nextIndex === 'function' ? nextIndex(currentIndex) : nextIndex
        }
      })
    },
    [reviewerCursorResetKey]
  )

  if (item.type !== 'pr') {
    return (
      <span className="text-[11px] text-muted-foreground">
        {translate('auto.components.TaskPage.b1eaa18ace', 'Issue')}
      </span>
    )
  }

  const itemWithLocalReviewRequests = { ...item, reviewRequests: localReviewRequests }
  const primaryReviewer = getGitHubPRPrimaryReviewer(itemWithLocalReviewRequests)
  const reviewerRows = getGitHubPRReviewerRows(itemWithLocalReviewRequests)
  const extraReviewerCount = Math.max(0, reviewerRows.length - 1)
  const hasReviewerMetadata =
    item.reviewDecision !== undefined ||
    localReviewRequests.length > 0 ||
    item.reviewRequests !== undefined ||
    item.latestReviews !== undefined

  const handleRequestReview = async (requestedLogins?: string[]): Promise<void> => {
    if (!repo || submitting) {
      return
    }
    const logins = normalizeGitHubReviewerLogins(
      requestedLogins ?? parseGitHubReviewerInputLogins(reviewerInput),
      selectedReviewerLogins
    )
    if (logins.length === 0) {
      toast.error(translate('auto.components.TaskPage.d00571d9b1', 'Enter a reviewer'))
      return
    }
    if (localReviewRequests.length + logins.length > 15) {
      toast.error(
        translate('auto.components.TaskPage.969e26577c', 'You can request up to 15 reviewers')
      )
      return
    }
    setSubmitting(true)
    try {
      const target = getActiveRuntimeTarget(sourceSettings)
      const runtimeRepoId =
        sourceContext?.provider === 'github' ? (sourceContext.repoId ?? repo.id) : repo.id
      const result =
        target.kind === 'environment'
          ? await callRuntimeRpc<{ ok: boolean; error?: string }>(
              target,
              'github.requestPRReviewers',
              { repo: runtimeRepoId, prNumber: item.number, reviewers: logins },
              { timeoutMs: 30_000 }
            )
          : await window.api.gh.requestPRReviewers({
              repoPath: repo.path,
              repoId: repo.id,
              sourceContext,
              prNumber: item.number,
              reviewers: logins
            })
      if (result.ok) {
        toast.success(translate('auto.components.TaskPage.8f06dbb9e5', 'Reviewer requested'))
        const nextReviewRequests = buildRequestedReviewUsers(
          logins,
          reviewerCandidates,
          localReviewRequests
        )
        setLocalReviewRequests(nextReviewRequests)
        patchWorkItem(item.id, { reviewRequests: nextReviewRequests }, item.repoId, {
          sourceContext
        })
        setReviewerInput('')
        useAppStore.getState().recordFeatureInteraction('github-tasks')
      } else {
        toast.error(result.error)
      }
    } catch {
      toast.error(translate('auto.components.TaskPage.dc67f69962', 'Failed to request reviewer'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleRemoveReviewers = async (reviewersToRemove: string[]): Promise<void> => {
    if (!repo || submitting) {
      return
    }
    const selected = new Set(localReviewRequests.map((reviewer) => reviewer.login.toLowerCase()))
    const logins = reviewersToRemove
      .map((reviewer) => reviewer.trim().replace(/^@/, ''))
      .filter((reviewer) => reviewer.length > 0 && selected.has(reviewer.toLowerCase()))
    if (logins.length === 0) {
      return
    }
    setSubmitting(true)
    try {
      const target = getActiveRuntimeTarget(sourceSettings)
      const runtimeRepoId =
        sourceContext?.provider === 'github' ? (sourceContext.repoId ?? repo.id) : repo.id
      const result =
        target.kind === 'environment'
          ? await callRuntimeRpc<{ ok: boolean; error?: string }>(
              target,
              'github.removePRReviewers',
              { repo: runtimeRepoId, prNumber: item.number, reviewers: logins },
              { timeoutMs: 30_000 }
            )
          : await window.api.gh.removePRReviewers({
              repoPath: repo.path,
              repoId: repo.id,
              sourceContext,
              prNumber: item.number,
              reviewers: logins
            })
      if (result.ok) {
        toast.success(
          logins.length === 1
            ? translate('auto.components.TaskPage.f9191d1714', 'Reviewer removed')
            : translate('auto.components.TaskPage.837bb901ec', 'Reviewers removed')
        )
        const removed = new Set(logins.map((login) => login.toLowerCase()))
        const nextReviewRequests = localReviewRequests.filter(
          (reviewer) => !removed.has(reviewer.login.toLowerCase())
        )
        setLocalReviewRequests(nextReviewRequests)
        patchWorkItem(item.id, { reviewRequests: nextReviewRequests }, item.repoId, {
          sourceContext
        })
        setReviewerInput('')
      } else {
        toast.error(result.error)
      }
    } catch {
      toast.error(translate('auto.components.TaskPage.ed1daeb49a', 'Failed to remove reviewer'))
    } finally {
      setSubmitting(false)
    }
  }

  const requestReviewer = async (reviewer: GitHubAssignableUser): Promise<void> => {
    // Close the popover immediately so the UI feels responsive; the GitHub
    // request/remove runs in the background and toasts on completion.
    setOpen(false)
    setReviewerInput('')
    await (selectedReviewerLogins.has(reviewer.login.toLowerCase())
      ? handleRemoveReviewers([reviewer.login])
      : handleRequestReview([reviewer.login]))
  }

  const handleReviewerPickerOpenChange = (nextOpen: boolean): void => {
    if (nextOpen) {
      const rect = reviewerTriggerRef.current?.getBoundingClientRect()
      const gap = 8
      const availableBelow = rect ? window.innerHeight - rect.bottom - gap : 0
      const availableAbove = rect ? rect.top - gap : 0
      const nextSide = availableBelow < 240 && availableAbove > availableBelow ? 'top' : 'bottom'
      const available = nextSide === 'top' ? availableAbove : availableBelow
      setReviewerPickerSide(nextSide)
      setReviewerPickerMaxHeight(Math.max(180, Math.min(360, available || 360)))
    }
    setOpen(nextOpen)
    if (nextOpen) {
      cancelReviewerInputFocusFrame()
      reviewerInputFocusFrameRef.current = requestAnimationFrame(() => {
        reviewerInputFocusFrameRef.current = null
        reviewerInputRef.current?.focus()
      })
      return
    }
    cancelReviewerInputFocusFrame()
    setReviewerInput('')
  }

  const renderReviewerPickerRow = (
    reviewer: GitHubAssignableUser,
    options: { suggested: boolean; activeIndex: number }
  ): React.JSX.Element => {
    const selected = selectedReviewerLogins.has(reviewer.login.toLowerCase())
    const active = actionableReviewerRows[activeReviewerIndex]?.login === reviewer.login
    return (
      <button
        key={`${options.suggested ? 'suggested' : 'reviewer'}:${reviewer.login}`}
        type="button"
        className={cn(
          'flex min-h-10 w-full items-center gap-2 border-b border-border/50 px-3 py-2 text-left text-[13px] outline-none last:border-b-0 hover:bg-accent/70',
          active && 'bg-accent text-accent-foreground',
          selected && 'font-medium'
        )}
        onMouseEnter={() => setActiveReviewerIndex(options.activeIndex)}
        onMouseDown={(event) => {
          event.preventDefault()
          void requestReviewer(reviewer)
        }}
      >
        <span className="flex size-4 shrink-0 items-center justify-center text-foreground">
          {selected ? <Check className="size-3.5" /> : null}
        </span>
        {reviewer.avatarUrl ? (
          <img src={reviewer.avatarUrl} alt="" className="size-5 shrink-0 rounded-full" />
        ) : (
          <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground">
            {reviewer.login.slice(0, 1).toUpperCase()}
          </span>
        )}
        <span className="min-w-0 flex-1">
          <span className="block truncate">
            <span className="font-semibold text-foreground">{reviewer.login}</span>
            {reviewer.name ? (
              <span className="ml-1 font-normal text-muted-foreground">{reviewer.name}</span>
            ) : null}
          </span>
          {options.suggested ? (
            <span className="block truncate text-[12px] leading-4 text-muted-foreground">
              {translate(
                'auto.components.TaskPage.5d4fd69a6a',
                'Recently active in this pull request'
              )}
            </span>
          ) : null}
        </span>
      </button>
    )
  }

  return (
    <Popover open={open} onOpenChange={handleReviewerPickerOpenChange}>
      <PopoverTrigger asChild>
        <button
          ref={reviewerTriggerRef}
          type="button"
          onClick={(event) => event.stopPropagation()}
          className={cn(
            'inline-flex h-7 max-w-full items-center justify-center text-[12px] font-medium transition hover:brightness-110',
            primaryReviewer
              ? 'gap-1 rounded-full border border-border/40 bg-background/70 px-1.5 text-muted-foreground hover:text-foreground'
              : 'min-w-7 text-muted-foreground hover:text-foreground'
          )}
          aria-label={translate(
            'auto.components.TaskPage.editReviewersWithCurrent',
            'Edit reviewers: {{value0}}',
            { value0: getGitHubPRReviewLabel(itemWithLocalReviewRequests) }
          )}
          title={getGitHubPRReviewLabel(itemWithLocalReviewRequests)}
        >
          {primaryReviewer ? (
            <>
              <ReviewChipAvatar reviewer={primaryReviewer} />
              {extraReviewerCount > 0 ? (
                <span className="text-[10px] tabular-nums text-muted-foreground">
                  +{extraReviewerCount}
                </span>
              ) : null}
              <ChevronDown className="size-3 text-muted-foreground" />
            </>
          ) : (
            <span aria-hidden="true">-</span>
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent
        className="flex w-[330px] flex-col overflow-hidden rounded-md border-border/70 p-0"
        align="start"
        side={reviewerPickerSide}
        sideOffset={6}
        avoidCollisions={false}
        style={{ maxHeight: reviewerPickerMaxHeight ? `${reviewerPickerMaxHeight}px` : undefined }}
        onClick={(event) => event.stopPropagation()}
        onOpenAutoFocus={(event) => {
          event.preventDefault()
        }}
      >
        <div className="border-b border-border/70 px-3 py-2">
          <div className="text-[13px] font-semibold text-foreground">
            {translate('auto.components.TaskPage.62c7bd789f', 'Request up to 15 reviewers')}
          </div>
        </div>
        <div className="border-b border-border/70 p-3">
          <Input
            ref={setReviewerInputNode}
            value={reviewerInput}
            onChange={(event) => setReviewerInput(event.target.value)}
            placeholder={translate('auto.components.TaskPage.0b9b04f4b5', 'Type or choose a user')}
            disabled={!repo || submitting}
            className="h-8 rounded-md bg-background px-2 text-[13px]"
            aria-label={translate('auto.components.TaskPage.0b9b04f4b5', 'Type or choose a user')}
            aria-autocomplete="list"
            onKeyDown={(event) => {
              if (event.key === 'ArrowDown' && actionableReviewerRows.length > 0) {
                event.preventDefault()
                setActiveReviewerIndex((current) => (current + 1) % actionableReviewerRows.length)
                return
              }
              if (event.key === 'ArrowUp' && actionableReviewerRows.length > 0) {
                event.preventDefault()
                setActiveReviewerIndex(
                  (current) =>
                    (current - 1 + actionableReviewerRows.length) % actionableReviewerRows.length
                )
                return
              }
              if (event.key === 'Enter') {
                event.preventDefault()
                const activeReviewer = actionableReviewerRows[activeReviewerIndex]
                if (activeReviewer) {
                  void requestReviewer(activeReviewer)
                  return
                }
                void handleRequestReview()
                return
              }
              if (event.key === 'Escape') {
                event.preventDefault()
                handleReviewerPickerOpenChange(false)
              }
            }}
          />
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto scrollbar-sleek">
          {reviewerMetadata.loading ? (
            <div className="px-3 py-2 text-[13px] text-muted-foreground">
              {translate('auto.components.TaskPage.0eacf48491', 'Loading…')}
            </div>
          ) : filteredReviewerCandidates.length > 0 ? (
            <>
              {suggestedReviewerRows.length > 0 ? (
                <>
                  <div className="border-b border-border/70 bg-muted/50 px-3 py-1.5 text-[12px] font-semibold text-foreground">
                    {translate('auto.components.TaskPage.3ace2e6bcf', 'Suggestions')}
                  </div>
                  {suggestedReviewerRows.map((reviewer, index) =>
                    renderReviewerPickerRow(reviewer, { suggested: true, activeIndex: index })
                  )}
                </>
              ) : null}
              <div className="border-b border-border/70 bg-muted/50 px-3 py-1.5 text-[12px] font-semibold text-foreground">
                {translate('auto.components.TaskPage.67755a83a1', 'Everyone else')}
              </div>
              {everyoneElseReviewerRows.length > 0 ? (
                everyoneElseReviewerRows.map((reviewer, index) =>
                  renderReviewerPickerRow(reviewer, {
                    suggested: false,
                    activeIndex: suggestedReviewerRows.length + index
                  })
                )
              ) : (
                <div className="px-3 py-2 text-[13px] text-muted-foreground">
                  {translate('auto.components.TaskPage.8a22eb3f7b', 'No matching reviewers.')}
                </div>
              )}
            </>
          ) : (
            <div className="px-3 py-2 text-[13px] text-muted-foreground">
              {reviewerMetadata.error ??
                (hasReviewerMetadata
                  ? translate('auto.components.TaskPage.8a22eb3f7b', 'No matching reviewers.')
                  : translate(
                      'auto.components.TaskPage.9e03c17847',
                      'Open the PR details to view current reviewers.'
                    ))}
            </div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
export function PRChecksCell({
  item,
  onOpen,
  onLoadChecks
}: {
  item: GitHubWorkItem
  onOpen: () => void
  onLoadChecks: () => void
}): React.JSX.Element {
  const triggerRef = useRef<HTMLButtonElement | null>(null)

  useEffect(() => {
    if (item.type !== 'pr' || item.checksSummary) {
      return
    }
    const node = triggerRef.current
    if (!node || typeof IntersectionObserver === 'undefined') {
      return
    }
    let requested = false
    const observer = new IntersectionObserver(
      (entries) => {
        if (requested || !entries.some((entry) => entry.isIntersecting)) {
          return
        }
        requested = true
        onLoadChecks()
        observer.disconnect()
      },
      { rootMargin: '160px 0px' }
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [item.checksSummary, item.type, onLoadChecks])

  if (item.type !== 'pr') {
    return (
      <span className="text-[11px] text-muted-foreground">
        {translate('auto.components.TaskPage.b1eaa18ace', 'Issue')}
      </span>
    )
  }
  const summary = item.checksSummary
  const Icon =
    summary?.state === 'success'
      ? CheckCircle2
      : summary?.state === 'failure'
        ? AlertCircle
        : summary?.state === 'pending'
          ? Clock3
          : Minus
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          ref={triggerRef}
          type="button"
          onFocus={onLoadChecks}
          onMouseEnter={onLoadChecks}
          onClick={(event) => {
            event.stopPropagation()
            onLoadChecks()
            onOpen()
          }}
          className={cn(
            'inline-flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium transition hover:brightness-110',
            getChecksPillTone(item)
          )}
        >
          <Icon className="size-3" />
          <span className="truncate">{getChecksLabel(item)}</span>
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom" sideOffset={6}>
        {translate('auto.components.TaskPage.995dd6af9b', 'Open PR checks')}
      </TooltipContent>
    </Tooltip>
  )
}
export function PRMergeCell({
  item,
  repo,
  sourceContext,
  onRefresh
}: {
  item: GitHubWorkItem
  repo: Repo | null
  sourceContext?: TaskSourceContext | null
  onRefresh: () => void
}): React.JSX.Element {
  const [merging, setMerging] = useState(false)
  const confirm = useConfirmationDialog()
  const repoOwnerSettings = useAppStore(
    useShallow((s) => getSettingsForRepoRuntimeOwner(s, repo?.id ?? null))
  )
  const sourceSettings = useMemo(
    () =>
      sourceContext?.provider === 'github'
        ? ({
            ...repoOwnerSettings,
            ...getTaskSourceRuntimeSettings(sourceContext)
          } as typeof repoOwnerSettings)
        : repoOwnerSettings,
    [repoOwnerSettings, sourceContext]
  )
  if (item.type !== 'pr') {
    return (
      <span className="text-[11px] text-muted-foreground">
        {translate('auto.components.TaskPage.b1eaa18ace', 'Issue')}
      </span>
    )
  }
  const mergePresentation = presentGitHubPRMergeState(item)
  const mergeMethods = resolveGitHubPRMergeMethods(item.mergeMethodSettings)
  const mergeDisabled = !repo || merging || !mergePresentation.directMergeAvailable

  const handleMerge = async (method: GitHubPRMergeMethod): Promise<void> => {
    if (!repo || mergeDisabled) {
      return
    }
    const label = GITHUB_PR_MERGE_METHOD_LABELS[method]
    const confirmed = await confirm({
      title: translate('auto.components.TaskPage.844dc193c7', '{{value0}} PR #{{value1}}?', {
        value0: label,
        value1: item.number
      }),
      description: translate(
        'auto.components.TaskPage.0506a78337',
        'This will update the pull request on GitHub.'
      ),
      confirmLabel: label
    })
    if (!confirmed) {
      return
    }
    setMerging(true)
    try {
      const target = getActiveRuntimeTarget(sourceSettings)
      const runtimeRepoId =
        sourceContext?.provider === 'github' ? (sourceContext.repoId ?? repo.id) : repo.id
      const result =
        target.kind === 'environment'
          ? await callRuntimeRpc<{ ok: boolean; error?: string }>(
              target,
              'github.mergePR',
              {
                repo: runtimeRepoId,
                prNumber: item.number,
                method,
                prRepo: item.prRepo ?? null
              },
              { timeoutMs: 30_000 }
            )
          : await window.api.gh.mergePR({
              repoPath: repo.path,
              repoId: repo.id,
              sourceContext,
              prNumber: item.number,
              method,
              prRepo: item.prRepo ?? null
            })
      if (result.ok) {
        useAppStore.getState().recordFeatureInteraction('github-tasks')
        toast.success(translate('auto.components.TaskPage.a161925adc', 'Pull request merged'))
        onRefresh()
      } else {
        toast.error(result.error)
      }
    } catch {
      toast.error(translate('auto.components.TaskPage.88f478cdef', 'Failed to merge pull request'))
    } finally {
      setMerging(false)
    }
  }

  const handleAutoMerge = async (): Promise<void> => {
    if (!repo || !mergePresentation.autoMergeAction) {
      return
    }
    const enabled = mergePresentation.autoMergeAction.kind === 'enable'
    setMerging(true)
    try {
      const target = getActiveRuntimeTarget(sourceSettings)
      const runtimeRepoId =
        sourceContext?.provider === 'github' ? (sourceContext.repoId ?? repo.id) : repo.id
      const result =
        target.kind === 'environment'
          ? await callRuntimeRpc<{ ok: boolean; error?: string }>(
              target,
              'github.setPRAutoMerge',
              {
                repo: runtimeRepoId,
                prNumber: item.number,
                enabled,
                method: enabled ? mergeMethods.defaultMethod : undefined,
                prRepo: item.prRepo ?? null
              },
              { timeoutMs: 30_000 }
            )
          : await window.api.gh.setPRAutoMerge({
              repoPath: repo.path,
              repoId: repo.id,
              sourceContext,
              prNumber: item.number,
              enabled,
              method: enabled ? mergeMethods.defaultMethod : undefined,
              prRepo: item.prRepo ?? null
            })
      if (result.ok) {
        useAppStore.getState().recordFeatureInteraction('github-tasks')
        toast.success(
          enabled
            ? translate('auto.components.TaskPage.fed317634c', 'Auto-merge enabled')
            : translate('auto.components.TaskPage.a5bf86defe', 'Auto-merge disabled')
        )
        onRefresh()
      } else {
        toast.error(result.error)
      }
    } catch {
      toast.error(
        enabled
          ? translate('auto.components.TaskPage.a3318684bc', 'Failed to enable auto-merge')
          : translate('auto.components.TaskPage.1a9ea003dc', 'Failed to disable auto-merge')
      )
    } finally {
      setMerging(false)
    }
  }

  return (
    <DropdownMenu modal={false}>
      <Tooltip>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              onClick={(event) => event.stopPropagation()}
              className={cn(
                'inline-flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium transition hover:brightness-110',
                mergePresentation.tone
              )}
            >
              {merging ? (
                <LoaderCircle className="size-3 animate-spin text-muted-foreground" />
              ) : (
                <GitMerge className="size-3" />
              )}
              <span className="truncate">{mergePresentation.label}</span>
              <ChevronDown className="size-2.5 opacity-60" />
            </button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent side="bottom" sideOffset={6}>
          {mergePresentation.tooltip}
        </TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="start" onClick={(event) => event.stopPropagation()}>
        {mergePresentation.autoMergeAction && (
          <DropdownMenuItem disabled={!repo || merging} onSelect={() => void handleAutoMerge()}>
            <GitMerge className="size-4" />
            {mergePresentation.autoMergeAction.label}
          </DropdownMenuItem>
        )}
        {mergePresentation.autoMergeAction && <DropdownMenuSeparator />}
        {mergeMethods.methods.map(({ method, label }) => (
          <DropdownMenuItem
            key={method}
            disabled={mergeDisabled}
            onSelect={() => void handleMerge(method)}
          >
            <GitMerge className="size-4" />
            {label}
          </DropdownMenuItem>
        ))}
        <DropdownMenuItem onSelect={() => window.api.shell.openUrl(item.url)}>
          <ExternalLink className="size-4" />
          {translate('auto.components.TaskPage.37d60046e3', 'Open GitHub merge box')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
