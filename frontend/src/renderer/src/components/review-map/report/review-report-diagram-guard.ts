/**
 * review-report-diagram-guard.ts — FE-CV-TASK-090-03
 *
 * Validates and sanitizes Mermaid diagram strings from the backend.
 * Returns null when the diagram is invalid or potentially unsafe.
 *
 * Safety rules:
 * - Max 8 KiB length
 * - Must start with a recognized Mermaid diagram type keyword
 * - No HTML tags, script tags, or onerror attributes
 * - No URL-encoded or unicode-escaped payloads (basic check)
 *
 * @module components/review-map/report/review-report-diagram-guard
 */

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const MAX_DIAGRAM_BYTES = 8 * 1024

/** Recognized Mermaid diagram type keywords (per Mermaid docs) */
const VALID_DIAGRAM_TYPES = new Set([
  'graph', 'flowchart', 'sequencediagram', 'classDiagram', 'statediagram',
  'erDiagram', 'gantt', 'pie', 'gitgraph', 'mindmap', 'timeline',
  'xychart', 'block', 'sankey', 'quadrantchart', 'requirementdiagram',
  'c4diagram', 'journey'
])

const HTML_TAG_PATTERN = /<[a-z][a-z0-9]*[\s>\/]/i
const SCRIPT_PATTERN = /<script/i
const ONERROR_PATTERN = /\bonerror\b/i
const JAVASCRIPT_PATTERN = /javascript\s*:/i
const PERCENT_ENCODED_HTML = /%3c%73%63%72%69%70%74/i // %3C%73...

// ---------------------------------------------------------------------------
// Guard
// ---------------------------------------------------------------------------

/**
 * Validate a Mermaid diagram string.
 * Returns the (trimmed) diagram if valid, null otherwise.
 */
export function guardMermaidDiagram(raw: unknown): string | null {
  if (typeof raw !== 'string') return null
  const trimmed = raw.trim()

  if (!trimmed) return null

  // Size check
  if (new TextEncoder().encode(trimmed).length > MAX_DIAGRAM_BYTES) return null

  // Must start with a recognized diagram type (case-insensitive on first word)
  const firstToken = trimmed.split(/[\s\-({[]/)[0]?.toLowerCase()
  if (!firstToken || !VALID_DIAGRAM_TYPES.has(firstToken)) return null

  // Block HTML injection patterns
  if (HTML_TAG_PATTERN.test(trimmed)) return null
  if (SCRIPT_PATTERN.test(trimmed)) return null
  if (ONERROR_PATTERN.test(trimmed)) return null
  if (JAVASCRIPT_PATTERN.test(trimmed)) return null
  if (PERCENT_ENCODED_HTML.test(trimmed)) return null

  return trimmed
}

/**
 * Check if a string is a valid Mermaid diagram without modifying it.
 */
export function isValidMermaidDiagram(raw: unknown): boolean {
  return guardMermaidDiagram(raw) !== null
}
