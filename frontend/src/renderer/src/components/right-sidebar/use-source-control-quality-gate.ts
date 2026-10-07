/**
 * use-source-control-quality-gate.ts — FE-CV-TASK-085-03
 *
 * Hook for the Source Control panel quality gate notice.
 * Manages loading the quality gate result, refresh on HEAD change,
 * push event subscription, display timer (3 s), and runChecks action.
 *
 * @module components/right-sidebar/use-source-control-quality-gate
 */

import { useEffect, useRef, useCallback, useState } from 'react'
import { useQualityFeatureFlags } from '../../hooks/useQualityFeatureFlags'
import { buildQualityNoticeViewModel } from './source-control-quality-gate-view-model'
import type { QualityGateNoticeViewModel } from './source-control-quality-gate-view-model'
import type { QualityGate } from '../../../../shared/code-intel-quality-types'

// ---------------------------------------------------------------------------
// Input
// ---------------------------------------------------------------------------

export type UseSourceControlQualityGateOpts = {
  worktreeId: string
  projectId: string | null | undefined
  headOid: string | null
  /** Base commit for comparison */
  base?: string | null
}

// ---------------------------------------------------------------------------
// Output
// ---------------------------------------------------------------------------

export type UseSourceControlQualityGateResult = {
  /** View model ready for rendering; null = not visible */
  viewModel: QualityGateNoticeViewModel
  /** Trigger quality scan run */
  runChecks: () => void
  /** Open the Review tab with quality lens */
  openReason: (checkName: string) => void
  isLoading: boolean
  timedOut: boolean
}

// ---------------------------------------------------------------------------
// Error kind classification for silent hide
// ---------------------------------------------------------------------------

const SILENT_HIDE_KINDS = new Set([
  'disabled', 'unsupported', 'forbidden', 'unknown', 'quality_running'
])

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

const DEBOUNCE_MS = 400
const DISPLAY_TIMEOUT_MS = 3000

export function useSourceControlQualityGate({
  worktreeId,
  projectId,
  headOid,
  base
}: UseSourceControlQualityGateOpts): UseSourceControlQualityGateResult {
  const flags = useQualityFeatureFlags()

  const [gate, setGate] = useState<QualityGate | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [timedOut, setTimedOut] = useState(false)

  const abortRef = useRef<AbortController | null>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const displayTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // visible = flags.quality && worktreeId && projectId
  const visible = flags.quality && Boolean(worktreeId) && Boolean(projectId)

  const fetchGate = useCallback(async (signal: AbortSignal) => {
    if (!visible || !worktreeId || !projectId) return

    setIsLoading(true)
    setTimedOut(false)

    // Display timer: 3s → timedOut
    displayTimerRef.current = setTimeout(() => {
      setTimedOut(true)
    }, DISPLAY_TIMEOUT_MS)

    try {
      // In a real implementation, this calls the code-intel client.
      // We use a stub here that is overridden in tests.
      const result = await loadQualityGate({ worktreeId, projectId, base: base ?? null, signal })
      if (signal.aborted) return
      setGate(result)
    } catch (err: unknown) {
      if (signal.aborted) return
      // Classify error kind — SILENT_HIDE_KINDS are hidden without toast
      const kindMsg = err instanceof Error ? err.message : ''
      const isSilent = Array.from(SILENT_HIDE_KINDS).some((k) => kindMsg.includes(k))
      if (!isSilent) {
        // Non-silent errors: show stale/unknown gate
        setGate((prev) => prev ? { ...prev, stale: true } : null)
      }
    } finally {
      if (!signal.aborted) {
        setIsLoading(false)
        if (displayTimerRef.current) clearTimeout(displayTimerRef.current)
      }
    }
  }, [visible, worktreeId, projectId, base])

  // Reload when headOid changes (debounced)
  useEffect(() => {
    if (!visible) {
      setGate(null)
      return
    }

    if (debounceRef.current) clearTimeout(debounceRef.current)
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl

    debounceRef.current = setTimeout(() => {
      void fetchGate(ctrl.signal)
    }, DEBOUNCE_MS)

    return () => {
      ctrl.abort()
      if (debounceRef.current) clearTimeout(debounceRef.current)
      if (displayTimerRef.current) clearTimeout(displayTimerRef.current)
    }
  }, [visible, headOid, fetchGate])

  const runChecks = useCallback(() => {
    if (!visible || !worktreeId || !projectId) return
    // Profile check + quality.start with scope 'changed'
    void startQualityRun({ worktreeId, projectId })
  }, [visible, worktreeId, projectId])

  const openReason = useCallback((_checkName: string) => {
    if (!worktreeId) return
    // Open Review tab with quality lens
    openReviewFromEntryPoint(worktreeId, 'source-control', { lens: 'quality' })
  }, [worktreeId])

  const viewModel = buildQualityNoticeViewModel(gate)

  return { viewModel, runChecks, openReason, isLoading, timedOut }
}

// ---------------------------------------------------------------------------
// Injectable stubs (overridden in tests via vi.mock)
// ---------------------------------------------------------------------------

/**
 * Load the quality gate from the code-intel client.
 * Injected at runtime — tests should mock this module.
 */
export async function loadQualityGate(_opts: {
  worktreeId: string
  projectId: string
  base: string | null
  signal: AbortSignal
}): Promise<QualityGate | null> {
  // Real implementation delegates to getCodeIntelClient().call(...)
  // Resolved by FE-CV-TASK-050-07 (code-intel-client) + wiring in 085-04
  throw new Error('loadQualityGate: not wired (requires code-intel-client init)')
}

export async function startQualityRun(_opts: {
  worktreeId: string
  projectId: string
}): Promise<void> {
  // Resolved by 087-02 store action + code-intel-client
  throw new Error('startQualityRun: not wired')
}

export function openReviewFromEntryPoint(
  _worktreeId: string,
  _source: string,
  _opts: { lens?: string }
): void {
  // Resolved by FE-CV-SOL-061 (entry points wiring)
  // No-op stub until 061 is implemented
}
