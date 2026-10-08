/**
 * code-intel-quality-slice-context.ts — FE-CV-TASK-087-02
 *
 * Plumbing shared by the quality slice action groups: the per-worktree patch helper and the
 * RPC seam (injectable so slice tests never need the real bridge).
 *
 * @module store/slices/code-intel-quality-slice-context
 */

import type { CodeIntelRpcError } from '../../runtime/code-intel-client'
import { EMPTY_QUALITY_WORKTREE_STATE } from './code-intel-quality-state-types'
import type { QualityWorktreeState } from './code-intel-quality-state-types'

export type QualityCallResult =
  | { ok: true; result: unknown }
  | { ok: false; error: CodeIntelRpcError }

export type QualityCall = (
  worktreeId: string,
  method: string,
  params: Record<string, unknown>
) => Promise<QualityCallResult>

export type QualitySliceData = {
  codeIntelQualityByWorktree: Record<string, QualityWorktreeState>
  codeIntelEventsState?: 'idle' | 'streaming' | 'polling'
}

export type QualitySliceContext = {
  set: (fn: (prev: QualitySliceData) => Partial<QualitySliceData>) => void
  get: () => QualitySliceData
  call: QualityCall
  now: () => number
  /** Latest request sequence per `worktree|resource`; older responses are dropped. */
  sequences: Map<string, number>
  timers: Map<string, ReturnType<typeof setTimeout>>
}

export const defaultQualityCall: QualityCall = async (worktreeId, method, params) => {
  // Lazy import keeps the client (and the store it reads) out of this module's load graph.
  const { getCodeIntelClient } = await import('../../runtime/code-intel-client')
  return getCodeIntelClient().call(worktreeId, method, params, {})
}

export function readWorktreeState(
  ctx: QualitySliceContext,
  worktreeId: string
): QualityWorktreeState {
  return ctx.get().codeIntelQualityByWorktree[worktreeId] ?? EMPTY_QUALITY_WORKTREE_STATE
}

export function patchWorktree(
  ctx: QualitySliceContext,
  worktreeId: string,
  patch: (current: QualityWorktreeState) => Partial<QualityWorktreeState>
): void {
  ctx.set((prev) => {
    const current = prev.codeIntelQualityByWorktree[worktreeId] ?? EMPTY_QUALITY_WORKTREE_STATE
    return {
      codeIntelQualityByWorktree: {
        ...prev.codeIntelQualityByWorktree,
        [worktreeId]: { ...current, ...patch(current) }
      }
    }
  })
}

export function nextSequence(ctx: QualitySliceContext, key: string): number {
  const next = (ctx.sequences.get(key) ?? 0) + 1
  ctx.sequences.set(key, next)
  return next
}

export function isLatestSequence(ctx: QualitySliceContext, key: string, seq: number): boolean {
  return ctx.sequences.get(key) === seq
}

/** Like patchWorktree but never resurrects an entry removed while a request was in flight. */
export function patchExistingWorktree(
  ctx: QualitySliceContext,
  worktreeId: string,
  patch: (current: QualityWorktreeState) => Partial<QualityWorktreeState>
): void {
  if (ctx.get().codeIntelQualityByWorktree[worktreeId]) {
    patchWorktree(ctx, worktreeId, patch)
  }
}
