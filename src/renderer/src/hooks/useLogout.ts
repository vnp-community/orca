// TASK-FE-009: useLogout hook
// Encapsulates the full logout sequence: POST /auth/logout → clear store → redirect.
import { useCallback } from 'react'
import { useAppStore } from '../store'
import { logoutUser } from '../auth/auth-api-client'

/**
 * Returns an async callback that logs out the current user.
 * Sequence: POST /auth/logout → clearAuth() → window.location.href = '/login'
 *
 * The redirect ensures a full page reload so any in-memory state is cleared.
 */
export function useLogout(): () => Promise<void> {
  const clearAuth = useAppStore((s) => s.clearAuth)

  return useCallback(async () => {
    try {
      await logoutUser()
    } catch {
      // Non-fatal: the server session may already be gone. Still clear local state.
    }
    clearAuth()
    window.location.href = '/login'
  }, [clearAuth])
}
