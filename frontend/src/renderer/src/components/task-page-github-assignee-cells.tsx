/* eslint-disable max-lines -- Why: straight verbatim extraction of TaskPage.tsx's
GitHub status/assignee cell components (GHStatusCell, GitHubAssigneeAvatar,
GitHubIssueLabelSelector, GitHubIssueAssigneeSelector, GHAssigneesCell), which were
already coupled at this size inside TaskPage.tsx's own grandfathered max-lines disable
before this move (TASK-BIGFILE-030). Registered in config/max-lines-baseline.txt per
AGENTS.md — NEEDS PR REVIEW. Further internal splitting is a separate, un-tracked
refactor candidate; not addressed here to keep this a pure Move. */
import React, { useCallback, useMemo, useRef, useState } from 'react'
import { useShallow } from 'zustand/react/shallow'
import {
  Ban,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleDot,
  Copy,
  LoaderCircle,
  Search
} from 'lucide-react'
import { toast } from 'sonner'

import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { parseGitHubIssueOrPRLink } from '@/lib/github-links'
import { useRepoAssigneesBySlug } from '@/hooks/useGitHubSlugMetadata'
import { getSettingsForRepoRuntimeOwner } from '@/lib/repo-runtime-owner'
import { cn } from '@/lib/utils'
import {
  getTaskSourceRuntimeSettings,
  type TaskSourceContext
} from '../../../shared/task-source-context'
import {
  createTaskPageGitHubStatusStateDraft,
  resolveTaskPageGitHubStatusStateDraft,
  updateTaskPageGitHubStatusLocalState
} from '@/components/task-page-github-status-state'
import { TaskPageGitHubWorkItemStateBadge } from '@/components/task-page-github-work-item-status-badge'
import {
  buildTaskPageGitHubCloseUpdate,
  getTaskPageGitHubDuplicateCandidates,
  getTaskPageGitHubDuplicateTargetErrorMessage,
  validateTaskPageGitHubDuplicateTarget,
  type TaskPageGitHubCloseAction
} from '@/components/task-page-github-status-actions'
import { translate } from '@/i18n/i18n'
import type {
  GitHubAssignableUser,
  GitHubIssueUpdate,
  GitHubWorkItem,
  Repo
} from '../../../shared/types'

export function GHStatusCell({
  item,
  repo,
  sourceContext
}: {
  item: GitHubWorkItem
  repo: Repo | null
  sourceContext?: TaskSourceContext | null
}): React.JSX.Element {
  const patchWorkItem = useAppStore((s) => s.patchWorkItem)
  const [statusStateDraft, setStatusStateDraft] = useState(() =>
    createTaskPageGitHubStatusStateDraft(item)
  )
  const [open, setOpen] = useState(false)
  const [duplicatePickerOpen, setDuplicatePickerOpen] = useState(false)
  const [duplicateSearch, setDuplicateSearch] = useState('')
  const [duplicateError, setDuplicateError] = useState<string | null>(null)
  const duplicateIssueCandidates = useAppStore(
    useShallow((s) => {
      if (!duplicatePickerOpen) {
        return []
      }
      const deduped = new Map<number, GitHubWorkItem>()
      for (const entry of Object.values(s.workItemsCache)) {
        for (const candidate of entry.data ?? []) {
          if (
            candidate.type === 'issue' &&
            candidate.repoId === item.repoId &&
            candidate.number !== item.number &&
            !deduped.has(candidate.number)
          ) {
            deduped.set(candidate.number, candidate)
          }
        }
      }
      return Array.from(deduped.values()).sort((a, b) => b.number - a.number)
    })
  )
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
  const reqRef = useRef(0)
  const parsedIssueLink = useMemo(() => parseGitHubIssueOrPRLink(item.url), [item.url])
  const filteredDuplicateCandidates = useMemo(
    () =>
      getTaskPageGitHubDuplicateCandidates(duplicateIssueCandidates, item.number, duplicateSearch),
    [duplicateIssueCandidates, duplicateSearch, item.number]
  )
  const directDuplicateTarget = useMemo(() => {
    const trimmed = duplicateSearch.trim()
    const validation = validateTaskPageGitHubDuplicateTarget(trimmed, item.number)
    if (!trimmed || !validation.ok) {
      return null
    }
    if (
      filteredDuplicateCandidates.some((candidate) => candidate.number === validation.duplicateOf)
    ) {
      return null
    }
    return validation.duplicateOf
  }, [duplicateSearch, filteredDuplicateCandidates, item.number])
  const duplicatePickerTitle = parsedIssueLink?.slug
    ? `${parsedIssueLink.slug.owner}/${parsedIssueLink.slug.repo}`
    : (repo?.displayName ?? translate('auto.components.TaskPage.repository', 'Repository'))

  const resolvedStatusStateDraft = resolveTaskPageGitHubStatusStateDraft(statusStateDraft, item)
  if (resolvedStatusStateDraft !== statusStateDraft) {
    // Why: item rows can refresh from the GitHub cache while this cell is still
    // mounted; reconcile before paint instead of showing one stale status frame.
    setStatusStateDraft(resolvedStatusStateDraft)
  }
  const localState = resolvedStatusStateDraft.localState
  const updateLocalState = useCallback(
    (nextState: GitHubWorkItem['state']) => {
      setStatusStateDraft((current) =>
        updateTaskPageGitHubStatusLocalState(current, item, nextState)
      )
    },
    [item]
  )

  const handleStateChange = useCallback(
    (newState: 'open' | 'closed', closeAction?: TaskPageGitHubCloseAction) => {
      if (newState === localState || item.type !== 'issue') {
        return
      }
      const parsedOwnerRepo = parsedIssueLink?.slug
      if (!repo && !parsedOwnerRepo) {
        return
      }
      reqRef.current += 1
      const reqId = reqRef.current
      const updates: GitHubIssueUpdate =
        newState === 'closed' && closeAction
          ? buildTaskPageGitHubCloseUpdate(closeAction)
          : { state: newState }
      updateLocalState(newState)
      patchWorkItem(item.id, { state: newState }, item.repoId, { sourceContext })
      const target = getActiveRuntimeTarget(sourceSettings)
      // Why: issue rows can be sourced by owner/repo URL instead of the local
      // repo context; slug-aware writes preserve close reasons and duplicates.
      const updatePromise = parsedOwnerRepo
        ? target.kind === 'environment'
          ? callRuntimeRpc<{ ok?: boolean; error?: { message?: string } | string }>(
              target,
              'github.project.updateIssueBySlug',
              {
                owner: parsedOwnerRepo.owner,
                repo: parsedOwnerRepo.repo,
                number: item.number,
                updates
              },
              { timeoutMs: 30_000 }
            )
          : window.api.gh.updateIssueBySlug({
              owner: parsedOwnerRepo.owner,
              repo: parsedOwnerRepo.repo,
              number: item.number,
              updates
            })
        : (() => {
            if (!repo) {
              throw new Error('No GitHub repository context available for this issue.')
            }
            const runtimeRepoId =
              sourceContext?.provider === 'github' ? (sourceContext.repoId ?? repo.id) : repo.id
            return target.kind === 'environment'
              ? callRuntimeRpc<{ ok?: boolean; error?: string }>(
                  target,
                  'github.updateIssue',
                  { repo: runtimeRepoId, number: item.number, updates },
                  { timeoutMs: 30_000 }
                )
              : window.api.gh.updateIssue({
                  repoPath: repo.path,
                  repoId: repo.id,
                  sourceContext,
                  number: item.number,
                  updates
                })
          })()
      updatePromise
        .then((result) => {
          if (reqId !== reqRef.current) {
            return
          }
          const typed = result as { ok?: boolean; error?: string | { message?: string } }
          if (typed && typed.ok === false) {
            updateLocalState(newState === 'closed' ? 'open' : 'closed')
            patchWorkItem(
              item.id,
              { state: newState === 'closed' ? 'open' : 'closed' },
              item.repoId,
              { sourceContext }
            )
            toast.error(
              typeof typed.error === 'string'
                ? typed.error
                : (typed.error?.message ??
                    translate('auto.components.TaskPage.1c893195ac', 'Failed to update state'))
            )
            return
          }
          if (repo) {
            useAppStore.getState().evictGitHubRepoCaches(repo.id, repo.path)
          }
          useAppStore.getState().recordFeatureInteraction('github-tasks')
        })
        .catch(() => {
          if (reqId !== reqRef.current) {
            return
          }
          updateLocalState(newState === 'closed' ? 'open' : 'closed')
          patchWorkItem(
            item.id,
            { state: newState === 'closed' ? 'open' : 'closed' },
            item.repoId,
            {
              sourceContext
            }
          )
          toast.error(translate('auto.components.TaskPage.1c893195ac', 'Failed to update state'))
        })
    },
    [
      item,
      localState,
      parsedIssueLink,
      patchWorkItem,
      repo,
      sourceContext,
      sourceSettings,
      updateLocalState
    ]
  )

  const closeAsDuplicate = useCallback(
    (targetIssueNumber: number | string) => {
      const validation = validateTaskPageGitHubDuplicateTarget(
        String(targetIssueNumber),
        item.number
      )
      if (!validation.ok) {
        setDuplicateError(getTaskPageGitHubDuplicateTargetErrorMessage(validation, translate))
        return
      }
      setDuplicateError(null)
      handleStateChange('closed', { stateReason: 'duplicate', duplicateOf: validation.duplicateOf })
      setOpen(false)
      setDuplicatePickerOpen(false)
    },
    [handleStateChange, item.number]
  )

  const handleDuplicateSearchSubmit = useCallback(() => {
    const validation = validateTaskPageGitHubDuplicateTarget(duplicateSearch, item.number)
    if (!validation.ok) {
      setDuplicateError(getTaskPageGitHubDuplicateTargetErrorMessage(validation, translate))
      return
    }
    closeAsDuplicate(validation.duplicateOf)
  }, [closeAsDuplicate, duplicateSearch, item.number])

  const handlePopoverOpenChange = useCallback((nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) {
      setDuplicatePickerOpen(false)
      setDuplicateSearch('')
      setDuplicateError(null)
    }
  }, [])

  if (item.type !== 'issue' || (!repo && !parsedIssueLink?.slug)) {
    return <TaskPageGitHubWorkItemStateBadge item={item} />
  }

  return (
    <Popover open={open} onOpenChange={handlePopoverOpenChange}>
      <PopoverTrigger asChild>
        <button
          type="button"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
          className={cn(
            'group/status inline-flex cursor-pointer items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium transition hover:brightness-125 hover:ring-1 hover:ring-white/10',
            localState === 'closed'
              ? 'border-primary/40 bg-primary/10 text-primary'
              : 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-200'
          )}
        >
          {localState === 'open' ? <CircleDot className="size-2.5" /> : null}
          <span>
            {localState === 'closed'
              ? translate('auto.components.TaskPage.d09bf34db7', 'Closed')
              : translate('auto.components.TaskPage.606a85c774', 'Open')}
          </span>
          <ChevronDown className="size-2.5 opacity-50" />
        </button>
      </PopoverTrigger>
      <PopoverContent
        className={cn(duplicatePickerOpen ? 'w-[360px]' : 'w-56', 'p-1')}
        align="start"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => e.stopPropagation()}
      >
        {duplicatePickerOpen ? (
          <div>
            <div className="flex items-center gap-2 px-1 py-1.5">
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                className="size-7"
                onClick={() => {
                  setDuplicatePickerOpen(false)
                  setDuplicateSearch('')
                  setDuplicateError(null)
                }}
                aria-label={translate('auto.components.TaskPage.backToCloseReasons', 'Back')}
              >
                <ChevronLeft className="size-4" />
              </Button>
              <span className="min-w-0 truncate text-[12px] font-semibold">
                {duplicatePickerTitle}
              </span>
            </div>
            <div className="relative px-1 pb-2">
              <Search className="pointer-events-none absolute left-3 top-2.5 size-4 text-muted-foreground" />
              <Input
                autoFocus
                value={duplicateSearch}
                onChange={(event) => {
                  setDuplicateSearch(event.target.value)
                  setDuplicateError(null)
                }}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault()
                    handleDuplicateSearchSubmit()
                  }
                }}
                placeholder={translate('auto.components.TaskPage.searchIssues', 'Search issues')}
                className="h-9 pl-8 text-[12px]"
                aria-invalid={duplicateError ? true : undefined}
              />
            </div>
            {duplicateError ? (
              <p className="px-2 pb-2 text-[11px] text-destructive">{duplicateError}</p>
            ) : null}
            <div className="scrollbar-sleek max-h-72 overflow-y-auto pr-1">
              {directDuplicateTarget ? (
                <button
                  type="button"
                  onClick={() => closeAsDuplicate(directDuplicateTarget)}
                  className="flex w-full items-center gap-2 rounded-sm px-2 py-2 text-left hover:bg-accent"
                >
                  <Copy className="size-4 text-primary" />
                  <span className="min-w-0 flex-1 text-[12px] font-medium">
                    {translate('auto.components.TaskPage.useIssueNumber', 'Use issue #{{value0}}', {
                      value0: directDuplicateTarget
                    })}
                  </span>
                </button>
              ) : null}
              {filteredDuplicateCandidates.map((candidate) => (
                <button
                  key={`${candidate.repoId}:${candidate.number}`}
                  type="button"
                  onClick={() => closeAsDuplicate(candidate.number)}
                  className="flex w-full items-start gap-2 rounded-sm px-2 py-2 text-left hover:bg-accent"
                >
                  {candidate.state === 'closed' ? (
                    <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-primary" />
                  ) : (
                    <CircleDot className="mt-0.5 size-4 shrink-0 text-emerald-500" />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block text-[12px] font-medium leading-snug">
                      {candidate.title}
                    </span>
                  </span>
                  <span className="shrink-0 text-[12px] text-muted-foreground">
                    #{candidate.number}
                  </span>
                </button>
              ))}
              {!directDuplicateTarget && filteredDuplicateCandidates.length === 0 ? (
                <p className="px-2 py-3 text-[12px] text-muted-foreground">
                  {translate(
                    'auto.components.TaskPage.noMatchingIssuesLoaded',
                    'No matching issues loaded.'
                  )}
                </p>
              ) : null}
            </div>
          </div>
        ) : (
          <>
            <button
              type="button"
              onClick={() => {
                handleStateChange('open')
                setOpen(false)
              }}
              className={cn(
                'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-[12px] hover:bg-accent',
                localState === 'open' && 'bg-accent/50'
              )}
            >
              <CircleDot className="size-4 text-muted-foreground" />
              {translate('auto.components.TaskPage.606a85c774', 'Open')}
            </button>
            <button
              type="button"
              onClick={() => {
                handleStateChange('closed', { stateReason: 'completed' })
                setOpen(false)
              }}
              className={cn(
                'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-[12px] hover:bg-accent',
                localState === 'closed' && 'bg-accent/50'
              )}
            >
              <CheckCircle2 className="size-4 text-muted-foreground" />
              {translate('auto.components.TaskPage.closeAsCompleted', 'Close as completed')}
            </button>
            <button
              type="button"
              onClick={() => {
                handleStateChange('closed', { stateReason: 'not_planned' })
                setOpen(false)
              }}
              className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-[12px] hover:bg-accent"
            >
              <Ban className="size-4 text-muted-foreground" />
              {translate('auto.components.TaskPage.closeAsNotPlanned', 'Close as not planned')}
            </button>
            <button
              type="button"
              onClick={() => {
                setDuplicatePickerOpen(true)
                setDuplicateSearch('')
                setDuplicateError(null)
              }}
              className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-[12px] hover:bg-accent"
            >
              <Copy className="size-4 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate">
                {translate('auto.components.TaskPage.closeAsDuplicate', 'Close as duplicate')}
              </span>
              <ChevronRight className="size-3.5 text-muted-foreground" />
            </button>
          </>
        )}
      </PopoverContent>
    </Popover>
  )
}
export function GitHubAssigneeAvatar({
  assignee
}: {
  assignee: GitHubAssignableUser
}): React.JSX.Element {
  if (assignee.avatarUrl) {
    return (
      <img
        src={assignee.avatarUrl}
        alt={assignee.login}
        loading="lazy"
        decoding="async"
        title={assignee.name ? `${assignee.name} (${assignee.login})` : assignee.login}
        className="size-5 rounded-full border border-border/40 bg-muted object-cover"
      />
    )
  }
  return (
    <span
      title={assignee.login}
      className="inline-flex size-5 items-center justify-center rounded-full border border-border/40 bg-muted text-[10px] font-medium text-muted-foreground"
    >
      {assignee.login.slice(0, 1).toUpperCase()}
    </span>
  )
}
export function GitHubIssueLabelSelector({
  labels,
  selectedLabels,
  loading,
  error,
  disabled,
  onChange
}: {
  labels: string[]
  selectedLabels: string[]
  loading: boolean
  error: string | null
  disabled: boolean
  onChange: (labels: string[]) => void
}): React.JSX.Element {
  const selectedSet = useMemo(() => new Set(selectedLabels), [selectedLabels])
  const toggleLabel = useCallback(
    (label: string) => {
      onChange(
        selectedSet.has(label)
          ? selectedLabels.filter((name) => name !== label)
          : [...selectedLabels, label]
      )
    },
    [onChange, selectedLabels, selectedSet]
  )

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <label className="text-[11px] font-medium text-muted-foreground">
        {translate('auto.components.TaskPage.d0ca4aa1d0', 'Labels')}
      </label>
      <Popover>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="outline"
            disabled={disabled}
            className="h-auto min-h-9 justify-start gap-2 px-3 py-2 text-left"
          >
            {selectedLabels.length === 0 ? (
              <span className="text-muted-foreground">
                {translate('auto.components.TaskPage.5ebff3a0aa', 'None')}
              </span>
            ) : (
              <span className="flex min-w-0 flex-wrap gap-1.5">
                {selectedLabels.map((label) => (
                  <span
                    key={label}
                    className="rounded-full border border-border/50 bg-muted/40 px-2 py-0.5 text-[11px] font-medium"
                  >
                    {label}
                  </span>
                ))}
              </span>
            )}
            {loading ? <LoaderCircle className="ml-auto size-3.5 animate-spin" /> : null}
          </Button>
        </PopoverTrigger>
        <PopoverContent className="popover-scroll-content scrollbar-sleek w-64 p-1" align="start">
          {error ? (
            <div className="px-2 py-2 text-xs text-destructive">{error}</div>
          ) : labels.length === 0 ? (
            <div className="px-2 py-2 text-xs text-muted-foreground">
              {translate('auto.components.TaskPage.b36f4bf9de', 'No labels.')}
            </div>
          ) : (
            labels.map((label) => (
              <button
                key={label}
                type="button"
                onClick={() => toggleLabel(label)}
                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-xs hover:bg-accent"
              >
                <span
                  className={cn(
                    'flex size-3.5 shrink-0 items-center justify-center rounded-sm border',
                    selectedSet.has(label)
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'border-input'
                  )}
                >
                  {selectedSet.has(label) ? <Check className="size-2.5" /> : null}
                </span>
                <span className="min-w-0 truncate">{label}</span>
              </button>
            ))
          )}
        </PopoverContent>
      </Popover>
    </div>
  )
}
export function GitHubIssueAssigneeSelector({
  assignees,
  selectedAssignees,
  loading,
  error,
  disabled,
  onChange
}: {
  assignees: GitHubAssignableUser[]
  selectedAssignees: GitHubAssignableUser[]
  loading: boolean
  error: string | null
  disabled: boolean
  onChange: (assignees: GitHubAssignableUser[]) => void
}): React.JSX.Element {
  const selectedLogins = useMemo(
    () => new Set(selectedAssignees.map((assignee) => assignee.login.toLowerCase())),
    [selectedAssignees]
  )
  const toggleAssignee = useCallback(
    (assignee: GitHubAssignableUser) => {
      const key = assignee.login.toLowerCase()
      onChange(
        selectedLogins.has(key)
          ? selectedAssignees.filter((current) => current.login.toLowerCase() !== key)
          : [...selectedAssignees, assignee]
      )
    },
    [onChange, selectedAssignees, selectedLogins]
  )

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <label className="text-[11px] font-medium text-muted-foreground">
        {translate('auto.components.TaskPage.8aba10579d', 'Assignees')}
      </label>
      <Popover>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="outline"
            disabled={disabled}
            className="h-auto min-h-9 justify-start gap-2 px-3 py-2 text-left"
          >
            {selectedAssignees.length === 0 ? (
              <span className="text-muted-foreground">
                {translate('auto.components.TaskPage.42a9160321', 'Unassigned')}
              </span>
            ) : (
              <span className="flex min-w-0 items-center gap-1.5">
                <span className="flex -space-x-1">
                  {selectedAssignees.slice(0, 3).map((assignee) => (
                    <GitHubAssigneeAvatar key={assignee.login} assignee={assignee} />
                  ))}
                </span>
                <span className="min-w-0 truncate text-xs">
                  {selectedAssignees.map((assignee) => assignee.login).join(', ')}
                </span>
              </span>
            )}
            {loading ? <LoaderCircle className="ml-auto size-3.5 animate-spin" /> : null}
          </Button>
        </PopoverTrigger>
        <PopoverContent className="popover-scroll-content scrollbar-sleek w-72 p-1" align="start">
          {error ? (
            <div className="px-2 py-2 text-xs text-destructive">{error}</div>
          ) : assignees.length === 0 ? (
            <div className="px-2 py-2 text-xs text-muted-foreground">
              {translate('auto.components.TaskPage.edf4bc4135', 'No assignable users.')}
            </div>
          ) : (
            assignees.map((assignee) => {
              const selected = selectedLogins.has(assignee.login.toLowerCase())
              return (
                <button
                  key={assignee.login}
                  type="button"
                  onClick={() => toggleAssignee(assignee)}
                  className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-xs hover:bg-accent"
                >
                  <span
                    className={cn(
                      'flex size-3.5 shrink-0 items-center justify-center rounded-sm border',
                      selected
                        ? 'border-primary bg-primary text-primary-foreground'
                        : 'border-input'
                    )}
                  >
                    {selected ? <Check className="size-2.5" /> : null}
                  </span>
                  <GitHubAssigneeAvatar assignee={assignee} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium">{assignee.login}</span>
                    {assignee.name ? (
                      <span className="block truncate text-[11px] text-muted-foreground">
                        {assignee.name}
                      </span>
                    ) : null}
                  </span>
                </button>
              )
            })
          )}
        </PopoverContent>
      </Popover>
    </div>
  )
}
export function GHAssigneesCell({
  item,
  repo,
  sourceContext
}: {
  item: GitHubWorkItem
  repo: Repo | null
  sourceContext?: TaskSourceContext | null
}): React.JSX.Element {
  const patchWorkItem = useAppStore((s) => s.patchWorkItem)
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
  const [open, setOpen] = useState(false)
  const [pendingLogin, setPendingLogin] = useState<string | null>(null)
  const assignees = useMemo(() => item.assignees ?? [], [item.assignees])
  const parsed = useMemo(() => parseGitHubIssueOrPRLink(item.url), [item.url])
  const owner = parsed?.slug.owner ?? null
  const repoName = parsed?.slug.repo ?? null
  const seedLogins = useMemo(
    () =>
      assignees
        .map((a) => a.login)
        .sort()
        .filter(Boolean),
    [assignees]
  )
  const metadata = useRepoAssigneesBySlug(
    open ? owner : null,
    open ? repoName : null,
    seedLogins,
    sourceSettings
  )

  const toggleAssignee = useCallback(
    async (user: GitHubAssignableUser): Promise<void> => {
      if (item.type !== 'issue' || pendingLogin) {
        return
      }
      const userLoginKey = user.login.toLowerCase()
      const isOn = assignees.some((a) => a.login.toLowerCase() === userLoginKey)
      const previousAssignees = assignees
      const nextAssignees = isOn
        ? assignees.filter((a) => a.login.toLowerCase() !== userLoginKey)
        : [...assignees, user]
      setPendingLogin(user.login)
      patchWorkItem(item.id, { assignees: nextAssignees }, item.repoId, { sourceContext })

      try {
        const updates = isOn ? { removeAssignees: [user.login] } : { addAssignees: [user.login] }
        const target = getActiveRuntimeTarget(sourceSettings)
        if (owner && repoName) {
          const args = {
            owner,
            repo: repoName,
            number: item.number,
            updates
          }
          const res =
            target.kind === 'environment'
              ? await callRuntimeRpc<Awaited<ReturnType<typeof window.api.gh.updateIssueBySlug>>>(
                  target,
                  'github.project.updateIssueBySlug',
                  args,
                  { timeoutMs: 30_000 }
                )
              : await window.api.gh.updateIssueBySlug(args)
          if (!res.ok) {
            throw new Error(res.error.message)
          }
        } else if (repo) {
          const runtimeRepoId =
            sourceContext?.provider === 'github' ? (sourceContext.repoId ?? repo.id) : repo.id
          const res =
            target.kind === 'environment'
              ? await callRuntimeRpc<{ ok?: boolean; error?: string }>(
                  target,
                  'github.updateIssue',
                  { repo: runtimeRepoId, number: item.number, updates },
                  { timeoutMs: 30_000 }
                )
              : await window.api.gh.updateIssue({
                  repoPath: repo.path,
                  repoId: repo.id,
                  sourceContext,
                  number: item.number,
                  updates
                })
          if (res && res.ok === false) {
            throw new Error(res.error)
          }
        } else {
          throw new Error('No GitHub repository context available for this issue.')
        }
        useAppStore.getState().recordFeatureInteraction('github-tasks')
      } catch (err) {
        patchWorkItem(item.id, { assignees: previousAssignees }, item.repoId, { sourceContext })
        toast.error(
          err instanceof Error
            ? err.message
            : translate('auto.components.TaskPage.ca63694b4c', 'Failed to update assignees.')
        )
      } finally {
        setPendingLogin(null)
      }
    },
    [
      assignees,
      item.id,
      item.number,
      item.repoId,
      item.type,
      owner,
      patchWorkItem,
      pendingLogin,
      repo,
      repoName,
      sourceContext,
      sourceSettings
    ]
  )

  const triggerContent =
    assignees.length > 0 ? (
      <>
        <div className="flex min-w-0 -space-x-1 overflow-hidden">
          {assignees.slice(0, 3).map((assignee) => (
            <GitHubAssigneeAvatar key={assignee.login} assignee={assignee} />
          ))}
        </div>
        {assignees.length > 3 ? (
          <span className="ml-1 shrink-0 text-[10px] font-medium text-muted-foreground">
            +{assignees.length - 3}
          </span>
        ) : null}
      </>
    ) : (
      <span className="text-xs text-muted-foreground/60">-</span>
    )

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={
            assignees.length
              ? translate('auto.components.TaskPage.bb63046423', 'Assigned to {{value0}}', {
                  value0: assignees.map((a) => a.login).join(', ')
                })
              : translate('auto.components.TaskPage.7f94eb6395', 'Assign issue')
          }
          aria-busy={pendingLogin !== null}
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
          className={cn(
            'inline-flex h-6 max-w-full items-center gap-1 text-left transition disabled:opacity-60',
            assignees.length > 0
              ? 'rounded-full border border-border/40 bg-background/70 px-1.5 hover:bg-muted/60'
              : 'w-full rounded-sm border border-transparent bg-transparent px-1 hover:bg-muted/40'
          )}
        >
          {triggerContent}
          {pendingLogin ? (
            <LoaderCircle className="size-3 shrink-0 animate-spin text-muted-foreground" />
          ) : assignees.length > 0 ? (
            <ChevronDown className="size-3 shrink-0 text-muted-foreground" />
          ) : null}
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="popover-scroll-content scrollbar-sleek w-64 p-1"
        onClick={(event) => event.stopPropagation()}
      >
        {!owner || !repoName ? (
          <div className="px-2 py-2 text-xs text-muted-foreground">
            {translate('auto.components.TaskPage.53e002d895', 'Issue has no repo slug.')}
          </div>
        ) : metadata.loading ? (
          <div className="px-2 py-2 text-xs text-muted-foreground">
            {translate('auto.components.TaskPage.0eacf48491', 'Loading…')}
          </div>
        ) : metadata.error ? (
          <div className="px-2 py-2 text-xs text-destructive">{metadata.error}</div>
        ) : metadata.data.length === 0 ? (
          <div className="px-2 py-2 text-xs text-muted-foreground">
            {translate('auto.components.TaskPage.edf4bc4135', 'No assignable users.')}
          </div>
        ) : (
          metadata.data.map((user) => {
            const isOn = assignees.some((a) => a.login.toLowerCase() === user.login.toLowerCase())
            const pending = pendingLogin === user.login
            return (
              <button
                key={user.login}
                type="button"
                disabled={pendingLogin !== null}
                className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-muted/50 disabled:opacity-60"
                onClick={(event) => {
                  event.stopPropagation()
                  void toggleAssignee(user)
                }}
              >
                <span
                  className={cn(
                    'flex size-3.5 shrink-0 items-center justify-center rounded-sm border',
                    isOn ? 'border-primary bg-primary text-primary-foreground' : 'border-input'
                  )}
                >
                  {pending ? (
                    <LoaderCircle className="size-3 animate-spin" />
                  ) : isOn ? (
                    <Check className="size-3" />
                  ) : null}
                </span>
                {user.avatarUrl ? (
                  <img src={user.avatarUrl} alt="" className="size-5 shrink-0 rounded-full" />
                ) : (
                  <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground">
                    {user.login.slice(0, 1).toUpperCase()}
                  </span>
                )}
                <span className="min-w-0 flex-1">
                  <span className="block truncate">{user.login}</span>
                  {user.name ? (
                    <span className="block truncate text-[11px] text-muted-foreground">
                      {user.name}
                    </span>
                  ) : null}
                </span>
              </button>
            )
          })
        )}
      </PopoverContent>
    </Popover>
  )
}
