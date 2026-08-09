// TASK-FE-009: useLogout hook
// Encapsulates the full logout sequence: POST /auth/logout → clear browser state → redirect.
import { useCallback } from 'react'
import { useAppStore } from '../store'
import { logoutUser } from '../auth/auth-api-client'

/**
 * Clear all cookies for the current domain (all paths).
 * Handles cookies set with path=/ as well as path-scoped ones.
 */
function clearAllCookies(): void {
  const cookies = document.cookie.split(';')
  for (const cookie of cookies) {
    const name = cookie.split('=')[0].trim()
    if (!name) {continue}
    // Expire the cookie for common path variants
    const expire = 'expires=Thu, 01 Jan 1970 00:00:00 GMT'
    document.cookie = `${name}=; ${expire}; path=/`
    document.cookie = `${name}=; ${expire}; path=/; domain=${location.hostname}`
    document.cookie = `${name}=; ${expire}; path=/; domain=.${location.hostname}`
  }
}

/**
 * Returns an async callback that fully logs out the current user.
 *
 * Sequence:
 *   1. POST /auth/logout  — invalidate server session
 *   2. localStorage.clear()  — remove all persisted app state
 *   3. sessionStorage.clear() — remove all tab-scoped state
 *   4. clearAllCookies()  — expire all cookies (including stale session tokens)
 *   5. clearAuth()        — reset Zustand auth slice
 *   6. window.location.href = '/login'  — full page reload → clean state
 *
 * Step 1 is non-fatal: stale/missing sessions are handled gracefully.
 */
export function useLogout(): () => Promise<void> {
  const clearAuth = useAppStore((s) => s.clearAuth)

  return useCallback(async () => {
    // 1. Invalidate server session (best-effort)
    try {
      await logoutUser()
    } catch {
      // Non-fatal: the server session may already be gone.
    }

    // 2–4. Purge all browser-side state
    try {
      localStorage.clear()
    } catch {
      /* sandboxed iframe: ignore */
    }
    try {
      sessionStorage.clear()
    } catch {
      /* sandboxed iframe: ignore */
    }
    clearAllCookies()

    // 5. Reset Zustand store
    clearAuth()

    // 6. Full-page redirect → ensures React re-mounts cleanly from scratch
    window.location.href = '/login'
  }, [clearAuth])
}
