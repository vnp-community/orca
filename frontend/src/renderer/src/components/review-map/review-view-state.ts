/**
 * review-view-state.ts — FE-CV-TASK-051-02
 *
 * Pure decision table for what the Review body shows. Priority (first match wins):
 * unsupported/disabled > scope-error > no-binding > forbidden > loading-initial >
 * tool-unavailable > repo-not-registered/path-not-allowed > index-missing > index-building >
 * offline > error > no-changes > ready(+banners).
 */

import type { DefaultScopeResult } from './review-scope-model'
import type { ChangeOverlayView, IndexStatusView, ReviewError } from './review-wire-types'

export type ReviewScreen =
  | { id: 'unsupported' }
  | { id: 'disabled' }
  | {
      id: 'scope-error'
      reason: 'invalid-base' | 'unborn-head' | 'no-merge-base' | 'error'
      message?: string
    }
  | { id: 'no-binding'; error: ReviewError }
  | { id: 'forbidden' }
  | { id: 'loading-initial' }
  | { id: 'tool-unavailable' }
  | { id: 'repo-not-registered' }
  | { id: 'path-not-allowed' }
  | { id: 'index-missing' }
  | { id: 'index-building'; percent: number | null; stage: string | null }
  | { id: 'offline' }
  | { id: 'error'; error: ReviewError }
  | { id: 'no-changes'; reason: 'unborn-head' | null }

export type ReviewBanner =
  | { id: 'stale'; indexedCommit: string | null; headCommit: string | null }
  | { id: 'truncated'; shown: number; total: number }
  | { id: 'scope-mismatch' }
  | { id: 'offline-cached' }
  | { id: 'error-cached'; error: ReviewError }
  | { id: 'new-data' }

export type ReviewViewStateInput = {
  support: 'unknown' | 'enabled' | 'disabled' | 'unsupported'
  /** SOL-050 selector says this worktree cannot be addressed. */
  selectorUnsupported: boolean
  scope: DefaultScopeResult
  status: { data: IndexStatusView | null; error: ReviewError | null }
  overlay: { data: ChangeOverlayView | null; error: ReviewError | null; pending: boolean }
  /** Backend pushed `changed` while the overlay is on screen. */
  hasNewDataSignal: boolean
}

export type ReviewViewState =
  | { kind: 'screen'; screen: ReviewScreen }
  | { kind: 'ready'; banners: ReviewBanner[] }

const screen = (s: ReviewScreen): ReviewViewState => ({ kind: 'screen', screen: s })

export function computeReviewViewState(input: ReviewViewStateInput): ReviewViewState {
  const { support, scope, status, overlay } = input
  if (support === 'disabled') {
    return screen({ id: 'disabled' })
  }
  if (support === 'unsupported' || input.selectorUnsupported) {
    return screen({ id: 'unsupported' })
  }
  if (scope.status === 'blocked') {
    return screen({ id: 'scope-error', reason: scope.reason, message: scope.message })
  }

  const error = overlay.error ?? status.error
  const overall = status.data?.overall ?? null

  if (error?.kind === 'no-binding') {
    return screen({ id: 'no-binding', error })
  }
  if (error?.kind === 'forbidden') {
    return screen({ id: 'forbidden' })
  }
  if (!error && !overlay.data && (scope.status === 'loading' || overlay.pending || !status.data)) {
    return screen({ id: 'loading-initial' })
  }
  if (error?.kind === 'tool-unavailable' || overall === 'NOT_INSTALLED') {
    return screen({ id: 'tool-unavailable' })
  }
  if (error?.kind === 'repo-not-registered') {
    return screen({ id: 'repo-not-registered' })
  }
  if (error?.kind === 'path-not-allowed') {
    return screen({ id: 'path-not-allowed' })
  }
  if (error?.kind === 'index-missing' || overall === 'MISSING') {
    return screen({ id: 'index-missing' })
  }
  if (overall === 'BUILDING' && !overlay.data) {
    return screen({
      id: 'index-building',
      percent: status.data?.activeJob?.percent ?? null,
      stage: status.data?.activeJob?.stage ?? null
    })
  }

  const offline = error?.kind === 'offline' || overall === 'OFFLINE'
  if (offline && !overlay.data) {
    return screen({ id: 'offline' })
  }
  if (error && !offline && !overlay.data) {
    return screen({ id: 'error', error })
  }

  const data = overlay.data
  if (!data) {
    return screen({ id: 'loading-initial' })
  }
  if (data.changedFiles.length === 0 && !data.limits.truncated.files) {
    return screen({
      id: 'no-changes',
      reason: data.emptyReason === 'unborn-head' ? 'unborn-head' : null
    })
  }

  const banners: ReviewBanner[] = []
  if (offline) {
    banners.push({ id: 'offline-cached' })
  } else if (error) {
    banners.push({ id: 'error-cached', error })
  }
  const freshness = data.indexFreshness
  if (overall === 'STALE' || freshness?.state === 'behind') {
    banners.push({
      id: 'stale',
      indexedCommit: freshness?.indexedCommit ?? null,
      headCommit: freshness?.headOid ?? null
    })
  }
  const truncated = Object.entries(data.limits.truncated).some(([, v]) => v === true)
  if (truncated) {
    const total = data.limits.totalCounts.changedFiles ?? data.changedFiles.length
    banners.push({ id: 'truncated', shown: data.changedFiles.length, total })
  }
  if (status.data?.scopeMismatch) {
    banners.push({ id: 'scope-mismatch' })
  }
  if (input.hasNewDataSignal) {
    banners.push({ id: 'new-data' })
  }
  return { kind: 'ready', banners }
}
