// "Initialize as Git repo" — the remedy WorktreeCreationPanel offers when a
// worktree-create attempt fails with the 'not-a-git-repo' signature (see
// workspace-create-error-format.ts): the repo's folder exists and is
// already registered with Orca, but was never `git init`'d. Runs `git
// init` (optionally attaching a remote, in the same call) against the
// repo's OWN devServerId/path, then retries the original worktree create.
import { toast } from 'sonner'
import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { retryBackgroundWorktreeCreation } from '@/lib/worktree-creation-flow'
import { translate } from '@/i18n/i18n'

export type InitRepoAsGitOptions = {
  defaultBranch?: string
  remoteUrl?: string
  remoteName?: string
}

type InitRepoResult = { path: string; defaultBranch: string; remoteAdded: boolean }

type InitRepoAsGitTargetRepo = { path: string; devServerId?: string | null }

/** Shared core: runs git init (+ optional remote add) against `repo`'s own
 *  dev-server/path. Returns an error message on failure, null on success. */
async function runInitRepoAsGit(
  settings: ReturnType<typeof useAppStore.getState>['settings'],
  repo: InitRepoAsGitTargetRepo,
  options: InitRepoAsGitOptions
): Promise<string | null> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return translate(
      'auto.lib.init.repo.as.git.noRuntimeTarget',
      'No active runtime environment to run git init on.'
    )
  }

  try {
    await callRuntimeRpc<InitRepoResult>(
      target,
      'repo.create',
      {
        devServerId: repo.devServerId ?? '',
        destPath: repo.path,
        defaultBranch: options.defaultBranch ?? '',
        remoteUrl: options.remoteUrl ?? '',
        remoteName: options.remoteName ?? ''
      },
      { timeoutMs: 30_000 }
    )
  } catch (err) {
    const message =
      err instanceof Error
        ? err.message
        : translate(
            'auto.lib.init.repo.as.git.failed',
            'Failed to initialize this folder as a Git repo.'
          )
    toast.error(message)
    return message
  }
  return null
}

/** Returns an error message on failure, or null on success (and retries the
 *  worktree creation that originally failed for creationId). */
export async function initializeRepoAsGitAndRetry(
  creationId: string,
  options: InitRepoAsGitOptions
): Promise<string | null> {
  const state = useAppStore.getState()
  const entry = state.pendingWorktreeCreations[creationId]
  if (!entry) {
    return translate(
      'auto.lib.init.repo.as.git.creationGone',
      'This workspace creation is no longer pending.'
    )
  }
  const repo = state.repos.find((candidate) => candidate.id === entry.request.repoId)
  if (!repo) {
    return translate('auto.lib.init.repo.as.git.repoNotFound', 'Could not find this repo.')
  }

  const failure = await runInitRepoAsGit(state.settings, repo, options)
  if (failure) {
    return failure
  }
  retryBackgroundWorktreeCreation(creationId)
  return null
}

/** Standalone variant for a Settings-page-triggered init — no pending
 *  worktree creation exists to retry, so the caller (RepositoryPane) is
 *  responsible for re-running its own "is this a git repo" check on
 *  success. Returns an error message on failure, or null on success. */
export async function initializeRepoAsGit(
  repoId: string,
  options: InitRepoAsGitOptions
): Promise<string | null> {
  const state = useAppStore.getState()
  const repo = state.repos.find((candidate) => candidate.id === repoId)
  if (!repo) {
    return translate('auto.lib.init.repo.as.git.repoNotFound', 'Could not find this repo.')
  }
  return runInitRepoAsGit(state.settings, repo, options)
}
