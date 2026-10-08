/**
 * use-source-control-quality-gate.ts — FE-CV-TASK-085-03
 *
 * Loads the quality gate for the Source Control notice. Informational only:
 * it never blocks commit / create-PR, and fails closed (no RPC) when the
 * quality flag is off or the worktree has no projectId.
 *
 * @module components/right-sidebar/use-source-control-quality-gate
 */

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForWorktree } from '@/lib/worktree-runtime-owner'
import { ensureReviewTab } from '@/lib/ensure-review-tab'
import { useQualityFeatureFlags } from '../../hooks/useQualityFeatureFlags'
import { getCodeIntelClient } from '../../runtime/code-intel-client'
import { subscribeCodeIntelEvents } from '../../lib/code-intel-event-bus'
import { trackQualityGateViewed } from '../../lib/review-telemetry'
import { CODE_INTEL_RPC_METHODS } from '../../../../shared/code-intel-rpc-methods'
import { buildQualityNoticeViewModel } from './source-control-quality-gate-view-model'
import type {
  QualityGate,
  QualityGateNoticeViewModel
} from './source-control-quality-gate-view-model'

export type UseSourceControlQualityGateOpts = {
  worktreeId: string
  projectId: string | null | undefined
  headOid: string | null
  /** Base ref for comparison */
  base?: string | null
}

export type UseSourceControlQualityGateResult = {
  /** `{ visible: false }` when the notice must not render */
  viewModel: QualityGateNoticeViewModel
  runChecks: () => void
  openReason: (checkName: string) => void
  isLoading: boolean
  timedOut: boolean
  running: boolean
  /** Verdict for telemetry; 'none' when no gate result is known. */
  verdict: 'pass' | 'warn' | 'fail' | 'unknown' | 'none'
  reasonCount: number
}

// Why: these kinds mean "feature not available here"; the notice hides silently, no toast.
const SILENT_HIDE_KINDS = new Set([
  'disabled',
  'quality-disabled',
  'unsupported',
  'forbidden',
  'no-binding',
  'not-found'
])

const DEBOUNCE_MS = 400
// Display-only timeout: the RPC is NOT cancelled so a late result still lands.
const DISPLAY_TIMEOUT_MS = 3000
const RETRY_DELAY_MS = 3000
const MAX_RETRY_MS = 90_000

function readEnvironmentId(worktreeId: string): string | null {
  return getRuntimeEnvironmentIdForWorktree(useAppStore.getState(), worktreeId)
}

function pickGate(result: unknown): QualityGate | null {
  const gate = (result as { gate?: unknown } | null)?.gate
  return gate && typeof gate === 'object' && typeof (gate as QualityGate).result === 'string'
    ? (gate as QualityGate)
    : null
}

// One quality_gate_viewed per (worktree, HEAD) per session; keeps the event count per turn low.
const viewedGateKeys = new Set<string>()

export function useSourceControlQualityGate({
  worktreeId,
  projectId,
  headOid,
  base
}: UseSourceControlQualityGateOpts): UseSourceControlQualityGateResult {
  const flags = useQualityFeatureFlags()
  const visible = flags.quality && Boolean(worktreeId) && Boolean(projectId)

  const [gate, setGate] = useState<QualityGate | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [timedOut, setTimedOut] = useState(false)
  const [failed, setFailed] = useState(false)
  const [running, setRunning] = useState(false)

  useEffect(() => {
    if (!visible) {
      setGate(null)
      setFailed(false)
      setTimedOut(false)
      setIsLoading(false)
      return
    }
    const ctrl = new AbortController()
    let debounce: ReturnType<typeof setTimeout> | null = null
    let displayTimer: ReturnType<typeof setTimeout> | null = null
    let retryTimer: ReturnType<typeof setTimeout> | null = null

    const load = async (startedAt: number): Promise<void> => {
      if (ctrl.signal.aborted) {
        return
      }
      setIsLoading(true)
      try {
        const response = await getCodeIntelClient().call(
          worktreeId,
          CODE_INTEL_RPC_METHODS.QUALITY_GATE,
          { projectId, worktreeId, ...(base ? { base } : {}) },
          { environmentId: readEnvironmentId(worktreeId), signal: ctrl.signal }
        )
        if (ctrl.signal.aborted) {
          return
        }
        if (response.ok) {
          setGate(pickGate(response.result))
          setFailed(false)
        } else if (SILENT_HIDE_KINDS.has(response.error.kind)) {
          setGate(null)
          setFailed(false)
        } else if (
          response.error.message?.includes('inProgress') &&
          Date.now() - startedAt < MAX_RETRY_MS
        ) {
          retryTimer = setTimeout(() => void load(startedAt), RETRY_DELAY_MS)
          return
        } else {
          // Keep the last gate but mark it stale; with none, show "unknown".
          setGate((prev) => (prev ? { ...prev, stale: true } : null))
          setFailed(true)
        }
      } catch {
        if (!ctrl.signal.aborted) {
          setFailed(true)
        }
      }
      if (!ctrl.signal.aborted) {
        setIsLoading(false)
        setTimedOut(false)
        if (displayTimer) {
          clearTimeout(displayTimer)
        }
      }
    }

    const schedule = (immediate: boolean): void => {
      if (debounce) {
        clearTimeout(debounce)
      }
      if (displayTimer) {
        clearTimeout(displayTimer)
      }
      setTimedOut(false)
      displayTimer = setTimeout(() => setTimedOut(true), DISPLAY_TIMEOUT_MS)
      debounce = setTimeout(() => void load(Date.now()), immediate ? 0 : DEBOUNCE_MS)
    }
    schedule(false)

    const unsubscribe = subscribeCodeIntelEvents((event) => {
      if (event.worktreeId !== worktreeId) {
        return
      }
      if (event.event === 'gateChanged' || event.event === 'qualityFinished') {
        if (event.event === 'qualityFinished') {
          setRunning(false)
        }
        schedule(true)
      }
    })
    const onVisible = (): void => {
      if (document.visibilityState === 'visible') {
        schedule(true)
      }
    }
    document.addEventListener('visibilitychange', onVisible)

    return () => {
      ctrl.abort()
      unsubscribe()
      document.removeEventListener('visibilitychange', onVisible)
      for (const timer of [debounce, displayTimer, retryTimer]) {
        if (timer) {
          clearTimeout(timer)
        }
      }
    }
  }, [visible, worktreeId, projectId, headOid, base])

  useEffect(() => {
    if (!visible || !gate) {
      return
    }
    const key = `${worktreeId}:${headOid ?? ''}`
    if (viewedGateKeys.has(key)) {
      return
    }
    viewedGateKeys.add(key)
    trackQualityGateViewed({
      verdict: gate.result === 'pass' || gate.result === 'warn' || gate.result === 'fail' ? gate.result : 'unknown',
      reasonCount: gate.reasons.length,
      stale: gate.stale === true,
      surface: 'source_control',
      source: 'local'
    })
  }, [visible, gate, worktreeId, headOid])

  const runChecks = useCallback(() => {
    if (!visible || running) {
      return
    }
    setRunning(true)
    void (async () => {
      try {
        const client = getCodeIntelClient()
        const opts = { environmentId: readEnvironmentId(worktreeId) }
        const profileResponse = await client.call(
          worktreeId,
          CODE_INTEL_RPC_METHODS.QUALITY_PROFILE_GET,
          { projectId, worktreeId },
          opts
        )
        const profiles = profileResponse.ok
          ? ((profileResponse.result as { runnableProfiles?: { name: string; ready?: boolean; heavy?: boolean }[] })
              .runnableProfiles ?? [])
          : []
        const chosen = profiles.find((p) => p.ready !== false && !p.heavy)
        if (!chosen) {
          setRunning(false)
          return
        }
        const started = await client.call(
          worktreeId,
          CODE_INTEL_RPC_METHODS.QUALITY_START,
          { projectId, worktreeId, profile: chosen.name, scope: 'changed' },
          opts
        )
        // Why: errors (env not ready, run in progress) are reflected by the next gate load, not a toast.
        if (!started.ok) {
          setRunning(false)
        }
      } catch {
        setRunning(false)
      }
    })()
  }, [visible, running, worktreeId, projectId])

  const openReason = useCallback(
    (_checkName: string) => {
      if (worktreeId) {
        // Why: the 'quality' lens may not be registered yet; opening Review without a lens is the fallback.
        ensureReviewTab(worktreeId)
      }
    },
    [worktreeId]
  )

  const viewModel = useMemo<QualityGateNoticeViewModel>(() => {
    if (!visible) {
      return { visible: false }
    }
    if (!gate && !timedOut && !failed) {
      return { visible: false }
    }
    return buildQualityNoticeViewModel(gate)
  }, [visible, gate, timedOut, failed])

  const verdict: UseSourceControlQualityGateResult['verdict'] = !gate
    ? 'none'
    : gate.result === 'pass' || gate.result === 'warn' || gate.result === 'fail'
      ? gate.result
      : 'unknown'

  return {
    viewModel,
    runChecks,
    openReason,
    isLoading,
    timedOut,
    running,
    verdict,
    reasonCount: gate?.reasons.length ?? 0
  }
}
