/**
 * useReviewDecisionTelemetry.ts — FE-CV-TASK-095-05
 *
 * Feeds review-decision-tracker: registers each finished agent turn (only while code
 * intelligence or the quality gate is enabled) and exposes a stable `recordDecision`
 * for the commit / create-PR paths. Adds no behavior of its own; telemetry never throws.
 *
 * @module hooks/useReviewDecisionTelemetry
 */

import { useCallback, useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import { detectAgentTurnCompletions } from '../components/review-map/turns/agent-turn-completion-detector'
import { decide, registerCompletion } from '../lib/review-decision-tracker'
import type { ReviewDecision, ReviewDecisionContext } from '../lib/review-decision-tracker'
import { useQualityFeatureFlags } from './useQualityFeatureFlags'

export function useReviewDecisionTelemetry(context: ReviewDecisionContext): {
  recordDecision: (worktreeId: string, decision: ReviewDecision) => void
} {
  const { codeIntel, quality } = useQualityFeatureFlags()
  const enabled = codeIntel || quality
  // Why: callers invoke recordDecision from long-lived callbacks; read the latest gate lazily.
  const contextRef = useRef(context)
  contextRef.current = context

  useEffect(() => {
    if (!enabled) {
      return
    }
    return useAppStore.subscribe((state, prev) => {
      for (const { entry } of detectAgentTurnCompletions(prev.agentStatusByPaneKey, state.agentStatusByPaneKey)) {
        if (entry.worktreeId) {
          registerCompletion(entry.worktreeId, entry.stateStartedAt)
        }
      }
    })
  }, [enabled])

  const recordDecision = useCallback(
    (worktreeId: string, decision: ReviewDecision): void => {
      if (!enabled) {
        return
      }
      try {
        decide(worktreeId, decision, contextRef.current)
      } catch {
        // Telemetry must never break commit / PR creation.
      }
    },
    [enabled]
  )

  return { recordDecision }
}
