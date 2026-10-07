/**
 * ai-summary-wire-parser.ts — FE-CV-TASK-093-01
 *
 * Safe parsers for AI review summary wire format.
 * Text guard: strips HTML injection, limits length.
 *
 * @module components/review-map/ai-summary/ai-summary-wire-parser
 */

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type AiSummarySection = {
  id: string
  title: string
  body: string
  severity: 'info' | 'warn' | 'error' | 'unknown'
}

export type AiSummaryModel = {
  title: string
  summary: string
  sections: AiSummarySection[]
  generatedAt: string | null
  modelId: string | null
  /** Whether the summary is a preview/partial (stream in progress) */
  isPartial: boolean
}

// ---------------------------------------------------------------------------
// Text guard
// ---------------------------------------------------------------------------

const HTML_STRIP = /<[^>]+>/g
const MAX_BODY_CHARS = 8000
const MAX_TITLE_CHARS = 256

/**
 * Guard AI-generated text: strip HTML tags and limit length.
 * Never throws.
 */
export function guardAiText(raw: unknown, maxChars = MAX_BODY_CHARS): string {
  if (typeof raw !== 'string') return ''
  return raw.replace(HTML_STRIP, '').slice(0, maxChars)
}

// ---------------------------------------------------------------------------
// Parsers
// ---------------------------------------------------------------------------

const SEVERITIES = new Set(['info', 'warn', 'error'])

function parseSection(raw: unknown): AiSummarySection {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  const severity = SEVERITIES.has(r.severity as string)
    ? (r.severity as 'info' | 'warn' | 'error')
    : 'unknown'
  return {
    id: guardAiText(r.id, 64),
    title: guardAiText(r.title, MAX_TITLE_CHARS),
    body: guardAiText(r.body),
    severity
  }
}

/**
 * Parse AI summary from wire format.
 * Safe defaults for all missing/invalid fields.
 */
export function parseAiSummaryModel(raw: unknown): AiSummaryModel {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  return {
    title: guardAiText(r.title, MAX_TITLE_CHARS),
    summary: guardAiText(r.summary),
    sections: Array.isArray(r.sections) ? r.sections.map(parseSection) : [],
    generatedAt: typeof r.generatedAt === 'string' ? r.generatedAt : null,
    modelId: typeof r.modelId === 'string' ? r.modelId.slice(0, 128) : null,
    isPartial: r.isPartial === true
  }
}
