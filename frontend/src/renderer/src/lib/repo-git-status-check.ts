// Proactive "is this repo's folder actually a git repository" check for
// Settings' RepositoryPane — reuses the SAME read-only worktree.detectedList
// RPC that WorktreeCreationPanel's reactive 'not-a-git-repo' detection is
// keyed off of (workspace-create-error-format.ts), but calls it directly
// (not via worktrees.ts's fetchDetectedWorktrees, which discards the raw
// error/message once it classifies "no worktrees yet") so this caller can
// see whether the failure specifically means "no .git here," as opposed to
// a benign or unrelated gap (an empty on-disk worktree list, a
// project-service-only repo git-gateway-service hasn't registered yet, a
// transient network error). `git worktree list --porcelain` is read-only —
// safe to run unprompted on mount, unlike repo.create (initializeRepoAsGit),
// which must stay strictly user-triggered.
import type { GlobalSettings, Repo } from '../../../shared/types'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { formatWorkspaceCreateError } from './workspace-create-error-format'

type DetectedWorktreeListResult = { repoId: string; worktrees: unknown[] }

/** Returns true only when the detection call itself confirms the folder
 *  isn't a git repository yet. Every other outcome (success, an unrelated
 *  known gap, a transient error) returns false — fail open, since a wrongly
 *  shown "not a git repo" banner is worse than a missed one. */
export async function checkRepoIsNotAGitRepo(
  repo: Pick<Repo, 'id' | 'projectId'>,
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined
): Promise<boolean> {
  if (!repo.projectId) {
    // Why: worktree.detectedList's Go handler binds an empty projectId into
    // a uuid-typed column and fails PROJECT_MEMBERSHIP_LOOKUP_FAILED —
    // indistinguishable from a real check without a resolved projectId, so
    // skip rather than risk a false positive.
    return false
  }
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    // Electron/local mode has no equivalent proactive check today.
    return false
  }
  try {
    await callRuntimeRpc<DetectedWorktreeListResult>(
      target,
      'worktree.detectedList',
      { projectId: repo.projectId, repoId: repo.id },
      { timeoutMs: 15_000 }
    )
    return false
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    if (
      message.includes('WORKTREE_REPO_NOT_FOUND') ||
      message.includes('PROJECT_MEMBERSHIP_LOOKUP_FAILED')
    ) {
      // Known, disclosed architectural gaps unrelated to the repo's actual
      // git status (see worktrees.ts's listDetectedWorktreesForRepo) — not
      // a "missing .git" signal.
      return false
    }
    return formatWorkspaceCreateError(error).kind === 'not-a-git-repo'
  }
}
