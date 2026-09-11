// Serializes diffComments' disk/annotation-service writes per worktree.
// Split out of diffComments.ts to keep that file under the max-lines
// budget; this module owns only write ordering, not the CRUD actions
// themselves (those still call enqueuePersist from diffComments.ts).
import type { AppState } from '../types'
import type { DiffComment } from '../../../../shared/types'
import { getRepoIdFromWorktreeId } from './worktree-helpers'
import { getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { normalizeDiffComment, settingsForWorktreeOwner } from './diffComments'
import { persistRemote } from './diff-comments-annotation-sync'

async function persist(
  settings: AppState['settings'],
  worktreeId: string,
  diffComments: DiffComment[]
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    await window.api.worktrees.updateMeta({
      worktreeId,
      updates: { diffComments }
    })
    return
  }
  // Why (TASK-FE-ANNOTATE-001): desktop/local mode has no bridge to
  // annotation-service (no IPC handler for annotation.create/list/delete —
  // confirmed by grep across desktop/src) and no multi-user/OPA concern to
  // justify building one — only the 'environment' (web/multi-user) target
  // routes through annotation-service. worktree.set's bulk-metadata path
  // above (desktop) is unchanged; this branch replaces the old
  // worktree.set-based remote persistence with per-record CRUD against
  // annotation-service, per SOL-FE-ANNOTATE-001 §1-2.
  await persistRemote(target, worktreeId, diffComments)
}

// Why: IPC writes from `persist` are not ordered with respect to each other.
// If two mutations (e.g. rapid add then delete, or two adds) are in flight
// concurrently, their `updateMeta` resolutions can arrive out of call order,
// letting an older snapshot overwrite a newer one on disk. We serialize per
// worktree so only one write runs at a time. We also defer reading the
// snapshot until the queued work actually starts — at dequeue time we pull
// the LATEST `diffComments` from the store — which collapses a burst of N
// mutations into at most 2 in-flight writes per worktree (1 running + 1
// queued) and guarantees the last disk write reflects the newest state.
const persistQueueByWorktree: Map<string, Promise<void>> = new Map()

// Why: chain each new write onto the prior promise for this worktree so
// writes land in call order. We use `.then(..., ..)` with both handlers so a
// failing previous write doesn't break the chain — we still proceed with the
// next write. The queued work reads the latest list from the store via
// `get()` at dequeue time (not via a captured parameter) so it writes the
// most recent snapshot rather than a stale one from when it was enqueued.
// The returned promise resolves/rejects when THIS specific write commits so
// callers can preserve their optimistic-update + rollback flow.
export function enqueuePersist(worktreeId: string, get: () => AppState): Promise<void> {
  const prior = persistQueueByWorktree.get(worktreeId) ?? Promise.resolve()
  const run = async (): Promise<void> => {
    const repoId = getRepoIdFromWorktreeId(worktreeId)
    const repoList = get().worktreesByRepo[repoId]
    const target = repoList?.find((w) => w.id === worktreeId)
    const latest = (target?.diffComments ?? []).map(normalizeDiffComment)
    await persist(settingsForWorktreeOwner(get(), worktreeId), worktreeId, latest)
  }
  const next = prior.then(run, run)
  persistQueueByWorktree.set(worktreeId, next)
  // Why: once this write settles, clear the queue entry only if no later
  // write has been chained on top. Otherwise the map should keep pointing at
  // the latest tail so subsequent enqueues chain onto the real in-flight
  // tail, not a stale resolved promise. Use `then(cleanup, cleanup)` (not
  // `finally`) so a rejection on `next` is fully consumed by this branch —
  // otherwise the `.finally()` chain propagates the rejection as an
  // unhandledRejection even though the caller `await`s `next` in its own
  // try/catch.
  const cleanup = (): void => {
    if (persistQueueByWorktree.get(worktreeId) === next) {
      persistQueueByWorktree.delete(worktreeId)
    }
  }
  next.then(cleanup, cleanup)
  return next
}
