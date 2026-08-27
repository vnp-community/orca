// Registry of gh/glab CLI providers for remote-connection-backed repos, keyed
// by connectionId — mirrors ssh-git-dispatch.ts exactly (same connectionId
// space: either a real SSH target id or a devServerId). Two separate maps
// (not one keyed by `${cli}:${connectionId}`) so the get/require helpers stay
// type-distinct per CLI, matching how git/fs/pty each get their own registry.
import type { IHostedCliProvider } from './types'

const githubCliProviders = new Map<string, IHostedCliProvider>()
const gitlabCliProviders = new Map<string, IHostedCliProvider>()

export const REMOTE_HOSTED_CLI_PROVIDER_UNAVAILABLE_MESSAGE =
  'Remote connection dropped. Reconnect the host before retrying.'

export function registerRemoteGithubCliProvider(connectionId: string, provider: IHostedCliProvider): void {
  githubCliProviders.set(connectionId, provider)
}

export function unregisterRemoteGithubCliProvider(connectionId: string): void {
  githubCliProviders.delete(connectionId)
}

export function getRemoteGithubCliProvider(connectionId: string): IHostedCliProvider | undefined {
  return githubCliProviders.get(connectionId)
}

export function registerRemoteGitlabCliProvider(connectionId: string, provider: IHostedCliProvider): void {
  gitlabCliProviders.set(connectionId, provider)
}

export function unregisterRemoteGitlabCliProvider(connectionId: string): void {
  gitlabCliProviders.delete(connectionId)
}

export function getRemoteGitlabCliProvider(connectionId: string): IHostedCliProvider | undefined {
  return gitlabCliProviders.get(connectionId)
}
