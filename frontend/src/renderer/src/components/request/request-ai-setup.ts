/**
 * request-ai-setup.ts — TASK-FE-E2E-005
 *
 * Which backend failures mean "the AI cannot run on the dev server, and here is the fix".
 * They arrive three ways: as an RPC error, as a failed run's `errorCode` (solution runs,
 * plan proposals) and as a failed classification's `classificationReason`.
 *
 * @module components/request/request-ai-setup
 */

export type AiSetupCode =
  | 'REQUEST_AI_NOT_CONFIGURED'
  | 'REQUEST_AI_NO_PROVIDER'
  | 'REQUEST_AI_CREDENTIAL_UNAVAILABLE'
  | 'REQUEST_AI_SESSION_EXPIRED'
  | 'REQUEST_AI_QUOTA_EXHAUSTED'
  | 'REQUEST_AI_AGENT_UNAVAILABLE'
  | 'REQUEST_AI_BUSY'

const AI_SETUP_CODES: ReadonlySet<string> = new Set<AiSetupCode>([
  'REQUEST_AI_NOT_CONFIGURED',
  'REQUEST_AI_NO_PROVIDER',
  'REQUEST_AI_CREDENTIAL_UNAVAILABLE',
  'REQUEST_AI_SESSION_EXPIRED',
  'REQUEST_AI_QUOTA_EXHAUSTED',
  'REQUEST_AI_AGENT_UNAVAILABLE',
  'REQUEST_AI_BUSY'
])

// Why: these are the other ways the service says "no dev server to run the AI on": a
// machine-stable classification reason, and the connection codes of the analysis stage.
const ALIASES: Readonly<Record<string, AiSetupCode>> = {
  'no dev server connected': 'REQUEST_AI_AGENT_UNAVAILABLE',
  REQUEST_SOLUTION_NO_CONNECTION: 'REQUEST_AI_AGENT_UNAVAILABLE',
  REQUEST_ANALYSIS_NO_CONNECTION: 'REQUEST_AI_AGENT_UNAVAILABLE'
}

const LEADING_CODE_RE = /^([A-Z][A-Z0-9_]+)(?::|\s|$)/

/**
 * The AI setup code a value stands for, or null. Accepts a bare code, `CODE: detail`, or a
 * known reason string; free text such as a successful classification rationale never matches.
 */
export function aiSetupCodeOf(value: string | null | undefined): AiSetupCode | null {
  const text = value?.trim()
  if (!text) {
    return null
  }
  if (ALIASES[text]) {
    return ALIASES[text]
  }
  const lead = LEADING_CODE_RE.exec(text)?.[1]
  if (!lead) {
    return null
  }
  if (AI_SETUP_CODES.has(lead)) {
    return lead as AiSetupCode
  }
  return ALIASES[lead] ?? null
}

/** First candidate that names an AI setup problem (error backendCode, run errorCode, reason...). */
export function firstAiSetupCode(...candidates: (string | null | undefined)[]): AiSetupCode | null {
  for (const c of candidates) {
    const code = aiSetupCodeOf(c)
    if (code) {
      return code
    }
  }
  return null
}
