/**
 * useRequestRefreshOnReturn — FE-REQ-TASK-040-04
 *
 * Refetches an open Request when the person comes back to the window or tab. The gateway forwards no event for a change of
 * the starting branch (`orca.request.request.base_ref_changed` is not in its request.subscribe registry), so a branch edited
 * by someone else would otherwise show stale until the next status event.
 *
 * @module hooks/useRequestRefreshOnReturn
 */

import { useEffect, useRef } from 'react'

// Why: focus and visibilitychange often fire together (switching to the tab), and every refetch is two RPCs.
export const RETURN_REFRESH_MIN_GAP_MS = 5_000

export function useRequestRefreshOnReturn(refetch: () => void): void {
  const refetchRef = useRef(refetch)
  const lastRunAt = useRef(0)

  useEffect(() => {
    refetchRef.current = refetch
  }, [refetch])

  useEffect(() => {
    function refresh(): void {
      if (document.visibilityState === 'hidden') {
        return
      }
      const now = Date.now()
      if (now - lastRunAt.current < RETURN_REFRESH_MIN_GAP_MS) {
        return
      }
      lastRunAt.current = now
      refetchRef.current()
    }
    window.addEventListener('focus', refresh)
    document.addEventListener('visibilitychange', refresh)
    return () => {
      window.removeEventListener('focus', refresh)
      document.removeEventListener('visibilitychange', refresh)
    }
  }, [])
}
