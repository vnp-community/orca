/**
 * use-review-ai-summary.ts — FE-CV-TASK-093-02
 *
 * State machine behind the AI summary card. It never generates on its own:
 * `preview` runs a dry run (no model call), the user confirms what will be sent, and
 * only `confirmAndGenerate` calls the model. Flag off: hidden, zero RPCs.
 *
 * @module components/review-map/ai-summary/use-review-ai-summary
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForWorktree } from '@/lib/worktree-runtime-owner'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'
import { getCodeIntelClient } from '../../../runtime/code-intel-client'
import { trackReviewAiSummary } from '@/lib/review-telemetry'
import { CODE_INTEL_ERROR_CODES } from '../../../../../shared/code-intel-parsers'
import { CODE_INTEL_RPC_METHODS } from '../../../../../shared/code-intel-rpc-methods'
import { hasConsentForLevel, recordConsentForLevel } from './ai-summary-consent-state'
import { parseAiSummaryResponse } from './ai-summary-wire-parser'
import type { AiSummaryLevel, AiSummaryManifest, AiSummaryResponse } from './ai-summary-wire-parser'

const MAX_RETRY_MS = 90_000
const DEFAULT_RETRY_MS = 3000
const HIDING_KINDS = new Set(['ai-disabled', 'quality-disabled', 'disabled', 'unsupported', 'no-binding'])

export type AiSummaryUiState = 'hidden' | 'idle' | 'previewing' | 'awaiting-consent' | 'generating' | 'ready' | 'error'
export type AiSummaryError = 'no-relay' | 'bad-output' | 'rate-limited' | 'timeout' | 'forbidden' | 'unknown'

export type UseReviewAiSummaryResult = {
  state: AiSummaryUiState
  level: AiSummaryLevel
  allowedLevels: AiSummaryLevel[]
  manifest: AiSummaryManifest | null
  view: AiSummaryResponse | null
  error: AiSummaryError | null
  retryAfterSeconds: number | null
  preview: (level: AiSummaryLevel) => Promise<void>
  confirmAndGenerate: (opts?: { forceRefresh?: boolean }) => Promise<void>
  cancel: () => void
}

type CallOutcome =
  | { kind: 'ok'; response: AiSummaryResponse }
  | { kind: 'hidden' }
  | { kind: 'aborted' }
  | { kind: 'error'; error: AiSummaryError; retryAfterSeconds?: number }

export function useReviewAiSummary(args: {
  projectId: string | null | undefined
  worktreeId: string | null
  base?: string | null
  locale: string
  /** Highest data level the tenant allows (Settings.tenant.aiReviewLevel); defaults to metadata. */
  maxLevel?: AiSummaryLevel
}): UseReviewAiSummaryResult {
  const { projectId, worktreeId, base, locale, maxLevel = 'metadata' } = args
  const { ai } = useQualityFeatureFlags()
  const available = ai && Boolean(projectId) && Boolean(worktreeId)

  const [state, setState] = useState<AiSummaryUiState>('idle')
  const [level, setLevel] = useState<AiSummaryLevel>('metadata')
  const [manifest, setManifest] = useState<AiSummaryManifest | null>(null)
  const [view, setView] = useState<AiSummaryResponse | null>(null)
  const [error, setError] = useState<AiSummaryError | null>(null)
  const [retryAfterSeconds, setRetryAfterSeconds] = useState<number | null>(null)
  const abortRef = useRef<AbortController | null>(null)
  const levelRef = useRef<AiSummaryLevel>('metadata')

  useEffect(() => () => abortRef.current?.abort(), [])

  const callSummary = useCallback(
    async (dryRun: boolean, forceRefresh: boolean, signal: AbortSignal): Promise<CallOutcome> => {
      if (!worktreeId) {return { kind: 'hidden' }}
      const startedAt = Date.now()
      const client = getCodeIntelClient()
      for (;;) {
        let response
        try {
          response = await client.call(
            worktreeId,
            CODE_INTEL_RPC_METHODS.QUALITY_SUMMARY,
            {
              projectId,
              worktreeId,
              level: levelRef.current,
              dryRun,
              locale,
              ...(base ? { base } : {}),
              ...(forceRefresh ? { forceRefresh: true } : {})
            },
            { environmentId: getRuntimeEnvironmentIdForWorktree(useAppStore.getState(), worktreeId), signal }
          )
        } catch {
          return signal.aborted ? { kind: 'aborted' } : { kind: 'error', error: 'unknown' }
        }
        // Why: a cancelled request still finishes server-side; its late result must be dropped.
        if (signal.aborted) {return { kind: 'aborted' }}
        if (response.ok) {return { kind: 'ok', response: parseAiSummaryResponse(response.result) }}

        const { error: err } = response
        if (HIDING_KINDS.has(err.kind)) {return { kind: 'hidden' }}
        if (err.kind === 'forbidden') {return { kind: 'error', error: 'forbidden' }}
        if (err.code === CODE_INTEL_ERROR_CODES.AI_NO_RELAY) {return { kind: 'error', error: 'no-relay' }}
        if (err.code === CODE_INTEL_ERROR_CODES.AI_BAD_OUTPUT) {return { kind: 'error', error: 'bad-output' }}
        if (err.kind === 'rate-limited') {
          const wait = err.data?.retryAfterSeconds
          return { kind: 'error', error: 'rate-limited', ...(typeof wait === 'number' ? { retryAfterSeconds: wait } : {}) }
        }
        if (err.kind === 'timeout' || err.message?.includes('inProgress')) {
          if (err.message?.includes('inProgress') && Date.now() - startedAt < MAX_RETRY_MS) {
            const wait = err.data?.retryAfterMs
            await new Promise((resolve) => setTimeout(resolve, typeof wait === 'number' && wait > 0 ? wait : DEFAULT_RETRY_MS))
            if (signal.aborted) {return { kind: 'aborted' }}
            continue
          }
          return { kind: 'error', error: 'timeout' }
        }
        return { kind: 'error', error: 'unknown' }
      }
    },
    [worktreeId, projectId, base, locale]
  )

  const settle = useCallback((outcome: CallOutcome, onOk: (r: AiSummaryResponse) => void, dryRunOnly = false): void => {
    if (outcome.kind === 'aborted') {return}
    if (outcome.kind === 'hidden') {
      setState('hidden')
    } else if (outcome.kind === 'error') {
      if (!dryRunOnly) {
        trackReviewAiSummary({
          outcome: outcome.error === 'bad-output' ? 'bad_output' : 'error',
          level: levelRef.current,
          cache_hit: false,
          feedback: 'none'
        })
      }
      setError(outcome.error)
      setRetryAfterSeconds(outcome.retryAfterSeconds ?? null)
      setState('error')
    } else {
      onOk(outcome.response)
    }
  }, [])

  const generate = useCallback(
    async (forceRefresh: boolean): Promise<void> => {
      abortRef.current?.abort()
      const ctrl = new AbortController()
      abortRef.current = ctrl
      setError(null)
      setState('generating')
      const outcome = await callSummary(false, forceRefresh, ctrl.signal)
      settle(outcome, (response) => {
        if (!response.summary) {
          trackReviewAiSummary({ outcome: 'bad_output', level: levelRef.current, cache_hit: false, feedback: 'none' })
          setError('bad-output')
          setState('error')
          return
        }
        setView(response)
        setState('ready')
        trackReviewAiSummary({
          outcome: 'ok',
          level: levelRef.current,
          cache_hit: response.cache?.hit === true,
          feedback: 'none'
        })
      })
    },
    [callSummary, settle]
  )

  const preview = useCallback(
    async (next: AiSummaryLevel): Promise<void> => {
      if (!available) {return}
      // Why: the tenant caps the level; never send more than it allows.
      const chosen: AiSummaryLevel = next === 'diff' && maxLevel !== 'diff' ? 'metadata' : next
      levelRef.current = chosen
      setLevel(chosen)
      if (hasConsentForLevel(chosen)) {
        await generate(false)
        return
      }
      abortRef.current?.abort()
      const ctrl = new AbortController()
      abortRef.current = ctrl
      setError(null)
      setState('previewing')
      const outcome = await callSummary(true, false, ctrl.signal)
      settle(outcome, (response) => {
        setManifest(response.manifest)
        setState('awaiting-consent')
      }, true)
    },
    [available, maxLevel, generate, callSummary, settle]
  )

  const confirmAndGenerate = useCallback(
    async (opts?: { forceRefresh?: boolean }): Promise<void> => {
      if (!available || state === 'generating') {return}
      recordConsentForLevel(levelRef.current)
      await generate(opts?.forceRefresh === true)
    },
    [available, state, generate]
  )

  const cancel = useCallback((): void => {
    abortRef.current?.abort()
    setState(view ? 'ready' : 'idle')
  }, [view])

  if (!available || state === 'hidden') {
    return {
      state: 'hidden', level, allowedLevels: [], manifest: null, view: null, error: null, retryAfterSeconds: null,
      preview: async () => {}, confirmAndGenerate: async () => {}, cancel: () => {}
    }
  }
  return {
    state, level, allowedLevels: maxLevel === 'diff' ? ['metadata', 'diff'] : ['metadata'], manifest, view, error,
    retryAfterSeconds, preview, confirmAndGenerate, cancel
  }
}
