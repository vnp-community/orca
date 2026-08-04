// Registry of filesystem providers for remote-connection-backed repos, keyed
// by connectionId (either a real SSH target id or a devServerId — both
// resolve through the same generic key, see getRepoProviderConnectionKey).
// Holds both SshFilesystemProvider and DevServerFilesystemProvider instances
// (see dev-server-provider-lifecycle.ts) interchangeably via IFilesystemProvider.
import type { IFilesystemProvider } from './types'

const filesystemConnectionProviders = new Map<string, IFilesystemProvider>()

export const REMOTE_FILESYSTEM_PROVIDER_UNAVAILABLE_MESSAGE =
  'Remote connection dropped. Reconnect the host before retrying.'

export function registerRemoteFilesystemProvider(
  connectionId: string,
  provider: IFilesystemProvider
): void {
  filesystemConnectionProviders.set(connectionId, provider)
}

export function unregisterRemoteFilesystemProvider(connectionId: string): void {
  filesystemConnectionProviders.delete(connectionId)
}

export function getRemoteFilesystemProvider(connectionId: string): IFilesystemProvider | undefined {
  return filesystemConnectionProviders.get(connectionId)
}

export function requireRemoteFilesystemProvider(connectionId: string): IFilesystemProvider {
  const provider = getRemoteFilesystemProvider(connectionId)
  if (!provider) {
    throw new Error(REMOTE_FILESYSTEM_PROVIDER_UNAVAILABLE_MESSAGE)
  }
  return provider
}
