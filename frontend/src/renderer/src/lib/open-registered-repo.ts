import { useAppStore } from '@/store'
import { activateAndRevealWorktree } from '@/lib/worktree-activation'

/**
 * "Open that project" for a repo that is already registered: reveal its default checkout instead
 * of adding a second entry. Resolves by repo id, else by the first repo of the project.
 */
export async function openRegisteredRepo(target: {
  repoId?: string
  projectId?: string
}): Promise<boolean> {
  const state = useAppStore.getState()
  const repo = state.repos.find((entry) =>
    target.repoId
      ? entry.id === target.repoId
      : !!target.projectId && entry.projectId === target.projectId
  )
  if (!repo) {
    return false
  }
  await state.fetchWorktrees(repo.id)
  const worktrees = useAppStore.getState().worktreesByRepo[repo.id] ?? []
  const worktree = worktrees.find((entry) => entry.isMainWorktree) ?? worktrees[0]
  if (!worktree) {
    return false
  }
  activateAndRevealWorktree(worktree.id, { sidebarRevealBehavior: 'auto' })
  return true
}
