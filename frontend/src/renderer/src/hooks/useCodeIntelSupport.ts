/**
 * useCodeIntelSupport.ts — FE-CV-TASK-050-12
 *
 * Hook that manages code-intel support state via settings.get.
 * Polls every 60s when the Review tab is open and window is visible.
 *
 * Rules:
 * - Never calls any other channel when state !== 'enabled'
 * - Network/offline errors keep previous state (fail-closed on first call)
 * - Does NOT use typeof window.api.codeIntel (Proxy withFallback issue)
 *
 * @module hooks/useCodeIntelSupport
 */

import { useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import type { CodeIntelSupportState } from '../store/slices/code-intel'

const POLL_INTERVAL_MS = 60_000
const SETTINGS_METHOD = 'codeIntel.settings.get'

// Error kinds that should keep previous state (transient failures)
const KEEP_PREVIOUS_KINDS = new Set(['offline', 'rate-limited', 'timeout', 'unknown'])

// Error kinds that map to 'disabled'
const DISABLED_KINDS = new Set(['disabled'])

// Error kinds that map to 'unsupported'
const UNSUPPORTED_KINDS = new Set(['unsupported', 'forbidden'])

type CallResult =
  | { ok: true; result: unknown }
  | { ok: false; error: { kind: string; code: string | null; message: string } }

/**
 * Map a settings.get result to CodeIntelSupportState.
 */
export function mapSettingsResult(
  result: unknown,
  previous: CodeIntelSupportState
): CodeIntelSupportState {
  if (typeof result !== 'object' || result === null) {
    return { state: 'unknown' }
  }

  const r = result as Record<string, unknown>
  const effective = r.effective as Record<string, boolean> | undefined

  // Contract §6: settings.get effective.codeIntelEnabled is the single source of truth.
  if (effective?.codeIntelEnabled === false) {
    return { state: 'disabled', effective: null }
  }

  if (effective) {
    return {
      state: 'enabled',
      effective: {
        codeIntelEnabled: effective.codeIntelEnabled === true,
        qualityGateEnabled: effective.qualityGateEnabled === true,
        aiReviewEnabled: effective.aiReviewEnabled === true
      },
      lastPolledAt: new Date().toISOString()
    }
  }

  return previous
}

/**
 * Map an error to CodeIntelSupportState.
 * Transient errors keep previous state.
 */
function mapErrorToState(
  errorKind: string,
  previous: CodeIntelSupportState
): CodeIntelSupportState {
  if (KEEP_PREVIOUS_KINDS.has(errorKind)) {
    // Keep previous — fail closed on first
    return previous.state === 'unknown' ? { state: 'unknown' } : previous
  }
  if (DISABLED_KINDS.has(errorKind)) {
    return { state: 'disabled', effective: null }
  }
  if (UNSUPPORTED_KINDS.has(errorKind)) {
    return { state: 'unsupported', effective: null }
  }
  // Default for unrecognized errors: keep previous (fail closed on first)
  return previous.state === 'unknown' ? { state: 'unknown' } : previous
}

export type UseCodeIntelSupportOpts = {
  worktreeId: string | null
  environmentId: string | null
  isReviewTabOpen: boolean
  /** Injectable caller for testing; defaults to getCodeIntelClient().call() */
  callSettings?: (environmentId: string | null) => Promise<CallResult>
}

export type UseCodeIntelSupportResult = {
  supportState: CodeIntelSupportState
}

export function useCodeIntelSupport({
  worktreeId,
  environmentId,
  isReviewTabOpen,
  callSettings
}: UseCodeIntelSupportOpts): UseCodeIntelSupportResult {
  const setCodeIntelSupportState = useAppStore((s) => s.setCodeIntelSupportState)
  const supportState = useAppStore((s) => s.codeIntelSupportState)

  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    if (!isReviewTabOpen || !worktreeId) {return}

    async function poll() {
      abortRef.current?.abort()
      const ctrl = new AbortController()
      abortRef.current = ctrl

      try {
        let result: CallResult

        if (callSettings) {
          result = await callSettings(environmentId)
        } else {
          // Lazy-load client to avoid circular deps
          const { getCodeIntelClient } = await import('../runtime/code-intel-client')
          const client = getCodeIntelClient()
          result = await client.call(
            worktreeId ?? '',
            SETTINGS_METHOD,
            {},
            { environmentId, signal: ctrl.signal }
          )
        }

        if (ctrl.signal.aborted) {return}

        const prev: CodeIntelSupportState = useAppStore.getState().codeIntelSupportState ?? { state: 'unknown' }

        if (result.ok) {
          setCodeIntelSupportState(mapSettingsResult(result.result, prev))
        } else {
          setCodeIntelSupportState(mapErrorToState(result.error.kind, prev))
        }
      } catch {
        // Silently swallow — next poll will retry
      }
    }

    void poll()

    pollingRef.current = setInterval(() => {
      void poll()
    }, POLL_INTERVAL_MS)

    return () => {
      if (pollingRef.current) {clearInterval(pollingRef.current)}
      abortRef.current?.abort()
    }
  }, [worktreeId, environmentId, isReviewTabOpen, callSettings, setCodeIntelSupportState])

  return { supportState }
}
