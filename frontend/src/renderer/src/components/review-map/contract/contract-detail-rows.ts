/**
 * contract-detail-rows.ts — FE-CV-TASK-059-01
 *
 * ContractChange has no before/after field in the contract; the UI convention (open question,
 * SOL-059 §7) is `details.before` / `details.after`. If the backend settles on another
 * convention only this file changes. Every value is plain text and passes through the masker.
 *
 * @module components/review-map/contract/contract-detail-rows
 */

import type { ContractChange } from '../../../../../shared/code-intel-types'
import { maskSensitiveText } from '../sensitive-text-masking'

export const CONTRACT_DETAIL_VALUE_MAX = 400
const SIGNATURE_KEYS = new Set(['before', 'after'])

export type ContractDetailRows = {
  signature?: { before?: string; after?: string }
  rows: { key: string; value: string }[]
}

function safeText(raw: string): string {
  const masked = maskSensitiveText(raw).text
  return masked.length > CONTRACT_DETAIL_VALUE_MAX ? `${masked.slice(0, CONTRACT_DETAIL_VALUE_MAX)}…` : masked
}

export function contractDetailRows(change: Pick<ContractChange, 'details'>): ContractDetailRows {
  const details = change.details ?? {}
  const rows: { key: string; value: string }[] = []
  for (const key of Object.keys(details).sort()) {
    if (SIGNATURE_KEYS.has(key)) {
      continue
    }
    rows.push({ key, value: safeText(String(details[key])) })
  }
  const hasBefore = typeof details.before === 'string'
  const hasAfter = typeof details.after === 'string'
  if (!hasBefore && !hasAfter) {
    return { rows }
  }
  return {
    signature: {
      ...(hasBefore ? { before: safeText(details.before) } : {}),
      ...(hasAfter ? { after: safeText(details.after) } : {})
    },
    rows
  }
}
