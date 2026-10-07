/**
 * AgentTurnVerificationLine.tsx — FE-CV-TASK-089-06
 *
 * One row per claim in the agent turn verification display.
 * Follows the view model from agent-turn-verification-view-model.ts (089-05).
 *
 * Rules:
 * - STYLEGUIDE tokens, lucide icons, size-3.5
 * - Icon + text per state (not color alone)
 * - No hex colors
 * - Tooltip: "Recorded, not verified"
 * - Plain text; no emoji
 * - Warnings shown inline (not just tooltip)
 *
 * @module components/review-map/turns/AgentTurnVerificationLine
 */

import React from 'react'
import { CircleHelp, TriangleAlert, Terminal, CircleCheck, Circle } from 'lucide-react'
import type { ClaimViewModel } from './agent-turn-verification-view-model'

// ---------------------------------------------------------------------------
// Icon per agreement
// ---------------------------------------------------------------------------

function ClaimIcon({ agreement }: { agreement: string }): React.ReactElement {
  const cls = 'size-3.5 shrink-0'
  switch (agreement) {
    case 'verified':
      return <CircleCheck className={`${cls} text-muted-foreground`} aria-hidden />
    case 'contradicted':
      return <TriangleAlert className={`${cls} text-destructive`} aria-hidden />
    case 'partial':
      return <TriangleAlert className={`${cls} text-yellow-500`} aria-hidden />
    case 'unverified':
    case 'unknown':
    default:
      return <CircleHelp className={`${cls} text-muted-foreground`} aria-hidden />
  }
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type AgentTurnVerificationLineProps = {
  claim: ClaimViewModel
  verifyingRunId?: string | null
  canRun?: boolean
  onRunChecks?: () => void
  onViewRun?: (runId: string) => void
  translate: (key: string, params?: Record<string, unknown>) => string
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function AgentTurnVerificationLine({
  claim,
  verifyingRunId,
  canRun,
  onRunChecks,
  onViewRun,
  translate,
}: AgentTurnVerificationLineProps): React.ReactElement {
  return (
    <li
      className="flex items-start gap-2 py-1.5"
      aria-label={translate(claim.labelKey)}
    >
      <ClaimIcon agreement={claim.agreement} />

      <div className="flex-1 min-w-0">
        {/* Claim text — plain text only, no HTML */}
        <p className="text-sm break-words">{claim.text}</p>

        <div className="flex items-center gap-2 flex-wrap mt-0.5">
          {/* Agreement label */}
          <span className="text-xs text-muted-foreground">
            {translate(claim.labelKey)}
          </span>

          {/* Basis label — "Inferred from agent statement" */}
          {claim.basisLabelKey && (
            <span className="text-xs text-muted-foreground italic">
              {translate(claim.basisLabelKey)}
            </span>
          )}

          {/* "Recorded, not verified" tooltip note — inline for warnings */}
          <span
            className="text-xs text-muted-foreground"
            title={translate(
              'auto.components.reviewMap.turns.verification.recordedNote'
            )}
          >
            <Terminal className="size-3 inline mr-0.5" aria-hidden />
          </span>

          {/* Link to verifying run */}
          {verifyingRunId && onViewRun && (
            <button
              type="button"
              onClick={() => onViewRun(verifyingRunId)}
              className="text-xs text-muted-foreground underline hover:no-underline"
            >
              {translate('auto.components.reviewMap.turns.verification.viewRun')}
            </button>
          )}

          {/* Run checks button */}
          {canRun && onRunChecks && (
            <button
              type="button"
              onClick={onRunChecks}
              className="text-xs text-muted-foreground underline hover:no-underline"
            >
              {translate('auto.components.reviewMap.turns.verification.runChecks')}
            </button>
          )}
        </div>

        {/* Warnings — inline, not tooltip-only (important warnings) */}
        {claim.warnings.length > 0 && (
          <ul className="mt-1 space-y-0.5">
            {claim.warnings.map((w, i) => (
              <li key={i} className="flex items-start gap-1">
                <TriangleAlert className="size-3 text-yellow-500 shrink-0 mt-0.5" aria-hidden />
                <span className="text-xs text-muted-foreground">{w}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </li>
  )
}
