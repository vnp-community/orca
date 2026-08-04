import { gitExecFileAsync } from '../git/runner'
import type { IGitProvider } from '../providers/types'
import { getRepoProviderConnectionKey } from '../../shared/execution-host'

type LocalGitExecOptions = {
  cwd: string
  wslDistro?: string
}

// Why: the relay's read-only git.exec channel rejects `fetch`, so SSH repos
// must use the dedicated git.fetchRemoteTrackingRef RPC.
export async function fetchPrHeadTrackingRef(
  repo: { path: string; connectionId?: string | null; devServerId?: string | null },
  sshGitProvider: IGitProvider | null | undefined,
  remote: string,
  branch: string,
  options: { localGitExecOptions?: LocalGitExecOptions } = {}
): Promise<void> {
  const ref = `refs/remotes/${remote}/${branch}`
  if (!getRepoProviderConnectionKey(repo)) {
    await gitExecFileAsync(
      ['fetch', remote, `+refs/heads/${branch}:${ref}`],
      options.localGitExecOptions ?? { cwd: repo.path }
    )
    return
  }
  if (!sshGitProvider) {
    throw new Error('SSH Git provider is not available. Reconnect to this target and try again.')
  }
  await sshGitProvider.fetchRemoteTrackingRef!(repo.path, remote, branch, ref)
}
