import { relativePathInsideRoot } from '../../../shared/cross-platform-path'

type PathedWorktree = { path: string }

export type SettledWorktreeLists<W extends PathedWorktree> = {
  worktrees: W[]
  failedCount: number
}

// Why allSettled: git/file calls resolve a worktree by path against the rows of EVERY repo the
// caller can see. One repo whose dev server is offline (or whose directory is gone) makes
// `worktree.detectedList` fail; with Promise.all that blanked the whole list, so Source Control
// of a perfectly healthy worktree showed "No runtime worktree owns <path>" and no changes.
export async function collectWorktreeListsIgnoringFailures<V, W extends PathedWorktree>(
  views: readonly V[],
  fetchWorktrees: (view: V) => Promise<readonly W[]>,
  onFailure: (view: V, error: unknown) => void
): Promise<SettledWorktreeLists<W>> {
  const settled = await Promise.allSettled(views.map((view) => fetchWorktrees(view)))
  const worktrees: W[] = []
  let failedCount = 0
  settled.forEach((result, index) => {
    if (result.status === 'fulfilled') {
      worktrees.push(...result.value)
      return
    }
    failedCount += 1
    onFailure(views[index], result.reason)
  })
  return { worktrees, failedCount }
}

// Deepest root wins so a nested checkout is not claimed by its parent repo.
export function findWorktreeOwningPath<W extends PathedWorktree>(
  worktrees: readonly W[],
  targetPath: string
): W | null {
  const matches = worktrees
    .filter((worktree) => relativePathInsideRoot(worktree.path, targetPath) !== null)
    .sort((a, b) => b.path.length - a.path.length)
  return matches[0] ?? null
}
