/**
 * review-report-model-parser.ts — FE-CV-TASK-090-01
 *
 * Parses and validates the review report model from backend wire format.
 * Safe defaults for all missing/invalid fields (never throws).
 *
 * @module components/review-map/report/review-report-model-parser
 */

import type { ReviewState, ChangeOverlay } from '../../../../../shared/code-intel-types'

// ---------------------------------------------------------------------------
// Report model types
// ---------------------------------------------------------------------------

export type ReviewReportSection = {
  id: string
  title: string
  body: string
  severity: 'info' | 'warn' | 'error' | 'unknown'
  /** Raw Mermaid diagram source (validated by diagram-guard before rendering) */
  diagram?: string | null
}

export type ReviewReportSummary = {
  overallRisk: 'HIGH' | 'MEDIUM' | 'LOW' | 'unknown'
  testedBehavior: string[]
  untestedBehavior: string[]
  checklistItems: Array<{ id: string; text: string; checked: boolean }>
}

export type ReviewReportModel = {
  title: string
  description: string
  summary: ReviewReportSummary
  sections: ReviewReportSection[]
  overlay: ChangeOverlay[]
  headCommit: string | null
  generatedAt: string | null
  /** Model version — for forward/backward compatibility */
  modelVersion: number
}

// ---------------------------------------------------------------------------
// Parsers
// ---------------------------------------------------------------------------

function str(v: unknown, fallback = ''): string {
  return typeof v === 'string' ? v : fallback
}

function strArr(v: unknown): string[] {
  if (!Array.isArray(v)) return []
  return v.map((item) => str(item)).filter(Boolean)
}

function boolVal(v: unknown): boolean {
  return v === true
}

function safeInt(v: unknown, fallback = 0): number {
  if (typeof v !== 'number' || !isFinite(v)) return fallback
  return Math.floor(v)
}

const SEVERITIES = new Set(['info', 'warn', 'error'])
const RISK_LEVELS = new Set(['HIGH', 'MEDIUM', 'LOW'])

function parseSection(raw: unknown): ReviewReportSection {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  const severity = SEVERITIES.has(r.severity as string)
    ? (r.severity as 'info' | 'warn' | 'error')
    : 'unknown'
  return {
    id: str(r.id),
    title: str(r.title),
    body: str(r.body),
    severity,
    diagram: typeof r.diagram === 'string' ? r.diagram : null
  }
}

function parseChecklistItem(raw: unknown): { id: string; text: string; checked: boolean } {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  return { id: str(r.id), text: str(r.text), checked: boolVal(r.checked) }
}

function parseSummary(raw: unknown): ReviewReportSummary {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  return {
    overallRisk: RISK_LEVELS.has(r.overallRisk as string)
      ? (r.overallRisk as 'HIGH' | 'MEDIUM' | 'LOW')
      : 'unknown',
    testedBehavior: strArr(r.testedBehavior),
    untestedBehavior: strArr(r.untestedBehavior),
    checklistItems: Array.isArray(r.checklistItems)
      ? r.checklistItems.map(parseChecklistItem)
      : []
  }
}

function parseOverlay(raw: unknown): ChangeOverlay[] {
  if (!Array.isArray(raw)) return []
  return raw.map((item) => {
    const o = (typeof item === 'object' && item !== null ? item : {}) as Record<string, unknown>
    return {
      path: str(o.path),
      changeType: str(o.changeType, 'unknown') as ChangeOverlay['changeType'],
      oldPath: typeof o.oldPath === 'string' ? o.oldPath : null
    }
  })
}

/**
 * Parse a raw backend wire object into a ReviewReportModel.
 * Safe defaults for all missing/invalid fields.
 */
export function parseReviewReportModel(raw: unknown): ReviewReportModel {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>

  return {
    title: str(r.title, 'Review Report'),
    description: str(r.description),
    summary: parseSummary(r.summary),
    sections: Array.isArray(r.sections) ? r.sections.map(parseSection) : [],
    overlay: parseOverlay(r.overlay),
    headCommit: typeof r.headCommit === 'string' ? r.headCommit : null,
    generatedAt: typeof r.generatedAt === 'string' ? r.generatedAt : null,
    modelVersion: safeInt(r.modelVersion, 1)
  }
}
