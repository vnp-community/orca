/**
 * useQualityFeatureFlags.ts — FE-CV-TASK-085-01
 *
 * Aggregated feature flag hook for code-intel / quality / AI.
 * Reads from the code-intel slice (050) — does NOT call RPCs directly.
 *
 * Fail-closed: when support state is 'unknown', all flags are false.
 *
 * @module hooks/useQualityFeatureFlags
 */

import { useAppStore } from '@/store'

export type QualityFeatureFlags = {
  /** Overall state of the code-intel settings probe */
  state: 'unknown' | 'enabled' | 'disabled' | 'unsupported'
  /** Code-intel is ready and enabled */
  codeIntel: boolean
  /** Quality gate feature is enabled (requires codeIntel) */
  quality: boolean
  /** AI review feature is enabled (requires quality) */
  ai: boolean
}

const CLOSED_FLAGS: QualityFeatureFlags = {
  state: 'unknown',
  codeIntel: false,
  quality: false,
  ai: false,
}

type SupportSnapshot = {
  state: string
  effective?: {
    codeIntelEnabled?: boolean
    qualityGateEnabled?: boolean
    aiReviewEnabled?: boolean
  } | null
}

function readSupport(state: unknown): SupportSnapshot | undefined {
  return (state as Record<string, unknown>).codeIntelSupportState as SupportSnapshot | undefined
}

/**
 * Returns aggregated feature flags; every flag is false unless support is 'enabled'.
 * Does NOT call codeIntelClient directly.
 */
export function useQualityFeatureFlags(): QualityFeatureFlags {
  // Why: primitive selectors keep each store snapshot referentially stable
  // (zustand v5 loops on selectors that return a fresh object every call).
  const supportState = useAppStore((s) => readSupport(s)?.state ?? CLOSED_FLAGS.state)
  const codeIntel = useAppStore((s) => {
    const support = readSupport(s)
    return support?.state === 'enabled' && support.effective?.codeIntelEnabled === true
  })
  const quality = useAppStore((s) => {
    const support = readSupport(s)
    return (
      support?.state === 'enabled' &&
      support.effective?.codeIntelEnabled === true &&
      support.effective.qualityGateEnabled === true
    )
  })
  const ai = useAppStore((s) => {
    const support = readSupport(s)
    return (
      support?.state === 'enabled' &&
      support.effective?.codeIntelEnabled === true &&
      support.effective.qualityGateEnabled === true &&
      support.effective.aiReviewEnabled === true
    )
  })

  return { state: supportState as QualityFeatureFlags['state'], codeIntel, quality, ai }
}
