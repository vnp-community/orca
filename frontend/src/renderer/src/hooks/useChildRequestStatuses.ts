/**
 * useChildRequestStatuses — FE-OR-T312
 *
 * The status of the follow-up Requests the Outcome Review sent items to. A follow-up the store already holds is read from it (the
 * request events keep it current); one it does not hold is fetched once and put in the store. Nothing here starts a review.
 *
 * @module hooks/useChildRequestStatuses
 */

import { useEffect, useMemo } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseRequest } from '../../../shared/request-wire-parsers'
import { useAppStore } from '../store'
import type { RequestStatus } from '../../../shared/request-types'

const asked = new Set<string>()

/** For tests: forget which follow-ups were already fetched. */
export function resetChildRequestFetches(): void {
  asked.clear()
}

export function useChildRequestStatuses(ids: string[]): Record<string, RequestStatus | undefined> {
  const byId = useAppStore((s) => s.requestsById)
  const key = ids.join(',')
  const stable = useMemo(() => (key ? key.split(',') : []), [key])
  useEffect(() => {
    for (const id of stable) {
      if (useAppStore.getState().requestsById[id] || asked.has(id)) {
        continue
      }
      asked.add(id)
      void callRequestRpc<unknown>(REQUEST_RPC_METHODS.GET, { id }).then((res) => {
        // The entity comes wrapped as {request}; an earlier draft returned it bare (see useRequest).
        const raw = res.ok
          ? ((res.value as { request?: unknown } | null)?.request ?? res.value)
          : null
        if (raw) {
          useAppStore.getState().upsertRequests([parseRequest(raw)])
        } else {
          asked.delete(id) // try again the next time the tab opens
        }
      })
    }
  }, [stable])
  return Object.fromEntries(ids.map((id) => [id, byId[id]?.status]))
}
