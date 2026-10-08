/**
 * useMinuteClock — CR-REQ-022-03
 *
 * One 60 s timer per screen (not per row) so relative times in a long list
 * stay fresh without a timer per row. Re-syncs when the tab becomes visible.
 *
 * @module hooks/useMinuteClock
 */

import { useEffect, useState } from 'react'

export const MINUTE_CLOCK_MS = 60_000

export function useMinuteClock(enabled = true): number {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!enabled) {return}
    setNow(Date.now())
    const timer = setInterval(() => setNow(Date.now()), MINUTE_CLOCK_MS)
    const onVisible = (): void => {
      if (document.visibilityState === 'visible') {setNow(Date.now())}
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [enabled])

  return now
}
