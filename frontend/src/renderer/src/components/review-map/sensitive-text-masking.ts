/**
 * sensitive-text-masking.ts — FE-CV-TASK-057-01
 *
 * Second masking layer for free-form backend strings (ERD defaults/comments, storage
 * via/payload...). Pure and idempotent; never logs or stores the text it sees.
 * Every scan is bounded (no unbounded backtracking) so a 1 MB string stays linear.
 */

export const MASK_PLACEHOLDER = '•••'

// Why: the patterns below must stay in step with the backend masker (CR-CV-072, O-16).
const SENSITIVE_KEY_WORDS =
  'password|passwd|secret|token|api[_-]?key|apikey|dsn|private[_-]?key|credential'

const PRIVATE_KEY_BLOCK =
  /-----BEGIN [A-Z ]{0,24}PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]{0,24}PRIVATE KEY-----|$)/g
const URL_USERINFO = /\b([a-z][a-z0-9+.-]{1,20}:\/\/)[^\s/@:]{1,256}(?::[^\s/@]{0,256})?@/gi
const KEYED_VALUE = new RegExp(
  `(${SENSITIVE_KEY_WORDS})([\\w-]{0,64}["']?\\s{0,16}[:=]\\s{0,16})` +
    `("[^"\\n]{1,2048}"|'[^'\\n]{1,2048}'|[^\\s,;&"'}\\]]{1,2048})`,
  'gi'
)
const JWT_LIKE = /\beyJ[\w-]{8,}\.[\w-]{8,}\.[\w-]*/g
// Why: '/', '.', '-' and '_' are excluded so paths, snake_case names and UUIDs never match.
const LONG_TOKEN = /(?<![\w/.+=-])[A-Za-z0-9+]{32,}={0,2}(?![\w/.+=-])/g

const HEX_ONLY = /^[0-9a-f]+$/i
// Why: a 40-hex string is a git commit; hex is only treated as a secret from 64 chars.
const HEX_SECRET_MIN_LENGTH = 64

function looksLikeSecretToken(candidate: string): boolean {
  if (HEX_ONLY.test(candidate)) {
    return candidate.length >= HEX_SECRET_MIN_LENGTH
  }
  const hasDigit = /\d/.test(candidate)
  const hasLower = /[a-z]/.test(candidate)
  const hasUpper = /[A-Z]/.test(candidate)
  return hasDigit && hasLower && hasUpper
}

function maskQuotedValue(value: string): string {
  const quote = value[0]
  if (quote === '"' || quote === "'") {
    return `${quote}${MASK_PLACEHOLDER}${quote}`
  }
  return MASK_PLACEHOLDER
}

export function maskSensitiveText(text: string): { text: string; masked: boolean } {
  if (text === '') {
    return { text, masked: false }
  }
  const out = text
    .replace(PRIVATE_KEY_BLOCK, MASK_PLACEHOLDER)
    .replace(URL_USERINFO, `$1${MASK_PLACEHOLDER}@`)
    .replace(
      KEYED_VALUE,
      (_m, key: string, sep: string, value: string) => `${key}${sep}${maskQuotedValue(value)}`
    )
    .replace(JWT_LIKE, MASK_PLACEHOLDER)
    .replace(LONG_TOKEN, (candidate) =>
      looksLikeSecretToken(candidate) ? MASK_PLACEHOLDER : candidate
    )
  return { text: out, masked: out !== text }
}

/** Masks only the listed string fields of a shallow copy; `masked` is true if any changed. */
export function maskSensitiveRecord<T extends object>(
  value: T,
  keys: readonly (keyof T)[]
): { value: T; masked: boolean } {
  const copy = { ...value }
  let masked = false
  for (const key of keys) {
    const field = copy[key]
    if (typeof field !== 'string') {
      continue
    }
    const result = maskSensitiveText(field)
    if (result.masked) {
      copy[key] = result.text as T[keyof T]
      masked = true
    }
  }
  return { value: copy, masked }
}
