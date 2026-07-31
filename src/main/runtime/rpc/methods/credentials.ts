import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { isWebCredentialMode, getWebCredentialStore } from '../../../credentials'
import type { CredentialService } from '../../../credentials/web-credential-store'

const ServiceEnum = z.enum(['bitbucket', 'azure-devops', 'gitea', 'linear', 'jira'])

const SetTokenParams = z.object({
  service: ServiceEnum,
  token: z.string().min(1, 'Token cannot be empty'),
  // Optional service-specific config: email, apiBaseUrl, activeSiteId, etc.
  config: z.record(z.string(), z.string()).optional()
})

const ServiceParams = z.object({
  service: ServiceEnum
})

// Safe fields to expose per service (config minus secrets)
const SAFE_CONFIG_FIELDS: Record<CredentialService, string[]> = {
  'bitbucket': ['email', 'apiBaseUrl'],
  'azure-devops': ['apiBaseUrl', 'username'],
  'gitea': ['apiBaseUrl'],
  'linear': [],
  'jira': ['activeSiteId', 'siteUrl']
}

function sanitizeConfig(
  service: CredentialService,
  config: Record<string, string> | null
): Record<string, string> | null {
  if (!config) return null
  const allowed = SAFE_CONFIG_FIELDS[service] ?? []
  return Object.fromEntries(Object.entries(config).filter(([k]) => allowed.includes(k)))
}

// Why: credential management is only meaningful in Web Server mode.
// In Electron mode the user interacts with OS keychain through safeStorage.
// These methods are no-ops / throw in Electron mode so callers can always
// call them safely without checking the mode.

export const CREDENTIAL_METHODS: RpcMethod[] = [
  /**
   * Store an integration token for the currently authenticated user.
   * The token is AES-256-GCM encrypted via WebCredentialStore.
   *
   * Takes effect on the next session spawn (child process restart);
   * the current running session must reconnect to receive updated env vars.
   */
  defineMethod({
    name: 'credentials.set',
    params: SetTokenParams,
    handler: async (params, _ctx) => {
      if (!isWebCredentialMode()) {
        throw new Error(
          'credentials.set is only available in Web Server mode (ORCA_MULTI_USER=1). ' +
          'In Electron mode, use the native integration connect UI.'
        )
      }
      const store = getWebCredentialStore()
      await store.setToken(params.service as CredentialService, params.token, params.config as Record<string, string> | undefined)
      return { success: true }
    }
  }),

  /**
   * Remove a stored integration token.
   * Does not disconnect an already-running session (reconnect required).
   */
  defineMethod({
    name: 'credentials.revoke',
    params: ServiceParams,
    handler: async (params, _ctx) => {
      if (!isWebCredentialMode()) {
        throw new Error(
          'credentials.revoke is only available in Web Server mode (ORCA_MULTI_USER=1).'
        )
      }
      const store = getWebCredentialStore()
      await store.deleteToken(params.service as CredentialService)
      return { success: true }
    }
  }),

  /**
   * Check whether a token has been stored for a given service.
   * Returns sanitized config metadata (never the token itself).
   */
  defineMethod({
    name: 'credentials.status',
    params: ServiceParams,
    handler: async (params, _ctx) => {
      if (!isWebCredentialMode()) {
        return { configured: false, mode: 'electron' }
      }
      const store = getWebCredentialStore()
      const configured = await store.hasToken(params.service as CredentialService)
      const config = configured
        ? await store.getConfig(params.service as CredentialService)
        : null
      return {
        configured,
        mode: 'web',
        // Only expose safe fields — never expose the token value itself
        config: sanitizeConfig(params.service as CredentialService, config)
      }
    }
  }),

  /**
   * List all services that have stored credentials.
   */
  defineMethod({
    name: 'credentials.list',
    params: null,
    handler: async (_params, _ctx) => {
      if (!isWebCredentialMode()) {
        return { services: [], mode: 'electron' }
      }
      const store = getWebCredentialStore()
      const services = await store.listServices()
      return { services, mode: 'web' }
    }
  })
]
