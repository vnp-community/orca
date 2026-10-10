/**
 * useRequestPostAnswerRefresh — UI-STALE-SOLUTION (TASK-FE-E2E-009 item 6h)
 *
 * After a Clarification answer the backend moves the Request on (analyzing, then the next
 * approval) with no guaranteed event. Re-read the Request on a short, bounded timer until it
 * settles, so the screen does not keep showing the state from before the answer.
 *
 * @module hooks/useRequestPostAnswerRefresh
 */

import { useCallback, useEffect, useState } from 'react'
import type { RequestStatus } from '../../../shared/request-types'

export const POST_ANSWER_REFRESH_INTERVAL_MS = 3000
export const POST_ANSWER_REFRESH_MAX_TRIES = 20

const IN_PROGRESS: ReadonlySet<RequestStatus | undefined> = new Set<RequestStatus | undefined>([
  'awaiting_information',
  'analyzing',
  'planning'
])

/** Returns the callback to arm the refresh (call it once an answer was accepted). */
export function useRequestPostAnswerRefresh(
  status: RequestStatus | undefined,
  refetch: () => void
): () => void {
  const [armed, setArmed] = useState(false)
  const arm = useCallback(() => setArmed(true), [])

  // Why: stop as soon as the Request reached a state that waits for a person again.
  const settled = armed && !IN_PROGRESS.has(status)
  useEffect(() => {
    if (settled) {
      setArmed(false)
    }
  }, [settled])

  useEffect(() => {
    if (!armed) {
      return undefined
    }
    let tries = 0
    const t = setInterval(() => {
      tries += 1
      if (tries > POST_ANSWER_REFRESH_MAX_TRIES) {
        clearInterval(t)
        setArmed(false)
        return
      }
      refetch()
    }, POST_ANSWER_REFRESH_INTERVAL_MS)
    return () => clearInterval(t)
  }, [armed, refetch])

  return arm
}
