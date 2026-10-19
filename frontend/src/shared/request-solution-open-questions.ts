/**
 * Solution open questions parser — CR-REQ-027 §2.5, CONTRACT-request-ui-api (SolutionView.options)
 *
 * The AI document keeps snake_case keys: `open_questions: [{id, text, blocking}]`.
 * Older drafts sent `string[]` (never blocking). Tolerant: anything unreadable is dropped.
 *
 * @module shared/request-solution-open-questions
 */

import type { SolutionOpenQuestion } from './request-types'

function questionText(item: Record<string, unknown>): string {
  for (const key of ['text', 'question', 'prompt']) {
    const v = item[key]
    if (typeof v === 'string' && v.trim()) {
      return v.trim()
    }
  }
  return ''
}

/** Read `open_questions` (or `openQuestions`) from a Solution `options` object. */
export function parseSolutionOpenQuestions(
  doc: Record<string, unknown> | undefined
): SolutionOpenQuestion[] | undefined {
  const list = doc?.open_questions ?? doc?.openQuestions
  if (!Array.isArray(list)) {
    return undefined
  }
  const out: SolutionOpenQuestion[] = []
  list.forEach((item, index) => {
    const fallbackId = `Q-${index + 1}`
    if (typeof item === 'string') {
      if (item.trim()) {
        out.push({ id: fallbackId, text: item.trim(), blocking: false })
      }
      return
    }
    if (item === null || typeof item !== 'object' || Array.isArray(item)) {
      return
    }
    const r = item as Record<string, unknown>
    const text = questionText(r)
    if (!text) {
      return
    }
    const id = typeof r.id === 'string' && r.id.trim() ? r.id.trim() : fallbackId
    out.push({ id, text, blocking: r.blocking === true })
  })
  return out
}
