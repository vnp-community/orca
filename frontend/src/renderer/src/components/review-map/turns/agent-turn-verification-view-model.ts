/**
 * agent-turn-verification-view-model.ts — FE-CV-TASK-089-05
 *
 * Pure view model for agent turn verification/reconciliation display.
 * Maps agreement values to neutral i18n keys — no attribution language.
 *
 * Language rules:
 * - NEVER use: "lying", "deceiving", "faking", "consistent" (for unverified)
 * - "unverified" must NOT map to "consistent"
 * - "basis:stated" → "Inferred from agent statement"
 *
 * @module components/review-map/turns/agent-turn-verification-view-model
 */

import type { NormalizedCommand } from './agent-tool-use-command-summarizer'

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

export type ClaimAgreement = 'verified' | 'unverified' | 'contradicted' | 'partial' | 'unknown'

export type AgentClaim = {
  id: string
  text: string
  agreement: ClaimAgreement | string
  basis?: 'stated' | 'observed' | 'inferred' | string
}

export type AgentTurnVerificationInput = {
  claims: AgentClaim[]
  commandsSummary?: {
    commands: NormalizedCommand[]
    toolCounts: Record<string, number>
    truncated: boolean
  } | null
}

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

export type ClaimViewModel = {
  id: string
  text: string
  /** Neutral i18n key */
  labelKey: string
  /** Optional basis label key */
  basisLabelKey?: string
  agreement: ClaimAgreement
}

export type AgentTurnVerificationViewModel = {
  /** Human-readable summary of commands run (i18n key + params) */
  ranLineKey: string
  ranLineParams: Record<string, unknown>
  claims: ClaimViewModel[]
  hasClaims: boolean
}

// ---------------------------------------------------------------------------
// Agreement mapping
// ---------------------------------------------------------------------------

const AGREEMENT_KEY_MAP: Record<ClaimAgreement, string> = {
  verified: 'auto.components.reviewMap.turns.verification.agreement.verified',
  unverified: 'auto.components.reviewMap.turns.verification.agreement.unverified',
  contradicted: 'auto.components.reviewMap.turns.verification.agreement.contradicted',
  partial: 'auto.components.reviewMap.turns.verification.agreement.partial',
  unknown: 'auto.components.reviewMap.turns.verification.agreement.unknown'
}

function toAgreement(raw: string): ClaimAgreement {
  if (['verified', 'unverified', 'contradicted', 'partial'].includes(raw)) {
    return raw as ClaimAgreement
  }
  return 'unknown'
}

const BASIS_STATED_KEY = 'auto.components.reviewMap.turns.verification.basis.stated'

function basisLabelKey(basis?: string): string | undefined {
  if (basis === 'stated') return BASIS_STATED_KEY
  return undefined
}

// ---------------------------------------------------------------------------
// Command summary line builder
// ---------------------------------------------------------------------------

/**
 * Build a localized "Agent ran: test (3 times), lint" display summary.
 */
function buildRanLine(
  commandsSummary: AgentTurnVerificationInput['commandsSummary']
): { ranLineKey: string; ranLineParams: Record<string, unknown> } {
  if (!commandsSummary || Object.keys(commandsSummary.toolCounts).length === 0) {
    return {
      ranLineKey: 'auto.components.reviewMap.turns.verification.ranNothing',
      ranLineParams: {}
    }
  }

  // Build a short list from commands: "test (3x), lint, git"
  const countsByCategory: Record<string, number> = {}
  for (const cmd of commandsSummary.commands) {
    countsByCategory[cmd.category] = (countsByCategory[cmd.category] ?? 0) + 1
  }

  const parts = Object.entries(countsByCategory)
    .slice(0, 4)
    .map(([cat, count]) => ({ cat, count }))

  return {
    ranLineKey: 'auto.components.reviewMap.turns.verification.ran',
    ranLineParams: { parts, truncated: commandsSummary.truncated }
  }
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

/**
 * Build the verification view model for an agent turn.
 * Pure function — no side effects.
 */
export function buildAgentTurnVerificationViewModel(
  input: AgentTurnVerificationInput
): AgentTurnVerificationViewModel {
  const { ranLineKey, ranLineParams } = buildRanLine(input.commandsSummary)

  const claims: ClaimViewModel[] = input.claims.map((c) => {
    const agreement = toAgreement(c.agreement)
    return {
      id: c.id,
      text: c.text,
      labelKey: AGREEMENT_KEY_MAP[agreement],
      basisLabelKey: basisLabelKey(c.basis),
      agreement
    }
  })

  return {
    ranLineKey,
    ranLineParams,
    claims,
    hasClaims: claims.length > 0
  }
}
