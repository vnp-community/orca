// Pure, non-JSX SourceControl.tsx helpers: base-ref resolution, commit-draft
// persistence, and post-remote-action refresh/error-clearing logic.
// Extracted verbatim from SourceControl.tsx (TASK-BIGFILE-020) to shrink the
// component file; SourceControl.tsx re-exports these as a barrel so existing
// imports (including tests and ChecksPanel.tsx) are unaffected.
import { pickSourceControlLaunchAgent } from '@/lib/source-control-launch-agent-selection'
import type {
  GitConflictOperation,
  GitUpstreamStatus,
  SourceControlViewMode,
  TuiAgent
} from '../../../../shared/types'
import type { SourceControlActionError } from './source-control-action-error'

export function resolveSourceControlBaseRef(input: {
  worktreeBaseRef?: string | null
  reviewBaseRefName?: string | null
  repoBaseRef?: string | null
  defaultBaseRef?: string | null
}): string | null {
  const worktreeBaseRef = input.worktreeBaseRef?.trim() || null
  const hasReviewBaseRefName = Boolean(input.reviewBaseRefName?.trim())
  const reviewBaseRef = resolveHostedReviewCompareBaseRef(input.reviewBaseRefName, [
    input.repoBaseRef,
    input.defaultBaseRef
  ])
  if (worktreeBaseRef && isFullGitCommitOid(worktreeBaseRef) && hasReviewBaseRefName) {
    return reviewBaseRef
  }
  return worktreeBaseRef || input.repoBaseRef?.trim() || input.defaultBaseRef?.trim() || null
}

// Why: the compare/diff view's base is conceptually distinct from the PR/rebase
// merge target (effectiveBaseRef). When the setting is on, default the compare
// base to the current branch's upstream so the panel surfaces local changes
// instead of the full delta vs the repo default branch. Branches without an
// upstream fall back to effectiveBaseRef so the automatic policy never makes
// the committed-changes comparison disappear unexpectedly. When the setting is
// off, fall back to effectiveBaseRef so behavior is unchanged.
export function resolveSourceControlCompareBaseRef(input: {
  enabled: boolean
  worktreeBaseRef?: string | null
  repoBaseRef?: string | null
  upstreamName?: string | null
  fallbackBaseRef?: string | null
}): string | null {
  if (!input.enabled) {
    return input.fallbackBaseRef?.trim() || null
  }
  const pinned = input.worktreeBaseRef?.trim() || input.repoBaseRef?.trim()
  if (pinned) {
    return pinned
  }
  return input.upstreamName?.trim() || input.fallbackBaseRef?.trim() || null
}

// Why: only drop a stale branch-compare summary once we know there is truly no
// compare base. While upstream status is still loading (remoteStatus undefined)
// compareBaseRef can momentarily resolve to null, so clearing then would make
// the committed-changes summary flicker until upstream loads.
export function shouldClearBranchCompareForMissingBase(input: {
  isFolder: boolean
  compareBaseRef: string | null
  remoteStatus: GitUpstreamStatus | undefined
}): boolean {
  if (input.isFolder || input.compareBaseRef) {
    return false
  }
  return input.remoteStatus !== undefined
}

export function resolveSourceControlPickerBaseRef(input: {
  pinnedBaseRef?: string | null
  effectiveBaseRef?: string | null
}): string | undefined {
  const pinnedBaseRef = input.pinnedBaseRef?.trim()
  if (!pinnedBaseRef) {
    return undefined
  }
  return input.effectiveBaseRef?.trim() || pinnedBaseRef
}

function isFullGitCommitOid(value: string): boolean {
  return /^[0-9a-f]{40}$/i.test(value)
}

function resolveHostedReviewCompareBaseRef(
  baseRefName: string | null | undefined,
  candidates: (string | null | undefined)[]
): string | null {
  const branch = baseRefName?.trim()
  if (!branch) {
    return null
  }
  for (const candidate of candidates) {
    const trimmed = candidate?.trim()
    if (!trimmed) {
      continue
    }
    if (getCompareBaseCandidateBranchName(trimmed) === branch) {
      return trimmed
    }
  }
  for (const candidate of candidates) {
    const rewritten = rewriteCompareBaseBranchFromCandidate(candidate, branch)
    if (rewritten) {
      return rewritten
    }
  }
  return null
}

function getCompareBaseCandidateBranchName(candidate: string): string {
  const remoteRefPrefix = 'refs/remotes/'
  if (candidate.startsWith(remoteRefPrefix)) {
    const remoteAndBranch = candidate.slice(remoteRefPrefix.length)
    const slashIndex = remoteAndBranch.indexOf('/')
    return slashIndex > 0 ? remoteAndBranch.slice(slashIndex + 1) : remoteAndBranch
  }
  const headsRefPrefix = 'refs/heads/'
  if (candidate.startsWith(headsRefPrefix)) {
    return candidate.slice(headsRefPrefix.length)
  }
  const slashIndex = candidate.indexOf('/')
  return slashIndex > 0 ? candidate.slice(slashIndex + 1) : candidate
}

function rewriteCompareBaseBranchFromCandidate(
  candidate: string | null | undefined,
  branch: string
): string | null {
  const trimmed = candidate?.trim()
  if (!trimmed) {
    return null
  }
  const remoteRefPrefix = 'refs/remotes/'
  if (trimmed.startsWith(remoteRefPrefix)) {
    const remoteAndBranch = trimmed.slice(remoteRefPrefix.length)
    const slashIndex = remoteAndBranch.indexOf('/')
    return slashIndex > 0
      ? `${remoteRefPrefix}${remoteAndBranch.slice(0, slashIndex)}/${branch}`
      : null
  }
  const headsRefPrefix = 'refs/heads/'
  if (trimmed.startsWith(headsRefPrefix)) {
    return `${headsRefPrefix}${branch}`
  }
  const slashIndex = trimmed.indexOf('/')
  return slashIndex > 0 ? `${trimmed.slice(0, slashIndex)}/${branch}` : null
}

// Why: 5s branch compare polling churned git subprocesses in large repos.
// Explicit commit, remote, manual, and base-ref refresh paths still run immediately.
export const BRANCH_REFRESH_INTERVAL_MS = 30_000

export function normalizeSourceControlViewMode(value: unknown): SourceControlViewMode {
  return value === 'tree' || value === 'list' ? value : 'list'
}

export type CommitDraftsByWorktree = Record<string, string>

export function readCommitDraftForWorktree(
  drafts: CommitDraftsByWorktree,
  worktreeId: string | null | undefined
): string {
  return drafts[worktreeId ?? ''] ?? ''
}

export function writeCommitDraftForWorktree(
  drafts: CommitDraftsByWorktree,
  worktreeId: string,
  value: string
): CommitDraftsByWorktree {
  return { ...drafts, [worktreeId]: value }
}

export function shouldRenderCommitArea(
  unresolvedConflictCount: number,
  conflictOperation: GitConflictOperation
): boolean {
  return unresolvedConflictCount === 0 && conflictOperation === 'unknown'
}

export function pickDefaultSourceControlAgent(
  defaultAgent: TuiAgent | 'blank' | null | undefined,
  detectedAgents: TuiAgent[],
  disabledAgents?: TuiAgent[]
): TuiAgent | null {
  return pickSourceControlLaunchAgent({
    defaultAgent,
    detectedAgents,
    disabledAgents
  })
}

export function refreshSourceControlAfterRemoteAction({
  refreshGitStatus,
  refreshBranchCompare,
  refreshGitHistory,
  onError = (error) => console.warn('[SourceControl] post-remote refresh failed', error)
}: {
  refreshGitStatus: () => Promise<void>
  refreshBranchCompare: () => Promise<void>
  refreshGitHistory: () => Promise<void>
  onError?: (error: unknown) => void
}): void {
  // Why: fetch/sync can move the remote base ref without changing local files.
  // Refresh all three visible git projections so the branch comparison table
  // re-runs against the newly fetched base instead of waiting for polling.
  void Promise.all([refreshGitStatus(), refreshBranchCompare(), refreshGitHistory()]).catch(onError)
}

function remoteActionErrorMatchesSettledConflictOperation(
  kind: SourceControlActionError['kind'],
  operation: GitConflictOperation
): boolean {
  if (kind === 'rebase' || kind === 'abort_rebase') {
    return operation === 'rebase'
  }
  if (kind === 'abort_merge') {
    return operation === 'merge'
  }
  if (kind === 'pull' || kind === 'sync') {
    return operation === 'merge' || operation === 'rebase'
  }
  return false
}

export function clearRemoteActionErrorsForCompletedConflictOperations({
  remoteActionErrors,
  previousConflictOperations,
  currentConflictOperations
}: {
  remoteActionErrors: Record<string, SourceControlActionError | null>
  previousConflictOperations: Record<string, GitConflictOperation>
  currentConflictOperations: Record<string, GitConflictOperation>
}): Record<string, SourceControlActionError | null> {
  let next: Record<string, SourceControlActionError | null> | null = null
  for (const [worktreeId, error] of Object.entries(remoteActionErrors)) {
    if (!error) {
      continue
    }
    const previousOperation = previousConflictOperations[worktreeId] ?? 'unknown'
    const currentOperation = currentConflictOperations[worktreeId] ?? 'unknown'
    if (
      previousOperation === 'unknown' ||
      currentOperation !== 'unknown' ||
      !remoteActionErrorMatchesSettledConflictOperation(error.kind, previousOperation)
    ) {
      continue
    }
    next ??= { ...remoteActionErrors }
    next[worktreeId] = null
  }
  return next ?? remoteActionErrors
}
