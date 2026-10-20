/**
 * usePlanBundleFile — CR-REQ-045
 *
 * One document of a Plan version, read when its tab is opened and remembered for the life of the Request on screen. The
 * whole change (up to 1 MiB) is never fetched at once: most readers open two or three of its files.
 *
 * @module hooks/usePlanBundleFile
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parsePlanBundleFile } from '../../../shared/request-plan-revision-parsers'
import { isOmittedPlaceholder } from '../components/request/plan/documents/plan-document-model'
import { backendErrorCode } from '../components/request/solution/solution-error-classification'
import { requestErrorMessageForError } from '../components/request/request-error-message'

export type PlanBundleFileState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ready'; content: string }
  | { status: 'omitted' }
  /** The version's text was pruned to bound storage; only its file index is left. */
  | { status: 'pruned' }
  | { status: 'notFound' }
  | { status: 'error'; message: string }

type Settled = Extract<PlanBundleFileState, { status: 'ready' | 'omitted' | 'pruned' }>

export type PlanBundleFileApi = PlanBundleFileState & { retry: () => void }

const LOADING: PlanBundleFileState = { status: 'loading' }
const IDLE: PlanBundleFileState = { status: 'idle' }

/**
 * `revision` empty or 0 reads the current version. `currentKey` says which current version that is (its commit or digest), so a
 * revision that rewrote the same path is read again instead of served from memory.
 */
export function usePlanBundleFile(
  requestId: string,
  path: string | null,
  revision?: number,
  currentKey = ''
): PlanBundleFileApi {
  const asked = revision && revision > 0 ? revision : 0
  const key = path ? `${requestId}|${asked || `current:${currentKey}`}|${path}` : ''
  const cache = useRef(new Map<string, Settled>())
  const [fetched, setFetched] = useState<{ key: string; state: PlanBundleFileState } | null>(null)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    cache.current = new Map()
  }, [requestId])

  useEffect(() => {
    if (!key || !path || cache.current.has(key)) {
      return
    }
    let cancelled = false
    setFetched({ key, state: LOADING })
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.PLAN_BUNDLE_FILE, {
      id: requestId,
      path,
      ...(asked ? { revision: asked } : {})
    }).then((res) => {
      if (cancelled) {
        return
      }
      if (!res.ok) {
        const code = backendErrorCode(res.error)
        if (code === 'REQUEST_PLAN_REVISION_TEXT_PRUNED') {
          cache.current.set(key, { status: 'pruned' })
          setFetched({ key, state: { status: 'pruned' } })
        } else if (
          res.error.kind === 'not_found' ||
          code === 'REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND' ||
          code === 'REQUEST_PLAN_BUNDLE_NOT_FOUND'
        ) {
          setFetched({ key, state: { status: 'notFound' } })
        } else {
          // Not remembered: the next look at this tab asks again.
          setFetched({
            key,
            state: { status: 'error', message: requestErrorMessageForError(res.error) }
          })
        }
        return
      }
      const file = parsePlanBundleFile(res.value)
      const settled: Settled =
        file.omitted || isOmittedPlaceholder(file.content)
          ? { status: 'omitted' }
          : { status: 'ready', content: file.content }
      cache.current.set(key, settled)
      setFetched({ key, state: settled })
    })
    return () => {
      cancelled = true
    }
  }, [key, path, requestId, asked, attempt])

  const retry = useCallback(() => setAttempt((n) => n + 1), [])
  if (!key) {
    return { ...IDLE, retry }
  }
  const remembered = cache.current.get(key)
  const state = remembered ?? (fetched?.key === key ? fetched.state : LOADING)
  return { ...state, retry }
}
