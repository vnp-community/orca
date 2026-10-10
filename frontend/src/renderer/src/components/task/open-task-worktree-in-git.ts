import { useAppStore } from '../../store'
import { activateAndRevealWorktree } from '../../lib/worktree-activation'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'

export type OpenTaskWorktreeResult =
  | 'opened'
  /** The backend has no such worktree row (deleted / never existed). */
  | 'unknown'
  /** The row exists but the host no longer lists the checkout (cleaned up, host offline). */
  | 'not-on-host'

type WorktreeRow = { id: string; repoId?: string }

// Why: `task.worktreeId` is the backend worktree row; the sidebar store only holds worktrees it
// has loaded, and a Request's task sheet opens long before the user has visited that worktree.
async function loadWorktreeIntoStore(projectId: string, worktreeId: string): Promise<boolean> {
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  const { worktrees } = await callRuntimeRpc<{ worktrees: WorktreeRow[] }>(
    target,
    'worktree.list',
    { projectId }
  )
  const row = worktrees.find((w) => w.id === worktreeId)
  if (!row) {
    return false
  }
  const state = useAppStore.getState()
  if (row.repoId && !state.repos.some((repo) => repo.id === row.repoId)) {
    await state.fetchRepos()
  }
  await (row.repoId
    ? useAppStore.getState().fetchWorktrees(row.repoId)
    : useAppStore.getState().fetchAllWorktrees())
  return true
}

/**
 * Takes the user from a task's sheet to the files its agent changed: the task's worktree,
 * with Source Control open (same landing as "View" after a failed worktree delete).
 */
export async function openTaskWorktreeInGit(
  projectId: string,
  worktreeId: string
): Promise<OpenTaskWorktreeResult> {
  if (!useAppStore.getState().getKnownWorktreeById(worktreeId)) {
    if (!(await loadWorktreeIntoStore(projectId, worktreeId))) {
      return 'unknown'
    }
  }
  const state = useAppStore.getState()
  const wasInWorkspaceView = state.activeView === 'workspace'
  if (!state.getKnownWorktreeById(worktreeId) || !activateAndRevealWorktree(worktreeId)) {
    return 'not-on-host'
  }
  // Why: Project Workspace has its own Git tab and keeps the user in place; every other view
  // (Requests, Tasks) lands on the worktree with the Source Control sidebar.
  if (wasInWorkspaceView) {
    state.setActiveWorkspaceTab('git')
  } else {
    state.setRightSidebarTab('source-control')
    state.setRightSidebarOpen(true)
  }
  return 'opened'
}
