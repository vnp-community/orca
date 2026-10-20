/**
 * useOutcomeReviewEnabled — FE-OR-T120
 *
 * Whether the Outcome Review is on for a project now (its choice, else the deployment default, never past the kill switch),
 * from the engine settings the gateway already serves. The tab and every `request.review.*` call wait for this: with the review
 * off there is no review traffic at all. Any failure to read it counts as off, so a project the viewer cannot read never shows
 * a tab that would only fail.
 *
 * @module hooks/useOutcomeReviewEnabled
 */

import { useEffect, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseEngineSettings } from '../../../shared/request-run-all'

/** Switching requests of one project must not read the settings each time; a changed setting shows up within this time. */
export const OUTCOME_REVIEW_FLAG_TTL_MS = 60_000

const cache = new Map<string, { enabled: boolean; at: number }>()

/** For tests: forget what was read. */
export function resetOutcomeReviewFlagCache(): void {
  cache.clear()
}

function cached(projectId: string, now: number): boolean | null {
  const hit = cache.get(projectId)
  return hit && now - hit.at < OUTCOME_REVIEW_FLAG_TTL_MS ? hit.enabled : null
}

export function useOutcomeReviewEnabled(projectId: string | null | undefined): {
  enabled: boolean
  loading: boolean
} {
  const [state, setState] = useState<{ projectId: string; enabled: boolean } | null>(null)

  useEffect(() => {
    if (!projectId) {
      return undefined
    }
    const known = cached(projectId, Date.now())
    if (known !== null) {
      setState({ projectId, enabled: known })
      return undefined
    }
    let live = true
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.ENGINE_SETTINGS, { projectId }).then((res) => {
      if (!live) {
        return
      }
      const enabled = res.ok && parseEngineSettings(res.value)?.outcomeReviewEnabled === true
      // Why: only a real answer is kept; a failed read is retried the next time the pane opens.
      if (res.ok) {
        cache.set(projectId, { enabled, at: Date.now() })
      }
      setState({ projectId, enabled })
    })
    return () => {
      live = false
    }
  }, [projectId])

  if (!projectId) {
    return { enabled: false, loading: false }
  }
  const mine = state?.projectId === projectId ? state : null
  return { enabled: mine?.enabled === true, loading: mine === null }
}
