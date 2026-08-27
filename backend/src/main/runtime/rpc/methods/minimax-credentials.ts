import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import {
  clearMiniMaxSessionCookie,
  hasMiniMaxSessionCookie,
  saveMiniMaxSessionCookie
} from '../../../minimax/minimax-cookie-store'
import { clearMiniMaxSessionCookieJar } from '../../../rate-limits/minimax-request-context'
import type { RpcContext } from '../core'

const SaveCookieParams = z.object({
  cookie: z.string()
})

function getMiniMaxCredentialsStatus(): { configured: boolean } {
  return { configured: hasMiniMaxSessionCookie() }
}

// Why: fire-and-forget, mirrors desktop's minimax-credentials.ts -- callers
// get the persisted cookie status immediately; the rate-limit refresh runs
// in the background and only logs on failure. getRateLimitService() throws
// when accountServices isn't wired (server mode never calls
// setAccountServices() today, see orca-runtime-account-services.ts) — that's
// caught below and credential status still returns correctly without it.
function refreshAfterMiniMaxCredentialChange(
  runtime: RpcContext['runtime'],
  action: 'save' | 'clear'
): void {
  let rateLimits: ReturnType<RpcContext['runtime']['getRateLimitService']>
  try {
    rateLimits = runtime.getRateLimitService()
  } catch {
    return
  }
  rateLimits.invalidateMiniMaxCredentialState()
  void rateLimits.refresh().catch((error: unknown) => {
    console.error(`[minimax] failed to trigger rate-limit refresh after ${action}:`, error)
  })
}

// Why: ports desktop/src/main/runtime/rpc/methods/minimax-credentials.ts.
// MiniMax auth is a session cookie the user pastes into Settings > Accounts
// (not an interactive PTY login and not a live-browser-captured session), so
// unlike claudeAccounts.*/codexAccounts.* this has no "which machine ran the
// login" ambiguity — it's a plain encrypted-string read/save/clear scoped to
// wherever this runtime process runs, same shape as the already-ported
// cache.getGitHub/sparsePresets.* namespaces.
export const MINIMAX_CREDENTIALS_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'minimaxCredentials.getStatus',
    params: null,
    handler: async (_params, _ctx) => getMiniMaxCredentialsStatus()
  }),
  defineMethod({
    name: 'minimaxCredentials.saveCookie',
    params: SaveCookieParams,
    handler: async (params, { runtime }) => {
      saveMiniMaxSessionCookie(params.cookie)
      refreshAfterMiniMaxCredentialChange(runtime, 'save')
      return getMiniMaxCredentialsStatus()
    }
  }),
  defineMethod({
    name: 'minimaxCredentials.clearCookie',
    params: null,
    handler: async (_params, { runtime }) => {
      clearMiniMaxSessionCookie()
      try {
        await clearMiniMaxSessionCookieJar()
      } catch (error) {
        console.error('[minimax] failed to clear session cookie jar after credential clear:', error)
      }
      refreshAfterMiniMaxCredentialChange(runtime, 'clear')
      return getMiniMaxCredentialsStatus()
    }
  })
]
