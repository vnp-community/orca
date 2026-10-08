/**
 * useFindingDismissal.ts — FE-CV-TASK-059-03
 *
 * Dismiss / resolve / restore a structural Finding via `codeIntel.dismissFinding`.
 * Optimistic: the row flips at once and is reverted (with the error kept per row, never a
 * toast) when the call fails. Dismissal is stored per repo by findingKey and is NOT a quality
 * gate waiver. Only args[0] is sent (U1).
 *
 * @module hooks/useFindingDismissal
 */

import { useCallback, useRef, useState } from 'react'
import type { Finding } from '../../../shared/code-intel-types'
import { defaultCodeIntelCall } from './useCodeIntelQuery'
import type { CodeIntelCallFn, CodeIntelQueryError } from './useCodeIntelQuery'
import type { FindingDismissalOverride } from './useCodeIntelFindings'

export const FINDING_TEXT_MAX = 500

export type FindingDismissalResult = { ok: true } | { ok: false; error: CodeIntelQueryError }

export type UseFindingDismissalArgs = {
  worktreeId: string | null
  environmentId: string | null
  setOverride: (findingKey: string, dismissed: FindingDismissalOverride) => void
  clearOverride: (findingKey: string) => void
  reload: () => void
  callFn?: CodeIntelCallFn
}

export type UseFindingDismissalResult = {
  dismiss: (finding: Pick<Finding, 'findingKey'>, input: { reason: string; note?: string }) => Promise<FindingDismissalResult>
  resolve: (finding: Pick<Finding, 'findingKey'>, input?: { note?: string }) => Promise<FindingDismissalResult>
  restore: (finding: Pick<Finding, 'findingKey'>) => Promise<FindingDismissalResult>
  pendingKeys: ReadonlySet<string>
  errorsByKey: Readonly<Record<string, CodeIntelQueryError>>
  clearError: (findingKey: string) => void
}

const clip = (value: string): string => value.slice(0, FINDING_TEXT_MAX)

export function useFindingDismissal(args: UseFindingDismissalArgs): UseFindingDismissalResult {
  const { worktreeId, environmentId, setOverride, clearOverride, reload } = args
  const callFn = args.callFn ?? defaultCodeIntelCall
  const [pendingKeys, setPending] = useState<ReadonlySet<string>>(new Set())
  const [errorsByKey, setErrors] = useState<Record<string, CodeIntelQueryError>>({})
  const lockedRef = useRef(new Set<string>())

  const clearError = useCallback((key: string) => {
    setErrors((prev) => {
      if (!(key in prev)) {
        return prev
      }
      const next = { ...prev }
      delete next[key]
      return next
    })
  }, [])

  const run = useCallback(
    async (
      findingKey: string,
      params: Record<string, unknown>,
      optimistic: FindingDismissalOverride
    ): Promise<FindingDismissalResult> => {
      if (!worktreeId) {
        return { ok: false, error: { kind: 'unsupported', code: null, message: 'No worktree', retryable: false } }
      }
      // Lock synchronously so a double click cannot send two writes for one finding.
      if (lockedRef.current.has(findingKey)) {
        return { ok: false, error: { kind: 'run-in-progress', code: null, message: 'Busy', retryable: true } }
      }
      lockedRef.current.add(findingKey)
      setPending((prev) => new Set(prev).add(findingKey))
      clearError(findingKey)
      setOverride(findingKey, optimistic)
      try {
        const outcome = await callFn(
          worktreeId,
          'codeIntel.dismissFinding',
          { findingKey, ...params },
          new AbortController().signal,
          environmentId
        )
        if (outcome.ok) {
          return { ok: true }
        }
        clearOverride(findingKey)
        setErrors((prev) => ({ ...prev, [findingKey]: outcome.error }))
        // Someone else changed it or it is gone: show the server truth.
        if (outcome.error.kind === 'conflict' || outcome.error.kind === 'not-found') {
          reload()
        }
        return { ok: false, error: outcome.error }
      } catch {
        clearOverride(findingKey)
        const error: CodeIntelQueryError = { kind: 'unknown', code: null, message: 'Unexpected error', retryable: true }
        setErrors((prev) => ({ ...prev, [findingKey]: error }))
        return { ok: false, error }
      } finally {
        lockedRef.current.delete(findingKey)
        setPending((prev) => {
          const next = new Set(prev)
          next.delete(findingKey)
          return next
        })
      }
    },
    [worktreeId, environmentId, callFn, setOverride, clearOverride, reload, clearError]
  )

  const stamp = (
    disposition: 'ignored' | 'resolved',
    reason: string,
    note?: string
  ): NonNullable<Finding['dismissed']> => ({
    by: '',
    at: new Date().toISOString(),
    reason,
    disposition,
    ...(note ? { note } : {})
  })

  const dismiss: UseFindingDismissalResult['dismiss'] = useCallback(
    async (finding, input) => {
      const reason = clip(input.reason.trim())
      if (!reason) {
        // Ignoring needs a reason; never send an empty one.
        return { ok: false, error: { kind: 'validation', code: null, message: 'Reason required', retryable: false } }
      }
      const note = input.note?.trim() ? clip(input.note.trim()) : undefined
      return run(
        finding.findingKey,
        { action: 'dismiss', disposition: 'ignored', reason, ...(note ? { note } : {}) },
        stamp('ignored', reason, note)
      )
    },
    [run]
  )

  const resolve: UseFindingDismissalResult['resolve'] = useCallback(
    async (finding, input) => {
      const note = input?.note?.trim() ? clip(input.note.trim()) : undefined
      return run(
        finding.findingKey,
        { action: 'dismiss', disposition: 'resolved', ...(note ? { note } : {}) },
        stamp('resolved', '', note)
      )
    },
    [run]
  )

  const restore: UseFindingDismissalResult['restore'] = useCallback(
    (finding) => run(finding.findingKey, { action: 'restore' }, null),
    [run]
  )

  return { dismiss, resolve, restore, pendingKeys, errorsByKey, clearError }
}
