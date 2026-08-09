import { WebCredentialStore } from './web-credential-store'

let _store: WebCredentialStore | null = null

/**
 * Initialize the singleton WebCredentialStore for Web Server mode.
 * Must be called once during server bootstrap when ORCA_MULTI_USER=1.
 *
 * @param userDataPath - Base data path for the server (e.g. /data/orca)
 * @param userId       - The authenticated Orca user ID for this process
 * @param serverSecret - Encryption master key from ORCA_SERVER_SECRET env var
 */
export function initWebCredentialStore(
  userDataPath: string,
  userId: string,
  serverSecret: string
): void {
  _store = new WebCredentialStore(userDataPath, userId, serverSecret)
}

/**
 * Return the initialized singleton WebCredentialStore.
 * @throws Error if initWebCredentialStore() has not been called yet.
 */
export function getWebCredentialStore(): WebCredentialStore {
  if (!_store) {
    throw new Error(
      '[WebCredentialStore] Not initialized. Call initWebCredentialStore() first.'
    )
  }
  return _store
}

/**
 * Returns true when the server is running in Web/multi-user mode
 * and should use WebCredentialStore instead of Electron safeStorage.
 */
export function isWebCredentialMode(): boolean {
  return process.env['ORCA_MULTI_USER'] === '1'
}

export { WebCredentialStore } from './web-credential-store'
export type { CredentialService, CredentialConfig } from './web-credential-store'
