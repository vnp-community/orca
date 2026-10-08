/**
 * useQualitySupport.ts — FE-CV-TASK-087-03
 *
 * Whether quality surfaces may render. The source of truth is `settings.get.effective`
 * (read through useQualityFeatureFlags); `typeof window.api.codeIntel` is never probed.
 * `disabled` and `unsupported` mean "render nothing": no tab, no chip, no toast.
 *
 * @module hooks/useQualitySupport
 */

import { useAppStore } from '@/store'
import { useQualityFeatureFlags } from './useQualityFeatureFlags'

export type QualitySupport = 'unknown' | 'enabled' | 'disabled' | 'unsupported'

// Why: these load errors mean the feature is switched off or unreachable here, not "failed".
const DISABLED_KINDS = new Set(['quality-disabled', 'disabled'])
const UNSUPPORTED_KINDS = new Set(['unsupported'])

export function useQualitySupport(worktreeId?: string | null): QualitySupport {
  const flags = useQualityFeatureFlags()
  const gateErrorKind = useAppStore((s) =>
    worktreeId ? s.codeIntelQualityByWorktree[worktreeId]?.errors.gate?.kind : undefined
  )
  if (flags.state === 'unknown') {
    return 'unknown'
  }
  if (flags.state === 'disabled') {
    return 'disabled'
  }
  if (flags.state === 'unsupported') {
    return 'unsupported'
  }
  if (!flags.quality) {
    return 'disabled'
  }
  if (gateErrorKind && DISABLED_KINDS.has(gateErrorKind)) {
    return 'disabled'
  }
  if (gateErrorKind && UNSUPPORTED_KINDS.has(gateErrorKind)) {
    return 'unsupported'
  }
  return 'enabled'
}
