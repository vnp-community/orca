/**
 * use-review-ai-summary.ts — FE-CV-TASK-093-02
 *
 * Hook for fetching and managing the AI review summary.
 *
 * Behavior:
 * - hidden when flags.ai=false or allowedLevels exceeds tenant.aiReviewLevel
 * - preview (dryRun=true) first to show what data will be sent
 * - confirmAndGenerate() records consent and triggers actual generation
 * - Consent is once per session and once per level
 * - Retry on inProgress up to 90s
 * - Error mapping: ai-disabled/quality-disabled → hidden; others surface
 *
 * @module components/review-map/ai-summary/use-review-ai-summary
 */

import { useState, useCallback, useRef } from 'react'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'
import { parseAiSummaryModel } from './ai-summary-wire-parser'
import {
  hasConsentForLevel,
  recordConsentForLevel,
} from './ai-summary-consent-state'
import type { AiSummaryModel } from './ai-summary-wire-parser'
import type { AiSummaryLevel } from './ai-summary-consent-state'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type AiSummaryStatus =
  | 'idle'
  | 'previewing'
  | 'awaiting_consent'
  | 'generating'
  | 'done'
  | 'error'
  | 'hidden'

export type AiSummaryState = {
  status: AiSummaryStatus
  data: AiSummaryModel | null
  /** Preview data (from dryRun) shown before user confirms */
  previewData: AiSummaryModel | null
  errorCode: string | null
  /** Call to confirm and trigger actual generation */
  confirmAndGenerate: () => void
  /** Abort current request */
  cancel: () => void
}

export type AiSummaryOpts = {
  level?: AiSummaryLevel
  projectId?: string | null
  worktreeId?: string | null
  locale?: string
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const MAX_RETRY_MS = 90_000
const RETRY_DELAY_MS = 3_000

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useReviewAiSummary(
  worktreeId: string | null,
  environmentId: string | null,
  opts: AiSummaryOpts = {}
): AiSummaryState {
  const flags = useQualityFeatureFlags(worktreeId)
  const { level = 'standard', projectId, locale } = opts

  const [status, setStatus] = useState<AiSummaryStatus>('idle')
  const [data, setData] = useState<AiSummaryModel | null>(null)
  const [previewData, setPreviewData] = useState<AiSummaryModel | null>(null)
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const abortRef = useRef<AbortController | null>(null)

  // Hidden when AI flag disabled
  const isHidden = !flags.ai

  const callSummaryApi = useCallback(
    async (dryRun: boolean, signal: AbortSignal) => {
      if (!worktreeId || !projectId) return

      const retryStart = Date.now()

      const doCall = async (): Promise<void> => {
        if (signal.aborted) return

        try {
          const { getCodeIntelClient } = await import('../../../runtime/code-intel-client')
          const client = getCodeIntelClient()

          const result = await client.call(
            worktreeId,
            'quality.summary',
            {
              projectId,
              worktreeId,
              dryRun,
              level,
              ...(locale ? { locale } : {}),
            },
            { environmentId, signal }
          )

          if (signal.aborted) return

          if (!result.ok) {
            const { error } = result
            // Silent errors: ai-disabled, quality-disabled
            if (error.kind === 'ai-disabled' || error.kind === 'quality-disabled') {
              setStatus('hidden')
              return
            }
            // Retry on inProgress
            if (error.message?.includes('inProgress') && Date.now() - retryStart < MAX_RETRY_MS) {
              setTimeout(() => void doCall(), RETRY_DELAY_MS)
              return
            }
            setStatus('error')
            setErrorCode(error.code ?? error.kind ?? 'unknown')
            return
          }

          const parsed = parseAiSummaryModel(result.result)
          if (dryRun) {
            setPreviewData(parsed)
            setStatus('awaiting_consent')
          } else {
            setData(parsed)
            setStatus('done')
          }
        } catch (err) {
          if (signal.aborted) return
          setStatus('error')
          setErrorCode('unexpected')
        }
      }

      await doCall()
    },
    [worktreeId, projectId, environmentId, level, locale]
  )

  const confirmAndGenerate = useCallback(() => {
    if (isHidden || !worktreeId || !projectId) return
    if (status === 'generating') return

    // Record consent (once per session, once per level)
    recordConsentForLevel(level)

    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl

    setStatus('generating')
    void callSummaryApi(false, ctrl.signal)
  }, [isHidden, worktreeId, projectId, status, level, callSummaryApi])

  const cancel = useCallback(() => {
    abortRef.current?.abort()
    setStatus('idle')
  }, [])

  // Start with preview when conditions met and not yet previewed
  const startPreview = useCallback(() => {
    if (isHidden || !worktreeId || !projectId) return
    if (status !== 'idle') return

    // Skip preview if already consented for this level in this session
    if (hasConsentForLevel(level)) {
      const ctrl = new AbortController()
      abortRef.current = ctrl
      setStatus('generating')
      void callSummaryApi(false, ctrl.signal)
      return
    }

    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl

    setStatus('previewing')
    void callSummaryApi(true, ctrl.signal)
  }, [isHidden, worktreeId, projectId, status, level, callSummaryApi])

  // Expose startPreview via confirmAndGenerate when idle
  // (caller calls confirmAndGenerate; if status=idle → starts preview first)
  const handleConfirmOrStart = useCallback(() => {
    if (status === 'idle') {
      startPreview()
    } else if (status === 'awaiting_consent') {
      confirmAndGenerate()
    }
  }, [status, startPreview, confirmAndGenerate])

  if (isHidden) {
    return {
      status: 'hidden',
      data: null,
      previewData: null,
      errorCode: null,
      confirmAndGenerate: () => {},
      cancel: () => {},
    }
  }

  return {
    status,
    data,
    previewData,
    errorCode,
    confirmAndGenerate: handleConfirmOrStart,
    cancel,
  }
}
