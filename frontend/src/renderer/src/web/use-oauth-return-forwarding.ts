import { useEffect, useMemo } from 'react'
import type { AuthUser } from '../auth/auth-types'
import { isOAuthConsentPath, stashReturnToFromLocation, takeForwardTarget } from './oauth-return-to'

/**
 * Why: SameSite=Strict drops the session cookie on the first cross-site hop, so after sign-in the
 * SPA itself forwards to the OAuth return_to. Returns the target while a redirect is in flight.
 */
export function useOAuthReturnForwarding(sessionUser: AuthUser | null): string | null {
  const forwardTo = useMemo(
    () =>
      sessionUser !== null && !isOAuthConsentPath(window.location)
        ? takeForwardTarget(window.location)
        : null,
    [sessionUser]
  )
  useEffect(() => {
    if (sessionUser === null) {
      stashReturnToFromLocation(window.location)
    } else if (forwardTo) {
      window.location.replace(forwardTo)
    }
  }, [sessionUser, forwardTo])
  return forwardTo
}
