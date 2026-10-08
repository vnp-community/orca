/**
 * requirement-evidence-actions.ts — FE-CV-TASK-092-03
 *
 * Writes for the requirements lens: confirm/reject one piece of evidence, link or unlink
 * the worktree's task. Non-optimistic: the UI only changes with the trace the backend returns.
 *
 * @module components/review-map/requirements/requirement-evidence-actions
 */

import { CODE_INTEL_RPC_METHODS } from '../../../../../shared/code-intel-rpc-methods'
import { parseRequirementTrace } from './requirement-trace-view-model'
import type { RequirementTrace } from './requirement-trace-view-model'

export type TraceActionClient = {
  call: (
    method: string,
    params: Record<string, unknown>
  ) => Promise<{ ok: true; result: unknown } | { ok: false; error: { kind: string; message?: string } }>
}

export type TraceActionResult =
  | { ok: true; trace: RequirementTrace }
  | { ok: false; kind: string; busy?: boolean }

export type EvidenceRef = { kind: string; ref: string }

export type RequirementEvidenceActions = {
  confirm: (requirementKey: string, evidence: EvidenceRef) => Promise<TraceActionResult>
  reject: (requirementKey: string, evidence: EvidenceRef) => Promise<TraceActionResult>
  linkTask: (taskId: string) => Promise<TraceActionResult>
  /** Contract: an empty taskId removes the link. */
  unlinkTask: () => Promise<TraceActionResult>
}

export function createRequirementEvidenceActions(
  ctx: { projectId: string; worktreeId: string },
  client: TraceActionClient
): RequirementEvidenceActions {
  // Why: a double click must not send the same write twice (SSH latency makes it likely).
  const inFlight = new Set<string>()

  async function run(lockKey: string, method: string, params: Record<string, unknown>): Promise<TraceActionResult> {
    if (inFlight.has(lockKey)) {
      return { ok: false, kind: 'busy', busy: true }
    }
    inFlight.add(lockKey)
    try {
      const response = await client.call(method, { projectId: ctx.projectId, worktreeId: ctx.worktreeId, ...params })
      if (!response.ok) {
        return { ok: false, kind: response.error.kind }
      }
      const trace = (response.result as { trace?: unknown } | null)?.trace
      if (!trace) {
        return { ok: false, kind: 'invalid_response' }
      }
      return { ok: true, trace: parseRequirementTrace(trace) }
    } catch {
      return { ok: false, kind: 'unknown' }
    } finally {
      inFlight.delete(lockKey)
    }
  }

  const decide =
    (linkKind: 'confirm' | 'reject') =>
    (requirementKey: string, evidence: EvidenceRef) =>
      run(`${linkKind}:${requirementKey}:${evidence.ref}`, CODE_INTEL_RPC_METHODS.QUALITY_TRACE_CONFIRM, {
        requirementKey,
        evidenceKind: evidence.kind,
        evidenceRef: evidence.ref,
        linkKind,
        // Contract leaves `scope` values open; the backend default for a worktree is 'worktree'.
        scope: 'worktree'
      })

  return {
    confirm: decide('confirm'),
    reject: decide('reject'),
    linkTask: (taskId) => run('link', CODE_INTEL_RPC_METHODS.QUALITY_TRACE_LINK, { taskId }),
    unlinkTask: () => run('link', CODE_INTEL_RPC_METHODS.QUALITY_TRACE_LINK, { taskId: '' })
  }
}
