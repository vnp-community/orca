/**
 * use-agent-turn-verification.ts — FE-CV-TASK-089-06
 *
 * Reads recorded agent turns (`quality.turns`) and builds the "agent ran / re-run found" view
 * model per turn, keyed by clientTurnId (== ReviewTurnMarker.turnId). Quality flag off or no
 * markers: no RPC. Failures are silent; the line simply does not render.
 *
 * @module components/review-map/turns/use-agent-turn-verification
 */

import { useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForWorktree } from '@/lib/worktree-runtime-owner'
import { getCodeIntelClient } from '../../../runtime/code-intel-client'
import { CODE_INTEL_RPC_METHODS } from '../../../../../shared/code-intel-rpc-methods'
import { buildAgentTurnVerificationViewModel } from './agent-turn-verification-view-model'
import type {
  AgentTurnVerificationInput,
  AgentTurnVerificationViewModel
} from './agent-turn-verification-view-model'

const TURNS_LIMIT = 20

type WireTurn = { clientTurnId?: unknown } & AgentTurnVerificationInput

export function useAgentTurnVerification(opts: {
  worktreeId: string
  enabled: boolean
  /** Changes when a new marker is recorded, so the list is read again. */
  refreshKey: number
}): Readonly<Record<string, AgentTurnVerificationViewModel>> {
  const { worktreeId, enabled, refreshKey } = opts
  const [byTurn, setByTurn] = useState<Record<string, AgentTurnVerificationViewModel>>({})

  useEffect(() => {
    if (!enabled) {
      setByTurn({})
      return
    }
    const ctrl = new AbortController()
    void getCodeIntelClient()
      .call(
        worktreeId,
        CODE_INTEL_RPC_METHODS.QUALITY_TURNS,
        { limit: TURNS_LIMIT },
        {
          environmentId: getRuntimeEnvironmentIdForWorktree(useAppStore.getState(), worktreeId),
          signal: ctrl.signal
        }
      )
      .then((res) => {
        if (ctrl.signal.aborted || !res.ok) {
          return
        }
        const turns = (res.result as { turns?: unknown } | null)?.turns
        const next: Record<string, AgentTurnVerificationViewModel> = {}
        for (const turn of Array.isArray(turns) ? (turns as WireTurn[]) : []) {
          if (typeof turn?.clientTurnId === 'string') {
            next[turn.clientTurnId] = buildAgentTurnVerificationViewModel(turn)
          }
        }
        setByTurn(next)
      })
      .catch(() => undefined)
    return () => ctrl.abort()
  }, [worktreeId, enabled, refreshKey])

  return byTurn
}
