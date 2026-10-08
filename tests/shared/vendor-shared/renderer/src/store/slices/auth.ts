// AuthSlice — user identity and authentication state (CR-006, TASK-006-B, CR-LOGIN-001)
import type { StateCreator } from 'zustand'
import type { AppState } from '../types'

// ─── Types ────────────────────────────────────────────────────────────────────

export type OrcaUserRole = 'developer' | 'lead' | 'admin'

export type OrcaUser = {
  id: string
  email: string
  name: string
  avatarUrl?: string
  teams: string[]
  projects: string[]
  role: OrcaUserRole
}

export type AuthStatus =
  | 'unknown'         // bootstrap has not yet called GET /auth/me
  | 'unauthenticated'
  | 'authenticating'
  | 'authenticated'
  | 'error'

export type AuthSlice = {
  currentUser: OrcaUser | null
  authStatus: AuthStatus
  authError: string | null

  setCurrentUser: (user: OrcaUser | null) => void
  setAuthStatus: (status: AuthStatus, error?: string) => void
  clearAuth: () => void
  /** Check the current session by calling GET /auth/me and update store. */
  checkSession: () => Promise<void>
}

// ─── Slice Factory ────────────────────────────────────────────────────────────

export const createAuthSlice: StateCreator<AppState, [], [], AuthSlice> = (set) => ({
  currentUser: null,
  authStatus: 'unknown',   // unknown until bootstrap resolves
  authError: null,

  setCurrentUser: (user) =>
    set(() => ({ currentUser: user })),

  setAuthStatus: (status, error) =>
    set(() => ({ authStatus: status, authError: error ?? null })),

  clearAuth: () =>
    set(() => ({
      currentUser: null,
      authStatus: 'unauthenticated',
      authError: null
    })),

  checkSession: async () => {
    // Why lazy import: avoids circular dependency between store ↔ auth-api-client
    const { fetchCurrentUser } = await import('../../auth/auth-api-client')
    set(() => ({ authStatus: 'authenticating', authError: null }))
    try {
      const user = await fetchCurrentUser()
      if (user) {
        // Map AuthUser (HTTP layer) → OrcaUser (store layer)
        set(() => ({
          authStatus: 'authenticated',
          authError: null,
          currentUser: {
            id: user.id,
            email: user.email,
            name: user.name,
            avatarUrl: user.avatarUrl,
            role: user.role,
            teams: [],
            projects: []
          }
        }))
      } else {
        set(() => ({ authStatus: 'unauthenticated', authError: null, currentUser: null }))
      }
    } catch (err) {
      set(() => ({
        authStatus: 'error',
        authError: (err as Error).message,
        currentUser: null
      }))
    }
  }
})
