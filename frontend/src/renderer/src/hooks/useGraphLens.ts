/**
 * useGraphLens — FE-REQ-TASK-032-03
 *
 * Loads one GraphPayload per lens. Client lenses are built locally via
 * `buildClient`; backend lenses call `impact.graph`.
 *
 * @module hooks/useGraphLens
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { subscribeRequestBus } from '../lib/request-event-bus'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { splitErrorCode } from '../../../shared/request-errors'
import { GRAPH_LENSES_CLIENT } from '../../../shared/graph-types'
import { parseGraphPayload } from '../../../shared/graph-wire-parsers'
import type { RequestRpcError } from '../../../shared/request-errors'
import type { GraphLens, GraphPayload, GraphSubjectType } from '../../../shared/graph-types'

export const GRAPH_REFETCH_EVENTS = [
  'impact.assessed',
  'impact.drift_detected',
  'plan.generated',
  'phase.started',
  'phase.completed'
] as const

export const GRAPH_MAX_NODES_DEFAULT = 500
export const GRAPH_MAX_NODES_CEILING = 2000
export const GRAPH_CACHE_PER_REQUEST = 3
export const GRAPH_SKELETON_DELAY_MS = 200
export const GRAPH_PENDING_RETRY_MS = 3000
export const GRAPH_PENDING_RETRY_LIMIT = 5

export type GraphLensStatus = 'idle' | 'loading' | 'ready' | 'error'

type Params = {
  request: { id: string } | null
  lens: GraphLens
  subjectType?: GraphSubjectType
  subjectId?: string
  enabled?: boolean
  maxNodes?: number
  /** Builds client-side lenses (flow, plan, execution). */
  buildClient?: () => GraphPayload
}

type Result = {
  payload: GraphPayload | null
  status: GraphLensStatus
  error: RequestRpcError | null
  showSkeleton: boolean
  refetch: () => void
}

const cache = new Map<string, Map<string, GraphPayload>>()

export function clearGraphLensCache(): void {
  cache.clear()
}

function cachePut(requestId: string, key: string, payload: GraphPayload): void {
  const bucket = cache.get(requestId) ?? new Map<string, GraphPayload>()
  bucket.delete(key)
  bucket.set(key, payload)
  while (bucket.size > GRAPH_CACHE_PER_REQUEST) {
    const oldest = bucket.keys().next().value
    if (oldest === undefined) {break}
    bucket.delete(oldest)
  }
  cache.set(requestId, bucket)
}

export function useGraphLens({
  request,
  lens,
  subjectType = 'plan',
  subjectId = '',
  enabled = true,
  maxNodes = GRAPH_MAX_NODES_DEFAULT,
  buildClient
}: Params): Result {
  const [payload, setPayload] = useState<GraphPayload | null>(null)
  const [status, setStatus] = useState<GraphLensStatus>('idle')
  const [error, setError] = useState<RequestRpcError | null>(null)
  const [showSkeleton, setShowSkeleton] = useState(false)
  const [tick, setTick] = useState(0)
  const buildRef = useRef(buildClient)
  buildRef.current = buildClient
  const pendingRetries = useRef(0)

  const requestId = request?.id ?? null
  const refetch = useCallback(() => setTick((n) => n + 1), [])

  useEffect(() => {
    if (!requestId || !enabled) {return}
    return subscribeRequestBus((event) => {
      if (event.requestId !== requestId) {return}
      if (GRAPH_REFETCH_EVENTS.some((t) => event.eventType.endsWith(t))) {
        cache.get(requestId)?.clear()
        setTick((n) => n + 1)
      }
    })
  }, [requestId, enabled])

  useEffect(() => {
    if (!requestId || !enabled) {
      setStatus('idle')
      setPayload(null)
      return
    }

    let cancelled = false
    let skeletonTimer: ReturnType<typeof setTimeout> | null = null
    let retryTimer: ReturnType<typeof setTimeout> | null = null
    const isClient = (GRAPH_LENSES_CLIENT as readonly string[]).includes(lens)

    if (isClient) {
      const built = buildRef.current?.()
      setError(null)
      setShowSkeleton(false)
      if (built) {
        setPayload(built)
        setStatus('ready')
      } else {
        setPayload(null)
        setStatus('idle')
      }
      return
    }

    const cacheKey = `${lens}|${subjectType}|${subjectId}`
    const cached = tick === 0 ? cache.get(requestId)?.get(cacheKey) : undefined
    if (cached) {
      setPayload(cached)
      setStatus('ready')
      setError(null)
      return
    }

    setStatus('loading')
    setError(null)
    setShowSkeleton(false)
    skeletonTimer = setTimeout(() => {
      if (!cancelled) {setShowSkeleton(true)}
    }, GRAPH_SKELETON_DELAY_MS)

    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.IMPACT_GRAPH, {
      requestId,
      subjectType,
      subjectId,
      lens,
      maxNodes: Math.min(Math.max(1, maxNodes), GRAPH_MAX_NODES_CEILING)
    }).then((result) => {
      if (cancelled) {return}
      if (skeletonTimer) {clearTimeout(skeletonTimer)}
      setShowSkeleton(false)

      if (result.ok) {
        pendingRetries.current = 0
        const parsed = parseGraphPayload(result.value, lens)
        cachePut(requestId, cacheKey, parsed)
        setPayload(parsed)
        setStatus('ready')
        return
      }

      const err = result.error
      setError(err)
      if (err.kind === 'unsupported') {
        setPayload(null)
        setStatus('idle')
        return
      }
      const code = splitErrorCode(err.message).code || err.code
      if (code === 'REQUEST_RISK_ASSESSMENT_PENDING' && pendingRetries.current < GRAPH_PENDING_RETRY_LIMIT) {
        pendingRetries.current += 1
        setStatus('loading')
        retryTimer = setTimeout(() => {
          if (!cancelled) {setTick((n) => n + 1)}
        }, GRAPH_PENDING_RETRY_MS)
        return
      }
      // Why: keep the previous payload on transient failures so the canvas does not blank.
      if (err.kind === 'forbidden') {setPayload(null)}
      setStatus('error')
    })

    return () => {
      cancelled = true
      if (skeletonTimer) {clearTimeout(skeletonTimer)}
      if (retryTimer) {clearTimeout(retryTimer)}
    }
  }, [requestId, lens, subjectType, subjectId, enabled, maxNodes, tick])

  return { payload, status, error, showSkeleton, refetch }
}
