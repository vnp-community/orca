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

  const target = getActiveRuntimeTarget(state.settings)
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

  retryBackgroundWorktreeCreation(creationId)
  return null
}
