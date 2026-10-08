/**
 * use-agent-turn-backend-recorder.ts — FE-CV-TASK-089-04
 *
 * Samples agent tool use and, when a turn finishes, records metadata to the backend
 * via `quality.turn.record`. Quality flag off: no store subscription, no RPC.
 * Errors never reach the UI or block the local review marker (SOL-060).
 *
 * @module components/review-map/turns/use-agent-turn-backend-recorder
 */

import { useEffect } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForWorktree } from '@/lib/worktree-runtime-owner'
import { findWorktreeById } from '../../../store/slices/worktree-helpers'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'
import { getCodeIntelClient, classifyCodeIntelError } from '../../../runtime/code-intel-client'
import { CODE_INTEL_RPC_METHODS } from '../../../../../shared/code-intel-rpc-methods'
import { createAgentTurnToolCollector } from './agent-tool-use-command-summarizer'
import { detectAgentTurnCompletions } from './agent-turn-completion-detector'
import { createAgentTurnRecordQueue } from './agent-turn-record-queue'
import { buildReviewTurnMarker } from './review-turn-marker-builder'
import { buildAgentTurnRecordParams } from './agent-turn-record-params'
import type { AgentTurnRecordParams } from './agent-turn-record-params'

// Let git status settle after the agent's final write before reading HEAD / dirty state.
const GIT_SETTLE_MS = 3000

type RecordRpcError = Error & { kind: string }

async function sendTurnRecord(params: AgentTurnRecordParams): Promise<void> {
  const state = useAppStore.getState()
  const response = await getCodeIntelClient().call(
    params.worktreeId,
    CODE_INTEL_RPC_METHODS.QUALITY_TURN_RECORD,
    params,
    { environmentId: getRuntimeEnvironmentIdForWorktree(state, params.worktreeId) }
  )
  if (!response.ok) {
    throw Object.assign(new Error(response.error.message), { kind: response.error.kind }) as RecordRpcError
  }
}

function classifyRecordError(error: unknown): { kind: string } {
  const kind = (error as { kind?: unknown } | null)?.kind
  return typeof kind === 'string' ? { kind } : { kind: classifyCodeIntelError(error).kind }
}

/** Mount once at App level (use-app-agent-turn-recorders). No-op while the quality flag is off. */
export function useAgentTurnBackendRecorder(opts?: {
  /** Tenant setting `agentTurnStorePromptExcerpt`; excerpts additionally need a masker, which does not exist yet. */
  storePromptExcerpt?: boolean
}): void {
  const { quality } = useQualityFeatureFlags()
  const storePromptExcerpt = opts?.storePromptExcerpt ?? false

  useEffect(() => {
    if (!quality) {
      return
    }
    const collector = createAgentTurnToolCollector()
    const queue = createAgentTurnRecordQueue(sendTurnRecord, classifyRecordError)
    const timers = new Set<ReturnType<typeof setTimeout>>()

    const unsubscribe = useAppStore.subscribe((state, prevState) => {
      for (const [paneKey, entry] of Object.entries(state.agentStatusByPaneKey)) {
        collector.observe(paneKey, entry)
      }
      for (const { paneKey, entry } of detectAgentTurnCompletions(
        prevState.agentStatusByPaneKey,
        state.agentStatusByPaneKey
      )) {
        const commands = collector.take(paneKey)
        const timer = setTimeout(() => {
          timers.delete(timer)
          const current = useAppStore.getState()
          const worktreeId = entry.worktreeId ?? ''
          const summary = current.gitBranchCompareSummaryByWorktree[worktreeId]
          const status = current.gitStatusByWorktree[worktreeId] ?? []
          // Why: same per-file identity the local ReviewTurnMarker stores, so both records agree.
          const marker = buildReviewTurnMarker({
            worktreeId,
            paneKey,
            entry,
            headOid: summary?.headOid ?? null,
            baseOid: summary?.baseOid ?? null,
            mergeBase: summary?.mergeBase ?? null,
            statusEntries: status,
            symbolKeys: null
          })
          const params = buildAgentTurnRecordParams({
            projectId: findWorktreeById(current.worktreesByRepo, worktreeId)?.projectId,
            worktreeId,
            entry: {
              paneKey,
              agentType: entry.agentType,
              prompt: entry.prompt,
              doneAt: entry.stateStartedAt,
              stateHistory: entry.stateHistory,
              interrupted: entry.interrupted
            },
            headOid: summary?.headOid,
            treeDirty: status.length > 0,
            fileIdentities: marker.files.map((f) => `${f.p}|${f.h}`),
            commands,
            storePromptExcerpt
          })
          if (params) {
            queue.enqueue(params)
          }
        }, GIT_SETTLE_MS)
        timers.add(timer)
      }
    })

    return () => {
      unsubscribe()
      queue.dispose()
      for (const timer of timers) {
        clearTimeout(timer)
      }
    }
  }, [quality, storePromptExcerpt])
}
