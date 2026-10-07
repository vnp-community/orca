// src/relay/agent-result-block-parser.ts
// Parses the structured ORCA_RESULT_BEGIN/END block from claude's stdout.
// Pure function — no fs, no process, no logging — so it can serve as a
// golden reference for the backend Go implementation (CR-REQ-029 §2.5).
//
// Golden test cases in agent-result-block-parser.test.ts are shared with backend.

// ── Constants ──────────────────────────────────────────────────────────────────

export const RESULT_BLOCK_MAX_BYTES = 256 * 1024  // 256 KiB

// ── Types ──────────────────────────────────────────────────────────────────────

export type ResultBlockErrorCode =
  | 'RESULT_BLOCK_MISSING'
  | 'RESULT_BLOCK_INVALID_JSON'
  | 'RESULT_BLOCK_TOO_LARGE'
  | 'RESULT_BLOCK_NOT_OBJECT'

export type ParsedResult =
  | { ok: true; value: Record<string, unknown> }
  | { ok: false; code: ResultBlockErrorCode; detail: string }

// ── Parser ─────────────────────────────────────────────────────────────────────

/**
 * Parses the last well-formed ORCA_RESULT_BEGIN … END block from stdout.
 * Algorithm is O(n) in the number of lines — single backward scan.
 * All branches return ParsedResult; never throws.
 */
export function parseResultBlock(stdout: string, nonce: string): ParsedResult {
  // Step 1: split lines and strip trailing \r
  const lines = stdout.split('\n').map(l => (l.endsWith('\r') ? l.slice(0, -1) : l))

  if (!nonce) {
    return { ok: false, code: 'RESULT_BLOCK_MISSING', detail: 'nonce is empty' }
  }

  const beginMarker = `ORCA_RESULT_BEGIN ${nonce}`
  const endMarker = `ORCA_RESULT_END ${nonce}`

  // Step 2: find the LAST END marker
  let endIndex = -1
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i]!.trim() === endMarker) {
      endIndex = i
      break
    }
  }
  if (endIndex === -1) {
    return { ok: false, code: 'RESULT_BLOCK_MISSING', detail: 'no END marker found' }
  }

  // Step 3: find the nearest BEGIN before endIndex
  let beginIndex = -1
  for (let i = endIndex - 1; i >= 0; i--) {
    if (lines[i]!.trim() === beginMarker) {
      beginIndex = i
      break
    }
  }
  if (beginIndex === -1) {
    return { ok: false, code: 'RESULT_BLOCK_MISSING', detail: 'no BEGIN marker found before END' }
  }

  // Step 4: extract body
  const body = lines.slice(beginIndex + 1, endIndex).join('\n')

  // Step 5: check size BEFORE JSON.parse
  if (Buffer.byteLength(body, 'utf8') > RESULT_BLOCK_MAX_BYTES) {
    return {
      ok: false,
      code: 'RESULT_BLOCK_TOO_LARGE',
      detail: `body is ${Buffer.byteLength(body, 'utf8')} bytes, max is ${RESULT_BLOCK_MAX_BYTES}`
    }
  }

  // Step 6: parse JSON
  let parsed: unknown
  try {
    parsed = JSON.parse(body)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message.slice(0, 200) : String(err).slice(0, 200)
    // Do NOT include body content in detail — could contain sensitive data
    return { ok: false, code: 'RESULT_BLOCK_INVALID_JSON', detail: msg }
  }

  // Step 7: must be a plain object (not null, not array, not primitive)
  if (
    parsed === null ||
    typeof parsed !== 'object' ||
    Array.isArray(parsed)
  ) {
    return {
      ok: false,
      code: 'RESULT_BLOCK_NOT_OBJECT',
      detail: `expected object, got ${parsed === null ? 'null' : Array.isArray(parsed) ? 'array' : typeof parsed}`
    }
  }

  return { ok: true, value: parsed as Record<string, unknown> }
}
