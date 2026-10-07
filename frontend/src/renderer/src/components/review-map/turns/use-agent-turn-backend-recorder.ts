/**
 * use-agent-turn-backend-recorder.ts — FE-CV-TASK-089-04
 *
 * Hook that subscribes to AgentTurnCompletion events and records
 * each completed turn to the backend via quality.turn.record.
 *
 * Rules:
 * - quality flag off → no-op, no store subscription, no RPC
 * - Errors in RPC do NOT block local ReviewTurnMarker creation (SOL-060)
 * - PQ-35: only called from renderer
 * - Prompt excerpts read from settings, not the full text
 *
 * @module components/review-map/turns/use-agent-turn-backend-recorder
 */

import { useEffect, useRef } from 'react'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'

// ---------------------------------------------------------------------------
// Types (matching the contracts from SOL-060/061 — assumed interface)
// ---------------------------------------------------------------------------

export type AgentTurnCompletion = {
  worktreeId: string
  turnId: string
  completedAt: number
  headOid: string | null
  /** Prompt excerpt — never the full user text */
  promptExcerpt?: string | null
}

export type TurnRecordParams = {
  worktreeId: string
  turnId: string
  headOid: string | null
  promptExcerpt?: string | null
  completedAt: number
}

export type AgentTurnBackendRecorderOpts = {
  worktreeId: string | null
  /** Subscribe to turn completion events */
  onSubscribe: (handler: (event: AgentTurnCompletion) => void) => () => void
  /** RPC call function */
  rpcCall?: (method: string, params: TurnRecordParams) => Promise<void>
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useAgentTurnBackendRecorder({
  worktreeId,
  onSubscribe,
  rpcCall,
}: AgentTurnBackendRecorderOpts): void {
  const flags = useQualityFeatureFlags(worktreeId)
  const rpcCallRef = useRef(rpcCall)
  rpcCallRef.current = rpcCall

  useEffect(() => {
    // CR-089: quality flag off → no subscription, no RPC
    if (!flags.quality || !worktreeId) return

    const unsubscribe = onSubscribe(async (event: AgentTurnCompletion) => {
      if (event.worktreeId !== worktreeId) return

      const params: TurnRecordParams = {
        worktreeId: event.worktreeId,
        turnId: event.turnId,
        headOid: event.headOid,
        promptExcerpt: event.promptExcerpt ?? null,
        completedAt: event.completedAt,
      }

      // Fire and forget — RPC errors do NOT block local turn marker creation
      void (async () => {
        try {
          if (rpcCallRef.current) {
            await rpcCallRef.current('quality.turn.record', params)
          } else {
            // Production path: use code-intel client
            const { getCodeIntelClient } = await import('../../../runtime/code-intel-client')
            const client = getCodeIntelClient()
            await client.call(worktreeId, 'quality.turn.record', params, {})
          }
        } catch {
          // Non-fatal: local ReviewTurnMarker was already created by SOL-060
        }
      })()
    })

    return unsubscribe
  }, [flags.quality, worktreeId, onSubscribe])
}
