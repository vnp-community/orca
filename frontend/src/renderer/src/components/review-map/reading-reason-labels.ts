/**
 * reading-reason-labels.ts — FE-CV-TASK-052-01
 *
 * ReasonCode -> i18n key + English fallback. Unknown codes render verbatim (never guessed).
 */

import { translate } from '@/i18n/i18n'

const REASON_LABELS: Record<string, { key: string; fallback: string }> = {
  contract: {
    key: 'auto.components.reviewMap.readingOrder.reason.contract',
    fallback: 'Contract change'
  },
  'dependency-of': {
    key: 'auto.components.reviewMap.readingOrder.reason.dependencyOf',
    fallback: 'Dependency of a later step'
  },
  leaf: { key: 'auto.components.reviewMap.readingOrder.reason.leaf', fallback: 'Leaf change' },
  cycle: {
    key: 'auto.components.reviewMap.readingOrder.reason.cycle',
    fallback: 'In a dependency cycle'
  },
  'no-edges': {
    key: 'auto.components.reviewMap.readingOrder.reason.noEdges',
    fallback: 'No known dependencies'
  },
  test: { key: 'auto.components.reviewMap.readingOrder.reason.test', fallback: 'Test' },
  doc: { key: 'auto.components.reviewMap.readingOrder.reason.doc', fallback: 'Documentation' },
  generated: {
    key: 'auto.components.reviewMap.readingOrder.reason.generated',
    fallback: 'Generated file'
  },
  overflow: {
    key: 'auto.components.reviewMap.readingOrder.reason.overflow',
    fallback: 'More files not listed'
  }
}

export function readingReasonLabel(code: string): string {
  const entry = REASON_LABELS[code]
  return entry ? translate(entry.key, entry.fallback) : code
}

export const KNOWN_READING_REASON_CODES: readonly string[] = Object.keys(REASON_LABELS)
