import { deriveGitRemoteIdentity, type GitRemoteIdentity } from '../shared/git-remote-identity'
import { gitExecFileAsync } from './git/runner'
import { getRemoteGitProvider } from './providers/ssh-git-dispatch'

export async function detectGitRemoteIdentity(
  repoPath: string,
  connectionId?: string | null
): Promise<GitRemoteIdentity | null> {
  try {
    const result = connectionId
      ? await getRemoteGitProvider(connectionId)?.exec(['remote', '-v'], repoPath)
      : await gitExecFileAsync(['remote', '-v'], { cwd: repoPath })
    return result ? deriveGitRemoteIdentity(result.stdout) : null
  } catch {
    // Repo creation must not fail because a best-effort remote probe failed.
    return null
  }
}
