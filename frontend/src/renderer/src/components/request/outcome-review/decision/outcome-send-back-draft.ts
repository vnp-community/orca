/**
 * Outcome Review send-back draft — FE-OR-T310
 *
 * Builds the description of the follow-up Request from the items a person sent back: what each one says, what it rests on, and the
 * knowledge-graph ids with their snapshot, so the next person can find the same thing. Pure. The text goes through the masking
 * layer before it leaves the screen, and is cut at a size the Request body accepts.
 *
 * @module components/request/outcome-review/decision/outcome-send-back-draft
 */

import { maskSensitiveText } from '../../../review-map/sensitive-text-masking'
import type {
  OutcomeReview,
  ReviewEvidence,
  ReviewItem
} from '../../../../../../shared/outcome-review-types'

/** The title field of the child dialog holds this many characters. */
export const MAX_DRAFT_TITLE = 500
/** A few screens of findings; beyond this the draft says how many were left out instead of becoming unreadable. */
export const MAX_DRAFT_BODY = 6000
export const MAX_NOTE_IN_DRAFT = 1000

export type SendBackSuggestedType = 'bug' | 'task'

export type SendBackDraft = {
  title: string
  body: string
  suggestedType: SendBackSuggestedType
  /** Findings that did not fit and were counted instead of written. */
  omitted: number
  /** Something looked like a secret and was masked. */
  masked: boolean
}

export type SendBackInput = {
  requestTitle: string
  review: Pick<OutcomeReview, 'reviewId' | 'phase' | 'kgSnapshot'>
  items: ReviewItem[]
  /** The person's own words, added above the findings. */
  note: string
}

function evidenceLine(e: ReviewEvidence): string {
  switch (e.kind) {
    case 'kg':
      return e.snapshot ? `kg ${e.ref} (snapshot ${e.snapshot})` : `kg ${e.ref}`
    case 'source':
      return `sad ${e.doc || e.ref}${e.section ? ` §${e.section}` : ''}${e.line ? `:${e.line}` : ''}`
    default:
      return `${e.kind} ${e.ref}`
  }
}

function blockOf(item: ReviewItem): string {
  const lines = [
    `### ${item.title}`,
    `- ${item.section} · ${item.change} · ${item.severity}${item.origin === 'agent' ? ' · agent reported' : ''}`
  ]
  if (item.detail) {
    lines.push(`- ${item.detail}`)
  }
  if (item.containers.length > 0) {
    lines.push(`- containers: ${item.containers.join(', ')}`)
  }
  if (item.evidence.length > 0) {
    lines.push(`- evidence: ${item.evidence.map(evidenceLine).join('; ')}`)
  }
  return lines.join('\n')
}

export function suggestedTypeOf(items: ReviewItem[]): SendBackSuggestedType {
  const regressions = items.filter((i) => i.severity === 'regression').length
  return items.length > 0 && regressions * 2 > items.length ? 'bug' : 'task'
}

export function buildSendBackDraft(input: SendBackInput): SendBackDraft {
  const header = [
    `Findings of the outcome review of "${input.requestTitle}" (review ${input.review.reviewId.slice(0, 8)}).`
  ]
  const note = input.note.trim().slice(0, MAX_NOTE_IN_DRAFT)
  if (note) {
    header.push('', note)
  }
  const parts = [header.join('\n')]
  let used = parts[0].length
  let omitted = 0
  for (const item of input.items) {
    const block = blockOf(item)
    if (used + block.length + 2 > MAX_DRAFT_BODY - 80) {
      omitted += 1
      continue
    }
    parts.push(block)
    used += block.length + 2
  }
  if (omitted > 0) {
    parts.push(`(${omitted} more finding(s) not listed here; see the outcome review.)`)
  }
  const body = maskSensitiveText(parts.join('\n\n'))
  const title = maskSensitiveText(`Address review findings for ${input.requestTitle}`)
  return {
    title: title.text.slice(0, MAX_DRAFT_TITLE),
    body: body.text,
    suggestedType: suggestedTypeOf(input.items),
    omitted,
    masked: body.masked || title.masked
  }
}
