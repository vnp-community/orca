/**
 * Answer document reader — SOL-FE-E2E-005
 *
 * The wire parser keeps AI output as a JSON string. For kind=answer the backend schema
 * (request-service answer.schema.json) names the text `answer_markdown`; older drafts used
 * `answer`/`response`. Read both so the Answer box never shows "No data" for a real answer.
 *
 * @module components/request/solution/solution-answer-document
 */

export type AnswerCitation = { ref: string; excerpt?: string }

export type AnswerFollowUp = { type?: string; title: string }

export type AnswerDocument = {
  answerMarkdown: string | null
  /** Whole percent (0-100) or null when absent/not a number. */
  confidencePercent: number | null
  citations: AnswerCitation[]
  assumptions: string[]
  limitations: string[]
  followUp: AnswerFollowUp | null
  /** Original text when the content is not a JSON object; shown as Markdown. */
  rawText: string | null
}

const ANSWER_KEYS = ['answer_markdown', 'answerMarkdown', 'answer', 'response']
const CITATION_KEYS = ['citations', 'sources', 'references']
const FOLLOW_UP_KEYS = ['suggested_follow_up', 'suggestedFollowUp']

function asNonEmptyString(value: unknown): string | null {
  return typeof value === 'string' && value.trim() ? value : null
}

function firstPresent(obj: Record<string, unknown>, keys: string[]): unknown {
  for (const key of keys) {
    if (obj[key] !== undefined && obj[key] !== null) {
      return obj[key]
    }
  }
  return undefined
}

function toStringList(value: unknown): string[] {
  if (typeof value === 'string') {
    return value.trim() ? [value] : []
  }
  if (!Array.isArray(value)) {
    return []
  }
  return value.flatMap((item) => {
    if (typeof item === 'string') {
      return item.trim() ? [item] : []
    }
    if (item && typeof item === 'object') {
      const text = asNonEmptyString((item as Record<string, unknown>).text)
      return text ? [text] : []
    }
    return []
  })
}

function toCitations(value: unknown): AnswerCitation[] {
  const list = Array.isArray(value) ? value : value === undefined ? [] : [value]
  return list.flatMap((item): AnswerCitation[] => {
    if (typeof item === 'string') {
      return item.trim() ? [{ ref: item }] : []
    }
    if (!item || typeof item !== 'object') {
      return []
    }
    const r = item as Record<string, unknown>
    const ref =
      asNonEmptyString(r.ref) ??
      asNonEmptyString(r.url) ??
      asNonEmptyString(r.title) ??
      asNonEmptyString(r.text)
    if (!ref) {
      return []
    }
    const excerpt = asNonEmptyString(r.excerpt)
    return [excerpt ? { ref, excerpt } : { ref }]
  })
}

function toConfidencePercent(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) {
    return null
  }
  // Why: the schema says 0..1, but drafts sent 0..100; both read as a percentage.
  return Math.round(Math.min(value <= 1 ? value * 100 : value, 100))
}

function toFollowUp(value: unknown): AnswerFollowUp | null {
  if (!value || typeof value !== 'object') {
    return null
  }
  const r = value as Record<string, unknown>
  const title = asNonEmptyString(r.title)
  if (!title) {
    return null
  }
  const type = asNonEmptyString(r.type)
  return type ? { type, title } : { title }
}

const EMPTY_DOCUMENT: AnswerDocument = {
  answerMarkdown: null,
  confidencePercent: null,
  citations: [],
  assumptions: [],
  limitations: [],
  followUp: null,
  rawText: null
}

export function readAnswerDocument(content: string | undefined): AnswerDocument {
  const text = content ?? ''
  const trimmed = text.trim()
  if (!trimmed) {
    return EMPTY_DOCUMENT
  }
  if (trimmed.startsWith('{')) {
    try {
      const parsed: unknown = JSON.parse(trimmed)
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        const obj = parsed as Record<string, unknown>
        return {
          answerMarkdown: asNonEmptyString(firstPresent(obj, ANSWER_KEYS)),
          confidencePercent: toConfidencePercent(obj.confidence),
          citations: toCitations(firstPresent(obj, CITATION_KEYS)),
          assumptions: toStringList(obj.assumptions),
          limitations: toStringList(obj.limitations),
          followUp: toFollowUp(firstPresent(obj, FOLLOW_UP_KEYS)),
          rawText: null
        }
      }
    } catch {
      // Not JSON: shown as Markdown below.
    }
  }
  return { ...EMPTY_DOCUMENT, rawText: text }
}
