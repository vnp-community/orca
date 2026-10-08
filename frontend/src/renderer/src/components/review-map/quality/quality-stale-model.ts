/**
 * quality-stale-model.ts — FE-CV-TASK-087-04
 *
 * Decides when a gate result must be presented as out of date. Staleness is trusted from the
 * backend (`basedOn.stale`) and from the run's HEAD versus the current HEAD; the index commit
 * differing from HEAD is only informational.
 *
 * @module components/review-map/quality/quality-stale-model
 */

import type { QualityGate, QualityRun } from '../../../../../shared/code-intel-quality-types'

export type GateStaleReason = 'backend' | 'head-moved' | 'cache'

export type GateStaleness = { stale: boolean; reasons: GateStaleReason[] }

export function isGateStale(args: {
  gate: QualityGate | null
  /** The cached response is older than a later event (changed/finished). */
  cacheStale?: boolean
  runs?: readonly QualityRun[] | null
  currentHead?: string | null
}): GateStaleness {
  const { gate, runs, currentHead } = args
  if (!gate) {
    return { stale: false, reasons: [] }
  }
  const reasons: GateStaleReason[] = []
  if (gate.basedOn.stale) {
    reasons.push('backend')
  }
  const gateHead = gate.basedOn.headCommit
  const runHead = (runs ?? []).find((r) => gate.basedOn.runIds.includes(r.id))?.headCommit
  const evaluatedHead = gateHead || runHead
  if (currentHead && evaluatedHead && evaluatedHead !== currentHead) {
    reasons.push('head-moved')
  }
  if (args.cacheStale) {
    reasons.push('cache')
  }
  return { stale: reasons.length > 0, reasons }
}

/** Informational only (shown in the provenance line, never as a banner). */
export function indexDiffersFromHead(
  gate: QualityGate | null,
  currentHead: string | null | undefined
): boolean {
  return Boolean(
    gate && currentHead && gate.basedOn.indexCommit && gate.basedOn.indexCommit !== currentHead
  )
}
