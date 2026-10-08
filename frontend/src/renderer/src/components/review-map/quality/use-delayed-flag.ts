/**
 * use-delayed-flag.ts — FE-CV-TASK-087-06
 *
 * True only after `active` has stayed true for `delayMs`. Used so a spinner appears after a
 * perceptible wait (longer for remote targets) while the control is already locked.
 *
 * @module components/review-map/quality/use-delayed-flag
 */

import { useEffect, useState } from 'react'

export function useDelayedFlag(active: boolean, delayMs: number): boolean {
  const [elapsed, setElapsed] = useState(false)
  useEffect(() => {
    if (!active) {
      setElapsed(false)
      return
    }
    const timer = setTimeout(() => setElapsed(true), delayMs)
    return () => clearTimeout(timer)
  }, [active, delayMs])
  return active && elapsed
}

/** Spinner delay: SSH/remote round trips are slower, so wait a little longer before spinning. */
export const SPINNER_DELAY_LOCAL_MS = 100
export const SPINNER_DELAY_REMOTE_MS = 200
