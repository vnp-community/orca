// TASK-FE-009: useAuthSession + useLogout hooks
// Thin wrappers around the Zustand AuthSlice so components don't directly
// import useAppStore and couple to the selector path.
import { useCallback } from 'react'
import { useAppStore } from '../store'
import type { AuthStatus, OrcaUser } from '../store/slices/auth'
import { logoutUser } from '../auth/auth-api-client'

// ─── useAuthSession ────────────────────────────────────────────────────────────

/**
 * Returns the current authentication status from the Zustand store.
 * Components can narrow on `status` to handle each state.
 */
export function useAuthStatus(): AuthStatus {
  return useAppStore((s) => s.authStatus)
}

/**
 * Returns the authenticated OrcaUser, or null when not authenticated.
 * Convenient alias that avoids checking authStatus in the component.
 */
export function useAuthUser(): OrcaUser | null {
  return useAppStore((s) => s.currentUser)
}

/**
 * Returns the full auth session shape: status, user, and error.
 * Useful when a component needs multiple fields at once.
 */
export function useAuthSession(): {
  status: AuthStatus
  user: OrcaUser | null
  error: string | null
} {
  const status = useAppStore((s) => s.authStatus)
  const user = useAppStore((s) => s.currentUser)
  const error = useAppStore((s) => s.authError)
  return { status, user, error }
}
