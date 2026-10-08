/**
 * review-data-api-default.ts
 *
 * Default ReviewDataApi over the SOL-050 code-intel client. Store/client are imported lazily:
 * the store index imports the review-progress slice, which imports this module, so a static
 * import would be circular.
 */

import {
  classifyReviewError,
  normalizeChangeOverlay,
  normalizeIndexStatus,
  normalizeReviewState,
  type ReviewApiResult,
  type ReviewDataApi
} from './review-shell-data'

function rec(v: unknown): Record<string, unknown> {
  return typeof v === 'object' && v !== null ? (v as Record<string, unknown>) : {}
}

async function callWorktree(
  worktreeId: string,
  method: string,
  extra: Record<string, unknown>,
  signal?: AbortSignal
): Promise<ReviewApiResult<unknown>> {
  const [{ useAppStore }, { getCodeIntelClient }, { resolveCodeIntelSelector }] = await Promise.all(
    [
      import('@/store'),
      import('../../runtime/code-intel-client'),
      import('../../lib/code-intel-worktree-selector')
    ]
  )
  const sel = resolveCodeIntelSelector(useAppStore.getState(), worktreeId)
  if (sel.state !== 'ready') {
    return { ok: false, error: { kind: 'no-binding', message: sel.reason } }
  }
  try {
    const res = await getCodeIntelClient().call(
      sel.worktreeId,
      method,
      { projectId: sel.projectId, worktreeId: sel.worktreeId, ...extra },
      { environmentId: sel.environmentId, signal }
    )
    return res.ok
      ? { ok: true, value: res.result }
      : { ok: false, error: classifyReviewError(res.error) }
  } catch (err) {
    return { ok: false, error: classifyReviewError(err) }
  }
}

function mapOk<T, U>(r: ReviewApiResult<T>, fn: (v: T) => U): ReviewApiResult<U> {
  return r.ok ? { ok: true, value: fn(r.value) } : r
}

export const defaultReviewDataApi: ReviewDataApi = {
  async getStatus(worktreeId, opts) {
    return mapOk(
      await callWorktree(
        worktreeId,
        'codeIntel.status',
        opts?.refresh ? { refresh: true } : {},
        opts?.signal
      ),
      normalizeIndexStatus
    )
  },
  async reindex(worktreeId, mode) {
    return mapOk(await callWorktree(worktreeId, 'codeIntel.reindex', { mode }), (v) => {
      const r = rec(v)
      return { jobId: String(r.jobId ?? ''), status: String(r.status ?? '') }
    })
  },
  async bindRepo(worktreeId) {
    return mapOk(await callWorktree(worktreeId, 'codeIntel.bindRepo', {}), (v) =>
      normalizeIndexStatus(rec(v).status)
    )
  },
  async getChangeOverlay(worktreeId, params, signal) {
    return mapOk(
      await callWorktree(
        worktreeId,
        'codeIntel.changeOverlay',
        { ...params, detail: 'full' },
        signal
      ),
      normalizeChangeOverlay
    )
  },
  async getReviewState(worktreeId, key) {
    return mapOk(await callWorktree(worktreeId, 'codeIntel.reviewState.get', key), (v) =>
      normalizeReviewState(v, key.baseCommit, key.headCommit)
    )
  },
  async saveReviewState(worktreeId, state, expectedVersion) {
    const { version: _version, updatedAt: _u, updatedBy: _b, ...rest } = state
    return mapOk(
      await callWorktree(worktreeId, 'codeIntel.reviewState.save', { ...rest, expectedVersion }),
      (v) => normalizeReviewState(v, state.baseCommit, state.headCommit)
    )
  }
}
