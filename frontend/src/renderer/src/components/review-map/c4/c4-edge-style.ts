/**
 * c4-edge-style.ts — FE-CV-TASK-055-01
 *
 * Single table that drives both the edge stroke and the legend, so the legend can never drift
 * from what the canvas draws.
 */

import type { C4Relation } from '../../../../../shared/code-intel-architecture-types'

export const C4_LOW_CONFIDENCE_THRESHOLD = 0.8

export type C4RelationKindId =
  | 'uses'
  | 'implements'
  | 'calls-rpc'
  | 'reads'
  | 'writes'
  | 'publishes'
  | 'subscribes'

/** Relation kind -> stroke dash. Distinct patterns so kinds are told apart without color. */
export const C4_RELATION_DASH: Record<C4RelationKindId, string | undefined> = {
  uses: undefined,
  implements: '8 3 2 3',
  'calls-rpc': '10 4',
  reads: '2 3',
  writes: '6 2',
  publishes: '12 3 2 3 2 3',
  subscribes: '4 4'
}

export const C4_RELATION_KINDS = Object.keys(C4_RELATION_DASH) as C4RelationKindId[]

export type C4EdgeStyle = {
  strokeWidth: number
  strokeDasharray: string | undefined
  opacity: number
  lowConfidence: boolean
  violation: boolean
}

export function c4StrokeWidth(count: number): number {
  const safe = Number.isFinite(count) && count > 0 ? count : 1
  return Math.min(5, Math.max(1, 1 + Math.log2(safe)))
}

export function c4EdgeStyle(relation: Pick<C4Relation, 'kind' | 'count' | 'confidence' | 'violatesLayering'>): C4EdgeStyle {
  const known = (C4_RELATION_KINDS as string[]).includes(relation.kind)
  const baseDash = known ? C4_RELATION_DASH[relation.kind as C4RelationKindId] : '1 5'
  const lowConfidence = relation.confidence < C4_LOW_CONFIDENCE_THRESHOLD
  return {
    strokeWidth: c4StrokeWidth(relation.count),
    // Low confidence adds a faint dash on otherwise solid strokes; kinds keep their own pattern.
    strokeDasharray: lowConfidence && baseDash === undefined ? '3 3' : baseDash,
    opacity: lowConfidence ? 0.5 : 1,
    lowConfidence,
    violation: relation.violatesLayering === true
  }
}
