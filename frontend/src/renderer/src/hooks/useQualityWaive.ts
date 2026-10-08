/**
 * useQualityWaive.ts — FE-CV-TASK-087-14
 *
 * Waive / revoke one check finding through `quality.waive`. The row is patched optimistically and
 * restored when the call fails; on success the quality cache is invalidated and the gate is
 * reloaded (the client never recomputes the gate). `scope` is not sent (backend default).
 *
 * @module hooks/useQualityWaive
 */

import { useCallback, useRef, useState } from 'react'
import { toast } from 'sonner'
import { useAppStore } from '@/store'
import { getCodeIntelClient } from '../runtime/code-intel-client'
import type { CodeIntelRpcError } from '../runtime/code-intel-client'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import { parseWaiverExpiryData } from '../../../shared/code-intel-quality-errors'
import type { QualityFinding, QualityFindingWaiver } from '../../../shared/code-intel-quality-types'
import { qf } from '../components/review-map/quality/findings/quality-findings-copy'

export const WAIVE_REASON_MAX_LENGTH = 1000

export type QualityWaiveOutcome = { ok: true } | { ok: false; message: string }

export type UseQualityWaiveResult = {
  busyFingerprint: string | null
  waive: (
    finding: QualityFinding,
    input: { reason: string; expiresAt: string }
  ) => Promise<QualityWaiveOutcome>
  revoke: (finding: QualityFinding) => Promise<QualityWaiveOutcome>
}

export function describeQualityWaiveError(error: CodeIntelRpcError): string {
  switch (error.kind) {
    case 'forbidden':
      return qf('waiveErrForbidden')
    case 'offline':
      return qf('waiveErrOffline')
    case 'conflict':
      return qf('waiveErrConflict')
    case 'validation': {
      const expiry = parseWaiverExpiryData(error.data)
      return expiry ? qf('waiveErrExpiry', { maxDays: expiry.maxDays }) : qf('waiveErrValidation')
    }
    default:
      return qf('waiveErrUnknown')
  }
}

export function useQualityWaive(
  worktreeId: string | null | undefined,
  patchWaiver: (fingerprint: string, waiver: QualityFindingWaiver | undefined) => () => void
): UseQualityWaiveResult {
  const [busyFingerprint, setBusyFingerprint] = useState<string | null>(null)
  const inFlight = useRef(new Set<string>())

  const run = useCallback(
    async (
      finding: QualityFinding,
      params: { action: 'waive' | 'revoke'; reason: string; expiresAt: string },
      optimistic: QualityFindingWaiver | undefined,
      doneMessage: string
    ): Promise<QualityWaiveOutcome> => {
      const key = finding.fingerprint
      if (!worktreeId || inFlight.current.has(key)) {
        return { ok: false, message: '' }
      }
      // Why: lock synchronously so a double click or double Mod+Enter sends one request (SSH latency).
      inFlight.current.add(key)
      setBusyFingerprint(key)
      const restore = patchWaiver(key, optimistic)
      const outcome = await getCodeIntelClient()
        .call(
          worktreeId,
          CODE_INTEL_RPC_METHODS.QUALITY_WAIVE,
          { subjectKind: 'finding', subjectKey: key, ...params },
          {}
        )
        .catch((): null => null)
      inFlight.current.delete(key)
      setBusyFingerprint(null)
      if (!outcome || !outcome.ok) {
        restore()
        return {
          ok: false,
          message: outcome ? describeQualityWaiveError(outcome.error) : qf('waiveErrUnknown')
        }
      }
      const store = useAppStore.getState()
      store.invalidateQuality(worktreeId)
      void store.loadQualityGate(worktreeId, { force: true })
      toast.success(doneMessage)
      return { ok: true }
    },
    [worktreeId, patchWaiver]
  )

  const waive = useCallback<UseQualityWaiveResult['waive']>(
    (finding, input) => {
      const reason = input.reason.trim()
      if (reason === '' || reason.length > WAIVE_REASON_MAX_LENGTH || input.expiresAt === '') {
        return Promise.resolve({ ok: false, message: qf('waiveErrReasonRequired') })
      }
      return run(
        finding,
        { action: 'waive', reason, expiresAt: input.expiresAt },
        { by: '', reason, expiresAt: input.expiresAt },
        qf('waiveToastWaived')
      )
    },
    [run]
  )

  const revoke = useCallback<UseQualityWaiveResult['revoke']>(
    (finding) => {
      const current = finding.waiver
      return run(
        finding,
        {
          action: 'revoke',
          reason: current?.reason || 'revoke',
          expiresAt: current?.expiresAt ?? ''
        },
        undefined,
        qf('waiveToastRevoked')
      )
    },
    [run]
  )

  return { busyFingerprint, waive, revoke }
}
