/**
 * useOpenFindingsBySymbolKey.ts — FE-CV-TASK-059-03
 *
 * Lets other lenses (impact, structure, architecture) mark graph nodes that carry open
 * structural findings. Reads through the same cached `findings` query as the panel.
 *
 * @module hooks/useOpenFindingsBySymbolKey
 */

import { useMemo } from 'react'
import { selectOpenFindingsBySymbolKey } from '../components/review-map/findings/open-findings-by-symbol'
import type { SymbolFindingSummary } from '../components/review-map/findings/open-findings-by-symbol'
import { useCodeIntelFindings } from './useCodeIntelFindings'

const EMPTY: Record<string, SymbolFindingSummary> = {}

export function useOpenFindingsBySymbolKey(
  worktreeId: string | null,
  environmentId: string | null,
  enabled = true
): Record<string, SymbolFindingSummary> {
  const { findings } = useCodeIntelFindings(worktreeId, environmentId, {}, { enabled })
  return useMemo(() => (findings.length ? selectOpenFindingsBySymbolKey(findings) : EMPTY), [findings])
}
