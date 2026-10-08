/**
 * agent-turn-verification-view-model.ts — FE-CV-TASK-089-05
 *
 * Pure view model for the "what the agent ran / what a re-run found" lines.
 * Wording stays neutral: the agent process self-reports, so we only compare
 * against an independent re-run and never attribute intent.
 *
 * @module components/review-map/turns/agent-turn-verification-view-model
 */

import type { AgentTurnCommandsSummary, CommandCategory } from './agent-tool-use-command-summarizer'

const BASE = 'auto.components.reviewMap.turns.verification'

export type ClaimAgreement = 'consistent' | 'contradicted' | 'unverified' | 'not_claimed' | 'unknown'

export type AgentTurnClaimItem = {
  kind: string
  basis: string
  confidence?: string
  agreement?: string
  agreementReason?: string
  verifyingRunId?: string
}

export type AgentTurnVerificationInput = {
  commandsSummary?: AgentTurnCommandsSummary | null
  claims?: { items: AgentTurnClaimItem[] } | null
}

export type RanLinePart = { categoryKey: string; count: number }

export type VerificationCheckViewModel = {
  id: string
  kind: string
  agreement: Exclude<ClaimAgreement, 'not_claimed'>
  labelKey: string
  kindKey: string
  basisKey: string | null
  reasonKey: string | null
  verifyingRunId: string | null
}

export type AgentTurnVerificationViewModel = {
  ranLine: { parts: RanLinePart[]; truncated: boolean } | null
  checks: VerificationCheckViewModel[]
  hasContradiction: boolean
}

const KNOWN_AGREEMENTS = new Set(['consistent', 'contradicted', 'unverified', 'not_claimed'])
const KNOWN_KINDS = new Set(['tests_pass', 'tests_fail', 'lint_clean', 'typecheck_clean', 'build_ok', 'all_done'])
const KNOWN_REASONS = new Set(['tree_may_differ', 'no_run', 'run_failed'])
const CATEGORY_ORDER: CommandCategory[] = ['test', 'lint', 'typecheck', 'build', 'install', 'git', 'other']
const AGREEMENT_RANK: Record<string, number> = { contradicted: 0, unverified: 1, unknown: 2, consistent: 3 }

function toAgreement(raw: string | undefined): ClaimAgreement {
  // Why: an absent or unrecognized value must read as "unknown", never as a pass.
  return raw && KNOWN_AGREEMENTS.has(raw) ? (raw as ClaimAgreement) : 'unknown'
}

function buildRanLine(summary: AgentTurnCommandsSummary | null | undefined) {
  if (!summary || summary.commands.length === 0) {
    return null
  }
  const counts = new Map<CommandCategory, number>()
  for (const command of summary.commands) {
    counts.set(command.category, (counts.get(command.category) ?? 0) + command.count)
  }
  const parts = CATEGORY_ORDER.filter((c) => counts.has(c)).map((category) => ({
    categoryKey: `${BASE}.category.${category}`,
    count: counts.get(category) ?? 0
  }))
  return { parts, truncated: summary.truncated }
}

export function buildAgentTurnVerificationViewModel(
  input: AgentTurnVerificationInput
): AgentTurnVerificationViewModel {
  const checks: VerificationCheckViewModel[] = []
  for (const [index, item] of (input.claims?.items ?? []).entries()) {
    const agreement = toAgreement(item.agreement)
    if (agreement === 'not_claimed') {
      continue
    }
    checks.push({
      id: `${item.kind}:${index}`,
      kind: item.kind,
      agreement,
      labelKey: `${BASE}.agreement.${agreement}`,
      kindKey: `${BASE}.kind.${KNOWN_KINDS.has(item.kind) ? item.kind : 'other'}`,
      basisKey: item.basis === 'stated' ? `${BASE}.basis.stated` : null,
      reasonKey:
        item.agreementReason && KNOWN_REASONS.has(item.agreementReason)
          ? `${BASE}.reason.${item.agreementReason}`
          : null,
      verifyingRunId: item.verifyingRunId ?? null
    })
  }
  checks.sort((a, b) => AGREEMENT_RANK[a.agreement] - AGREEMENT_RANK[b.agreement])
  return {
    ranLine: buildRanLine(input.commandsSummary),
    checks,
    hasContradiction: checks.some((c) => c.agreement === 'contradicted')
  }
}
