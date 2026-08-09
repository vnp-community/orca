// Registry of git providers for remote-connection-backed repos, keyed by
// connectionId (either a real SSH target id or a devServerId — both resolve
// through the same generic key, see getRepoProviderConnectionKey). Typed
// against the IGitProvider interface, not any concrete provider class: this
// map holds both SshGitProvider and DevServerGitProvider instances (see
// dev-server-provider-lifecycle.ts) interchangeably.
import type { IGitProvider } from './types'

const gitConnectionProviders = new Map<string, IGitProvider>()

export const REMOTE_GIT_PROVIDER_UNAVAILABLE_MESSAGE =
  'Remote connection dropped. Reconnect the host before retrying.'

export function registerRemoteGitProvider(connectionId: string, provider: IGitProvider): void {
  gitConnectionProviders.set(connectionId, provider)
}

export function unregisterRemoteGitProvider(connectionId: string): void {
  gitConnectionProviders.delete(connectionId)
}

export function getRemoteGitProvider(connectionId: string): IGitProvider | undefined {
  return gitConnectionProviders.get(connectionId)
}

export function requireRemoteGitProvider(connectionId: string): IGitProvider {
  const provider = getRemoteGitProvider(connectionId)
  if (!provider) {
    throw new Error(REMOTE_GIT_PROVIDER_UNAVAILABLE_MESSAGE)
  }
  return provider
}
