/**
 * review-report-diagram-guard.ts — FE-CV-TASK-090-03
 *
 * The backend returns Mermaid source; we only vet it before it goes into a Markdown
 * fence or `mermaid.render`. Anything that could break out of the fence or smuggle
 * HTML / init directives is rejected (the diagram's `alt[]` text is kept by callers).
 *
 * @module components/review-map/report/review-report-diagram-guard
 */

export const MAX_MERMAID_BYTES = 6 * 1024

export type MermaidGuardResult =
  | { ok: true; src: string }
  | { ok: false; reason: 'too_large' | 'fence' | 'init_directive' | 'html' | 'control_chars' | 'empty' }

// Control characters other than tab / newline / carriage return.
// eslint-disable-next-line no-control-regex
const CONTROL_CHARS = /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/
const FENCE = /(`{3,}|~{3,})/
const INIT_DIRECTIVE = /%%\s*\{/
const HTML_TAG = /<\s*\/?\s*[a-z!][^>]*>?/i
const SCRIPT_URL = /javascript\s*:/i

export function guardMermaidSource(src: string): MermaidGuardResult {
  const text = typeof src === 'string' ? src.trim() : ''
  if (!text) {return { ok: false, reason: 'empty' }}
  if (new TextEncoder().encode(text).length > MAX_MERMAID_BYTES) {return { ok: false, reason: 'too_large' }}
  if (CONTROL_CHARS.test(text)) {return { ok: false, reason: 'control_chars' }}
  if (FENCE.test(text)) {return { ok: false, reason: 'fence' }}
  if (INIT_DIRECTIVE.test(text)) {return { ok: false, reason: 'init_directive' }}
  if (HTML_TAG.test(text) || SCRIPT_URL.test(text)) {return { ok: false, reason: 'html' }}
  return { ok: true, src: text }
}
