// Optimistic-update primitives shared by diffComments.ts's CRUD actions and
// diff-comments-annotation-sync.ts's hydration — split out of diffComments.ts
// to keep that file under the max-lines budget.
import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { DiffComment, Worktree } from '../../../../shared/types'
import { getRepoIdFromWorktreeId } from './worktree-helpers'
import type { DiffCommentsSlice } from './diffComments'

// Why: derive the next comment list from the latest store snapshot inside
// the `set` updater so two concurrent writes (rapid add+delete, or a
// delete-while-add-in-flight) can't clobber each other via a stale closure.
export function mutateComments(
  set: Parameters<StateCreator<AppState, [], [], DiffCommentsSlice>>[0],
  worktreeId: string,
  mutate: (existing: DiffComment[]) => DiffComment[] | null
): { previous: DiffComment[] | undefined; next: DiffComment[] } | null {
  const repoId = getRepoIdFromWorktreeId(worktreeId)
  let previous: DiffComment[] | undefined
  let next: DiffComment[] | null = null
  set((s) => {
    const repoList = s.worktreesByRepo[repoId]
    if (!repoList) {
      return {}
    }
    const target = repoList.find((w) => w.id === worktreeId)
    if (!target) {
      return {}
    }
    previous = target.diffComments
    const computed = mutate(previous ?? [])
    if (computed === null) {
      return {}
    }
    next = computed
    const nextList: Worktree[] = repoList.map((w) =>
      w.id === worktreeId ? { ...w, diffComments: computed } : w
    )
    return { worktreesByRepo: { ...s.worktreesByRepo, [repoId]: nextList } }
  })
  if (next === null) {
    return null
  }
  return { previous, next }
}

// Why: if the IPC write fails, the optimistic renderer state drifts from
// disk. Roll back so what the user sees always matches what will survive a
// reload.
//
// Identity guard: we only revert when the current diffComments array is
// strictly identical (===) to the `next` array this mutation produced. If
// another mutation has already landed (e.g. Add B succeeded while Add A was
// still in flight), it will have replaced the array with a different
// identity. In that case we must leave the newer state alone — rolling back
// to our stale `previous` would erase B along with the failed A.
export function rollback(
  set: Parameters<StateCreator<AppState, [], [], DiffCommentsSlice>>[0],
  worktreeId: string,
  previous: DiffComment[] | undefined,
  expectedCurrent: DiffComment[]
): void {
  const repoId = getRepoIdFromWorktreeId(worktreeId)
  set((s) => {
    const repoList = s.worktreesByRepo[repoId]
    if (!repoList) {
      return {}
    }
    const target = repoList.find((w) => w.id === worktreeId)
    // Why: if the worktree was removed between the optimistic mutation and
    // this rollback, there is nothing to restore. Bail out before remapping
    // `repoList` so we don't allocate a new outer-array identity and trigger
    // spurious subscriber notifications.
    if (!target) {
      return {}
    }
    // Why: only roll back if no other mutation landed since this one. If a
    // later write already replaced the comments array with a different
    // identity, our stale `previous` would erase that newer state.
    if (target.diffComments !== expectedCurrent) {
      return {}
    }
    const nextList: Worktree[] = repoList.map((w) =>
      w.id === worktreeId ? { ...w, diffComments: previous } : w
    )
    return { worktreesByRepo: { ...s.worktreesByRepo, [repoId]: nextList } }
  })
}
