/**
 * ai-summary-wire-parser.ts — FE-CV-TASK-093-01
 *
 * Parses `quality.summary` (CONTRACT ui-api 4.7 `AiReviewSummary`). Model output is
 * untrusted: text is cleaned of control / bidi-override characters and capped, then
 * rendered as plain text only. The manifest/cache shapes are not pinned by the contract,
 * so the parser tolerates missing fields.
 *
 * @module components/review-map/ai-summary/ai-summary-wire-parser
 */

export type AiSummaryLevel = 'metadata' | 'diff'

export type AiReviewSummary = {
  summary: string
  risks: { text: string; refs: string[] }[]
  readFirst: { file: string; why: string }[]
  model: string
  level: AiSummaryLevel | 'unknown'
  promptVersion: string
  inputDigest: string
  generatedAt: string
  refsDropped: number
}

export type AiSummaryManifest = {
  level: AiSummaryLevel | 'unknown'
  files: { path: string; bytes: number; hunks: number; withheld: string | null }[]
  findingsCount: number
  redactions: number
  totalBytes: number
  estimatedTokens: number
  provider: string | null
  suspectedInjection: boolean
}

export type AiSummaryCache = { hit: boolean; createdAt: string | null; expiresAt: string | null }

export type AiSummaryResponse = {
  summary: AiReviewSummary | null
  manifest: AiSummaryManifest | null
  cache: AiSummaryCache | null
}

const MAX_SUMMARY_CHARS = 4000
const MAX_ITEM_CHARS = 500
const MAX_PATH_CHARS = 512
const MAX_ITEMS = 20

// C0/C1 controls (except \n and \t) and bidi embedding/override/isolate characters.
// eslint-disable-next-line no-control-regex
const UNSAFE_TEXT_CHARS = /[\u0000-\u0008\u000b-\u001f\u007f-\u009f‎‏‪-‮⁦-⁩]/g

/** Cleans untrusted text for display; HTML is not stripped because it is only ever rendered as text. */
export function guardAiText(raw: unknown, maxChars = MAX_ITEM_CHARS): string {
  if (typeof raw !== 'string') {return ''}
  return raw.replace(UNSAFE_TEXT_CHARS, '').slice(0, maxChars)
}

type Rec = Record<string, unknown>
const rec = (v: unknown): Rec => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Rec) : {})
const arr = (v: unknown): unknown[] => (Array.isArray(v) ? v : [])
const count = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) && v >= 0 ? Math.floor(v) : 0)
const level = (v: unknown): AiSummaryLevel | 'unknown' => (v === 'metadata' || v === 'diff' ? v : 'unknown')

function parseSummary(raw: unknown): AiReviewSummary | null {
  const r = rec(raw)
  if (typeof r.summary !== 'string' || r.summary.trim() === '') {return null}
  return {
    summary: guardAiText(r.summary, MAX_SUMMARY_CHARS),
    risks: arr(r.risks).slice(0, MAX_ITEMS).map((x) => ({
      text: guardAiText(rec(x).text),
      refs: arr(rec(x).refs).filter((p): p is string => typeof p === 'string').slice(0, MAX_ITEMS).map((p) => guardAiText(p, MAX_PATH_CHARS))
    })),
    readFirst: arr(r.readFirst).slice(0, MAX_ITEMS).map((x) => ({
      file: guardAiText(rec(x).file, MAX_PATH_CHARS),
      why: guardAiText(rec(x).why)
    })),
    model: guardAiText(r.model, 128),
    level: level(r.level),
    promptVersion: guardAiText(r.promptVersion, 64),
    inputDigest: guardAiText(r.inputDigest, 128),
    generatedAt: guardAiText(r.generatedAt, 64),
    refsDropped: count(r.refsDropped)
  }
}

function parseManifest(raw: unknown): AiSummaryManifest | null {
  if (typeof raw !== 'object' || raw === null) {return null}
  const m = rec(raw)
  return {
    level: level(m.level),
    files: arr(m.files).slice(0, 200).map((x) => {
      const f = rec(x)
      return {
        path: guardAiText(f.path, MAX_PATH_CHARS),
        bytes: count(f.bytes),
        hunks: count(f.hunks),
        withheld: typeof f.withheld === 'string' ? guardAiText(f.withheld, 64) : null
      }
    }),
    findingsCount: count(m.findingsCount),
    redactions: count(m.redactions),
    totalBytes: count(m.totalBytes),
    estimatedTokens: count(m.estimatedTokens),
    provider: typeof m.provider === 'string' ? guardAiText(m.provider, 64) : null,
    suspectedInjection: m.suspectedInjection === true
  }
}

function parseCache(raw: unknown): AiSummaryCache | null {
  if (typeof raw !== 'object' || raw === null) {return null}
  const c = rec(raw)
  return {
    hit: c.hit === true,
    createdAt: typeof c.createdAt === 'string' ? c.createdAt : null,
    expiresAt: typeof c.expiresAt === 'string' ? c.expiresAt : null
  }
}

export function parseAiSummaryResponse(raw: unknown): AiSummaryResponse {
  const r = rec(raw)
  return { summary: parseSummary(r.summary), manifest: parseManifest(r.manifest), cache: parseCache(r.cache) }
}
