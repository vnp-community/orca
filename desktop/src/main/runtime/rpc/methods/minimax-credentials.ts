import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
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

// Why: fire-and-forget, mirrors ipc/minimax-credentials.ts -- callers get the
// persisted cookie status immediately; the rate-limit refresh runs in the
// background and only logs on failure.
function refreshAfterMiniMaxCredentialChange(runtime: RpcContext['runtime'], action: 'save' | 'clear'): void {
  let rateLimits: ReturnType<RpcContext['runtime']['getRateLimitService']>
  try {
    rateLimits = runtime.getRateLimitService()
  } catch {
    // Why: rate-limit service isn't wired on every runtime construction
    // (e.g. some tests); credential status still returns correctly without it.
    return
  }
  rateLimits.invalidateMiniMaxCredentialState()
  void rateLimits.refresh().catch((error: unknown) => {
    console.error(`[minimax] failed to trigger rate-limit refresh after ${action}:`, error)
  })
}

// Why: additive local-desktop namespace -- NOT merged into accounts.* (that
// namespace's read+switch+remove-only mobile-bridge contract must stay as-is).
// MiniMax auth is a session cookie the user pastes in, not an interactive PTY
// login, so this namespace only needs plain get/save/clear.
export const MINIMAX_CREDENTIALS_METHODS: RpcMethod[] = [
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
