/**
 * review-scope-model.ts — FE-CV-TASK-051-01
 *
 * Pure scope model: what range of changes the Review workspace compares.
 * No new git commands: summaries come from the existing branch-compare result.
 */

import type { GitBranchCompareSummary } from '../../../../shared/types'
import type { HostedReviewProvider } from '../../../../shared/hosted-review'
import type { OverlayScopeView } from './review-wire-types'

export type ReviewScope =
  | { kind: 'branch'; baseRef: string; includeUncommitted: boolean }
  | { kind: 'range'; baseCommit: string; headCommit: string }
  | {
      kind: 'hostedReview'
      provider: HostedReviewProvider
      number: number
      headSha: string | null
      baseRefName: string | null
    }

export type DefaultScopeResult =
  | { status: 'ready'; scope: ReviewScope }
  | { status: 'loading' }
  | {
      status: 'blocked'
      reason: 'invalid-base' | 'unborn-head' | 'no-merge-base' | 'error'
      message?: string
    }

/** Default scope is the branch vs its base; non-ready summaries map to a guidance screen. */
export function resolveDefaultReviewScope(
  summary: GitBranchCompareSummary | null | undefined
): DefaultScopeResult {
  if (!summary || summary.status === 'loading') {
    return { status: 'loading' }
  }
  if (summary.status === 'ready') {
    return {
      status: 'ready',
      scope: { kind: 'branch', baseRef: summary.baseRef, includeUncommitted: true }
    }
  }
  return { status: 'blocked', reason: summary.status, message: summary.errorMessage }
}

export type ChangeOverlayParams = {
  base?: string
  head?: string
  mode: 'worktree' | 'committed'
}

/** Always sends an explicit `mode` so the backend never has to guess from an absent head (O-13). */
export function toChangeOverlayParams(scope: ReviewScope): ChangeOverlayParams {
  switch (scope.kind) {
    case 'branch':
      return {
        base: scope.baseRef,
        mode: scope.includeUncommitted ? 'worktree' : 'committed'
      }
    case 'range':
      return { base: scope.baseCommit, head: scope.headCommit, mode: 'committed' }
    case 'hostedReview':
      return {
        ...(scope.baseRefName ? { base: scope.baseRefName } : {}),
        ...(scope.headSha ? { head: scope.headSha } : {}),
        mode: 'committed'
      }
  }
}

/** Cache key; prefers the backend-resolved commits so a moved base ref changes the key. */
export function scopeKey(scope: ReviewScope, overlayScope?: OverlayScopeView | null): string {
  if (overlayScope && (overlayScope.mergeBase || overlayScope.baseOid || overlayScope.headOid)) {
    const base = overlayScope.mergeBase ?? overlayScope.baseOid ?? ''
    return `${scope.kind}:${base}..${overlayScope.headOid ?? ''}:${overlayScope.mode}`
  }
  switch (scope.kind) {
    case 'branch':
      return `branch:${scope.baseRef}:${scope.includeUncommitted ? 'worktree' : 'committed'}`
    case 'range':
      return `range:${scope.baseCommit}..${scope.headCommit}`
    case 'hostedReview':
      return `hostedReview:${scope.provider}:${scope.number}:${scope.headSha ?? ''}`
  }
}

/**
 * (base, head) pair that keys the persisted ReviewState row (SOL-052 open Q2: proposed
 * mergeBase/headOid; falls back to baseOid when no merge base).
 */
export function reviewStateKeyFromOverlayScope(
  scope: OverlayScopeView | null | undefined
): { baseCommit: string; headCommit: string } | null {
  if (!scope) {
    return null
  }
  const baseCommit = scope.mergeBase ?? scope.baseOid
  if (!baseCommit || !scope.headOid) {
    return null
  }
  return { baseCommit, headCommit: scope.headOid }
}

export function reviewScopeLabel(scope: ReviewScope): string {
  switch (scope.kind) {
    case 'branch':
      return scope.baseRef
    case 'range':
      return `${scope.baseCommit.slice(0, 7)}..${scope.headCommit.slice(0, 7)}`
    case 'hostedReview':
      return `#${scope.number}`
  }
}
