/**
 * c4-override-document.ts — FE-CV-TASK-055-05
 *
 * Local pre-flight check of a `c4.yaml` override before `codeIntel.c4.save`.
 * Why syntax-only: the semantic schema is still open (CONTRACT-open-issues O-12), so the
 * server stays the authority on meaning; we only block what can never be valid
 * (too big, not YAML, duplicate keys, root not a mapping).
 */

import { isMap, parseDocument } from 'yaml'

/** PQ-14(5): c4.yaml is capped at 64 KiB at every layer. */
export const C4_OVERRIDE_MAX_BYTES = 64 * 1024
const MAX_ALIAS_COUNT = 100

export type C4OverrideIssue = {
  /** 1-based; 0 when the issue is not tied to a position. */
  line: number
  column: number
  message: string
  severity: 'error' | 'warning'
}

export type C4OverrideValidation = {
  ok: boolean
  issues: C4OverrideIssue[]
  bytes: number
}

/** Semantic schema hook; stays null until O-12 is settled. */
export const c4OverrideDocumentSchema: { safeParse: (v: unknown) => { success: boolean; error?: { message: string } } } | null =
  null

function utf8Length(text: string): number {
  return new TextEncoder().encode(text).length
}

export function validateC4OverrideDocument(text: string): C4OverrideValidation {
  const input = typeof text === 'string' ? text : ''
  const bytes = utf8Length(input)
  const issues: C4OverrideIssue[] = []

  if (bytes > C4_OVERRIDE_MAX_BYTES) {
    issues.push({
      line: 0,
      column: 0,
      message: `Document is ${bytes} bytes; the limit is ${C4_OVERRIDE_MAX_BYTES}.`,
      severity: 'error'
    })
    // Why: don't parse an oversize document on every keystroke.
    return { ok: false, issues, bytes }
  }

  try {
    const doc = parseDocument(input, {
      uniqueKeys: true,
      prettyErrors: true,
      schema: 'core'
    })
    for (const err of doc.errors) {
      const pos = err.linePos?.[0]
      issues.push({
        line: pos?.line ?? 0,
        column: pos?.col ?? 0,
        message: err.message,
        severity: 'error'
      })
    }
    for (const warn of doc.warnings) {
      const pos = warn.linePos?.[0]
      issues.push({
        line: pos?.line ?? 0,
        column: pos?.col ?? 0,
        message: warn.message,
        severity: 'warning'
      })
    }
    if (doc.errors.length === 0) {
      // Empty / comment-only document is a valid "no overrides yet" draft.
      if (doc.contents !== null && !isMap(doc.contents)) {
        const pos = rangeStartToLinePos(input, doc.contents)
        issues.push({
          line: pos.line,
          column: pos.column,
          message: 'The document root must be a mapping.',
          severity: 'error'
        })
      } else {
        try {
          const value = doc.toJS({ maxAliasCount: MAX_ALIAS_COUNT })
          if (c4OverrideDocumentSchema) {
            const parsed = c4OverrideDocumentSchema.safeParse(value)
            if (!parsed.success) {
              issues.push({
                line: 0,
                column: 0,
                message: parsed.error?.message ?? 'Schema mismatch.',
                severity: 'error'
              })
            }
          }
        } catch (error) {
          issues.push({
            line: 0,
            column: 0,
            message: error instanceof Error ? error.message : 'Too many aliases.',
            severity: 'error'
          })
        }
      }
    }
  } catch (error) {
    issues.push({
      line: 0,
      column: 0,
      message: error instanceof Error ? error.message : 'Invalid YAML.',
      severity: 'error'
    })
  }

  return { ok: !issues.some((i) => i.severity === 'error'), issues, bytes }
}

function rangeStartToLinePos(text: string, node: unknown): { line: number; column: number } {
  const range = (node as { range?: [number, number, number] } | null)?.range
  return range ? offsetToLineColumn(text, range[0]) : { line: 1, column: 1 }
}

/** 0-based character offset -> 1-based line/column. */
export function offsetToLineColumn(text: string, offset: number): { line: number; column: number } {
  const end = Math.max(0, Math.min(offset, text.length))
  let line = 1
  let lineStart = 0
  for (let i = 0; i < end; i += 1) {
    if (text.charCodeAt(i) === 10) {
      line += 1
      lineStart = i + 1
    }
  }
  return { line, column: end - lineStart + 1 }
}

/** 1-based line/column -> 0-based character offset, clamped to the text. */
export function lineColumnToOffset(text: string, line: number, column: number): number {
  if (line <= 1 && column <= 1) {
    return 0
  }
  let currentLine = 1
  let i = 0
  while (currentLine < line && i < text.length) {
    if (text.charCodeAt(i) === 10) {
      currentLine += 1
    }
    i += 1
  }
  if (currentLine < line) {
    return text.length
  }
  let lineEnd = text.indexOf('\n', i)
  if (lineEnd < 0) {
    lineEnd = text.length
  }
  return Math.min(i + Math.max(0, column - 1), lineEnd)
}
