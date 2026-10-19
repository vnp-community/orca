/**
 * Request base ref (CR-REQ-039/040): the branch or tag every worktree of a Request starts from.
 *
 * `validateBaseRefSyntax` is the UI twin of the backend rule set in
 * backend-go/services/request-service/internal/domain/worktree_base_ref.go (ParseWorktreeBaseRef): same rules, same order,
 * so the dialog and the Overview row refuse early exactly what the server would refuse. Keep both in step; the shared case
 * table in request-base-ref.test.ts is copied from that Go file's tests.
 *
 * @module shared/request-base-ref
 */

import { parseRequest } from './request-wire-parsers'
import type { OrcaRequest } from './request-types'

/** `request.flowStatus.features` entry of a gateway that stores and enforces Request.baseRef (CR-REQ-039). */
export const REQUEST_BASE_REF_FEATURE = 'request.baseRef'

/** Matches the base_ref column (VARCHAR(255)); bytes, not characters, are the stricter bound. */
export const MAX_BASE_REF_BYTES = 255

/** The first broken rule; a stable code the UI turns into a localized message (the value itself is never echoed). */
export type BaseRefViolation =
  | 'blank'
  | 'too_long'
  | 'invalid_utf8'
  | 'leading_dash'
  | 'bare_at'
  | 'nul_byte'
  | 'control_character'
  | 'whitespace'
  | 'forbidden_character'
  | 'double_dot'
  | 'reflog_syntax'
  | 'double_slash'
  | 'leading_slash'
  | 'trailing_slash'
  | 'trailing_dot'
  | 'dot_component'
  | 'lock_component'

export type BaseRefSyntaxResult =
  | { ok: true; value: string }
  | { ok: false; reason: BaseRefViolation; character?: string }

// Characters git check-ref-format refuses outright.
const FORBIDDEN_REF_CHARACTERS = '\\~^:?*['

// A UTF-16 surrogate without its partner has no UTF-8 encoding: the JS spelling of the backend's "invalid UTF-8".
const LONE_SURROGATE = /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/

const utf8 = new TextEncoder()

// Go's unicode.IsSpace, not JS \s: JS also trims U+FEFF, Go does not, and the two must agree on what "blank" means.
function isGoSpace(codePoint: number): boolean {
  if (codePoint <= 0xff) {
    return (
      codePoint === 0x20 ||
      (codePoint >= 0x09 && codePoint <= 0x0d) ||
      codePoint === 0x85 ||
      codePoint === 0xa0
    )
  }
  return (
    codePoint === 0x1680 ||
    (codePoint >= 0x2000 && codePoint <= 0x200a) ||
    codePoint === 0x2028 ||
    codePoint === 0x2029 ||
    codePoint === 0x202f ||
    codePoint === 0x205f ||
    codePoint === 0x3000
  )
}

// Go's unicode.IsControl: category Cc.
function isGoControl(codePoint: number): boolean {
  return codePoint < 0x20 || (codePoint >= 0x7f && codePoint <= 0x9f)
}

function trimGoSpace(text: string): string {
  const chars = [...text]
  let start = 0
  let end = chars.length
  while (start < end && isGoSpace(chars[start].codePointAt(0) ?? 0)) {
    start += 1
  }
  while (end > start && isGoSpace(chars[end - 1].codePointAt(0) ?? 0)) {
    end -= 1
  }
  return chars.slice(start, end).join('')
}

function characterViolation(ref: string): BaseRefSyntaxResult | null {
  for (const character of ref) {
    const codePoint = character.codePointAt(0) ?? 0
    if (codePoint === 0) {
      return { ok: false, reason: 'nul_byte' }
    }
    if (isGoControl(codePoint)) {
      return { ok: false, reason: 'control_character' }
    }
    if (isGoSpace(codePoint)) {
      return { ok: false, reason: 'whitespace' }
    }
    if (FORBIDDEN_REF_CHARACTERS.includes(character)) {
      return { ok: false, reason: 'forbidden_character', character }
    }
  }
  return null
}

function sequenceViolation(ref: string): BaseRefViolation | null {
  if (ref.includes('..')) {
    return 'double_dot'
  }
  if (ref.includes('@{')) {
    return 'reflog_syntax'
  }
  if (ref.includes('//')) {
    return 'double_slash'
  }
  if (ref.startsWith('/')) {
    return 'leading_slash'
  }
  if (ref.endsWith('/')) {
    return 'trailing_slash'
  }
  if (ref.endsWith('.')) {
    return 'trailing_dot'
  }
  for (const part of ref.split('/')) {
    if (part.startsWith('.')) {
      return 'dot_component'
    }
    if (part.endsWith('.lock')) {
      return 'lock_component'
    }
  }
  return null
}

function violationOf(ref: string): BaseRefSyntaxResult | null {
  if (ref === '') {
    return { ok: false, reason: 'blank' }
  }
  if (utf8.encode(ref).length > MAX_BASE_REF_BYTES) {
    return { ok: false, reason: 'too_long' }
  }
  if (LONE_SURROGATE.test(ref)) {
    return { ok: false, reason: 'invalid_utf8' }
  }
  if (ref.startsWith('-')) {
    return { ok: false, reason: 'leading_dash' }
  }
  if (ref === '@') {
    return { ok: false, reason: 'bare_at' }
  }
  const character = characterViolation(ref)
  if (character) {
    return character
  }
  const sequence = sequenceViolation(ref)
  return sequence ? { ok: false, reason: sequence } : null
}

/**
 * Checks a typed ref before it is sent. `''` is valid and means "the repository default"; text that is only whitespace is
 * not, because the user typed something and it would silently turn into the default. Edge whitespace is trimmed.
 */
export function validateBaseRefSyntax(raw: string): BaseRefSyntaxResult {
  if (raw === '') {
    return { ok: true, value: '' }
  }
  const ref = trimGoSpace(raw)
  return violationOf(ref) ?? { ok: true, value: ref }
}

/**
 * Defense in depth after `request.create`/`request.setBaseRef`: a gateway older than CR-REQ-039 ignores an unknown `baseRef`
 * parameter and answers without the field, which would otherwise look like success while the branch was dropped.
 *  - `not_requested`: nothing was asked for, so there is nothing to check;
 *  - `match`: the stored ref is the one sent;
 *  - `mismatch`: the field is missing or differs.
 */
export type BaseRefEchoVerdict = 'not_requested' | 'match' | 'mismatch'

export function verifyBaseRefEcho(
  requested: string | undefined,
  request: Pick<OrcaRequest, 'baseRef'>
): BaseRefEchoVerdict {
  const sent = requested?.trim() ?? ''
  if (sent === '') {
    return 'not_requested'
  }
  return request.baseRef === sent ? 'match' : 'mismatch'
}

// ---------------------------------------------------------------------------
// request.create / request.setBaseRef wire shapes
// ---------------------------------------------------------------------------

/** The part of the `request.create` params that CR-REQ-039 adds; only sent when the user picked a branch. */
export type RequestCreateBaseRefParams = { baseRef?: string }

/** Params of `request.setBaseRef`. `expectedVersion` makes a concurrent edit fail with REQUEST_VERSION_CONFLICT. */
export type RequestSetBaseRefParams = {
  id: string
  baseRef: string
  expectedVersion?: number
}

/** A write that may have stored a ref the host could not confirm. */
export type RequestBaseRefWriteResult = {
  request: OrcaRequest
  /** True when the ref was stored but git could not confirm it exists (host unreachable). */
  baseRefUnverified: boolean
}

export type RequestCreateResult = RequestBaseRefWriteResult & {
  /** False when the same clientRequestId already created the Request; undefined when the gateway did not say. */
  created?: boolean
}

function asRecord(raw: unknown): Record<string, unknown> {
  return raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {}
}

function unverifiedOf(result: Record<string, unknown>, request: OrcaRequest): boolean {
  return result.baseRefUnverified === true || request.baseRefUnverified === true
}

/** Parses the answer of `request.setBaseRef`; never throws. */
export function parseSetBaseRefResult(raw: unknown): RequestBaseRefWriteResult {
  const result = asRecord(raw)
  const request = parseRequest(result.request ?? raw)
  return { request, baseRefUnverified: unverifiedOf(result, request) }
}

/** Parses the answer of `request.create`; never throws. */
export function parseRequestCreateResult(raw: unknown): RequestCreateResult {
  const result = asRecord(raw)
  const request = parseRequest(result.request)
  const created = typeof result.created === 'boolean' ? result.created : undefined
  return { request, created, baseRefUnverified: unverifiedOf(result, request) }
}
