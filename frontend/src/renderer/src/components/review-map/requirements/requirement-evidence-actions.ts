/**
 * requirement-evidence-actions.ts — FE-CV-TASK-092-03
 *
 * Actions for confirming, rejecting, and linking requirement trace evidence.
 * No optimistic updates — state only changes on successful API response.
 *
 * @module components/review-map/requirements/requirement-evidence-actions
 */

import type { RequirementTrace } from './requirement-trace-view-model'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type EvidenceActionResult =
  | { ok: true; trace: RequirementTrace }
  | { ok: false; errorCode: string }

export type EvidenceActionsClient = {
  /** Call the code-intel RPC and return raw result */
  call: (
    method: string,
    params: Record<string, unknown>
  ) => Promise<{ ok: true; result: unknown } | { ok: false; error: { kind: string; code?: string | null } }>
}

export type EvidenceActions = {
  confirm: (traceId: string) => Promise<EvidenceActionResult>
  reject: (traceId: string) => Promise<EvidenceActionResult>
  linkTask: (traceId: string, taskUrl: string) => Promise<EvidenceActionResult>
  unlinkTask: (traceId: string, taskUrl: string) => Promise<EvidenceActionResult>
}

// ---------------------------------------------------------------------------
// Lock guard — prevents concurrent calls for the same traceId+action
// ---------------------------------------------------------------------------

const pendingCalls = new Set<string>()

function lockKey(action: string, traceId: string): string {
  return `${action}:${traceId}`
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

/**
 * Build evidence actions bound to a project and worktree.
 * Actions are non-optimistic: caller receives the updated trace on success.
 */
export function buildRequirementEvidenceActions(
  worktreeId: string,
  projectId: string,
  client: EvidenceActionsClient
): EvidenceActions {
  async function callAction(
    method: string,
    traceId: string,
    extraParams: Record<string, unknown> = {}
  ): Promise<EvidenceActionResult> {
    const key = lockKey(method, traceId)
    // Prevent concurrent calls for the same action+trace
    if (pendingCalls.has(key)) {
      return { ok: false, errorCode: 'already_in_progress' }
    }

    pendingCalls.add(key)
    try {
      const result = await client.call(method, {
        worktreeId,
        projectId,
        traceId,
        scope: 'worktree', // Default scope per spec assumption
        ...extraParams,
      })

      if (!result.ok) {
        const code = result.error.code ?? result.error.kind ?? 'unknown'
        return { ok: false, errorCode: code }
      }

      const r = (typeof result.result === 'object' && result.result !== null
        ? result.result
        : {}) as Record<string, unknown>

      // Backend returns { trace: RequirementTrace }
      if (!r.trace) {
        return { ok: false, errorCode: 'invalid_response' }
      }

      return { ok: true, trace: r.trace as RequirementTrace }
    } finally {
      pendingCalls.delete(key)
    }
  }

  return {
    confirm: (traceId) => callAction('quality.trace.confirm', traceId),
    reject: (traceId) => callAction('quality.trace.reject', traceId),
    linkTask: (traceId, taskUrl) =>
      callAction('quality.trace.link', traceId, { taskUrl }),
    unlinkTask: (traceId, taskUrl) =>
      callAction('quality.trace.unlink', traceId, { taskUrl }),
  }
}
