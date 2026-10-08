/**
 * quality-marker-model.ts — FE-CV-TASK-087-09
 *
 * Pure mapping from QualityFinding to Monaco marker data and per-line gutter glyphs.
 * Monaco is not imported: the caller passes its MarkerSeverity numbers so this stays testable.
 * Columns are 1-based and 0 means "no column" (contract D5); file-level findings (line <= 0)
 * land on line 1.
 *
 * @module components/editor/quality-annotations/quality-marker-model
 */

import type { QualityFinding } from '../../../../../shared/code-intel-quality-types'

export type QualityMarkerSeverityMap = {
  error: number
  warning: number
  info: number
  hint: number
}

export type QualityMarkerData = {
  severity: number
  message: string
  startLineNumber: number
  startColumn: number
  endLineNumber: number
  endColumn: number
  source: string
  code: string
}

export type QualityGlyphLevel = 'error' | 'warning' | 'info'

export type QualityGlyph = {
  line: number
  severity: QualityGlyphLevel
  /** Fingerprint of the highest-severity finding on the line (first in input order). */
  fingerprint: string
  count: number
  /** Up to three "tool/ruleId: first line of message" strings for the hover. */
  summaries: string[]
}

export type BuildQualityMarkersOptions = {
  lineCount: number
  lineMaxColumn: (line: number) => number
  severityMap: QualityMarkerSeverityMap
}

const GLYPH_RANK: Record<QualityGlyphLevel, number> = { error: 3, warning: 2, info: 1 }
const MAX_GLYPH_SUMMARIES = 3

function toInt(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? Math.trunc(value) : fallback
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}

function glyphLevelOf(severity: string): QualityGlyphLevel {
  return severity === 'error' || severity === 'warning' ? severity : 'info'
}

function markerSeverityOf(severity: string, map: QualityMarkerSeverityMap): number {
  if (severity === 'error') {
    return map.error
  }
  if (severity === 'warning') {
    return map.warning
  }
  return severity === 'info' ? map.info : map.hint
}

function firstLine(text: string): string {
  const index = text.indexOf('\n')
  return (index === -1 ? text : text.slice(0, index)).trim()
}

export function resolveMarkerRange(
  finding: QualityFinding,
  opts: Pick<BuildQualityMarkersOptions, 'lineCount' | 'lineMaxColumn'>
): Pick<QualityMarkerData, 'startLineNumber' | 'startColumn' | 'endLineNumber' | 'endColumn'> {
  const rawLine = toInt(finding.line, 0)
  const fileLevel = rawLine <= 0
  const startLine = fileLevel ? 1 : clamp(rawLine, 1, opts.lineCount)
  const rawEnd = toInt(finding.endLine, startLine)
  const endLine =
    fileLevel || rawEnd < startLine ? startLine : clamp(rawEnd, startLine, opts.lineCount)
  const startMax = Math.max(opts.lineMaxColumn(startLine), 1)
  const endMax = Math.max(opts.lineMaxColumn(endLine), 1)
  const column = toInt(finding.column, 0)
  const endColumn = toInt(finding.endColumn, 0)
  const precise = !fileLevel && column >= 1 && column <= startMax && endColumn >= column
  if (!precise) {
    return { startLineNumber: startLine, startColumn: 1, endLineNumber: endLine, endColumn: endMax }
  }
  // Why: a zero-width range is not drawn by Monaco, so a point column spans one character.
  const resolvedEnd = endLine === startLine && endColumn === column ? column + 1 : endColumn
  return {
    startLineNumber: startLine,
    startColumn: column,
    endLineNumber: endLine,
    endColumn: clamp(resolvedEnd, 1, endMax)
  }
}

export function buildQualityMarkers(
  findings: readonly QualityFinding[],
  opts: BuildQualityMarkersOptions
): { markers: QualityMarkerData[]; glyphs: QualityGlyph[] } {
  const markers: QualityMarkerData[] = []
  const glyphByLine = new Map<number, QualityGlyph>()
  if (!Number.isFinite(opts.lineCount) || opts.lineCount < 1) {
    return { markers, glyphs: [] }
  }
  for (const finding of findings) {
    try {
      const range = resolveMarkerRange(finding, opts)
      const message = String(finding.message ?? '')
      const hint =
        typeof finding.fixHint === 'string' && finding.fixHint !== '' ? finding.fixHint : ''
      markers.push({
        ...range,
        severity: markerSeverityOf(finding.severity, opts.severityMap),
        message: hint ? `${message}\n${hint}` : message,
        source: String(finding.tool ?? ''),
        code: String(finding.ruleId ?? '')
      })
      accumulateGlyph(glyphByLine, range.startLineNumber, finding, message)
    } catch {
      // Why: one malformed finding must not blank the annotations of the whole file.
    }
  }
  const glyphs = [...glyphByLine.values()].sort((a, b) => a.line - b.line)
  return { markers, glyphs }
}

function accumulateGlyph(
  byLine: Map<number, QualityGlyph>,
  line: number,
  finding: QualityFinding,
  message: string
): void {
  const level = glyphLevelOf(finding.severity)
  const summary = `${finding.tool}/${finding.ruleId}: ${firstLine(message)}`
  const existing = byLine.get(line)
  if (!existing) {
    byLine.set(line, {
      line,
      severity: level,
      fingerprint: finding.fingerprint,
      count: 1,
      summaries: [summary]
    })
    return
  }
  existing.count += 1
  if (existing.summaries.length < MAX_GLYPH_SUMMARIES) {
    existing.summaries.push(summary)
  }
  if (GLYPH_RANK[level] > GLYPH_RANK[existing.severity]) {
    existing.severity = level
    existing.fingerprint = finding.fingerprint
  }
}
